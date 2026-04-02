package service

import (
	"core/config"
	"core/domain"
	"core/models"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	stripe "github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
	stripecustomer "github.com/stripe/stripe-go/v76/customer"
	stripesubscription "github.com/stripe/stripe-go/v76/subscription"
	"github.com/stripe/stripe-go/v76/webhook"
)

type SubscriptionService struct {
	SubscriptionDomain domain.SubscriptionDomain
	CreditOptionDomain domain.CreditOptionDomain
	UserDomain         domain.UserDomain
	UserCreditDomain   domain.UserCreditDomain
}

func (s *SubscriptionService) initStripe() {
	cfg := config.GetConfig()
	stripe.Key = cfg.StripeSecretKey
}

// CreateSubscription creates a Stripe Checkout Session in subscription mode.
// Price is fetched from the credit option and GST (18%) is added on top.
func (s *SubscriptionService) CreateSubscription(userID, creditOptionID int64) (models.CreateSubscriptionResp, error) {
	s.initStripe()
	cfg := config.GetConfig()

	// 0. Block if user already has an active subscription
	existing, err := s.SubscriptionDomain.GetActiveByUserID(userID)
	if err == nil && existing.ID != 0 {
		return models.CreateSubscriptionResp{}, errors.New("you already have an active subscription")
	}

	// 1. Fetch credit option
	option, err := s.CreditOptionDomain.GetByID(creditOptionID)
	if err != nil {
		return models.CreateSubscriptionResp{}, errors.New("credit option not found")
	}
	if option.StripePriceID == "" {
		return models.CreateSubscriptionResp{}, errors.New("credit option has no stripe price configured")
	}

	// 2. Calculate GST for our own records (Stripe charges the saved price of $10)
	var price models.IntLocale
	if err := json.Unmarshal([]byte(option.Price), &price); err != nil {
		return models.CreateSubscriptionResp{}, errors.New("invalid credit option price data")
	}
	baseAmount := price.En // USD cents
	gstAmount := baseAmount * models.GSTRate / 100
	totalAmount := baseAmount + gstAmount

	// 3. Fetch user
	user, err := s.UserDomain.Get(models.GetUserParam{ID: userID})
	if err != nil {
		return models.CreateSubscriptionResp{}, errors.New("user not found")
	}

	// 4. Get or create Stripe customer
	stripeCustomerID, err := s.getOrCreateStripeCustomer(user.Email, user.Name)
	if err != nil {
		return models.CreateSubscriptionResp{}, err
	}

	// 5. Create Stripe Checkout Session using saved Price ID (enables MRR tracking)
	successURL := cfg.FrontendUrl + "/subscription/success?session_id={CHECKOUT_SESSION_ID}"
	cancelURL := cfg.FrontendUrl + "/subscription/cancel"

	sessionParams := &stripe.CheckoutSessionParams{
		Customer: stripe.String(stripeCustomerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(option.StripePriceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
		Metadata: map[string]string{
			"user_id":          strconv.FormatInt(userID, 10),
			"credit_option_id": strconv.FormatInt(creditOptionID, 10),
		},
	}

	sess, err := session.New(sessionParams)
	if err != nil {
		return models.CreateSubscriptionResp{}, err
	}

	// 7. Persist a pending subscription record
	sub := models.Subscription{
		UserID:           userID,
		CreditOptionID:   creditOptionID,
		StripeCustomerID: stripeCustomerID,
		StripeSessionID:  sess.ID,
		Status:           models.SubscriptionStatusPending,
		Currency:         "usd",
		BaseAmount:       baseAmount,
		GSTAmount:        gstAmount,
		TotalAmount:      totalAmount,
		Credits:          option.Credits,
	}
	if _, err := s.SubscriptionDomain.Create(sub); err != nil {
		return models.CreateSubscriptionResp{}, err
	}

	return models.CreateSubscriptionResp{
		SessionURL: sess.URL,
		SessionID:  sess.ID,
	}, nil
}

// VerifySub retrieves the Stripe session and updates the local subscription record.
func (s *SubscriptionService) VerifySub(sessionID string) (models.SubscriptionResp, error) {
	s.initStripe()

	// Expand subscription to get period end
	params := &stripe.CheckoutSessionParams{}
	params.AddExpand("subscription")
	sess, err := session.Get(sessionID, params)
	if err != nil {
		return models.SubscriptionResp{}, err
	}

	sub, err := s.SubscriptionDomain.GetByStripeSessionID(sessionID)
	if err != nil {
		return models.SubscriptionResp{}, errors.New("subscription record not found")
	}

	newStatus := models.SubscriptionStatusPending
	var stripeSubID string
	var periodEnd time.Time

	if sess.PaymentStatus == stripe.CheckoutSessionPaymentStatusPaid && sess.Subscription != nil {
		newStatus = models.SubscriptionStatusActive
		stripeSubID = sess.Subscription.ID
		periodEnd = time.Unix(sess.Subscription.CurrentPeriodEnd, 0)
	}

	if stripeSubID != "" {
		if err := s.SubscriptionDomain.SetStripeSubID(sub.ID, stripeSubID, newStatus, periodEnd); err != nil {
			return models.SubscriptionResp{}, err
		}
		sub.StripeSubscriptionID = stripeSubID
		sub.CurrentPeriodEnd = periodEnd

		// Assign credits to user on successful payment
		if newStatus == models.SubscriptionStatusActive {
			option, err := s.CreditOptionDomain.GetByID(sub.CreditOptionID)
			if err == nil {
				credits := float64(option.Value)
				_ = s.UserCreditDomain.UpsertCredits(sub.UserID, credits, models.UserCreditTypePurchased)
				_ = s.UserCreditDomain.AddHistory(sub.UserID, credits, models.UserCreditHistoryTypePurchase)
			}
		}
	} else {
		if err := s.SubscriptionDomain.UpdateStatus(sub.ID, newStatus); err != nil {
			return models.SubscriptionResp{}, err
		}
	}
	sub.Status = newStatus

	return toSubscriptionResp(sub), nil
}

// CancelSub cancels a Stripe subscription at the end of the current billing period.
func (s *SubscriptionService) CancelSub(userID int64, subscriptionID int64) (models.SubscriptionResp, error) {
	s.initStripe()

	sub, err := s.SubscriptionDomain.GetByID(subscriptionID)
	if err != nil {
		return models.SubscriptionResp{}, errors.New("subscription not found")
	}
	if sub.UserID != userID {
		return models.SubscriptionResp{}, errors.New("unauthorized")
	}
	if sub.StripeSubscriptionID == "" {
		return models.SubscriptionResp{}, errors.New("subscription has no stripe id")
	}

	cancelParams := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(true),
	}
	if _, err := stripesubscription.Update(sub.StripeSubscriptionID, cancelParams); err != nil {
		return models.SubscriptionResp{}, err
	}

	if err := s.SubscriptionDomain.UpdateStatus(sub.ID, models.SubscriptionStatusCanceled); err != nil {
		return models.SubscriptionResp{}, err
	}

	sub.Status = models.SubscriptionStatusCanceled
	return toSubscriptionResp(sub), nil
}

// GetSubscriptionStatus returns all subscriptions for the given user.
func (s *SubscriptionService) GetSubscriptionStatus(userID int64) ([]models.SubscriptionResp, error) {
	subs, err := s.SubscriptionDomain.GetByUserID(userID)
	if err != nil {
		return nil, err
	}

	result := make([]models.SubscriptionResp, len(subs))
	for i, sub := range subs {
		result[i] = toSubscriptionResp(sub)
	}
	return result, nil
}

// HandleWebhook verifies the Stripe signature and processes the event.
func (s *SubscriptionService) HandleWebhook(payload []byte, sigHeader string) error {
	s.initStripe()
	cfg := config.GetConfig()

	event, err := webhook.ConstructEvent(payload, sigHeader, cfg.StripeWebhookSecret)
	if err != nil {
		return err
	}

	switch event.Type {
	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			return err
		}
		s.handleCheckoutCompleted(&sess)

	case "invoice.payment_succeeded":
		var invoice stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			return err
		}
		s.handleInvoicePaid(&invoice)

	case "invoice.payment_failed":
		var invoice stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			return err
		}
		if invoice.Subscription != nil {
			_ = s.SubscriptionDomain.UpdateStatus2(invoice.Subscription.ID, models.SubscriptionStatusPastDue)
		}

	case "customer.subscription.deleted":
		var stripeSub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
			return err
		}
		_ = s.SubscriptionDomain.UpdateStatus2(stripeSub.ID, models.SubscriptionStatusCanceled)
	}

	return nil
}

func (s *SubscriptionService) handleCheckoutCompleted(sess *stripe.CheckoutSession) {
	if sess.Subscription == nil {
		return
	}
	localSub, err := s.SubscriptionDomain.GetByStripeSessionID(sess.ID)
	if err != nil {
		return
	}
	_ = s.SubscriptionDomain.SetStripeSubID(
		localSub.ID,
		sess.Subscription.ID,
		models.SubscriptionStatusActive,
		time.Time{}, // period end populated later via invoice event
	)
}

func (s *SubscriptionService) handleInvoicePaid(invoice *stripe.Invoice) {
	if invoice.Subscription == nil {
		return
	}

	sub, err := s.SubscriptionDomain.GetByStripeSubID(invoice.Subscription.ID)
	if err != nil {
		return
	}

	// Add credits to user based on the credit option's value
	option, err := s.CreditOptionDomain.GetByID(sub.CreditOptionID)
	if err != nil {
		return
	}
	_ = s.UserCreditDomain.Add(sub.UserID, float64(option.Value), models.UserCreditTypePurchased)

	_ = s.SubscriptionDomain.UpdateStatus2(invoice.Subscription.ID, models.SubscriptionStatusActive)
}

func (s *SubscriptionService) getOrCreateStripeCustomer(email, name string) (string, error) {
	params := &stripe.CustomerListParams{
		Email: stripe.String(email),
	}
	params.Single = true
	iter := stripecustomer.List(params)
	for iter.Next() {
		return iter.Customer().ID, nil
	}
	if err := iter.Err(); err != nil {
		return "", err
	}

	c, err := stripecustomer.New(&stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(name),
	})
	if err != nil {
		return "", err
	}
	return c.ID, nil
}

func toSubscriptionResp(sub models.Subscription) models.SubscriptionResp {
	return models.SubscriptionResp{
		ID:                   sub.ID,
		CreditOptionID:       sub.CreditOptionID,
		StripeSubscriptionID: sub.StripeSubscriptionID,
		Status:               sub.Status,
		Currency:             sub.Currency,
		BaseAmount:           sub.BaseAmount,
		GSTAmount:            sub.GSTAmount,
		TotalAmount:          sub.TotalAmount,
		CurrentPeriodEnd:     sub.CurrentPeriodEnd,
	}
}
