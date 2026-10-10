package models

import (
	"testing"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupSummaryTestDB(t *testing.T) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:summary_model_test?mode=memory&cache=shared"), &gorm.Config{
		SkipDefaultTransaction: true,
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
			NoLowerCase:   true,
		},
	})
	if err != nil {
		t.Fatalf("failed to open test DB: %v", err)
	}

	database.DB = db

	t.Cleanup(func() {
		sqlDB, _ := database.DB.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
		database.DB = nil
	})
}

func TestSummaryTable(t *testing.T) {
	tests := []struct {
		kitab string
		want  string
	}{
		{kitab: "ShahihBukhari", want: "ShahihBukhariSummary"},
		{kitab: "ShahihMuslim", want: "ShahihMuslimSummary"},
	}

	for _, tt := range tests {
		if got := SummaryTable(tt.kitab); got != tt.want {
			t.Errorf("SummaryTable(%q) = %q, want %q", tt.kitab, got, tt.want)
		}
	}
}

func TestIsSummaryLanguage(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		wantOK bool
	}{
		{name: "exact", input: "Indonesia", want: "Indonesia", wantOK: true},
		{name: "lowercase", input: "english", want: "English", wantOK: true},
		{name: "mixed case and spaces", input: "  uRdU ", want: "Urdu", wantOK: true},
		{name: "bangla", input: "Bengali", want: "Bengali", wantOK: true},
		{name: "unknown", input: "Klingon", want: "", wantOK: false},
		{name: "empty", input: "", want: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := IsSummaryLanguage(tt.input)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("IsSummaryLanguage(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestEnsureSummaryTables(t *testing.T) {
	setupSummaryTestDB(t)

	if err := EnsureSummaryTables(); err != nil {
		t.Fatalf("EnsureSummaryTables() error = %v", err)
	}

	for kitab := range GetIndexOfKitab {
		table := SummaryTable(kitab)
		if !database.DB.Migrator().HasTable(table) {
			t.Errorf("summary table %q should exist", table)
		}
	}

	// Idempotent: running again must not error.
	if err := EnsureSummaryTables(); err != nil {
		t.Errorf("second EnsureSummaryTables() error = %v", err)
	}
}

func TestSaveAndGetHadithSummary(t *testing.T) {
	setupSummaryTestDB(t)
	if err := EnsureSummaryTables(); err != nil {
		t.Fatalf("EnsureSummaryTables() error = %v", err)
	}

	if err := SaveHadithSummary("ShahihBukhari", 1, "Indonesia", "ringkasan indonesia"); err != nil {
		t.Fatalf("SaveHadithSummary(Indonesia) error = %v", err)
	}
	if err := SaveHadithSummary("ShahihBukhari", 1, "English", "english summary"); err != nil {
		t.Fatalf("SaveHadithSummary(English) error = %v", err)
	}

	got, ok, err := GetHadithSummary("ShahihBukhari", "1", "Indonesia")
	if err != nil || !ok || got != "ringkasan indonesia" {
		t.Errorf("GetHadithSummary(Indonesia) = (%q, %v, %v), want ('ringkasan indonesia', true, nil)", got, ok, err)
	}

	got, ok, err = GetHadithSummary("ShahihBukhari", "1", "English")
	if err != nil || !ok || got != "english summary" {
		t.Errorf("GetHadithSummary(English) = (%q, %v, %v), want ('english summary', true, nil)", got, ok, err)
	}

	// Upsert must keep a single row and not clobber the other language column.
	var count int64
	database.DB.Table(SummaryTable("ShahihBukhari")).Where("Nomer = ?", 1).Count(&count)
	if count != 1 {
		t.Errorf("row count = %d, want 1 (upsert)", count)
	}

	if err := SaveHadithSummary("ShahihBukhari", 1, "Indonesia", "ringkasan baru"); err != nil {
		t.Fatalf("SaveHadithSummary upsert error = %v", err)
	}
	got, _, _ = GetHadithSummary("ShahihBukhari", "1", "Indonesia")
	if got != "ringkasan baru" {
		t.Errorf("Indonesia summary = %q, want 'ringkasan baru'", got)
	}
	got, _, _ = GetHadithSummary("ShahihBukhari", "1", "English")
	if got != "english summary" {
		t.Errorf("English summary = %q, want 'english summary' (untouched by upsert)", got)
	}
}

func TestGetHadithSummary_Missing(t *testing.T) {
	setupSummaryTestDB(t)
	if err := EnsureSummaryTables(); err != nil {
		t.Fatalf("EnsureSummaryTables() error = %v", err)
	}

	got, ok, err := GetHadithSummary("ShahihBukhari", "999", "Indonesia")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok || got != "" {
		t.Errorf("GetHadithSummary(missing) = (%q, %v), want ('', false)", got, ok)
	}
}

func TestGetHadithSummary_UnknownLanguage(t *testing.T) {
	setupSummaryTestDB(t)
	if err := EnsureSummaryTables(); err != nil {
		t.Fatalf("EnsureSummaryTables() error = %v", err)
	}

	if _, _, err := GetHadithSummary("ShahihBukhari", "1", "Klingon"); err == nil {
		t.Error("expected error for unknown language, got nil")
	}
}

func TestSaveHadithSummary_UnknownLanguage(t *testing.T) {
	setupSummaryTestDB(t)

	if err := SaveHadithSummary("ShahihBukhari", 1, "Klingon", "x"); err == nil {
		t.Error("expected error for unknown language, got nil")
	}
}
