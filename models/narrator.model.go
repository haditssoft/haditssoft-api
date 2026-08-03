package models

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"strings"
)

// contoh tabel BandingShahihBukhari, BandingSunanTirmidzi, dll
type Narrator struct {
	// NoUrut    uint   `json:"NoUrut"`
	// NoHdt     string `json:"NoHdt"`
	KodeRawi string `json:"KodeRawi"`
	Nama     string `json:"Nama"`
	Quality  string `json:"Quality"`
}

func GetNarrators(nameKunyahClassLevel []string) ([]Narrator, error) {
	var qwery string

	// for (let idx = 0; idx < nameKunyahClassLevel.length; idx++) {
	for idx, value := range nameKunyahClassLevel {

		// const value = nameKunyahClassLevel[idx];
		// nameKunyahClassLevel contains: [NamaRawi, KunyahRawi, KalanganRawi, LevelRawi]
		if value != "" { // only if not empty
			if qwery != "" {
				qwery = qwery + " And "
			}
			switch idx {
			case 0:
				qwery = qwery + "Nama Like '%" + strings.Replace(value, "'", "''", -1) + "%'"
			case 1:
				qwery = qwery + "Kuniyah Like '%" + strings.Replace(value, "'", "''", -1) + "%'"
			case 2:
				qwery = qwery + "Kalangan Like '%" + strings.Replace(value, "'", "''", -1) + "%'"
			case 3:
				// The value of level is something like '1. ' or '2. ', etc.
				// it then become 1 (number) by adding + (plus) in front of it
				qwery = qwery + "Quality = " + value
			default:
			}
		}
	}
	// }

	// return knex
	//     .select('KodeRawi', 'Nama', 'Quality')
	//     .from('DaftarRawi')
	//     .whereRaw(qwery)
	//     .orderBy('KodeRawi');

	var result []Narrator

	rows, err := database.DB.Raw(
		"SELECT KodeRawi, Nama, Quality FROM DaftarRawi WHERE " + qwery + " ORDER BY KodeRawi ASC",
	).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var narrator Narrator
		database.DB.ScanRows(rows, &narrator)
		result = append(result, narrator)
	}

	return result, nil
}
