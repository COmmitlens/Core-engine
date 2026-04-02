package models

import "time"

const (
	UserCreditTypePurchased int16 = 1
	UserCreditTypeBonus     int16 = 2
)

type UserCredit struct {
	ID           int64      `gorm:"column:id;primary_key"`
	UserID       int64      `gorm:"column:user_id"`
	TotalCredits float64    `gorm:"column:total_credits"`
	Type         int16      `gorm:"column:type"`
	ExpiredAt    *time.Time `gorm:"column:expired_at"`
}

func (UserCredit) TableName() string {
	return "user_credit"
}

type UserCreditResp struct {
	TotalCredits float64    `json:"total_credits"`
	Type         int16      `json:"type"`
	ExpiredAt    *time.Time `json:"expired_at"`
}
