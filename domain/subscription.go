package domain

import (
	"core/config"
	"core/models"
	"time"
)

type SubscriptionDomain interface {
	Create(sub models.Subscription) (models.Subscription, error)
	GetByUserID(userID int64) ([]models.Subscription, error)
	GetActiveByUserID(userID int64) (models.Subscription, error)
	GetByStripeSubID(stripeSubID string) (models.Subscription, error)
	GetByStripeSessionID(sessionID string) (models.Subscription, error)
	UpdateStatus(id int64, status string) error
	SetStripeSubID(id int64, stripeSubID string, status string, periodEnd time.Time) error
	UpdateStatus2(stripeSubID string, status string) error
}

type SubscriptionDomainCtx struct{}

func (d *SubscriptionDomainCtx) Create(sub models.Subscription) (models.Subscription, error) {
	db := config.DbManager()
	err := db.Create(&sub).Error
	if err != nil {
		return models.Subscription{}, err
	}
	return sub, nil
}

func (d *SubscriptionDomainCtx) GetByUserID(userID int64) ([]models.Subscription, error) {
	db := config.DbManager()
	var subs []models.Subscription
	err := db.Where("user_id = ?", userID).Order("created_at DESC").Find(&subs).Error
	if err != nil {
		return nil, err
	}
	return subs, nil
}

func (d *SubscriptionDomainCtx) GetActiveByUserID(userID int64) (models.Subscription, error) {
	db := config.DbManager()
	var sub models.Subscription
	err := db.Where("user_id = ? AND status = ?", userID, models.SubscriptionStatusActive).
		Order("created_at DESC").
		First(&sub).Error
	if err != nil {
		return models.Subscription{}, err
	}
	return sub, nil
}

func (d *SubscriptionDomainCtx) GetByStripeSubID(stripeSubID string) (models.Subscription, error) {
	db := config.DbManager()
	var sub models.Subscription
	err := db.Where("stripe_subscription_id = ?", stripeSubID).First(&sub).Error
	if err != nil {
		return models.Subscription{}, err
	}
	return sub, nil
}

func (d *SubscriptionDomainCtx) GetByStripeSessionID(sessionID string) (models.Subscription, error) {
	db := config.DbManager()
	var sub models.Subscription
	err := db.Where("stripe_session_id = ?", sessionID).First(&sub).Error
	if err != nil {
		return models.Subscription{}, err
	}
	return sub, nil
}

func (d *SubscriptionDomainCtx) UpdateStatus(id int64, status string) error {
	db := config.DbManager()
	return db.Model(&models.Subscription{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// SetStripeSubID links the stripe subscription ID after checkout completes and updates status + period end.
func (d *SubscriptionDomainCtx) SetStripeSubID(id int64, stripeSubID string, status string, periodEnd time.Time) error {
	db := config.DbManager()
	updates := map[string]interface{}{
		"stripe_subscription_id": stripeSubID,
		"status":                 status,
		"current_period_end":     periodEnd,
	}
	return db.Model(&models.Subscription{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateStatus2 updates status by stripe subscription ID (used in webhook).
func (d *SubscriptionDomainCtx) UpdateStatus2(stripeSubID string, status string) error {
	db := config.DbManager()
	return db.Model(&models.Subscription{}).
		Where("stripe_subscription_id = ?", stripeSubID).
		Update("status", status).Error
}
