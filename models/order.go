package models

import "time"

const (
	OrderStatusPending  int16 = 0
	OrderStatusPaid     int16 = 1
	OrderStatusFailed   int16 = 2
	OrderStatusRefunded int16 = 3

	OrderTypeCreditPurchase int16 = 1
)

type Order struct {
	ID                   int64     `gorm:"column:id;primary_key"`
	UserID               int64     `gorm:"column:user_id"`
	CreditOptionID       int64     `gorm:"column:credit_option_id"`
	Date                 time.Time `gorm:"column:date"`
	Type                 int16     `gorm:"column:type"`
	Quantity             int32     `gorm:"column:quantity"`
	Currency             string    `gorm:"column:currency"`
	Total                int64     `gorm:"column:total"`
	Status               int16     `gorm:"column:status"`
	InvoiceURL           string    `gorm:"column:invoice_url"`
	InvoiceID            string    `gorm:"column:invoice_id"`
	InvoiceNumber        string    `gorm:"column:invoice_number"`
	StripePaymentIntentID string   `gorm:"column:stripe_payment_intent_id"`
	StripeSessionID      string    `gorm:"column:stripe_session_id"`
}

func (Order) TableName() string {
	return "order"
}

type CreateCheckoutParam struct {
	CreditOptionID int64  `json:"credit_option_id"`
	Currency       string `json:"currency"`
}

type CheckoutSessionResp struct {
	OrderID    int64  `json:"order_id"`
	SessionURL string `json:"session_url"`
}

type OrderResp struct {
	ID             int64     `json:"id"`
	CreditOptionID int64     `json:"credit_option_id"`
	Date           time.Time `json:"date"`
	Type           int16     `json:"type"`
	Quantity       int32     `json:"quantity"`
	Currency       string    `json:"currency"`
	Total          int64     `json:"total"`
	Status         int16     `json:"status"`
	InvoiceURL     string    `json:"invoice_url"`
	InvoiceNumber  string    `json:"invoice_number"`
}
