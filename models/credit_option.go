package models

func (CreditOption) TableName() string {
	return "credit_option"
}

type CreditOption struct {
	ID            int64   `gorm:"column:id;primary_key"`
	Value         int32   `gorm:"column:value"`
	Price         string  `gorm:"column:price;type:json"`
	Points        string  `gorm:"column:points;type:json"`
	Other         *string `gorm:"column:other;type:json"`
	Credits       int64   `gorm:"column:credits"`
	IsActive      bool    `gorm:"column:is_active"`
	StripePriceID string  `gorm:"column:stripe_price_id"`
}

type CreditOptionListResp struct {
	ID     int64          `json:"id"`
	Value  int32          `json:"value"`
	Price  IntLocale      `json:"price"`
	Points []StringLocale `json:"points"`
	Other  StringLocale   `json:"other"`
}

type CreaditOptionByID struct {
	ID int64 `json:"id"`
}
