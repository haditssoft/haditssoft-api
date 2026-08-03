package models

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
)

// contoh table KitabShahihMuslim
// ini adalah data dropdown kitab
type Note struct {
	NKitab  string `json:"NKitab"`
	VMember string `json:"VMember"`
	Awalan  string `json:"Awalan"`
}

func GetAllNotes(bookName string) (*[]Note, error) {

	var allNotes []Note

	if err := database.DB.Select(
		"NKitab",
		"VMember",
		"Awalan",
	).Table(
		bookName + "Note",
	).Order(
		"VMember ASC",
	).Find(&allNotes).Error; err != nil {
		return nil, err
	}

	return &allNotes, nil
}
