package models

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"strconv"
)

// contoh table KitabShahihMuslim
// ini adalah data dropdown kitab
type Chapter struct {
	NBab       string `json:"NBab"`
	VMemberBab string `json:"VMemberBab"`
	AwalanBab  string `json:"AwalanBab"`
}

func GetAllChapters(kitabName, start, end string) (*[]Chapter, error) {
	asInt, err := strconv.Atoi(start)
	if err != nil {
		return nil, err
	}

	var chapters []Chapter

	if err := database.DB.Select(
		"NBab",
		"VMemberBab",
		"AwalanBab",
	).Table(
		"Bab"+kitabName,
	).Where(
		"AwalanBab BETWEEN ? AND ?", (asInt - 1), end,
	).Order(
		"VMemberBab ASC",
	).Find(&chapters).Error; err != nil {
		return nil, err
	}

	return &chapters, nil
}

// vSelectedK will be used to get the end bound first before jump to loadAllBab() above,
// this is because there is no way in front-end to get the end value
// if books titles aren't all loaded yet
// so get the end value first by 'vSelectedK' value of currently shown books title but increase it by 1
// to target the next books title, because its 'Awalan' value is the end bound we need
func GetBeginingOfNextBookTitle(kitabName, start, vSelectedK string) (*[]Chapter, error) {

	asInt, err := strconv.Atoi(vSelectedK)
	if err != nil {
		return nil, err
	}

	var begining string

	if err := database.DB.Select(
		"Awalan", // just get the Awalan number of next book's title
	).Table(
		"Kitab"+kitabName,
	).Where(
		"VMember = ?", (asInt + 1), // + 1 to target next book's title
	).Limit(1).Scan(&begining).Error; err != nil {
		return nil, err
	}

	chapters, err := GetAllChapters(kitabName, start, begining)
	if err != nil {
		return nil, err
	}

	return chapters, nil
}
