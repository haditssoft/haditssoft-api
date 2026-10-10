package models

import (
	"fmt"
	"strings"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"

	"gorm.io/gorm/clause"
)

// SummaryLanguages is the ordered set of per-language summary columns stored in
// every <kitabName>Summary table. The values match the translation language
// column names on the hadith tables.
var SummaryLanguages = []string{"Indonesia", "English", "Urdu", "Bengali"}

// HadithSummary mirrors one row of a <kitabName>Summary table. Each language is
// a separate column (named after the language, without a redundant "Summary"
// suffix since the table name already says so).
type HadithSummary struct {
	Nomer     uint    `json:"Nomer"`
	Indonesia *string `json:"Indonesia"`
	English   *string `json:"English"`
	Urdu      *string `json:"Urdu"`
	Bengali   *string `json:"Bengali"`
}

// SummaryTable returns the dedicated summary table name for a kitab, e.g.
// ShahihBukhari -> ShahihBukhariSummary.
func SummaryTable(kitab string) string {
	return kitab + "Summary"
}

// IsSummaryLanguage reports whether language matches one of SummaryLanguages
// (case-insensitive) and returns its canonical column name.
func IsSummaryLanguage(language string) (string, bool) {
	for _, lang := range SummaryLanguages {
		if strings.EqualFold(lang, strings.TrimSpace(language)) {
			return lang, true
		}
	}
	return "", false
}

// EnsureSummaryTables creates the <kitabName>Summary table for every kitab in
// GetIndexOfKitab if it does not exist yet. It is idempotent and safe to call
// on every startup.
func EnsureSummaryTables() error {
	for kitab := range GetIndexOfKitab {
		if err := ensureSummaryTable(SummaryTable(kitab)); err != nil {
			return err
		}
	}
	return nil
}

func ensureSummaryTable(table string) error {
	stmt := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS "%s" (`+
			`"Nomer" integer NOT NULL PRIMARY KEY, `+
			`"Indonesia" text, `+
			`"English" text, `+
			`"Urdu" text, `+
			`"Bengali" text`+
			`)`,
		table,
	)
	return database.DB.Exec(stmt).Error
}

// GetHadithSummary returns the stored summary for the given kitab, hadith number
// and language. ok is false when no row exists or the summary column is empty.
func GetHadithSummary(kitab, nomer, language string) (summary string, ok bool, err error) {
	column, valid := IsSummaryLanguage(language)
	if !valid {
		return "", false, fmt.Errorf("unknown summary language: %s", language)
	}

	var dest struct {
		Value *string
	}
	if err := database.DB.
		Table(SummaryTable(kitab)).
		Select(column+" AS Value").
		Where("Nomer = ?", nomer).
		Limit(1).
		Find(&dest).Error; err != nil {
		return "", false, err
	}

	if dest.Value == nil || strings.TrimSpace(*dest.Value) == "" {
		return "", false, nil
	}
	return *dest.Value, true, nil
}

// SaveHadithSummary upserts one language summary for a hadith without touching
// the other language columns of the same row.
func SaveHadithSummary(kitab string, nomer uint, language, summary string) error {
	column, valid := IsSummaryLanguage(language)
	if !valid {
		return fmt.Errorf("unknown summary language: %s", language)
	}

	return database.DB.
		Table(SummaryTable(kitab)).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "Nomer"}},
			DoUpdates: clause.AssignmentColumns([]string{column}),
		}).
		Create(map[string]interface{}{
			"Nomer": nomer,
			column:  summary,
		}).Error
}
