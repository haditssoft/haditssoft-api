package searchmode

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
	os.Setenv("JWT_SECRET", "test-secret-for-searchmode-unit-tests")
	validator.RegisterCustomValidations()
	os.Exit(m.Run())
}

var smTestDBCounter int

func setupSMTestDB(t *testing.T) {
	t.Helper()
	smTestDBCounter++
	dbName := fmt.Sprintf("file:test_sm_%d?mode=memory&cache=shared", smTestDBCounter)
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
	if err := db.AutoMigrate(&models.User{}, &models.SearchMode{}, &models.Activity{}, &models.BlacklistToken{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	database.DB = db
	t.Cleanup(func() { database.DB = nil })
}

func setupSMTestApp(t *testing.T) *fiber.App {
	t.Helper()
	setupSMTestDB(t)
	repo := NewRepository()
	svc := NewService(repo)
	h := NewHandler(svc)
	app := fiber.New()
	RegisterRoutes(app, h)
	return app
}

func seedSMUser(t *testing.T, email string) models.User {
	t.Helper()
	user := models.User{Email: email, Password: "hashedpassword", Active: boolPtr(true), Admin: boolPtr(false)}
	if err := database.DB.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return user
}

func boolPtr(b bool) *bool { return &b }

func generateSMToken(t *testing.T, userID uint, email string) string {
	t.Helper()
	token, err := auth.GenerateAccessToken(userID, email)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return token
}

func makeSMRequest(t *testing.T, app *fiber.App, method, path string, body interface{}, token string) *http.Response {
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

func decodeSMJSON(t *testing.T, resp *http.Response, dest interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

// ============================================================
// GET /search-mode (GetOne) tests
// ============================================================

func TestSMGetOne_Default(t *testing.T) {
	app := setupSMTestApp(t)
	user := seedSMUser(t, "sm@example.com")
	token := generateSMToken(t, user.ID, user.Email)

	resp := makeSMRequest(t, app, "GET", "/search-mode", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeSMJSON(t, resp, &result)
	if result["search_mode"] != float64(1) {
		t.Errorf("search_mode = %v, want 1", result["search_mode"])
	}
}

func TestSMGetOne_Existing(t *testing.T) {
	app := setupSMTestApp(t)
	user := seedSMUser(t, "smexist@example.com")
	database.DB.Create(&models.SearchMode{UserID: user.ID, SearchMode: 2})
	token := generateSMToken(t, user.ID, user.Email)

	resp := makeSMRequest(t, app, "GET", "/search-mode", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeSMJSON(t, resp, &result)
	if result["search_mode"] != float64(2) {
		t.Errorf("search_mode = %v, want 2", result["search_mode"])
	}
}

func TestSMGetOne_NoJWT(t *testing.T) {
	app := setupSMTestApp(t)
	resp := makeSMRequest(t, app, "GET", "/search-mode", nil, "")
	if resp.StatusCode == http.StatusOK {
		t.Error("request without JWT should not return 200")
	}
}

// ============================================================
// PUT /search-mode (Update) tests
// ============================================================

func TestSMUpdate_CreateNew(t *testing.T) {
	app := setupSMTestApp(t)
	user := seedSMUser(t, "smupd@example.com")
	token := generateSMToken(t, user.ID, user.Email)

	body := map[string]interface{}{"search_mode": 2}
	resp := makeSMRequest(t, app, "PUT", "/search-mode", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	var sm models.SearchMode
	database.DB.Where("user_id = ?", user.ID).First(&sm)
	if sm.SearchMode != 2 {
		t.Errorf("search_mode = %d, want 2", sm.SearchMode)
	}
}

func TestSMUpdate_Existing(t *testing.T) {
	app := setupSMTestApp(t)
	user := seedSMUser(t, "smupd2@example.com")
	database.DB.Create(&models.SearchMode{UserID: user.ID, SearchMode: 1})
	token := generateSMToken(t, user.ID, user.Email)

	body := map[string]interface{}{"search_mode": 2}
	resp := makeSMRequest(t, app, "PUT", "/search-mode", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	var sm models.SearchMode
	database.DB.Where("user_id = ?", user.ID).First(&sm)
	if sm.SearchMode != 2 {
		t.Errorf("search_mode = %d, want 2", sm.SearchMode)
	}
}

func TestSMUpdate_CreatesActivity(t *testing.T) {
	app := setupSMTestApp(t)
	user := seedSMUser(t, "smact@example.com")
	token := generateSMToken(t, user.ID, user.Email)

	body := map[string]interface{}{"search_mode": 2}
	resp := makeSMRequest(t, app, "PUT", "/search-mode", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("update failed: status %d", resp.StatusCode)
	}

	var count int
	database.DB.Raw("SELECT COUNT(*) FROM Activity WHERE UserID = ? AND Note = 'Create theme settings'", user.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 activity, got %d", count)
	}
}

func TestSMUpdate_NoJWT(t *testing.T) {
	app := setupSMTestApp(t)
	body := map[string]interface{}{"search_mode": 2}
	resp := makeSMRequest(t, app, "PUT", "/search-mode", body, "")
	if resp.StatusCode == http.StatusNoContent {
		t.Error("request without JWT should not return 204")
	}
}
