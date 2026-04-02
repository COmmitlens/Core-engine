package models

import "time"

const (
	SubscriptionStatusActive   = "active"
	SubscriptionStatusCanceled = "canceled"
	SubscriptionStatusPastDue  = "past_due"
	SubscriptionStatusPending  = "pending"

	GSTRate = 18 // percent
)

type Subscription struct {
	ID                   int64     `gorm:"column:id;primary_key"`
	UserID               int64     `gorm:"column:user_id"`
	CreditOptionID       int64     `gorm:"column:credit_option_id"`
	StripeCustomerID     string    `gorm:"column:stripe_customer_id"`
	StripeSubscriptionID string    `gorm:"column:stripe_subscription_id"`
	StripeSessionID      string    `gorm:"column:stripe_session_id"`
	Status               string    `gorm:"column:status"`
	Currency             string    `gorm:"column:currency"`
	Credits              int64     `gorm:"column:credits"`
	BaseAmount           int64     `gorm:"column:base_amount"`
	GSTAmount            int64     `gorm:"column:gst_amount"`
	TotalAmount          int64     `gorm:"column:total_amount"`
	CurrentPeriodEnd     time.Time `gorm:"column:current_period_end"`
	CreatedAt            time.Time `gorm:"column:created_at"`
	UpdatedAt            time.Time `gorm:"column:updated_at"`
}

func (Subscription) TableName() string {
	return "subscription"
}

type CreateSubscriptionReq struct {
	CreditOptionID int64 `json:"credit_option_id"`
}

type VerifySubReq struct {
	SessionID string `json:"session_id"`
}

type CancelSubReq struct {
	SubscriptionID int64 `json:"subscription_id"`
}

type SubscriptionResp struct {
	ID                   int64     `json:"id"`
	CreditOptionID       int64     `json:"credit_option_id"`
	StripeSubscriptionID string    `json:"stripe_subscription_id"`
	Status               string    `json:"status"`
	Currency             string    `json:"currency"`
	BaseAmount           int64     `json:"base_amount"`
	GSTAmount            int64     `json:"gst_amount"`
	TotalAmount          int64     `json:"total_amount"`
	CurrentPeriodEnd     time.Time `json:"current_period_end"`
}

type CreateSubscriptionResp struct {
	SessionURL string `json:"session_url"`
	SessionID  string `json:"session_id"`
}
