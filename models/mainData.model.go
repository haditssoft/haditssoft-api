package models

import (
	"database/sql/driver"
	"fmt"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"strconv"
	"strings"
)

type NewUint uint

func (n *NewUint) Scan(value interface{}) error {
	if value == nil {
		*n = 0
		return nil
	}
	switch v := value.(type) {
	case uint:
		*n = NewUint(v)
	case uint64:
		*n = NewUint(v)
	case int64:
		if v < 0 {
			return fmt.Errorf("NewUint: negative value %d", v)
		}
		*n = NewUint(v)
	case []byte:
		s := string(v)
		if s == "" {
			*n = 0
			return nil
		}
		u, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return err
		}
		*n = NewUint(u)
	case string:
		if v == "" {
			*n = 0
			return nil
		}
		u, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return err
		}
		*n = NewUint(u)
	default:
		return fmt.Errorf("NewUint: cannot scan type %T", value)
	}
	return nil
}

func (n NewUint) Value() (driver.Value, error) {
	return int64(n), nil
}

type MainData struct {
	Nomer       uint        `json:"Nomer"`
	Arabic      *string     `json:"Arabic"`
	Indonesia   *string     `json:"Indonesia"`
	English     *string     `json:"English"`
	Urdu        *string     `json:"Urdu"`
	Bengali     *string     `json:"Bengali"`
	Albani      *string     `json:"Albani"`
	Darussalam  *string     `json:"Darussalam"`
	VSelectedK  NewUint     `json:"VSelectedK"`
	VSelectedB  NewUint     `json:"VSelectedB"`
	KitabTitle  interface{} `gorm:"-:all" json:"kitabTitle"`
	BabTitle    interface{} `gorm:"-:all" json:"babTitle"`
	HadithCount interface{} `gorm:"-:all" json:"hadithCount"`
	OtherNumber *string     `gorm:"-:all" json:"otherNumber"`
}

func LoadMainData(kitab, number string) (MainData, error) {
	row := new(MainData)
	err := database.DB.Select(
		"Nomer",
		"Arabic",
		"Indonesia",
		"English",
		"Urdu",
		"Bengali",
		"Albani",
		"Darussalam",
		"VSelectedK",
		"VSelectedB",
	).Table(kitab).Where(
		"Nomer = ?", number,
	).First(row).Error
	if err != nil {
		return MainData{}, err
	}

	dataType := "MAINBOOKSDATA"

	countRowsChan := make(chan error)
	kitabChan := make(chan error)
	babChan := make(chan error)
	noLainChan := make(chan error)

	go countRows([]string{kitab, number}, row, dataType, countRowsChan)
	go getMinimalKitab([]string{kitab, number}, row, dataType, kitabChan)
	go getMinimalBab([]string{kitab, number}, row, dataType, babChan)
	go getNoLain([]string{kitab, number}, row, dataType, noLainChan)

	err = <-countRowsChan
	if err != nil {
		return MainData{}, err
	}
	err = <-kitabChan
	if err != nil {
		return MainData{}, err
	}
	err = <-babChan
	if err != nil {
		return MainData{}, err
	}
	err = <-noLainChan
	if err != nil {
		return MainData{}, err
	}
	return *row, nil
}

type AdminMainData struct {
	Nomer         uint   `gorm:"primaryKey;autoIncrement" json:"Nomer"`
	Arabic        string `json:"Arabic"`
	Gundul        string `json:"Gundul"`
	Indonesia     string `json:"Indonesia"`
	English       string `json:"English"`
	Urdu          string `json:"Urdu"`
	Bengali       string `json:"Bengali"`
	Albani        string `json:"Albani"`
	Darussalam    string `json:"Darussalam"`
	VSelectedK    uint   `json:"VSelectedK"`
	VSelectedB    uint   `json:"VSelectedB"`
	VSelectedKEng uint   `json:"VSelectedKEng"`
	VSelectedBEng uint   `json:"VSelectedBEng"`
}

func AdminGetOne(kitab, number string) (AdminMainData, error) {
	row := new(AdminMainData)
	err := database.DB.Select(
		"Nomer",
		"Arabic",
		"Gundul",
		"Indonesia",
		"English",
		"Urdu",
		"Bengali",
		"Albani",
		"Darussalam",
		"VSelectedK",
		"VSelectedB",
	).Table(kitab).Where(
		"Nomer = ?", number,
	).First(row).Error
	if err != nil {
		return AdminMainData{}, err
	}

	return *row, nil
}

func AdminPutOne(kitab, number string, data AdminMainData) (AdminMainData, error) {
	err := database.DB.Table(kitab).Where("Nomer = ?", number).Updates(data).Error
	if err != nil {
		return AdminMainData{}, err
	}

	return AdminGetOne(kitab, number)
}

func AdminPostOne(kitab string, data AdminMainData) (AdminMainData, error) {
	err := database.DB.Table(kitab).Create(&data).Error
	if err != nil {
		return AdminMainData{}, err
	}

	return data, nil
}

func AdminDeleteOne(kitab, number string) error {
	err := database.DB.Table(kitab).Where("Nomer = ?", number).Delete(nil).Error
	return err
}

func SearchMainData(kitab string, page, limit int, search string) ([]MainData, int64, error) {
	var rows []MainData
	var total int64

	db := database.DB.Table(kitab)

	if search != "" {
		db = db.Where("Indonesia LIKE ? OR Arabic LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	db.Count(&total)

	offset := (page - 1) * limit
	err := db.Select(
		"Nomer",
		"Arabic",
		"Indonesia",
		"English",
		"Urdu",
		"Bengali",
		"Albani",
		"Darussalam",
		"VSelectedK",
		"VSelectedB",
	).Offset(offset).Limit(limit).Find(&rows).Error

	if err != nil {
		return nil, 0, err
	}

	dataType := "MAINBOOKSDATA"

	for i := range rows {
		kitabChan := make(chan error)
		babChan := make(chan error)
		noLainChan := make(chan error)

		go getMinimalKitab([]string{kitab, ""}, &rows[i], dataType, kitabChan)
		go getMinimalBab([]string{kitab, ""}, &rows[i], dataType, babChan)
		go getNoLain([]string{kitab, strconv.Itoa(int(rows[i].Nomer))}, &rows[i], dataType, noLainChan)

		if err := <-kitabChan; err != nil {
			return nil, 0, err
		}
		if err := <-babChan; err != nil {
			return nil, 0, err
		}
		if err := <-noLainChan; err != nil {
			return nil, 0, err
		}
		rows[i].HadithCount = []interface{}{map[string]int64{"count(*)": total}}
	}

	return rows, total, nil
}

func countRows(kitab []string, row *MainData, dataType string, c chan error) {
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
	row.HadithCount = []interface{}{map[string]int64{"count(*)": count}}

	c <- nil
}

func getMinimalKitab(kitab []string, row *MainData, dataType string, c chan error) {
	var kitabTitle KitabTitle
	if row.VSelectedK != 0 {
		err := database.DB.
			Select(
				"NKitab",
				"VMember",
				"Awalan",
			).
			Table("Kitab"+kitab[0]).
			Where(
				"VMember = ?", row.VSelectedK,
			).
			Order("Awalan ASC").
			Limit(1).
			Find(&kitabTitle).
			Error
		if err != nil {
			c <- err
		}
	}

	row.KitabTitle = []interface{}{kitabTitle}
	c <- nil
}

func getMinimalBab(kitab []string, row *MainData, dataType string, c chan error) {
	var babTitle BabTitle
	if row.VSelectedB != 0 {
		err := database.DB.
			Select(
				"NBab",
				"VMemberBab",
				"AwalanBab",
			).
			Table("Bab"+kitab[0]).
			Where(
				"VMemberBab = ?", row.VSelectedB,
			).
			Order("AwalanBab ASC").
			Limit(1).
			Find(&babTitle).
			Error
		if err != nil {
			c <- err
		}
	}
	row.BabTitle = []interface{}{babTitle}
	c <- nil
}

func getNoLain(kitabAndNumber []string, row *MainData, dataType string, c chan error) {
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

	err := database.DB.
		Table(
			"NoLain"+kitabAndNumber[0],
		).
		Where(
			"No = ?", nomer,
		).
		Order("No ASC").
		Limit(1).
		Find(&noLain).
		Error
	if err != nil {
		c <- err
	}

	s := new(strings.Builder)

	s.WriteString(noLain.NoLain1)
	if noLain.NoLain2 != "" {
		s.WriteString(", ")
		s.WriteString(noLain.NoLain2)
	}
	if noLain.NoLain3 != "" {
		s.WriteString(", ")
		s.WriteString(noLain.NoLain3)
	}
	if noLain.NoLain4 != "" {
		s.WriteString(", ")
		s.WriteString(noLain.NoLain4)
	}
	if noLain.NoLain5 != "" {
		s.WriteString(", ")
		s.WriteString(noLain.NoLain5)
	}
	if noLain.NoLain6 != "" {
		s.WriteString(", ")
		s.WriteString(noLain.NoLain6)
	}
	if noLain.NoLain7 != "" {
		s.WriteString(", ")
		s.WriteString(noLain.NoLain7)
	}
	if noLain.NoLain8 != "" {
		s.WriteString(", ")
		s.WriteString(noLain.NoLain8)
	}
	if noLain.NoLain9 != "" {
		s.WriteString(", ")
		s.WriteString(noLain.NoLain9)
	}

	otherNum := strings.TrimSuffix(s.String(), ", ")
	row.OtherNumber = &otherNum
	c <- nil
}
