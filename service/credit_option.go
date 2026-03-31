package service

import (
	"core/domain"
	"core/models"
	"encoding/json"
)

type CreditOptionService struct {
	CreditOptionDomain domain.CreditOptionDomain
}

func (c *CreditOptionService) GetOptionList() ([]models.CreditOptionListResp, error) {
	data, err := c.CreditOptionDomain.GetAll()
	if err != nil {
		return nil, err
	}

	result := make([]models.CreditOptionListResp, 0, len(data))
	for _, opt := range data {
		var price models.IntLocale
		if err := json.Unmarshal([]byte(opt.Price), &price); err != nil {
			return nil, err
		}

		var points []models.StringLocale
		if err := json.Unmarshal([]byte(opt.Points), &points); err != nil {
			return nil, err
		}

		var other models.StringLocale
		if opt.Other != nil {
			if err := json.Unmarshal([]byte(*opt.Other), &other); err != nil {
				return nil, err
			}
		}

		result = append(result, models.CreditOptionListResp{
			ID:     opt.ID,
			Value:  opt.Value,
			Price:  price,
			Points: points,
			Other:  other,
		})
	}

	return result, nil
}
