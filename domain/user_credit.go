package domain

import (
	"core/config"
	"core/models"
)

type UserCreditDomain interface {
	Add(userID int64, amount float64, creditType int16) error
	UpsertCredits(userID int64, credits float64, creditType int16) error
	AddHistory(userID int64, credits float64, histType int16) error
	GetTotalCredits(userID int64) (float64, error)
}

type UserCreditDomainCtx struct{}

func (d *UserCreditDomainCtx) Add(userID int64, amount float64, creditType int16) error {
	db := config.DbManager()
	credit := models.UserCredit{
		UserID:       userID,
		TotalCredits: amount,
		Type:         creditType,
	}
	return db.Create(&credit).Error
}

// UpsertCredits adds credits to the user's existing row, or creates one if it doesn't exist.
func (d *UserCreditDomainCtx) UpsertCredits(userID int64, credits float64, creditType int16) error {
	db := config.DbManager()

	var existing models.UserCredit
	err := db.Where("user_id = ?", userID).First(&existing).Error
	if err != nil {
		// No row yet — create one
		return db.Create(&models.UserCredit{
			UserID:       userID,
			TotalCredits: credits,
			Type:         creditType,
		}).Error
	}

	// Row exists — add on top
	return db.Model(&models.UserCredit{}).
		Where("user_id = ?", userID).
		Update("total_credits", existing.TotalCredits+credits).Error
}

// GetTotalCredits returns the sum of all credits for a user, or 0 if none exist.
func (d *UserCreditDomainCtx) GetTotalCredits(userID int64) (float64, error) {
	db := config.DbManager()
	var total float64
	err := db.Model(&models.UserCredit{}).
		Where("user_id = ?", userID).
		Select("COALESCE(SUM(total_credits), 0)").
		Scan(&total).Error
	if err != nil {
		return 0, err
	}
	return total, nil
}

// AddHistory inserts a credit purchase record into user_credit_history.
func (d *UserCreditDomainCtx) AddHistory(userID int64, credits float64, histType int16) error {
	db := config.DbManager()
	history := models.UserCreditHistory{
		UserID:  userID,
		Credits: credits,
		Value:   credits,
		Type:    histType,
	}
	return db.Create(&history).Error
}
