package models

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
)

// contoh tabel RawiShahihBukhari, RawiSunanTirmidzi, dll
type RawiInKitab struct {
	NoUrut   uint   `json:"-"`
	NoHdt    string `json:"NoHdt"`
	KodeRawi string `json:"-"`
}

func GetTotalHadith(kitabName, narratorId string) ([]RawiInKitab, error) {
	rows := make([]RawiInKitab, 0)

	err := database.DB.Omit(
		"NoUrut", "KodeRawi",
	).Select(
		"NoHdt",
	).Table(
		"Rawi"+kitabName,
	).Where(
		"KodeRawi = ?", narratorId,
	).Order(
		"NoHdt ASC",
	).Find(&rows).Error
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func AfterGetTotalHadith(kitabName, NoHdt string) (ClassificationData, error) {
	row := new(ClassificationData)

	err := database.DB.Select(
		"Nomer",
		"Arabic",
		"Indonesia",
		"Albani",
		"Darussalam",
		"VSelectedK",
		"VSelectedB",
	).Table(
		kitabName,
	).Where(
		"Nomer = ?", NoHdt,
	).Limit(1).Find(row).Error
	if err != nil {
		return ClassificationData{}, err
	}

	// 1 is the current position, why? because this block of code
	// just for the first time when showing TotalHadith, so the position is 1
	// mainWindow.webContents.send('resultMainData', row, 'TOTALHADITHDATA', 1);
	// the use of 'MAINBOOKSDATA' has to do with index number
	// where number of hadits is located to perform search on No Lain.
	// 'MAINBOOKSDATA' designate index 1
	// while 'CLASSIFICATIONDATA' index 2
	dataType := "MAINBOOKSDATA"

	kitabChan := make(chan error)
	babChan := make(chan error)
	noLainChan := make(chan error)

	go row.getMinimalKitab([]string{kitabName, row.Nomer}, dataType, kitabChan)
	go row.getMinimalBab([]string{kitabName, row.Nomer}, dataType, babChan)
	go row.getNoLain([]string{kitabName, row.Nomer}, dataType, noLainChan)

	err = <-kitabChan
	if err != nil {
		return ClassificationData{}, err
	}
	err = <-babChan
	if err != nil {
		return ClassificationData{}, err
	}
	err = <-noLainChan
	if err != nil {
		return ClassificationData{}, err
	}
	return *row, nil
}
