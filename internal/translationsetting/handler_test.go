package translationsetting

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
	os.Setenv("JWT_SECRET", "test-secret-for-translation-setting-unit-tests")
	validator.RegisterCustomValidations()
	os.Exit(m.Run())
}

var tsTestDBCounter int

func setupTSTestDB(t *testing.T) {
	t.Helper()
	tsTestDBCounter++
	dbName := fmt.Sprintf("file:test_translation_setting_%d?mode=memory&cache=shared", tsTestDBCounter)
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
	if err := db.AutoMigrate(&models.User{}, &models.TranslationSetting{}, &models.Activity{}, &models.BlacklistToken{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	database.DB = db
	t.Cleanup(func() { database.DB = nil })
}

func setupTSTestApp(t *testing.T) *fiber.App {
	t.Helper()
	setupTSTestDB(t)
	repo := NewRepository()
	svc := NewService(repo)
	h := NewHandler(svc)
	app := fiber.New()
	RegisterRoutes(app, h)
	return app
}

func seedTSUser(t *testing.T, email string) models.User {
	t.Helper()
	user := models.User{Email: email, Password: "hashedpassword", Active: boolPtr(true), Admin: boolPtr(false)}
	if err := database.DB.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return user
}

func boolPtr(b bool) *bool { return &b }

func generateTSToken(t *testing.T, userID uint, email string) string {
	t.Helper()
	token, err := auth.GenerateAccessToken(userID, email)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return token
}

func makeTSRequest(t *testing.T, app *fiber.App, method, path string, body interface{}, token string) *http.Response {
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

func makeRawTSRequest(t *testing.T, app *fiber.App, method, path, rawBody string, token string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(rawBody))
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

func decodeTSJSON(t *testing.T, resp *http.Response, dest interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

func countTSRowsForUser(t *testing.T, userID uint) int {
	t.Helper()
	var count int
	database.DB.Raw("SELECT COUNT(*) FROM TranslationSetting WHERE user_id = ?", userID).Scan(&count)
	return count
}

// ============================================================
// GET /translation-setting (GetOne) tests
// ============================================================

func TestTSGetOne_Default(t *testing.T) {
	app := setupTSTestApp(t)
	user := seedTSUser(t, "ts@example.com")
	token := generateTSToken(t, user.ID, user.Email)

	resp := makeTSRequest(t, app, "GET", "/translation-setting", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeTSJSON(t, resp, &result)
	if result["language"] != "Indonesia" {
		t.Errorf("language = %v, want 'Indonesia'", result["language"])
	}
}

func TestTSGetOne_Existing(t *testing.T) {
	app := setupTSTestApp(t)
	user := seedTSUser(t, "tsexist@example.com")
	database.DB.Create(&models.TranslationSetting{UserID: user.ID, Language: "English"})
	token := generateTSToken(t, user.ID, user.Email)

	resp := makeTSRequest(t, app, "GET", "/translation-setting", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeTSJSON(t, resp, &result)
	if result["language"] != "English" {
		t.Errorf("language = %v, want 'English'", result["language"])
	}
}

func TestTSGetOne_NoJWT(t *testing.T) {
	app := setupTSTestApp(t)
	resp := makeTSRequest(t, app, "GET", "/translation-setting", nil, "")
	if resp.StatusCode == http.StatusOK {
		t.Error("request without JWT should not return 200")
	}
}

// ============================================================
// PUT /translation-setting (Update) tests
// ============================================================

func TestTSUpdate_CreateNew(t *testing.T) {
	app := setupTSTestApp(t)
	user := seedTSUser(t, "tsupd@example.com")
	token := generateTSToken(t, user.ID, user.Email)

	body := map[string]interface{}{"language": "English"}
	resp := makeTSRequest(t, app, "PUT", "/translation-setting", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	var setting models.TranslationSetting
	database.DB.Where("user_id = ?", user.ID).First(&setting)
	if setting.Language != "English" {
		t.Errorf("language = %q, want 'English'", setting.Language)
	}
}

func TestTSUpdate_Existing(t *testing.T) {
	app := setupTSTestApp(t)
	user := seedTSUser(t, "tsupd2@example.com")
	database.DB.Create(&models.TranslationSetting{UserID: user.ID, Language: "Indonesia"})
	token := generateTSToken(t, user.ID, user.Email)

	body := map[string]interface{}{"language": "Urdu"}
	resp := makeTSRequest(t, app, "PUT", "/translation-setting", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	var setting models.TranslationSetting
	database.DB.Where("user_id = ?", user.ID).First(&setting)
	if setting.Language != "Urdu" {
		t.Errorf("language = %q, want 'Urdu'", setting.Language)
	}
}

func TestTSUpdate_EverySupportedLanguage(t *testing.T) {
	for _, lang := range []string{"Indonesia", "English", "Urdu", "Bengali"} {
		t.Run(lang, func(t *testing.T) {
			app := setupTSTestApp(t)
			user := seedTSUser(t, "tslang-"+lang+"@example.com")
			token := generateTSToken(t, user.ID, user.Email)

			body := map[string]interface{}{"language": lang}
			resp := makeTSRequest(t, app, "PUT", "/translation-setting", body, token)
			if resp.StatusCode != http.StatusNoContent {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
			}

			var setting models.TranslationSetting
			database.DB.Where("user_id = ?", user.ID).First(&setting)
			if setting.Language != lang {
				t.Errorf("language = %q, want %q", setting.Language, lang)
			}
		})
	}
}

func TestTSUpdate_GetReflectsPersistedValue(t *testing.T) {
	app := setupTSTestApp(t)
	user := seedTSUser(t, "tsreflect@example.com")
	token := generateTSToken(t, user.ID, user.Email)

	body := map[string]interface{}{"language": "Bengali"}
	resp := makeTSRequest(t, app, "PUT", "/translation-setting", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("update failed: status %d", resp.StatusCode)
	}

	resp = makeTSRequest(t, app, "GET", "/translation-setting", nil, token)
	var result map[string]interface{}
	decodeTSJSON(t, resp, &result)
	if result["language"] != "Bengali" {
		t.Errorf("language = %v, want 'Bengali'", result["language"])
	}
}

func TestTSUpdate_CreatesActivityOnCreate(t *testing.T) {
	app := setupTSTestApp(t)
	user := seedTSUser(t, "tsactcreate@example.com")
	token := generateTSToken(t, user.ID, user.Email)

	body := map[string]interface{}{"language": "English"}
	resp := makeTSRequest(t, app, "PUT", "/translation-setting", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("update failed: status %d", resp.StatusCode)
	}

	var count int
	database.DB.Raw("SELECT COUNT(*) FROM Activity WHERE UserID = ? AND Note = 'Create translation settings'", user.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 create activity, got %d", count)
	}
}

func TestTSUpdate_CreatesActivityOnUpdate(t *testing.T) {
	app := setupTSTestApp(t)
	user := seedTSUser(t, "tsactupdate@example.com")
	database.DB.Create(&models.TranslationSetting{UserID: user.ID, Language: "Indonesia"})
	token := generateTSToken(t, user.ID, user.Email)

	body := map[string]interface{}{"language": "English"}
	resp := makeTSRequest(t, app, "PUT", "/translation-setting", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("update failed: status %d", resp.StatusCode)
	}

	var count int
	database.DB.Raw("SELECT COUNT(*) FROM Activity WHERE UserID = ? AND Note = 'Update translation settings'", user.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 update activity, got %d", count)
	}
}

func TestTSUpdate_NoJWT(t *testing.T) {
	app := setupTSTestApp(t)
	body := map[string]interface{}{"language": "English"}
	resp := makeTSRequest(t, app, "PUT", "/translation-setting", body, "")
	if resp.StatusCode == http.StatusNoContent {
		t.Error("request without JWT should not return 204")
	}
}

// ============================================================
// PUT validation tests
// ============================================================

func TestTSUpdate_UnsupportedLanguage(t *testing.T) {
	app := setupTSTestApp(t)
	user := seedTSUser(t, "tsunsupported@example.com")
	token := generateTSToken(t, user.ID, user.Email)

	body := map[string]interface{}{"language": "French"}
	resp := makeTSRequest(t, app, "PUT", "/translation-setting", body, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var errResp map[string]interface{}
	decodeTSJSON(t, resp, &errResp)
	if errResp["error"] != "language is not supported" {
		t.Errorf("error = %v, want 'language is not supported'", errResp["error"])
	}

	if countTSRowsForUser(t, user.ID) != 0 {
		t.Error("no row should be persisted for unsupported language")
	}
}

func TestTSUpdate_AlbaniAndDarussalamRejected(t *testing.T) {
	for _, lang := range []string{"Albani", "Darussalam"} {
		t.Run(lang, func(t *testing.T) {
			app := setupTSTestApp(t)
			user := seedTSUser(t, "tsnonlang-"+lang+"@example.com")
			token := generateTSToken(t, user.ID, user.Email)

			body := map[string]interface{}{"language": lang}
			resp := makeTSRequest(t, app, "PUT", "/translation-setting", body, token)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
		})
	}
}

func TestTSUpdate_EmptyLanguage(t *testing.T) {
	app := setupTSTestApp(t)
	user := seedTSUser(t, "tsempty@example.com")
	token := generateTSToken(t, user.ID, user.Email)

	body := map[string]interface{}{"language": ""}
	resp := makeTSRequest(t, app, "PUT", "/translation-setting", body, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var errResp map[string]interface{}
	decodeTSJSON(t, resp, &errResp)
	if errResp["error"] != "language is required" {
		t.Errorf("error = %v, want 'language is required'", errResp["error"])
	}
}

func TestTSUpdate_MissingLanguageKey(t *testing.T) {
	app := setupTSTestApp(t)
	user := seedTSUser(t, "tsmissing@example.com")
	token := generateTSToken(t, user.ID, user.Email)

	body := map[string]interface{}{}
	resp := makeTSRequest(t, app, "PUT", "/translation-setting", body, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestTSUpdate_WhitespaceLanguage(t *testing.T) {
	app := setupTSTestApp(t)
	user := seedTSUser(t, "tsspace@example.com")
	token := generateTSToken(t, user.ID, user.Email)

	body := map[string]interface{}{"language": "   "}
	resp := makeTSRequest(t, app, "PUT", "/translation-setting", body, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestTSUpdate_NonStringLanguage(t *testing.T) {
	app := setupTSTestApp(t)
	user := seedTSUser(t, "tsnum@example.com")
	token := generateTSToken(t, user.ID, user.Email)

	body := map[string]interface{}{"language": 123}
	resp := makeTSRequest(t, app, "PUT", "/translation-setting", body, token)
	if resp.StatusCode == http.StatusNoContent {
		t.Error("numeric language should not be accepted")
	}
}

func TestTSUpdate_MalformedJSON(t *testing.T) {
	app := setupTSTestApp(t)
	user := seedTSUser(t, "tsbadjson@example.com")
	token := generateTSToken(t, user.ID, user.Email)

	resp := makeRawTSRequest(t, app, "PUT", "/translation-setting", "not json", token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestTSUpdate_LanguageCaseSensitive(t *testing.T) {
	app := setupTSTestApp(t)
	user := seedTSUser(t, "tscase@example.com")
	token := generateTSToken(t, user.ID, user.Email)

	body := map[string]interface{}{"language": "english"}
	resp := makeTSRequest(t, app, "PUT", "/translation-setting", body, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d for lower-case 'english'", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestTSUpdate_ExtraFieldsIgnored(t *testing.T) {
	app := setupTSTestApp(t)
	user := seedTSUser(t, "tsextra@example.com")
	token := generateTSToken(t, user.ID, user.Email)

	body := map[string]interface{}{
		"language": "English",
		"user_id":  999,
		"id":       555,
		"bogus":    "ignored",
	}
	resp := makeTSRequest(t, app, "PUT", "/translation-setting", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	var setting models.TranslationSetting
	database.DB.Where("user_id = ?", user.ID).First(&setting)
	if setting.Language != "English" {
		t.Errorf("language = %q, want 'English'", setting.Language)
	}
}

// ============================================================
// User isolation & idempotency tests
// ============================================================

func TestTSUpdate_UserFromTokenNotBody(t *testing.T) {
	app := setupTSTestApp(t)
	userA := seedTSUser(t, "tsuserA@example.com")
	userB := seedTSUser(t, "tsuserB@example.com")
	tokenA := generateTSToken(t, userA.ID, userA.Email)
	tokenB := generateTSToken(t, userB.ID, userB.Email)

	// User A tries to write with user B's ID in the body
	body := map[string]interface{}{"language": "English", "user_id": userB.ID}
	resp := makeTSRequest(t, app, "PUT", "/translation-setting", body, tokenA)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	var settingA models.TranslationSetting
	database.DB.Where("user_id = ?", userA.ID).First(&settingA)
	if settingA.Language != "English" {
		t.Errorf("user A language = %q, want 'English'", settingA.Language)
	}

	if countTSRowsForUser(t, userB.ID) != 0 {
		t.Error("user B should not have any setting row")
	}

	// User B still gets default
	resp = makeTSRequest(t, app, "GET", "/translation-setting", nil, tokenB)
	var result map[string]interface{}
	decodeTSJSON(t, resp, &result)
	if result["language"] != "Indonesia" {
		t.Errorf("user B language = %v, want 'Indonesia'", result["language"])
	}
}

func TestTSUpdate_UsersAreIsolated(t *testing.T) {
	app := setupTSTestApp(t)
	userA := seedTSUser(t, "tsisolA@example.com")
	userB := seedTSUser(t, "tsisolB@example.com")
	tokenA := generateTSToken(t, userA.ID, userA.Email)
	tokenB := generateTSToken(t, userB.ID, userB.Email)

	resp := makeTSRequest(t, app, "PUT", "/translation-setting", map[string]interface{}{"language": "Urdu"}, tokenA)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("user A update failed: status %d", resp.StatusCode)
	}

	resp = makeTSRequest(t, app, "GET", "/translation-setting", nil, tokenB)
	var result map[string]interface{}
	decodeTSJSON(t, resp, &result)
	if result["language"] != "Indonesia" {
		t.Errorf("user B language = %v, want 'Indonesia' (unaffected by user A)", result["language"])
	}
}

func TestTSUpdate_IdempotentRepeatedPut(t *testing.T) {
	app := setupTSTestApp(t)
	user := seedTSUser(t, "tsidem@example.com")
	token := generateTSToken(t, user.ID, user.Email)

	for i := 0; i < 5; i++ {
		resp := makeTSRequest(t, app, "PUT", "/translation-setting", map[string]interface{}{"language": "English"}, token)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("attempt %d failed: status %d", i, resp.StatusCode)
		}
	}

	if countTSRowsForUser(t, user.ID) != 1 {
		t.Errorf("expected exactly 1 row after repeated PUTs, got %d", countTSRowsForUser(t, user.ID))
	}
}

func TestTSUpdate_AlternateLanguagesNoDuplicates(t *testing.T) {
	app := setupTSTestApp(t)
	user := seedTSUser(t, "tsalt@example.com")
	token := generateTSToken(t, user.ID, user.Email)

	langs := []string{"Indonesia", "English", "Urdu", "Bengali", "English", "Indonesia"}
	for _, lang := range langs {
		resp := makeTSRequest(t, app, "PUT", "/translation-setting", map[string]interface{}{"language": lang}, token)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("PUT %s failed: status %d", lang, resp.StatusCode)
		}
	}

	if countTSRowsForUser(t, user.ID) != 1 {
		t.Errorf("expected exactly 1 row after alternating PUTs, got %d", countTSRowsForUser(t, user.ID))
	}

	var setting models.TranslationSetting
	database.DB.Where("user_id = ?", user.ID).First(&setting)
	if setting.Language != "Indonesia" {
		t.Errorf("language = %q, want 'Indonesia' (last value)", setting.Language)
	}
}
