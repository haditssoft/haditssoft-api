package auth

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
	"github.com/haditssoft/haditssoft-backend/internal/shared/database/entities"
	"github.com/haditssoft/haditssoft-backend/internal/shared/middleware"
	"github.com/haditssoft/haditssoft-backend/internal/shared/validator"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestMain(m *testing.M) {
	os.Setenv("JWT_SECRET", "test-secret-for-auth-unit-tests")
	validator.RegisterCustomValidations()
	os.Exit(m.Run())
}

var testDBCounter int

func setupAuthTestDB(t *testing.T) {
	t.Helper()
	testDBCounter++
	dbName := fmt.Sprintf("file:test_%d?mode=memory&cache=shared", testDBCounter)
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		SkipDefaultTransaction:                   true,
		DisableForeignKeyConstraintWhenMigrating: true,
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
			NoLowerCase:   true,
		},
	})
	if err != nil {
		t.Fatalf("failed to open test DB: %v", err)
	}
	if err := db.AutoMigrate(&entities.User{}, &entities.BlacklistToken{}, &entities.Activity{}, &entities.RefreshToken{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	database.DB = db
	t.Cleanup(func() {
		database.DB = nil
	})
}

func seedAuthUser(t *testing.T, email, password string, active, admin bool) entities.User {
	t.Helper()
	hashed, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := entities.User{
		Email:    email,
		Password: hashed,
		Active:   &active,
		Admin:    &admin,
	}
	if err := database.DB.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return user
}

func makeAuthRequest(t *testing.T, app *fiber.App, method, path string, body interface{}, token string) *http.Response {
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

func decodeAuthJSON(t *testing.T, resp *http.Response, dest interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

func generateTestAuthToken(t *testing.T, userID uint, email string) string {
	t.Helper()
	token, err := auth.GenerateAccessToken(userID, email)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return token
}

func setupAuthTestApp(t *testing.T) *fiber.App {
	t.Helper()
	setupAuthTestDB(t)
	repo := NewRepository()
	svc := NewService(repo)
	h := NewHandler(svc)

	app := fiber.New()
	RegisterRoutes(app, h, middleware.Protected())
	return app
}

func setupAdminAuthTestApp(t *testing.T) *fiber.App {
	t.Helper()
	setupAuthTestDB(t)
	repo := NewRepository()
	svc := NewService(repo)
	h := NewAdminHandler(svc)

	app := fiber.New()
	v2 := app.Group("/admin")
	RegisterAdminRoutes(v2, h, middleware.Protected())
	return app
}

// ============================================================
// POST /auths/login — success
// ============================================================

func TestLogin_Success(t *testing.T) {
	app := setupAuthTestApp(t)
	seedAuthUser(t, "login@example.com", "secret123", true, false)

	body := map[string]string{"email": "login@example.com", "password": "secret123"}
	resp := makeAuthRequest(t, app, "POST", "/auths/login", body, "")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeAuthJSON(t, resp, &result)

	if result["status"] != "success" {
		t.Errorf("status = %v, want 'success'", result["status"])
	}
	if result["token"] == nil || result["token"] == "" {
		t.Error("response should contain a token")
	}
	if result["refresh_token"] == nil || result["refresh_token"] == "" {
		t.Error("response should contain a refresh_token")
	}
}

func TestLogin_CreatesRefreshToken(t *testing.T) {
	app := setupAuthTestApp(t)
	seedAuthUser(t, "rtoken@example.com", "pass123", true, false)

	body := map[string]string{"email": "rtoken@example.com", "password": "pass123"}
	resp := makeAuthRequest(t, app, "POST", "/auths/login", body, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login failed: status %d", resp.StatusCode)
	}

	var count int64
	database.DB.Model(&entities.RefreshToken{}).Where("user_id = ?", 1).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 refresh token, got %d", count)
	}
}

func TestLogin_CreatesActivity(t *testing.T) {
	app := setupAuthTestApp(t)
	seedAuthUser(t, "activity@example.com", "pass123", true, false)

	body := map[string]string{"email": "activity@example.com", "password": "pass123"}
	resp := makeAuthRequest(t, app, "POST", "/auths/login", body, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login failed: status %d", resp.StatusCode)
	}

	var activity entities.Activity
	result := database.DB.Where("note = ?", "Login").First(&activity)
	if result.Error != nil {
		t.Fatalf("activity not found: %v", result.Error)
	}
	if activity.ReqMethod != "POST" {
		t.Errorf("ReqMethod = %q, want 'POST'", activity.ReqMethod)
	}
}

// ============================================================
// POST /auths/login — failures
// ============================================================

func TestLogin_WrongPassword(t *testing.T) {
	app := setupAuthTestApp(t)
	seedAuthUser(t, "wp@example.com", "correct", true, false)

	body := map[string]string{"email": "wp@example.com", "password": "wrong"}
	resp := makeAuthRequest(t, app, "POST", "/auths/login", body, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	var result map[string]interface{}
	decodeAuthJSON(t, resp, &result)
	if result["message"] != "record not found" {
		t.Errorf("message = %v, want 'record not found'", result["message"])
	}
}

func TestLogin_NonexistentUser(t *testing.T) {
	app := setupAuthTestApp(t)

	body := map[string]string{"email": "nobody@example.com", "password": "pass123"}
	resp := makeAuthRequest(t, app, "POST", "/auths/login", body, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestLogin_InvalidEmailFormat(t *testing.T) {
	app := setupAuthTestApp(t)

	body := map[string]string{"email": "not-an-email", "password": "pass123"}
	resp := makeAuthRequest(t, app, "POST", "/auths/login", body, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	var result map[string]interface{}
	decodeAuthJSON(t, resp, &result)
	if result["message"] != "Invalid email format" {
		t.Errorf("message = %v, want 'Invalid email format'", result["message"])
	}
}

func TestLogin_MissingBody(t *testing.T) {
	app := setupAuthTestApp(t)

	resp := makeAuthRequest(t, app, "POST", "/auths/login", nil, "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// ============================================================
// POST /admin/auths/login
// ============================================================

func TestAdminLogin_Success(t *testing.T) {
	app := setupAdminAuthTestApp(t)
	seedAuthUser(t, "admin@example.com", "admin123", true, true)

	body := map[string]string{"email": "admin@example.com", "password": "admin123"}
	resp := makeAuthRequest(t, app, "POST", "/admin/auths/login", body, "")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeAuthJSON(t, resp, &result)
	if result["status"] != "success" {
		t.Errorf("status = %v, want 'success'", result["status"])
	}
}

func TestAdminLogin_NonAdminUser(t *testing.T) {
	app := setupAdminAuthTestApp(t)
	seedAuthUser(t, "user@example.com", "user123", true, false)

	body := map[string]string{"email": "user@example.com", "password": "user123"}
	resp := makeAuthRequest(t, app, "POST", "/admin/auths/login", body, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestAdminLogin_InactiveAdmin(t *testing.T) {
	app := setupAdminAuthTestApp(t)
	seedAuthUser(t, "inactiveadmin@example.com", "pass123", false, true)

	body := map[string]string{"email": "inactiveadmin@example.com", "password": "pass123"}
	resp := makeAuthRequest(t, app, "POST", "/admin/auths/login", body, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// ============================================================
// POST /auths/logout
// ============================================================

func TestLogout_Success(t *testing.T) {
	app := setupAuthTestApp(t)
	seedAuthUser(t, "logout@example.com", "pass123", true, false)
	token := generateTestAuthToken(t, 1, "logout@example.com")

	resp := makeAuthRequest(t, app, "POST", "/auths/logout", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeAuthJSON(t, resp, &result)
	if result["status"] != "success" {
		t.Errorf("status = %v, want 'success'", result["status"])
	}
}

func TestLogout_CreatesBlacklistToken(t *testing.T) {
	app := setupAuthTestApp(t)
	seedAuthUser(t, "bl@example.com", "pass123", true, false)
	token := generateTestAuthToken(t, 1, "bl@example.com")

	makeAuthRequest(t, app, "POST", "/auths/logout", nil, token)

	var count int64
	database.DB.Model(&entities.BlacklistToken{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 blacklist token, got %d", count)
	}
}

func TestLogout_NoToken(t *testing.T) {
	app := setupAuthTestApp(t)

	resp := makeAuthRequest(t, app, "POST", "/auths/logout", nil, "")
	if resp.StatusCode == http.StatusOK {
		t.Error("request without token should not return 200")
	}
}

func TestLogout_InvalidToken(t *testing.T) {
	app := setupAuthTestApp(t)

	resp := makeAuthRequest(t, app, "POST", "/auths/logout", nil, "invalid.token.here")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// ============================================================
// GET /auths/identity
// ============================================================

func TestIdentity_Success(t *testing.T) {
	app := setupAuthTestApp(t)
	seedAuthUser(t, "identity@example.com", "pass123", true, false)
	token := generateTestAuthToken(t, 1, "identity@example.com")

	resp := makeAuthRequest(t, app, "GET", "/auths/identity", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeAuthJSON(t, resp, &result)
	if result["email"] != "identity@example.com" {
		t.Errorf("email = %v, want 'identity@example.com'", result["email"])
	}
}

func TestIdentity_NoToken(t *testing.T) {
	app := setupAuthTestApp(t)

	resp := makeAuthRequest(t, app, "GET", "/auths/identity", nil, "")
	if resp.StatusCode == http.StatusOK {
		t.Error("request without token should not return 200")
	}
}

func TestIdentity_InvalidToken(t *testing.T) {
	app := setupAuthTestApp(t)

	resp := makeAuthRequest(t, app, "GET", "/auths/identity", nil, "bad.token.here")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// ============================================================
// POST /auths/refresh
// ============================================================

func TestRefresh_Success(t *testing.T) {
	app := setupAuthTestApp(t)
	seedAuthUser(t, "refresh@example.com", "pass123", true, false)

	loginBody := map[string]string{"email": "refresh@example.com", "password": "pass123"}
	loginResp := makeAuthRequest(t, app, "POST", "/auths/login", loginBody, "")
	var loginResult map[string]interface{}
	decodeAuthJSON(t, loginResp, &loginResult)

	rt := loginResult["refresh_token"].(string)

	refreshBody := map[string]string{"refresh_token": rt}
	resp := makeAuthRequest(t, app, "POST", "/auths/refresh", refreshBody, "")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeAuthJSON(t, resp, &result)
	if result["token"] == nil || result["token"] == "" {
		t.Error("response should contain a new access token")
	}
	if result["refresh_token"] == nil || result["refresh_token"] == "" {
		t.Error("response should contain a new refresh token")
	}
}

func TestRefresh_RotatesToken(t *testing.T) {
	app := setupAuthTestApp(t)
	seedAuthUser(t, "rotate@example.com", "pass123", true, false)

	loginBody := map[string]string{"email": "rotate@example.com", "password": "pass123"}
	loginResp := makeAuthRequest(t, app, "POST", "/auths/login", loginBody, "")
	var loginResult map[string]interface{}
	decodeAuthJSON(t, loginResp, &loginResult)

	oldRT := loginResult["refresh_token"].(string)

	refreshBody := map[string]string{"refresh_token": oldRT}
	resp := makeAuthRequest(t, app, "POST", "/auths/refresh", refreshBody, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("refresh failed: status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	decodeAuthJSON(t, resp, &result)
	newRT := result["refresh_token"].(string)

	if oldRT == newRT {
		t.Error("refresh token should be rotated (new != old)")
	}

	hashedOld := auth.HashToken(oldRT)
	var record entities.RefreshToken
	database.DB.Where("token_hash = ?", hashedOld).First(&record)
	if !record.IsUsed {
		t.Error("old refresh token should be marked as used")
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	app := setupAuthTestApp(t)

	body := map[string]string{"refresh_token": "invalid-refresh-token"}
	resp := makeAuthRequest(t, app, "POST", "/auths/refresh", body, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestRefresh_EmptyToken(t *testing.T) {
	app := setupAuthTestApp(t)

	body := map[string]string{"refresh_token": ""}
	resp := makeAuthRequest(t, app, "POST", "/auths/refresh", body, "")

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestRefresh_MissingBody(t *testing.T) {
	app := setupAuthTestApp(t)

	resp := makeAuthRequest(t, app, "POST", "/auths/refresh", nil, "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestRefresh_ReuseDetection(t *testing.T) {
	app := setupAuthTestApp(t)
	seedAuthUser(t, "reuse@example.com", "pass123", true, false)

	loginBody := map[string]string{"email": "reuse@example.com", "password": "pass123"}
	loginResp := makeAuthRequest(t, app, "POST", "/auths/login", loginBody, "")
	var loginResult map[string]interface{}
	decodeAuthJSON(t, loginResp, &loginResult)

	rt := loginResult["refresh_token"].(string)
	refreshBody := map[string]string{"refresh_token": rt}

	resp1 := makeAuthRequest(t, app, "POST", "/auths/refresh", refreshBody, "")
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first refresh failed: status %d", resp1.StatusCode)
	}

	resp2 := makeAuthRequest(t, app, "POST", "/auths/refresh", refreshBody, "")
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("reuse status = %d, want %d", resp2.StatusCode, http.StatusUnauthorized)
	}

	var result map[string]interface{}
	decodeAuthJSON(t, resp2, &result)
	if result["message"] != "refresh token reuse detected, all tokens revoked" {
		t.Errorf("message = %v", result["message"])
	}
}
