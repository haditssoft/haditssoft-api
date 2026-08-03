package models

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
)

// tabel DaftarRawi
type OtherNumber struct {
	No      uint `json:"No"`
	NoLain1 uint `json:"NoLain1"`
	NoLain2 uint `json:"NoLain2"`
	NoLain3 uint `json:"NoLain3"`
	NoLain4 uint `json:"NoLain4"`
	NoLain5 uint `json:"NoLain5"`
	NoLain6 uint `json:"NoLain6"`
	NoLain7 uint `json:"NoLain7"`
	NoLain8 uint `json:"NoLain8"`
	NoLain9 uint `json:"NoLain9"`
}

func GetOriginalNumber(kitabName, number string) (string, error) {

	var originalNumber string

	if err := database.DB.Select(
		"No",
	).Table(
		"NoLain"+kitabName,
	).Where(
		"NoLain1 = ?", number,
	).Or(
		"NoLain2 = ?", number,
	).Or(
		"NoLain3 = ?", number,
	).Or(
		"NoLain4 = ?", number,
	).Or(
		"NoLain5 = ?", number,
	).Or(
		"NoLain6 = ?", number,
	).Or(
		"NoLain7 = ?", number,
	).Or(
		"NoLain8 = ?", number,
	).Or(
		"NoLain9 = ?", number,
	).Order("No ASC").Limit(1).Scan(&originalNumber).Error; err != nil {
		return "", err
	}

	return originalNumber, nil
}
