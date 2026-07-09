package service

import (
	"core/domain"
	"core/models"
)

type WaitlistService struct {
	WaitlistDomain *domain.WaitlistDomainCtx
}

func (w *WaitlistService) AddToWaitlist(param models.WaitlistParam) error {
	return w.WaitlistDomain.AddToWaitlist(param)
}
