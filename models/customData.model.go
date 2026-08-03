package models

import (
	"errors"

	"gorm.io/gorm"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"strings"
)

type CustomData struct {
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

func LoadCustomData(kitabNumberPosition []string) (CustomData, error) {
	row := new(CustomData)
	err := database.DB.Select(
		"Nomer",
		"Arabic",
		"Indonesia",
		"Albani",
		"Darussalam",
		"VSelectedK",
		"VSelectedB",
	).Table(kitabNumberPosition[0]).Where(
		"Nomer = ?", kitabNumberPosition[1],
	).First(row).Error
	if err != nil {
		return CustomData{}, err
	}

	dataType := "MAINBOOKSDATA"

	// countRowsChan := make(chan error)
	kitabChan := make(chan error)
	babChan := make(chan error)
	noLainChan := make(chan error)

	// go row.countRows([]string{kitab, classify, row.Nomer}, dataType, countRowsChan)
	go row.getMinimalKitab(kitabNumberPosition, dataType, kitabChan)
	go row.getMinimalBab(kitabNumberPosition, dataType, babChan)
	go row.getNoLain(kitabNumberPosition, dataType, noLainChan)

	// err = <-countRowsChan
	// if err != nil {
	// 	return CustomData{}, err
	// }
	err = <-kitabChan
	if err != nil {
		return CustomData{}, err
	}
	err = <-babChan
	if err != nil {
		return CustomData{}, err
	}
	err = <-noLainChan
	if err != nil {
		return CustomData{}, err
	}
	return *row, nil
}

// func (me *CustomData) countRows(kitab []string, dataType string, c chan error) {
// 	table := kitab[1]
// 	if dataType == "MAINBOOKSDATA" {
// 		table = kitab[0]
// 	}

// 	var count int64
// 	err := database.DB.Table(
// 		table,
// 	).Count(&count).Error
// 	if err != nil {
// 		c <- err
// 	}
// 	me.HadithCount = []interface{}{map[string]int64{"count(*)": count}}

// 	c <- nil
// }

func (me *CustomData) getMinimalKitab(kitab []string, dataType string, c chan error) {
	var kitabTitle KitabTitle
	err := database.DB.Select(
		"NKitab",
		"VMember",
		"Awalan",
	).Table("Kitab"+kitab[0]).Where(
		"VMember = ?", me.VSelectedK,
	).First(&kitabTitle).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			me.KitabTitle = []interface{}{}
			c <- nil
			return
		}
		c <- err
		return
	}

	me.KitabTitle = []interface{}{kitabTitle}
	c <- nil
}

func (me *CustomData) getMinimalBab(kitab []string, dataType string, c chan error) { // array, 6823, 7008
	var babTitle BabTitle
	err := database.DB.Select(
		"NBab",
		"VMemberBab",
		"AwalanBab",
	).Table("Bab"+kitab[0]).Where(
		"VMemberBab = ?", me.VSelectedB,
	).First(&babTitle).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			me.BabTitle = []interface{}{}
			c <- nil
			return
		}
		c <- err
		return
	}
	me.BabTitle = []interface{}{babTitle}
	c <- nil
}

func (me *CustomData) getNoLain(kitabAndNumber []string, dataType string, c chan error) {
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			me.OtherNumber = ""
			c <- nil
			return
		}
		c <- err
		return
	}

	var parts []string
	if noLain.NoLain1 != "" {
		parts = append(parts, noLain.NoLain1)
	}
	if noLain.NoLain2 != "" {
		parts = append(parts, noLain.NoLain2)
	}
	if noLain.NoLain3 != "" {
		parts = append(parts, noLain.NoLain3)
	}
	if noLain.NoLain4 != "" {
		parts = append(parts, noLain.NoLain4)
	}
	if noLain.NoLain5 != "" {
		parts = append(parts, noLain.NoLain5)
	}
	if noLain.NoLain6 != "" {
		parts = append(parts, noLain.NoLain6)
	}
	if noLain.NoLain7 != "" {
		parts = append(parts, noLain.NoLain7)
	}
	if noLain.NoLain8 != "" {
		parts = append(parts, noLain.NoLain8)
	}
	if noLain.NoLain9 != "" {
		parts = append(parts, noLain.NoLain9)
	}

	me.OtherNumber = strings.Join(parts, ", ")
	c <- nil
}
