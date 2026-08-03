package hadithadmin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/haditssoft/haditssoft-backend/internal/shared/auth"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database/entities"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:test_hadithadmin?mode=memory&cache=shared"), &gorm.Config{
		SkipDefaultTransaction:                   true,
		PrepareStmt:                              true,
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

	if err := db.AutoMigrate(&entities.User{}, &entities.BlacklistToken{}); err != nil {
		t.Fatalf("failed to auto-migrate auth tables: %v", err)
	}

	if err := db.Exec(`CREATE TABLE "ShahihBukhari" (
		Nomer INTEGER PRIMARY KEY,
		Arabic TEXT,
		Gundul TEXT,
		Indonesia TEXT,
		English TEXT,
		Urdu TEXT,
		Bengali TEXT,
		Albani TEXT,
		Darussalam TEXT,
		VSelectedK INTEGER DEFAULT 0,
		VSelectedB INTEGER DEFAULT 0,
		VSelectedKEng INTEGER DEFAULT 0,
		VSelectedBEng INTEGER DEFAULT 0
	)`).Error; err != nil {
		t.Fatalf("failed to create ShahihBukhari table: %v", err)
	}

	type hadith struct {
		Nomer     int
		Arabic    string
		Gundul    string
		Indonesia string
		English   string
	}
	seed := []hadith{
		{1, "صَلُّوا كَمَا رَأَيْتُمُونِي أُصَلِّي", "Sholatlah kamu", "Sholatlah kamu sebagaimana kamu melihat aku sholat", "Pray as you see me pray"},
		{2, "بَيْنَمَا نَحْنُ", "Tentang sholat", "Tentang sholat lima waktu yang wajib", "About the five daily prayers"},
		{3, "خَيْرُكُمْ", "Hadits puasa", "Hadits tentang puasa ramadhan yang utama", "The best of you"},
	}
	for _, h := range seed {
		db.Exec(`INSERT INTO "ShahihBukhari" (Nomer, Arabic, Gundul, Indonesia, English) VALUES (?, ?, ?, ?, ?)`,
			h.Nomer, h.Arabic, h.Gundul, h.Indonesia, h.English)
	}

	db.Exec(`CREATE TABLE "NoLainShahihBukhari" (No TEXT, NoLain1 TEXT, NoLain2 TEXT, NoLain3 TEXT, NoLain4 TEXT, NoLain5 TEXT, NoLain6 TEXT, NoLain7 TEXT, NoLain8 TEXT, NoLain9 TEXT)`)
	for i := 1; i <= 3; i++ {
		db.Exec(`INSERT INTO "NoLainShahihBukhari" (No, NoLain1) VALUES (?, ?)`,
			strconv.Itoa(i), strconv.Itoa(i))
	}

	boolTrue := true
	admin := entities.User{
		ID:     1,
		Email:  "admin@example.com",
		Active: &boolTrue,
		Admin:  &boolTrue,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("failed to seed admin user: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, _ := database.DB.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
		database.DB = nil
	})

	return db
}

func setupTestApp(t *testing.T) *fiber.App {
	t.Helper()
	setupTestDB(t)

	os.Setenv("JWT_SECRET", "test-secret-key-for-hadithadmin")
	t.Cleanup(func() { os.Unsetenv("JWT_SECRET") })

	app := fiber.New()
	RegisterRoutes(app)
	return app
}

func makeRequest(t *testing.T, app *fiber.App, method, path string, body interface{}, token string) *http.Response {
	t.Helper()

	var reqBody *bytes.Buffer
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewBuffer(jsonBytes)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	return resp
}

func decodeJSON(t *testing.T, resp *http.Response, dest interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

func getAdminToken(t *testing.T) string {
	t.Helper()
	token, err := auth.GenerateAccessToken(1, "admin@example.com")
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}
	return token
}

// ============================================================
// GET /:kitabName (list)
// ============================================================

func TestGetList_NoAuth(t *testing.T) {
	app := setupTestApp(t)

	resp := makeRequest(t, app, "GET", "/ShahihBukhari", nil, "")

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestGetList_Success(t *testing.T) {
	app := setupTestApp(t)
	token := getAdminToken(t)

	resp := makeRequest(t, app, "GET", "/ShahihBukhari", nil, token)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	data, ok := result["data"].([]interface{})
	if !ok {
		t.Fatal("response should have 'data' array")
	}

	if len(data) != 3 {
		t.Errorf("expected 3 records, got %d", len(data))
	}

	total := int(result["total"].(float64))
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
}

func TestGetList_WithSearch(t *testing.T) {
	app := setupTestApp(t)
	token := getAdminToken(t)

	resp := makeRequest(t, app, "GET", "/ShahihBukhari?search=puasa", nil, token)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	total := int(result["total"].(float64))
	if total != 1 {
		t.Errorf("total = %d, want 1 for search 'puasa'", total)
	}
}

func TestGetList_Pagination(t *testing.T) {
	app := setupTestApp(t)
	token := getAdminToken(t)

	resp := makeRequest(t, app, "GET", "/ShahihBukhari?page=1&limit=2", nil, token)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	data := result["data"].([]interface{})
	if len(data) != 2 {
		t.Errorf("expected 2 records with limit=2, got %d", len(data))
	}

	limit := int(result["limit"].(float64))
	if limit != 2 {
		t.Errorf("limit = %d, want 2", limit)
	}
}

// ============================================================
// GET /:kitabName/:number (get one)
// ============================================================

func TestGetOne_Success(t *testing.T) {
	app := setupTestApp(t)
	token := getAdminToken(t)

	resp := makeRequest(t, app, "GET", "/ShahihBukhari/1", nil, token)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if result["Nomer"].(float64) != 1 {
		t.Errorf("Nomer = %v, want 1", result["Nomer"])
	}

	if result["Indonesia"] != "Sholatlah kamu sebagaimana kamu melihat aku sholat" {
		t.Errorf("Indonesia = %v, wrong content", result["Indonesia"])
	}
}

func TestGetOne_NotFound(t *testing.T) {
	app := setupTestApp(t)
	token := getAdminToken(t)

	resp := makeRequest(t, app, "GET", "/ShahihBukhari/999", nil, token)

	// GORM record not found error propagated as 500 by Fiber default handler
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d or %d", resp.StatusCode, http.StatusNotFound, http.StatusInternalServerError)
	}
}

// ============================================================
// POST /:kitabName (create)
// ============================================================

func TestPostOne_Success(t *testing.T) {
	app := setupTestApp(t)
	token := getAdminToken(t)

	body := map[string]interface{}{
		"Nomer":     10,
		"Arabic":    "بِسْمِ اللَّهِ",
		"Gundul":    "Bismillah",
		"Indonesia": "Dengan nama Allah",
		"English":   "In the name of Allah",
	}

	resp := makeRequest(t, app, "POST", "/ShahihBukhari", body, token)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if result["Nomer"].(float64) != 10 {
		t.Errorf("Nomer = %v, want 10", result["Nomer"])
	}

	if result["Indonesia"] != "Dengan nama Allah" {
		t.Errorf("Indonesia = %v, wrong content", result["Indonesia"])
	}
}

func TestPostOne_InvalidJSON(t *testing.T) {
	app := setupTestApp(t)
	token := getAdminToken(t)

	req := httptest.NewRequest("POST", "/ShahihBukhari", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d (body parse error)", resp.StatusCode, http.StatusInternalServerError)
	}
}

// ============================================================
// PUT /:kitabName/:number (update)
// ============================================================

func TestPutOne_Success(t *testing.T) {
	app := setupTestApp(t)
	token := getAdminToken(t)

	body := map[string]interface{}{
		"Indonesia": "Sholat wajib lima waktu",
	}

	resp := makeRequest(t, app, "PUT", "/ShahihBukhari/2", body, token)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if result["Indonesia"] != "Sholat wajib lima waktu" {
		t.Errorf("Indonesia = %v, want 'Sholat wajib lima waktu'", result["Indonesia"])
	}

	if result["Nomer"].(float64) != 2 {
		t.Errorf("Nomer = %v, want 2", result["Nomer"])
	}
}

func TestPutOne_NotFound(t *testing.T) {
	app := setupTestApp(t)
	token := getAdminToken(t)

	body := map[string]interface{}{
		"Indonesia": "test",
	}

	resp := makeRequest(t, app, "PUT", "/ShahihBukhari/999", body, token)

	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d or %d", resp.StatusCode, http.StatusNotFound, http.StatusInternalServerError)
	}
}

// ============================================================
// DELETE /:kitabName/:number (delete)
// ============================================================

func TestDeleteOne_Success(t *testing.T) {
	app := setupTestApp(t)
	token := getAdminToken(t)

	resp := makeRequest(t, app, "DELETE", "/ShahihBukhari/3", nil, token)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if result["message"] != "Record deleted successfully" {
		t.Errorf("message = %v, want 'Record deleted successfully'", result["message"])
	}
}

func TestDeleteOne_VerifyGone(t *testing.T) {
	app := setupTestApp(t)
	token := getAdminToken(t)

	// Delete record 3
	resp := makeRequest(t, app, "DELETE", "/ShahihBukhari/3", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete failed: %d", resp.StatusCode)
	}

	// Try to get the deleted record
	resp2 := makeRequest(t, app, "GET", "/ShahihBukhari/3", nil, token)
	if resp2.StatusCode != http.StatusNotFound && resp2.StatusCode != http.StatusInternalServerError {
		t.Errorf("after delete, GET status = %d, want %d or %d", resp2.StatusCode, http.StatusNotFound, http.StatusInternalServerError)
	}
}

// ============================================================
// Method not allowed tests
// ============================================================

func TestRoutes_WrongMethods(t *testing.T) {
	app := setupTestApp(t)
	token := getAdminToken(t)

	tests := []struct {
		method string
		path   string
	}{
		{"POST", "/ShahihBukhari/1"},
		{"PUT", "/ShahihBukhari"},
		{"DELETE", "/ShahihBukhari"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			resp := makeRequest(t, app, tt.method, tt.path, map[string]interface{}{}, token)
			if resp.StatusCode == http.StatusOK {
				t.Errorf("%s %s should not return 200", tt.method, tt.path)
			}
		})
	}
}
