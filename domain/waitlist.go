package domain

import (
	"core/config"
	"core/models"
)

type WaitlistDomain interface {
	AddToWaitlist(param models.WaitlistParam) error
}

type WaitlistDomainCtx struct{}

func (w *WaitlistDomainCtx) AddToWaitlist(param models.WaitlistParam) error {
	waitlist := models.Waitlist{
		Name:    param.Name,
		Company: param.Company,
		Email:   param.Email,
		Message: param.Message,
	}
	db := config.DbManager()
	if err := db.Create(&waitlist).Error; err != nil {
		return err
	}
	return nil
}
