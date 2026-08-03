package models

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"strings"
)

type ClassificationData struct {
	Nomer       string      `json:"Nomer"`
	Arabic      string      `json:"Arabic"`
	Indonesia   string      `json:"Indonesia"`
	Albani      string      `json:"Albani"`
	Darussalam  string      `json:"Darussalam"`
	VSelectedK  uint        `json:"VSelectedK"`
	VSelectedB  uint        `json:"VSelectedB"`
	KitabTitle  interface{} `gorm:"-:all" json:"kitabTitle"`
	BabTitle    interface{} `gorm:"-:all" json:"babTitle"`
	HadithCount interface{} `gorm:"-:all" json:"hadithCount"`
	OtherNumber string      `gorm:"-:all" json:"otherNumber"`
}

func LoadClassificationData(kitab, classify, number string) (ClassificationData, error) {
	row := new(ClassificationData)
	err := database.DB.Select(
		"Nomer",
		"Arabic",
		"Indonesia",
		"Albani",
		"Darussalam",
		"VSelectedK",
		"VSelectedB",
	).Table(kitab).Joins(
		"INNER JOIN "+classify+" ON NoHdt = Nomer AND No = ?", number,
	).First(row).Error
	if err != nil {
		return ClassificationData{}, err
	}

	dataType := "CLASSIFICATIONDATA"

	countRowsChan := make(chan error)
	kitabChan := make(chan error)
	babChan := make(chan error)
	noLainChan := make(chan error)

	go row.countRows([]string{kitab, classify, row.Nomer}, dataType, countRowsChan)
	go row.getMinimalKitab([]string{kitab, classify, row.Nomer}, dataType, kitabChan)
	go row.getMinimalBab([]string{kitab, classify, row.Nomer}, dataType, babChan)
	go row.getNoLain([]string{kitab, classify, row.Nomer}, dataType, noLainChan)

	err = <-countRowsChan
	if err != nil {
		return ClassificationData{}, err
	}
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

func (me *ClassificationData) countRows(kitab []string, dataType string, c chan error) {
	table := kitab[1]
	if dataType == "MAINBOOKSDATA" {
		table = kitab[0]
	}

	var count int64
	err := database.DB.Table(
		table,
	).Count(&count).Error
	if err != nil {
		c <- err
	}
	me.HadithCount = []interface{}{map[string]int64{"count(*)": count}}

	c <- nil
}

func (me *ClassificationData) getMinimalKitab(kitab []string, dataType string, c chan error) {
	var kitabTitle KitabTitle
	err := database.DB.Select(
		"NKitab",
		"VMember",
		"Awalan",
	).Table("Kitab"+kitab[0]).Where(
		"VMember = ?", me.VSelectedK,
	).First(&kitabTitle).Error
	if err != nil {
		c <- err
	}

	me.KitabTitle = []interface{}{kitabTitle}
	c <- nil
}

func (me *ClassificationData) getMinimalBab(kitab []string, dataType string, c chan error) { // array, 6823, 7008
	var babTitle BabTitle
	err := database.DB.Select(
		"NBab",
		"VMemberBab",
		"AwalanBab",
	).Table("Bab"+kitab[0]).Where(
		"VMemberBab = ?", me.VSelectedB,
	).First(&babTitle).Error
	if err != nil {
		c <- err
	}
	me.BabTitle = []interface{}{babTitle}
	c <- nil
}

func (me *ClassificationData) getNoLain(kitabAndNumber []string, dataType string, c chan error) {
	var nomer string
	if dataType == "MAINBOOKSDATA" {
		nomer = kitabAndNumber[1]
	} else {
		nomer = kitabAndNumber[2]
	}

	var noLain NoLain
	// .Select(
	// 	"NoLain1",
	// 	"NoLain2",
	// 	"NoLain3",
	// 	"NoLain4",
	// 	"NoLain5",
	// 	"NoLain6",
	// 	"NoLain7",
	// 	"NoLain8",
	// 	"NoLain9",
	// )

	err := database.DB.Table(
		"NoLain"+kitabAndNumber[0],
	).Where(
		"No = ?", nomer,
	).Order("No ASC").First(&noLain).Error
	if err != nil {
		c <- err
	}

	s := new(strings.Builder)

	s.WriteString(noLain.NoLain1)
	s.WriteString(", ")
	s.WriteString(noLain.NoLain2)
	s.WriteString(", ")
	s.WriteString(noLain.NoLain3)
	s.WriteString(", ")
	s.WriteString(noLain.NoLain4)
	s.WriteString(", ")
	s.WriteString(noLain.NoLain5)
	s.WriteString(", ")
	s.WriteString(noLain.NoLain6)
	s.WriteString(", ")
	s.WriteString(noLain.NoLain7)
	s.WriteString(", ")
	s.WriteString(noLain.NoLain8)
	s.WriteString(", ")
	s.WriteString(noLain.NoLain9)

	otherNum := strings.TrimSuffix(s.String(), ", ")
	me.OtherNumber = otherNum
	c <- nil
}
