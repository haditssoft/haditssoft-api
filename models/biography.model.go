package models

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/internal/shared/utils"
)

type Biography struct {
	Biografi string
}

func GetBiography(kitabName string) (*Biography, error) {

	replacedText := utils.GetKitabName(kitabName)

	biographyData := new(Biography)

	if err := database.DB.Select(
		"Biografi",
	).Table(
		"Biografi" + replacedText,
	).Find(biographyData).Error; err != nil {
		return nil, err
	}

	return biographyData, nil
}
