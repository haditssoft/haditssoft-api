package models

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
)

// contoh table KitabShahihMuslim
// ini adalah data dropdown kitab
type Book struct {
	NKitab  string `json:"NKitab"`
	VMember string `json:"VMember"`
	Awalan  string `json:"Awalan"`
}

func GetAllBooks(kitabName string) (*[]Book, error) {

	var allBooks []Book

	if err := database.DB.Select(
		"NKitab",
		"VMember",
		"Awalan",
	).Table(
		"Kitab" + kitabName,
	).Order(
		"VMember ASC",
	).Find(&allBooks).Error; err != nil {
		return nil, err
	}

	return &allBooks, nil
}
