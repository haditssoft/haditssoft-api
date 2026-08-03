package models

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
)

// tabel DaftarRawi
type DaftarRawi struct {
	KodeRawi    uint   `json:"KodeRawi"`
	Nama        string `json:"Nama"`
	Quality     uint   `json:"Quality"`
	Kalangan    string `json:"Kalangan"`
	Nasab       string `json:"Nasab"`
	Kuniyah     string `json:"Kuniyah"`
	Laqob       string `json:"Laqob"`
	NegeriHidup string `json:"NegeriHidup"`
	NegeriWafat string `json:"NegeriWafat"`
	TahunWafat  string `json:"TahunWafat"`
	RBukhari    uint   `json:"RBukhari"`
	RMuslim     uint   `json:"RMuslim"`
	RAbuDaud    uint   `json:"RAbuDaud"`
	RTirmidzi   uint   `json:"RTirmidzi"`
	RNasai      uint   `json:"RNasai"`
	RIbnuMajah  uint   `json:"RIbnuMajah"`
	RAhmad      uint   `json:"RAhmad"`
	RMalik      uint   `json:"RMalik"`
	RDarimi     uint   `json:"RDarimi"`
}

func GetNarratorCompleteProfile(narratorId string) (*DaftarRawi, error) {

	result := new(DaftarRawi)

	if err := database.DB.Table(
		"DaftarRawi",
	).Where(
		"KodeRawi = ?", narratorId,
	).Limit(1).Find(result).Error; err != nil {
		return nil, err
	}

	return result, nil
}
