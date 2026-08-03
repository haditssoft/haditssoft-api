package models

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
)

// contoh tabel BandingShahihBukhari, BandingSunanTirmidzi, dll
type Similar struct {
	// NoUrut    uint   `json:"NoUrut"`
	// NoHdt     string `json:"NoHdt"`
	Nama      string `json:"Nama"`
	NoBanding string `json:"NoBanding"`
}

func GetSimilarHadith(kitabName, number string) ([]Similar, error) {
	rows := make([]Similar, 0)

	err := database.DB.Select(
		"Nama", "NoBanding",
	).Table(
		kitabName,
	).Where(
		"NoHdt = ?", number,
	).Order(
		"NoUrut ASC",
	).Find(&rows).Error
	if err != nil {
		return nil, err
	}

	return rows, nil
}
