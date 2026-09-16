package models

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
)

// contoh table KitabShahihMuslim
// ini adalah data dropdown kitab
type Book struct {
	NKitab    string `json:"NKitab"`
	NKitabEng string `json:"NKitabEng"`
	NKitabUrd string `json:"NKitabUrd"`
	NKitabBen string `json:"NKitabBen"`
	VMember   string `json:"VMember"`
	Awalan    string `json:"Awalan"`
}

func GetAllBooks(kitabName string) (*[]Book, error) {

	var allBooks []Book

	if err := database.DB.Select(
		"NKitab",
		"NKitabEng",
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
