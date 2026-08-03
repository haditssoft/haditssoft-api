package models

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
)

func GetScholarComment(narratorId string) ([]map[string]interface{}, error) {
	rows := make([]map[string]interface{}, 0)

	err := database.DB.Select(
		"Komentar",
		"Ulama",
	).Table(
		"KomentarUlama",
	).Where(
		"KodeRawi = ?", narratorId,
	).Order(
		"Ulama ASC",
	).Find(&rows).Error
	if err != nil {
		return nil, err
	}

	return rows, nil
}
