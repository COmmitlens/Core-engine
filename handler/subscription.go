package handler

import (
	"core/models"
	"core/service"
	"core/utils"
	"io"
	"net/http"

	"github.com/labstack/echo"
)

type SubscriptionHandler struct {
	SubscriptionService service.SubscriptionService
}

// Subscribe creates a Stripe Checkout Session (subscription mode) for a credit option.
// POST /credit/subscription
func (h *SubscriptionHandler) Subscribe(c echo.Context) error {
	userID := c.Get("id").(int64)

	var req models.CreateSubscriptionReq
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
	}
	if req.CreditOptionID == 0 {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: "credit_option_id is required"})
	}

	resp, err := h.SubscriptionService.CreateSubscription(userID, req.CreditOptionID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, models.BasicResp{Message: utils.Success, Data: resp})
}

// VerifySub verifies a Stripe Checkout Session and updates subscription status.
// POST /credit/verify_sub
func (h *SubscriptionHandler) VerifySub(c echo.Context) error {
	var req models.VerifySubReq
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
	}
	if req.SessionID == "" {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: "session_id is required"})
	}

	resp, err := h.SubscriptionService.VerifySub(req.SessionID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, models.BasicResp{Message: utils.Success, Data: resp})
}

// CancelSub cancels the user's Stripe subscription at period end.
// POST /credit/cancel_sub
func (h *SubscriptionHandler) CancelSub(c echo.Context) error {
	userID := c.Get("id").(int64)

	var req models.CancelSubReq
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
	}
	if req.SubscriptionID == 0 {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: "subscription_id is required"})
	}

	resp, err := h.SubscriptionService.CancelSub(userID, req.SubscriptionID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, models.BasicResp{Message: utils.Success, Data: resp})
}

// SubscriptionStatus returns all subscriptions for the current user.
// GET /credit/subscription-status
func (h *SubscriptionHandler) SubscriptionStatus(c echo.Context) error {
	userID := c.Get("id").(int64)

	resp, err := h.SubscriptionService.GetSubscriptionStatus(userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, models.BasicResp{Message: utils.Success, Data: resp})
}

// StripeWebhook handles incoming Stripe webhook events.
// POST /stripe/webhook  (no JWT – verified by Stripe signature)
func (h *SubscriptionHandler) StripeWebhook(c echo.Context) error {
	payload, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: "failed to read request body"})
	}

	sigHeader := c.Request().Header.Get("Stripe-Signature")
	if sigHeader == "" {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: "missing Stripe-Signature header"})
	}

	if err := h.SubscriptionService.HandleWebhook(payload, sigHeader); err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, models.BasicResp{Message: utils.Success})
}
