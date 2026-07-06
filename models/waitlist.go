package models

type Waitlist struct {
	ID        int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string `gorm:"not null" json:"name"`
	Company   string `gorm:"not null" json:"company"`
	Email     string `gorm:"unique;not null" json:"email"`
	Message   string `gorm:"not null" json:"message"`
	CreatedAt int64  `gorm:"autoCreateTime" json:"created_at"`
}

type WaitlistParam struct {
	Name    string `json:"name" validate:"required"`
	Company string `json:"company" validate:"required"`
	Email   string `json:"email" validate:"required,email"`
	Message string `json:"message" validate:"required"`
}
