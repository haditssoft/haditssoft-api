package hadithdata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupHDRDataDB(t *testing.T) {
	t.Helper()
	hdrTestDBCounter++
	dbName := fmt.Sprintf("file:test_hdr_data_%d?mode=memory&cache=shared", hdrTestDBCounter)
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
			NoLowerCase:   true,
		},
	})
	if err != nil {
		t.Fatalf("failed to open test DB: %v", err)
	}
	database.DB = db
	t.Cleanup(func() { database.DB = nil })
}

func createHDRKitabSchema(t *testing.T) {
	t.Helper()
	statements := []string{
		`CREATE TABLE "ShahihBukhari" (Nomer INTEGER PRIMARY KEY, Arabic TEXT, Indonesia TEXT, English TEXT, Urdu TEXT, Bengali TEXT, Albani TEXT, Darussalam TEXT, VSelectedK INTEGER, VSelectedB INTEGER)`,
		`CREATE TABLE "KitabShahihBukhari" (NKitab TEXT, VMember INTEGER, Awalan TEXT)`,
		`CREATE TABLE "BabShahihBukhari" (NBab TEXT, VMemberBab INTEGER, AwalanBab TEXT)`,
		`CREATE TABLE "NoLainShahihBukhari" (No INTEGER, NoLain1 TEXT, NoLain2 TEXT, NoLain3 TEXT, NoLain4 TEXT, NoLain5 TEXT, NoLain6 TEXT, NoLain7 TEXT, NoLain8 TEXT, NoLain9 TEXT)`,
		`CREATE TABLE "Tema" (NoHdt INTEGER, No INTEGER)`,
	}
	for _, s := range statements {
		if err := database.DB.Exec(s).Error; err != nil {
			t.Fatalf("failed to run %q: %v", s, err)
		}
	}
}

func seedHDRFullRow(t *testing.T) {
	t.Helper()
	rows := [][]interface{}{
		{1, "صَلُّوا كَمَا رَأَيْتُمُونِي أُصَلِّي", "Sholatlah sebagaimana kalian melihat aku sholat", "Pray as you see me pray", "Urdu translation", "Bengali translation", "Shahih", "Sahih", 5, 10},
	}
	for _, r := range rows {
		if err := database.DB.Exec(
			`INSERT INTO "ShahihBukhari" (Nomer, Arabic, Indonesia, English, Urdu, Bengali, Albani, Darussalam, VSelectedK, VSelectedB) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r...,
		).Error; err != nil {
			t.Fatalf("failed to seed hadith: %v", err)
		}
	}
	if err := database.DB.Exec(`INSERT INTO "KitabShahihBukhari" (NKitab, VMember, Awalan) VALUES ('Kitab Iman', 5, '1')`).Error; err != nil {
		t.Fatalf("failed to seed kitab: %v", err)
	}
	if err := database.DB.Exec(`INSERT INTO "BabShahihBukhari" (NBab, VMemberBab, AwalanBab) VALUES ('Bab Pertama', 10, '1')`).Error; err != nil {
		t.Fatalf("failed to seed bab: %v", err)
	}
	if err := database.DB.Exec(`INSERT INTO "NoLainShahihBukhari" (No, NoLain1, NoLain2) VALUES (1, '7561', '243')`).Error; err != nil {
		t.Fatalf("failed to seed no lain: %v", err)
	}
	if err := database.DB.Exec(`INSERT INTO "Tema" (NoHdt, No) VALUES (1, 1)`).Error; err != nil {
		t.Fatalf("failed to seed tema: %v", err)
	}
}

func setupHDRDataApp(t *testing.T) *fiber.App {
	t.Helper()
	setupHDRDataDB(t)
	createHDRKitabSchema(t)
	seedHDRFullRow(t)
	app := fiber.New()
	handler := NewHandler()
	RegisterRoutes(app, handler)
	return app
}

func decodeHDRArray(t *testing.T, resp *http.Response) []interface{} {
	t.Helper()
	defer resp.Body.Close()
	var result []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return result
}

func firstRow(t *testing.T, arr []interface{}) map[string]interface{} {
	t.Helper()
	rows, ok := arr[0].([]interface{})
	if !ok || len(rows) == 0 {
		t.Fatal("expected first element to be a non-empty rows array")
	}
	row, ok := rows[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected row to be an object")
	}
	return row
}

// ============================================================
// GET /loadMainData tests
// ============================================================

func TestMainData_ReturnsAllTranslations(t *testing.T) {
	app := setupHDRDataApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadMainData/ShahihBukhari/1")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	arr := decodeHDRArray(t, resp)
	if len(arr) != 2 || arr[1] != "MAINBOOKSDATA" {
		t.Fatalf("unexpected response shape: %v", arr)
	}

	row := firstRow(t, arr)
	if row["English"] != "Pray as you see me pray" {
		t.Errorf("English = %v, want 'Pray as you see me pray'", row["English"])
	}
	if row["Urdu"] != "Urdu translation" {
		t.Errorf("Urdu = %v, want 'Urdu translation'", row["Urdu"])
	}
	if row["Bengali"] != "Bengali translation" {
		t.Errorf("Bengali = %v, want 'Bengali translation'", row["Bengali"])
	}
	if row["Arabic"] == nil {
		t.Error("Arabic should be present")
	}
	if row["Indonesia"] != "Sholatlah sebagaimana kalian melihat aku sholat" {
		t.Errorf("Indonesia = %v", row["Indonesia"])
	}
}

func TestMainData_NullTranslationsDoNotBreakResponse(t *testing.T) {
	app := setupHDRDataApp(t)
	if err := database.DB.Exec(
		`INSERT INTO "ShahihBukhari" (Nomer, Arabic, Indonesia, English, Urdu, Bengali, Albani, Darussalam, VSelectedK, VSelectedB) VALUES (2, 'arabic 2', 'indonesia 2', NULL, NULL, NULL, NULL, NULL, 0, 0)`,
	).Error; err != nil {
		t.Fatalf("failed to seed null row: %v", err)
	}

	resp := makeHDRRequest(t, app, "GET", "/loadMainData/ShahihBukhari/2")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	arr := decodeHDRArray(t, resp)
	row := firstRow(t, arr)
	if row["English"] != nil {
		t.Errorf("English should be null, got %v", row["English"])
	}
	if row["Urdu"] != nil {
		t.Errorf("Urdu should be null, got %v", row["Urdu"])
	}
	if row["Bengali"] != nil {
		t.Errorf("Bengali should be null, got %v", row["Bengali"])
	}
	if row["Indonesia"] != "indonesia 2" {
		t.Errorf("Indonesia = %v, want 'indonesia 2'", row["Indonesia"])
	}
}

func TestMainData_MissingRowHandled(t *testing.T) {
	app := setupHDRDataApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadMainData/ShahihBukhari/999")
	if resp.StatusCode == http.StatusOK {
		t.Error("missing row should not return 200")
	}
}

// ============================================================
// GET /classificationData tests
// ============================================================

func TestClassificationData_ReturnsAllTranslations(t *testing.T) {
	app := setupHDRDataApp(t)
	resp := makeHDRRequest(t, app, "GET", "/classificationData/ShahihBukhari/1/Tema")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	arr := decodeHDRArray(t, resp)
	if len(arr) != 3 || arr[1] != "CLASSIFICATIONDATA" {
		t.Fatalf("unexpected response shape: %v", arr)
	}

	row := firstRow(t, arr)
	if row["English"] != "Pray as you see me pray" {
		t.Errorf("English = %v, want 'Pray as you see me pray'", row["English"])
	}
	if row["Urdu"] != "Urdu translation" {
		t.Errorf("Urdu = %v, want 'Urdu translation'", row["Urdu"])
	}
	if row["Bengali"] != "Bengali translation" {
		t.Errorf("Bengali = %v, want 'Bengali translation'", row["Bengali"])
	}
}

func TestClassificationData_NoMatchReturnsError(t *testing.T) {
	app := setupHDRDataApp(t)
	resp := makeHDRRequest(t, app, "GET", "/classificationData/ShahihBukhari/999/Tema")
	if resp.StatusCode == http.StatusOK {
		t.Error("no-match classification should not return 200")
	}
}

// ============================================================
// GET /loadCustomData tests
// ============================================================

func TestCustomData_ReturnsAllTranslations(t *testing.T) {
	app := setupHDRDataApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadCustomData/ShahihBukhari/1/position/actionId")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	arr := decodeHDRArray(t, resp)
	row := firstRow(t, arr)
	if row["English"] != "Pray as you see me pray" {
		t.Errorf("English = %v, want 'Pray as you see me pray'", row["English"])
	}
	if row["Urdu"] != "Urdu translation" {
		t.Errorf("Urdu = %v, want 'Urdu translation'", row["Urdu"])
	}
	if row["Bengali"] != "Bengali translation" {
		t.Errorf("Bengali = %v, want 'Bengali translation'", row["Bengali"])
	}
}

func TestCustomData_MissingRowHandled(t *testing.T) {
	app := setupHDRDataApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadCustomData/ShahihBukhari/999/position/actionId")
	if resp.StatusCode == http.StatusOK {
		t.Error("missing row should not return 200")
	}
}
