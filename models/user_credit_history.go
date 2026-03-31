package models

import "time"

const (
	UserCreditHistoryTypePurchase int16 = 1
	UserCreditHistoryTypeDebit    int16 = 2
	UserCreditHistoryTypeExpiry   int16 = 3
	UserCreditHistoryTypeRefund   int16 = 4
)

type UserCreditHistory struct {
	ID        int64     `gorm:"column:id;primary_key"`
	UserID    int64     `gorm:"column:user_id"`
	Value     float64   `gorm:"column:value"`
	Credits   float64   `gorm:"column:credits"`
	Type      int16     `gorm:"column:type"`
	CreatedAt time.Time `gorm:"column:created_at"`

	User User `gorm:"foreignkey:UserID"`
}

func (UserCreditHistory) TableName() string {
	return "user_credit_history"
}

type UserCreditHistoryResp struct {
	ID        int64     `json:"id"`
	Value     float64   `json:"value"`
	Type      int16     `json:"type"`
	Credits   float64   `json:"credits"`
	CreatedAt time.Time `json:"created_at"`
}
