package font

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/haditssoft/haditssoft-backend/internal/shared/auth"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/internal/shared/validator"
	"github.com/haditssoft/haditssoft-backend/models"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestMain(m *testing.M) {
	os.Setenv("JWT_SECRET", "test-secret-for-font-unit-tests")
	validator.RegisterCustomValidations()
	os.Exit(m.Run())
}

var fontTestDBCounter int

func setupFontTestDB(t *testing.T) {
	t.Helper()
	fontTestDBCounter++
	dbName := fmt.Sprintf("file:test_font_%d?mode=memory&cache=shared", fontTestDBCounter)
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
	if err := db.AutoMigrate(&models.User{}, &models.Font{}, &models.Activity{}, &models.BlacklistToken{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	database.DB = db
	t.Cleanup(func() { database.DB = nil })
}

func setupFontTestApp(t *testing.T) *fiber.App {
	t.Helper()
	setupFontTestDB(t)
	repo := NewRepository()
	svc := NewService(repo)
	h := NewHandler(svc)
	app := fiber.New()
	RegisterRoutes(app, h)
	return app
}

func seedFontUser(t *testing.T, email string) models.User {
	t.Helper()
	user := models.User{Email: email, Password: "hashedpassword", Active: boolPtr(true), Admin: boolPtr(false)}
	if err := database.DB.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return user
}

func boolPtr(b bool) *bool { return &b }

func generateFontToken(t *testing.T, userID uint, email string) string {
	t.Helper()
	token, err := auth.GenerateAccessToken(userID, email)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return token
}

func makeFontRequest(t *testing.T, app *fiber.App, method, path string, body interface{}, token string) *http.Response {
	t.Helper()
	var reqBody *bytes.Buffer
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal body: %v", err)
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

func decodeFontJSON(t *testing.T, resp *http.Response, dest interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

// ============================================================
// GET /fonts (GetOne) tests
// ============================================================

func TestFontGetOne_NoSettings(t *testing.T) {
	app := setupFontTestApp(t)
	user := seedFontUser(t, "fontnone@example.com")
	token := generateFontToken(t, user.ID, user.Email)

	resp := makeFontRequest(t, app, "GET", "/fonts", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeFontJSON(t, resp, &result)
	if result["arabic"] == nil {
		t.Error("arabic should not be nil")
	}
	if result["translation"] == nil {
		t.Error("translation should not be nil")
	}
}

func TestFontGetOne_WithSettings(t *testing.T) {
	app := setupFontTestApp(t)
	user := seedFontUser(t, "fontyes@example.com")
	database.DB.Create(&models.Font{
		UserID:              user.ID,
		ArabicFamily:        "Amiri",
		ArabicFallback:      "serif",
		ArabicWeight:        700,
		ArabicSize:          24,
		TranslationFamily:   "Lato",
		TranslationFallback: "sans-serif",
		TranslationWeight:   400,
		TranslationSize:     16,
	})
	token := generateFontToken(t, user.ID, user.Email)

	resp := makeFontRequest(t, app, "GET", "/fonts", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]json.RawMessage
	decodeFontJSON(t, resp, &result)

	var arabic []interface{}
	json.Unmarshal(result["arabic"], &arabic)
	if arabic[0] != "Amiri" {
		t.Errorf("arabic family = %v, want 'Amiri'", arabic[0])
	}
	if int(arabic[2].(float64)) != 700 {
		t.Errorf("arabic weight = %v, want 700", arabic[2])
	}
}

func TestFontGetOne_NoJWT(t *testing.T) {
	app := setupFontTestApp(t)
	resp := makeFontRequest(t, app, "GET", "/fonts", nil, "")
	if resp.StatusCode == http.StatusOK {
		t.Error("request without JWT should not return 200")
	}
}

func TestFontGetOne_InvalidJWT(t *testing.T) {
	app := setupFontTestApp(t)
	resp := makeFontRequest(t, app, "GET", "/fonts", nil, "bad.token")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestFontGetOne_OtherUsersSettings_Empty(t *testing.T) {
	app := setupFontTestApp(t)
	userA := seedFontUser(t, "font_own_a@example.com")
	userB := seedFontUser(t, "font_own_b@example.com")
	database.DB.Create(&models.Font{
		UserID: userA.ID, ArabicFamily: "Amiri", ArabicFallback: "serif",
		ArabicWeight: 700, ArabicSize: 24,
		TranslationFamily: "Lato", TranslationFallback: "sans-serif",
		TranslationWeight: 400, TranslationSize: 16,
	})
	tokenB := generateFontToken(t, userB.ID, userB.Email)

	resp := makeFontRequest(t, app, "GET", "/fonts", nil, tokenB)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]json.RawMessage
	decodeFontJSON(t, resp, &result)

	var arabic []interface{}
	json.Unmarshal(result["arabic"], &arabic)
	if len(arabic) != 4 {
		t.Fatalf("arabic array length = %d, want 4", len(arabic))
	}
	if arabic[0] == "Amiri" {
		t.Error("user B saw user A's font family")
	}
	if arabic[0] != "" {
		t.Errorf("user B arabic family = %v, want empty (own settings only)", arabic[0])
	}
}

// ============================================================
// PUT /fonts (Update) tests
// ============================================================

func TestFontUpdate_CreateNew(t *testing.T) {
	app := setupFontTestApp(t)
	user := seedFontUser(t, "fontupd@example.com")
	token := generateFontToken(t, user.ID, user.Email)

	body := map[string]interface{}{
		"arabic":      []interface{}{"Amiri", "serif", 700, 24},
		"translation": []interface{}{"Lato", "sans-serif", 400, 16},
	}
	resp := makeFontRequest(t, app, "PUT", "/fonts", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	var count int
	database.DB.Raw("SELECT COUNT(*) FROM Font WHERE user_id = ?", user.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 font record, got %d", count)
	}
}

func TestFontUpdate_ExistingArabicOnly(t *testing.T) {
	app := setupFontTestApp(t)
	user := seedFontUser(t, "fontarab@example.com")
	database.DB.Create(&models.Font{
		UserID: user.ID, ArabicFamily: "Old", ArabicFallback: "serif",
		ArabicWeight: 400, ArabicSize: 18,
		TranslationFamily: "Trans", TranslationFallback: "sans",
		TranslationWeight: 400, TranslationSize: 14,
	})
	token := generateFontToken(t, user.ID, user.Email)

	body := map[string]interface{}{
		"arabic": []interface{}{"NewAmiri", "serif", 700, 28},
	}
	resp := makeFontRequest(t, app, "PUT", "/fonts", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	var font models.Font
	database.DB.Where("user_id = ?", user.ID).First(&font)
	if font.ArabicFamily != "NewAmiri" {
		t.Errorf("arabic_family = %q, want 'NewAmiri'", font.ArabicFamily)
	}
	if font.TranslationFamily != "Trans" {
		t.Errorf("translation_family should be unchanged, got %q", font.TranslationFamily)
	}
}

func TestFontUpdate_CreatesActivity(t *testing.T) {
	app := setupFontTestApp(t)
	user := seedFontUser(t, "fontact@example.com")
	token := generateFontToken(t, user.ID, user.Email)

	body := map[string]interface{}{
		"arabic":      []interface{}{"Amiri", "serif", 700, 24},
		"translation": []interface{}{"Lato", "sans-serif", 400, 16},
	}
	resp := makeFontRequest(t, app, "PUT", "/fonts", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("update failed: status %d", resp.StatusCode)
	}

	var count int
	database.DB.Raw("SELECT COUNT(*) FROM Activity WHERE UserID = ? AND Note = 'Create font settings'", user.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 activity, got %d", count)
	}
}

func TestFontUpdate_NoJWT(t *testing.T) {
	app := setupFontTestApp(t)
	body := map[string]interface{}{
		"arabic":      []interface{}{"Amiri", "serif", 700, 24},
		"translation": []interface{}{"Lato", "sans-serif", 400, 16},
	}
	resp := makeFontRequest(t, app, "PUT", "/fonts", body, "")
	if resp.StatusCode == http.StatusNoContent {
		t.Error("request without JWT should not return 204")
	}
}

func TestFontUpdate_DoesNotOverwriteOtherUsersRow(t *testing.T) {
	app := setupFontTestApp(t)
	userA := seedFontUser(t, "font_upd_a@example.com")
	userB := seedFontUser(t, "font_upd_b@example.com")
	database.DB.Create(&models.Font{
		UserID: userA.ID, ArabicFamily: "Amiri", ArabicFallback: "serif",
		ArabicWeight: 700, ArabicSize: 24,
		TranslationFamily: "Lato", TranslationFallback: "sans-serif",
		TranslationWeight: 400, TranslationSize: 16,
	})
	database.DB.Create(&models.Font{
		UserID: userB.ID, ArabicFamily: "OldB", ArabicFallback: "serif",
		ArabicWeight: 400, ArabicSize: 18,
		TranslationFamily: "TransB", TranslationFallback: "sans",
		TranslationWeight: 400, TranslationSize: 14,
	})
	tokenB := generateFontToken(t, userB.ID, userB.Email)

	body := map[string]interface{}{
		"arabic": []interface{}{"NewAmiri", "serif", 700, 28},
	}
	resp := makeFontRequest(t, app, "PUT", "/fonts", body, tokenB)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("update failed: status %d", resp.StatusCode)
	}

	var aFont, bFont models.Font
	database.DB.Where("user_id = ?", userA.ID).First(&aFont)
	database.DB.Where("user_id = ?", userB.ID).First(&bFont)
	if aFont.ArabicFamily != "Amiri" {
		t.Errorf("user A arabic_family = %q, want 'Amiri' (overwritten by user B)", aFont.ArabicFamily)
	}
	if bFont.ArabicFamily != "NewAmiri" {
		t.Errorf("user B arabic_family = %q, want 'NewAmiri'", bFont.ArabicFamily)
	}
}

func TestFontUpdate_CreatesOwnRow_NotOthers(t *testing.T) {
	app := setupFontTestApp(t)
	userA := seedFontUser(t, "font_new_a@example.com")
	userB := seedFontUser(t, "font_new_b@example.com")
	database.DB.Create(&models.Font{
		UserID: userA.ID, ArabicFamily: "Amiri", ArabicFallback: "serif",
		ArabicWeight: 700, ArabicSize: 24,
		TranslationFamily: "Lato", TranslationFallback: "sans-serif",
		TranslationWeight: 400, TranslationSize: 16,
	})
	tokenB := generateFontToken(t, userB.ID, userB.Email)

	body := map[string]interface{}{
		"arabic": []interface{}{"OwnFont", "serif", 500, 20},
	}
	resp := makeFontRequest(t, app, "PUT", "/fonts", body, tokenB)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("update failed: status %d", resp.StatusCode)
	}

	var count int
	database.DB.Raw("SELECT COUNT(*) FROM Font WHERE user_id = ?", userB.ID).Scan(&count)
	if count != 1 {
		t.Errorf("user B font rows = %d, want 1", count)
	}

	var aFont models.Font
	database.DB.Where("user_id = ?", userA.ID).First(&aFont)
	if aFont.ArabicFamily != "Amiri" {
		t.Errorf("user A arabic_family = %q, want 'Amiri' (untouched)", aFont.ArabicFamily)
	}
}
