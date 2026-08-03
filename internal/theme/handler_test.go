package theme

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
	os.Setenv("JWT_SECRET", "test-secret-for-theme-unit-tests")
	validator.RegisterCustomValidations()
	os.Exit(m.Run())
}

var themeTestDBCounter int

func setupThemeTestDB(t *testing.T) {
	t.Helper()
	themeTestDBCounter++
	dbName := fmt.Sprintf("file:test_theme_%d?mode=memory&cache=shared", themeTestDBCounter)
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
	if err := db.AutoMigrate(&models.User{}, &models.Theme{}, &models.Activity{}, &models.BlacklistToken{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	database.DB = db
	t.Cleanup(func() { database.DB = nil })
}

func setupThemeTestApp(t *testing.T) *fiber.App {
	t.Helper()
	setupThemeTestDB(t)
	repo := NewRepository()
	svc := NewService(repo)
	h := NewHandler(svc)
	app := fiber.New()
	RegisterRoutes(app, h)
	return app
}

func seedThemeUser(t *testing.T, email string) models.User {
	t.Helper()
	user := models.User{Email: email, Password: "hashedpassword", Active: boolPtr(true), Admin: boolPtr(false)}
	if err := database.DB.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return user
}

func boolPtr(b bool) *bool { return &b }

func generateThemeToken(t *testing.T, userID uint, email string) string {
	t.Helper()
	token, err := auth.GenerateAccessToken(userID, email)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return token
}

func makeThemeRequest(t *testing.T, app *fiber.App, method, path string, body interface{}, token string) *http.Response {
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

func decodeThemeJSON(t *testing.T, resp *http.Response, dest interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

// ============================================================
// GET /theme (GetOne) tests
// ============================================================

func TestThemeGetOne_Default(t *testing.T) {
	app := setupThemeTestApp(t)
	user := seedThemeUser(t, "theme@example.com")
	token := generateThemeToken(t, user.ID, user.Email)

	resp := makeThemeRequest(t, app, "GET", "/theme", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeThemeJSON(t, resp, &result)
	if result["theme"] != "l" {
		t.Errorf("theme = %v, want 'l'", result["theme"])
	}
}

func TestThemeGetOne_Existing(t *testing.T) {
	app := setupThemeTestApp(t)
	user := seedThemeUser(t, "themeexist@example.com")
	database.DB.Create(&models.Theme{UserID: user.ID, Theme: "d"})
	token := generateThemeToken(t, user.ID, user.Email)

	resp := makeThemeRequest(t, app, "GET", "/theme", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeThemeJSON(t, resp, &result)
	if result["theme"] != "d" {
		t.Errorf("theme = %v, want 'd'", result["theme"])
	}
}

func TestThemeGetOne_NoJWT(t *testing.T) {
	app := setupThemeTestApp(t)
	resp := makeThemeRequest(t, app, "GET", "/theme", nil, "")
	if resp.StatusCode == http.StatusOK {
		t.Error("request without JWT should not return 200")
	}
}

// ============================================================
// PUT /theme (Update) tests
// ============================================================

func TestThemeUpdate_CreateNew(t *testing.T) {
	app := setupThemeTestApp(t)
	user := seedThemeUser(t, "themeupd@example.com")
	token := generateThemeToken(t, user.ID, user.Email)

	body := map[string]interface{}{"theme": "d"}
	resp := makeThemeRequest(t, app, "PUT", "/theme", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	var theme models.Theme
	database.DB.Where("user_id = ?", user.ID).First(&theme)
	if theme.Theme != "d" {
		t.Errorf("theme = %q, want 'd'", theme.Theme)
	}
}

func TestThemeUpdate_Existing(t *testing.T) {
	app := setupThemeTestApp(t)
	user := seedThemeUser(t, "themeupd2@example.com")
	database.DB.Create(&models.Theme{UserID: user.ID, Theme: "l"})
	token := generateThemeToken(t, user.ID, user.Email)

	body := map[string]interface{}{"theme": "d"}
	resp := makeThemeRequest(t, app, "PUT", "/theme", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	var theme models.Theme
	database.DB.Where("user_id = ?", user.ID).First(&theme)
	if theme.Theme != "d" {
		t.Errorf("theme = %q, want 'd'", theme.Theme)
	}
}

func TestThemeUpdate_CreatesActivity(t *testing.T) {
	app := setupThemeTestApp(t)
	user := seedThemeUser(t, "themeact@example.com")
	token := generateThemeToken(t, user.ID, user.Email)

	body := map[string]interface{}{"theme": "d"}
	resp := makeThemeRequest(t, app, "PUT", "/theme", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("update failed: status %d", resp.StatusCode)
	}

	var count int
	database.DB.Raw("SELECT COUNT(*) FROM Activity WHERE UserID = ? AND Note = 'Create theme settings'", user.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 activity, got %d", count)
	}
}

func TestThemeUpdate_NoJWT(t *testing.T) {
	app := setupThemeTestApp(t)
	body := map[string]interface{}{"theme": "d"}
	resp := makeThemeRequest(t, app, "PUT", "/theme", body, "")
	if resp.StatusCode == http.StatusNoContent {
		t.Error("request without JWT should not return 204")
	}
}
