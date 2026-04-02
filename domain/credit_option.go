package domain

import (
	"core/config"
	"core/models"
)

type CreditOptionDomain interface {
	GetAll() ([]models.CreditOption, error)
	GetByID(id int64) (models.CreditOption, error)
}

type CreditOptionDomainCtx struct{}

func (c *CreditOptionDomainCtx) GetAll() ([]models.CreditOption, error) {
	db := config.DbManager()
	var options []models.CreditOption
	err := db.Where("is_active = ?", true).Find(&options).Error
	if err != nil {
		return nil, err
	}
	return options, nil
}

func (c *CreditOptionDomainCtx) GetByID(id int64) (models.CreditOption, error) {
	db := config.DbManager()
	var option models.CreditOption
	err := db.Where("id = ? AND is_active = ?", id, true).First(&option).Error
	if err != nil {
		return models.CreditOption{}, err
	}
	return option, nil
}
