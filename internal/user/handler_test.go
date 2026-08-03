package user

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/haditssoft/haditssoft-backend/internal/shared/auth"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/internal/shared/middleware"
	"github.com/haditssoft/haditssoft-backend/internal/shared/validator"
	"github.com/haditssoft/haditssoft-backend/models"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestMain(m *testing.M) {
	os.Setenv("JWT_SECRET", "test-secret-for-user-unit-tests")
	validator.RegisterCustomValidations()
	os.Exit(m.Run())
}

var testDBCounter int

func setupUserTestDB(t *testing.T) {
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
	if err := db.AutoMigrate(&models.User{}, &models.BlacklistToken{}, &models.Activity{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	database.DB = db
	t.Cleanup(func() {
		database.DB = nil
	})
}

func setupUserTestApp(t *testing.T) *fiber.App {
	t.Helper()
	setupUserTestDB(t)
	repo := NewRepository()
	svc := NewService(repo)
	h := NewHandler(svc)
	ah := NewAdminHandler(svc)

	app := fiber.New()
	RegisterRoutes(app, h, middleware.TokenOnly(), middleware.Protected())

	v2 := app.Group("/admin")
	RegisterAdminRoutes(v2, ah, middleware.Protected(), middleware.IsAdmin)

	return app
}

func seedUser(t *testing.T, email, code string, active bool) models.User {
	t.Helper()
	user := models.User{
		Email:                     email,
		Password:                  "hashedpassword",
		VerificationCode:          code,
		VerificationCodeExpiresAt: sql.NullTime{Time: time.Now().Add(15 * time.Minute), Valid: true},
		Active:                    &active,
		Admin:                     boolPtr(false),
	}
	if err := database.DB.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return user
}

func seedVerifiedUser(t *testing.T, email string) models.User {
	t.Helper()
	now := time.Now()
	user := models.User{
		Email:           email,
		Password:        "hashedpassword",
		EmailVerifiedAt: sql.NullTime{Time: now, Valid: true},
		Active:          boolPtr(true),
		Admin:           boolPtr(false),
	}
	if err := database.DB.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return user
}

func seedExpiredCodeUser(t *testing.T, email, code string) models.User {
	t.Helper()
	user := models.User{
		Email:                     email,
		Password:                  "hashedpassword",
		VerificationCode:          code,
		VerificationCodeExpiresAt: sql.NullTime{Time: time.Now().Add(-1 * time.Minute), Valid: true},
		Active:                    boolPtr(false),
		Admin:                     boolPtr(false),
	}
	if err := database.DB.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return user
}

func seedUserWithExpiry(t *testing.T, email, code string, active bool, expiresIn time.Duration) models.User {
	t.Helper()
	user := models.User{
		Email:                     email,
		Password:                  "hashedpassword",
		VerificationCode:          code,
		VerificationCodeExpiresAt: sql.NullTime{Time: time.Now().Add(expiresIn), Valid: true},
		Active:                    &active,
		Admin:                     boolPtr(false),
	}
	if err := database.DB.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return user
}

func boolPtr(b bool) *bool { return &b }

func generateTestToken(t *testing.T, userID uint, email string) string {
	t.Helper()
	token, err := auth.GenerateAccessToken(userID, email)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return token
}

func makeUserRequest(t *testing.T, app *fiber.App, method, path string, body interface{}, token string) *http.Response {
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

func decodeJSON(t *testing.T, resp *http.Response, dest interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

// ============================================================
// POST /users (Create) tests
// ============================================================

func TestCreateUser_Success(t *testing.T) {
	app := setupUserTestApp(t)

	body := map[string]string{
		"email":                 "test@example.com",
		"password":              "secret123",
		"password_confirmation": "secret123",
		"active":                "false",
		"admin":                 "false",
	}
	resp := makeUserRequest(t, app, "POST", "/users", body, "")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if result["status"] != "success" {
		t.Errorf("status = %v, want 'success'", result["status"])
	}
	if result["token"] == nil || result["token"] == "" {
		t.Error("response should contain a token")
	}
}

func TestCreateUser_StoresVerificationCode(t *testing.T) {
	app := setupUserTestApp(t)

	body := map[string]string{
		"email":                 "verify@example.com",
		"password":              "secret123",
		"password_confirmation": "secret123",
		"active":                "false",
		"admin":                 "false",
	}
	resp := makeUserRequest(t, app, "POST", "/users", body, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create failed: status %d", resp.StatusCode)
	}

	var user models.User
	result := database.DB.Where("email = ?", "verify@example.com").First(&user)
	if result.Error != nil {
		t.Fatalf("user not found: %v", result.Error)
	}

	if len(user.VerificationCode) != 6 {
		t.Errorf("VerificationCode = %q, want 6-digit code", user.VerificationCode)
	}
}

func TestCreateUser_StoresVerificationCodeExpiry(t *testing.T) {
	app := setupUserTestApp(t)

	body := map[string]string{
		"email":                 "expiry@example.com",
		"password":              "secret123",
		"password_confirmation": "secret123",
		"active":                "false",
		"admin":                 "false",
	}
	resp := makeUserRequest(t, app, "POST", "/users", body, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create failed: status %d", resp.StatusCode)
	}

	var user models.User
	database.DB.Where("email = ?", "expiry@example.com").First(&user)

	if !user.VerificationCodeExpiresAt.Valid {
		t.Fatal("VerificationCodeExpiresAt should be valid")
	}
	expiresAt := user.VerificationCodeExpiresAt.Time
	if expiresAt.Before(time.Now()) {
		t.Error("VerificationCodeExpiresAt should be in the future")
	}
	if expiresAt.After(time.Now().Add(16 * time.Minute)) {
		t.Error("VerificationCodeExpiresAt should be within 15 minutes")
	}
}

func TestCreateUser_StoresEmailNotVerified(t *testing.T) {
	app := setupUserTestApp(t)

	body := map[string]string{
		"email":                 "notverified@example.com",
		"password":              "secret123",
		"password_confirmation": "secret123",
		"active":                "false",
		"admin":                 "false",
	}
	resp := makeUserRequest(t, app, "POST", "/users", body, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create failed: status %d", resp.StatusCode)
	}

	var user models.User
	database.DB.Where("email = ?", "notverified@example.com").First(&user)

	if user.EmailVerifiedAt.Valid {
		t.Error("EmailVerifiedAt should be NULL for new user")
	}
}

func TestCreateUser_InvalidEmail(t *testing.T) {
	app := setupUserTestApp(t)

	body := map[string]string{
		"email":                 "not-an-email",
		"password":              "secret123",
		"password_confirmation": "secret123",
		"active":                "false",
		"admin":                 "false",
	}
	resp := makeUserRequest(t, app, "POST", "/users", body, "")

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestCreateUser_MissingPassword(t *testing.T) {
	app := setupUserTestApp(t)

	body := map[string]string{
		"email":                 "test@example.com",
		"password_confirmation": "secret123",
		"active":                "false",
		"admin":                 "false",
	}
	resp := makeUserRequest(t, app, "POST", "/users", body, "")

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestCreateUser_PasswordMismatch(t *testing.T) {
	app := setupUserTestApp(t)

	body := map[string]string{
		"email":                 "test@example.com",
		"password":              "secret123",
		"password_confirmation": "different",
		"active":                "false",
		"admin":                 "false",
	}
	resp := makeUserRequest(t, app, "POST", "/users", body, "")

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// ============================================================
// POST /users/verify (Verify) tests
// ============================================================

func TestVerifyEmail_Success(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedUser(t, "verify1@example.com", "123456", false)
	token := generateTestToken(t, user.ID, user.Email)

	body := map[string]string{"code": "123456"}
	resp := makeUserRequest(t, app, "POST", "/users/verify", body, token)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if result["status"] != "success" {
		t.Errorf("status = %v, want 'success'", result["status"])
	}
}

func TestVerifyEmail_SetsActiveTrue(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedUser(t, "active@example.com", "123456", false)
	token := generateTestToken(t, user.ID, user.Email)

	body := map[string]string{"code": "123456"}
	resp := makeUserRequest(t, app, "POST", "/users/verify", body, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verify failed: status %d", resp.StatusCode)
	}

	var updated models.User
	database.DB.Where("id = ?", user.ID).First(&updated)

	if updated.Active == nil || !*updated.Active {
		t.Error("Active should be true after verification")
	}
}

func TestVerifyEmail_SetsEmailVerifiedAt(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedUser(t, "verifiedat@example.com", "123456", false)
	token := generateTestToken(t, user.ID, user.Email)

	body := map[string]string{"code": "123456"}
	resp := makeUserRequest(t, app, "POST", "/users/verify", body, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verify failed: status %d", resp.StatusCode)
	}

	var updated models.User
	database.DB.Where("id = ?", user.ID).First(&updated)

	if !updated.EmailVerifiedAt.Valid {
		t.Error("EmailVerifiedAt should be set after verification")
	}
}

func TestVerifyEmail_ClearsVerificationCode(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedUser(t, "clearcode@example.com", "123456", false)
	token := generateTestToken(t, user.ID, user.Email)

	body := map[string]string{"code": "123456"}
	resp := makeUserRequest(t, app, "POST", "/users/verify", body, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verify failed: status %d", resp.StatusCode)
	}

	var updated models.User
	database.DB.Where("id = ?", user.ID).First(&updated)

	if updated.VerificationCode != "" {
		t.Errorf("VerificationCode should be empty after verification, got %q", updated.VerificationCode)
	}
}

func TestVerifyEmail_ClearsVerificationCodeExpiry(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedUser(t, "clearexpiry@example.com", "123456", false)
	token := generateTestToken(t, user.ID, user.Email)

	body := map[string]string{"code": "123456"}
	resp := makeUserRequest(t, app, "POST", "/users/verify", body, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verify failed: status %d", resp.StatusCode)
	}

	var updated models.User
	database.DB.Where("id = ?", user.ID).First(&updated)

	if updated.VerificationCodeExpiresAt.Valid {
		t.Error("VerificationCodeExpiresAt should be NULL after verification")
	}
}

func TestVerifyEmail_WrongCode(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedUser(t, "wrongcode@example.com", "123456", false)
	token := generateTestToken(t, user.ID, user.Email)

	body := map[string]string{"code": "000000"}
	resp := makeUserRequest(t, app, "POST", "/users/verify", body, token)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if result["message"] != "Invalid verification code" {
		t.Errorf("message = %v, want 'Invalid verification code'", result["message"])
	}
}

func TestVerifyEmail_ExpiredCode(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedExpiredCodeUser(t, "expired@example.com", "123456")
	token := generateTestToken(t, user.ID, user.Email)

	body := map[string]string{"code": "123456"}
	resp := makeUserRequest(t, app, "POST", "/users/verify", body, token)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if result["message"] != "Verification code expired" {
		t.Errorf("message = %v, want 'Verification code expired'", result["message"])
	}
}

func TestVerifyEmail_AlreadyVerified(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedVerifiedUser(t, "alreadyverified@example.com")
	token := generateTestToken(t, user.ID, user.Email)

	body := map[string]string{"code": "123456"}
	resp := makeUserRequest(t, app, "POST", "/users/verify", body, token)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if result["message"] != "Email already verified" {
		t.Errorf("message = %v, want 'Email already verified'", result["message"])
	}
}

func TestVerifyEmail_NoJWT(t *testing.T) {
	app := setupUserTestApp(t)

	body := map[string]string{"code": "123456"}
	resp := makeUserRequest(t, app, "POST", "/users/verify", body, "")

	if resp.StatusCode == http.StatusOK {
		t.Error("request without JWT should not return 200")
	}
}

func TestVerifyEmail_InvalidJWT(t *testing.T) {
	app := setupUserTestApp(t)

	body := map[string]string{"code": "123456"}
	resp := makeUserRequest(t, app, "POST", "/users/verify", body, "invalid.token.here")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestVerifyEmail_EmptyCode(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedUser(t, "emptycode@example.com", "123456", false)
	token := generateTestToken(t, user.ID, user.Email)

	body := map[string]string{}
	resp := makeUserRequest(t, app, "POST", "/users/verify", body, token)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestVerifyEmail_NonNumericCode(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedUser(t, "nonnumeric@example.com", "123456", false)
	token := generateTestToken(t, user.ID, user.Email)

	body := map[string]string{"code": "abcdef"}
	resp := makeUserRequest(t, app, "POST", "/users/verify", body, token)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestVerifyEmail_WrongLength(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedUser(t, "wronglen@example.com", "123456", false)
	token := generateTestToken(t, user.ID, user.Email)

	body := map[string]string{"code": "12345"}
	resp := makeUserRequest(t, app, "POST", "/users/verify", body, token)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// ============================================================
// POST /users/verify/resend (Resend) tests
// ============================================================

func TestResend_Success(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedUserWithExpiry(t, "resend@example.com", "111111", false, 10*time.Minute)
	token := generateTestToken(t, user.ID, user.Email)

	resp := makeUserRequest(t, app, "POST", "/users/verify/resend", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)
	if result["status"] != "success" {
		t.Errorf("status = %v, want 'success'", result["status"])
	}
}

func TestResend_UpdatesCode(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedUserWithExpiry(t, "resendcode@example.com", "111111", false, 10*time.Minute)
	token := generateTestToken(t, user.ID, user.Email)

	resp := makeUserRequest(t, app, "POST", "/users/verify/resend", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("resend failed: status %d", resp.StatusCode)
	}

	var updated models.User
	database.DB.Where("id = ?", user.ID).First(&updated)

	if updated.VerificationCode == "111111" {
		t.Error("VerificationCode should have changed after resend")
	}
	if len(updated.VerificationCode) != 6 {
		t.Errorf("VerificationCode = %q, want 6-digit code", updated.VerificationCode)
	}
}

func TestResend_UpdatesExpiry(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedUserWithExpiry(t, "resendexpiry@example.com", "111111", false, 10*time.Minute)
	token := generateTestToken(t, user.ID, user.Email)

	resp := makeUserRequest(t, app, "POST", "/users/verify/resend", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("resend failed: status %d", resp.StatusCode)
	}

	var updated models.User
	database.DB.Where("id = ?", user.ID).First(&updated)

	if !updated.VerificationCodeExpiresAt.Valid {
		t.Fatal("VerificationCodeExpiresAt should be valid after resend")
	}
	if updated.VerificationCodeExpiresAt.Time.Before(time.Now()) {
		t.Error("VerificationCodeExpiresAt should be in the future")
	}
}

func TestResend_AlreadyVerified(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedVerifiedUser(t, "resendverified@example.com")
	token := generateTestToken(t, user.ID, user.Email)

	resp := makeUserRequest(t, app, "POST", "/users/verify/resend", nil, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)
	if result["message"] != "Email already verified" {
		t.Errorf("message = %v, want 'Email already verified'", result["message"])
	}
}

func TestResend_NoJWT(t *testing.T) {
	app := setupUserTestApp(t)

	resp := makeUserRequest(t, app, "POST", "/users/verify/resend", nil, "")
	if resp.StatusCode == http.StatusOK {
		t.Error("request without JWT should not return 200")
	}
}

func TestResend_InvalidJWT(t *testing.T) {
	app := setupUserTestApp(t)

	resp := makeUserRequest(t, app, "POST", "/users/verify/resend", nil, "invalid.token.here")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestResend_TooSoon(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedUser(t, "toosoon@example.com", "222222", false)
	token := generateTestToken(t, user.ID, user.Email)

	resp := makeUserRequest(t, app, "POST", "/users/verify/resend", nil, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)
	if result["message"] != "Please wait before requesting a new code" {
		t.Errorf("message = %v, want 'Please wait before requesting a new code'", result["message"])
	}
}

func TestResend_AfterWait(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedUserWithExpiry(t, "afterwait@example.com", "333333", false, 12*time.Minute)
	token := generateTestToken(t, user.ID, user.Email)

	resp := makeUserRequest(t, app, "POST", "/users/verify/resend", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)
	if result["status"] != "success" {
		t.Errorf("status = %v, want 'success'", result["status"])
	}
}

// ============================================================
// PUT /users/:id (Update) tests
// ============================================================

func TestUpdateUser_Success(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedVerifiedUser(t, "ownupdate@example.com")
	token := generateTestToken(t, user.ID, user.Email)

	body := map[string]string{
		"email":  "ownupdate@example.com",
		"active": "true",
		"admin":  "false",
	}
	resp := makeUserRequest(t, app, "PUT", "/users/"+strconv.Itoa(int(user.ID)), body, token)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestUpdateUser_ForbiddenOnOtherUser(t *testing.T) {
	app := setupUserTestApp(t)

	owner := seedVerifiedUser(t, "owner@example.com")
	target := seedVerifiedUser(t, "target@example.com")
	token := generateTestToken(t, owner.ID, owner.Email)

	body := map[string]string{
		"email":  "target@example.com",
		"active": "true",
		"admin":  "true",
	}
	resp := makeUserRequest(t, app, "PUT", "/users/"+strconv.Itoa(int(target.ID)), body, token)

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)
	if result["message"] != "Forbidden" {
		t.Errorf("message = %v, want 'Forbidden'", result["message"])
	}
}

func TestUpdateUser_EmptyNewPasswordDoesNotClearPassword(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedVerifiedUser(t, "keepassword@example.com")
	token := generateTestToken(t, user.ID, user.Email)

	body := map[string]string{
		"email":  "keepassword@example.com",
		"active": "true",
		"admin":  "false",
	}
	resp := makeUserRequest(t, app, "PUT", "/users/"+strconv.Itoa(int(user.ID)), body, token)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("request failed: status %d", resp.StatusCode)
	}

	var updated models.User
	database.DB.Where("id = ?", user.ID).First(&updated)

	if updated.Password == "" {
		t.Error("Password should not be empty when new_password is not provided")
	}
}

func TestUpdateUser_NoJWT(t *testing.T) {
	app := setupUserTestApp(t)

	resp := makeUserRequest(t, app, "PUT", "/users/1", map[string]string{"email": "a@b.com", "active": "true", "admin": "false"}, "")

	if resp.StatusCode == http.StatusOK {
		t.Error("request without JWT should not return 200")
	}
}

func TestUpdateUser_InvalidJWT(t *testing.T) {
	app := setupUserTestApp(t)

	resp := makeUserRequest(t, app, "PUT", "/users/1", map[string]string{"email": "a@b.com", "active": "true", "admin": "false"}, "bad.token.here")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestUpdateUser_InvalidEmail(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedVerifiedUser(t, "validemail@example.com")
	token := generateTestToken(t, user.ID, user.Email)

	body := map[string]string{
		"email":  "not-an-email",
		"active": "true",
		"admin":  "false",
	}
	resp := makeUserRequest(t, app, "PUT", "/users/"+strconv.Itoa(int(user.ID)), body, token)

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
}

// ============================================================
// POST /users/forgot-password tests
// ============================================================

func TestForgotPassword_Success(t *testing.T) {
	app := setupUserTestApp(t)

	seedVerifiedUser(t, "forgot@example.com")

	body := map[string]string{"email": "forgot@example.com"}
	resp := makeUserRequest(t, app, "POST", "/users/forgot-password", body, "")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if result["status"] != "success" {
		t.Errorf("status = %v, want 'success'", result["status"])
	}
}

func TestForgotPassword_StoresCode(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedVerifiedUser(t, "forgotcode@example.com")

	body := map[string]string{"email": "forgotcode@example.com"}
	resp := makeUserRequest(t, app, "POST", "/users/forgot-password", body, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("request failed: status %d", resp.StatusCode)
	}

	var updated models.User
	database.DB.Where("id = ?", user.ID).First(&updated)

	if len(updated.VerificationCode) != 6 {
		t.Errorf("VerificationCode = %q, want 6-digit code", updated.VerificationCode)
	}
}

func TestForgotPassword_NonExistentEmail(t *testing.T) {
	app := setupUserTestApp(t)

	body := map[string]string{"email": "nonexistent@example.com"}
	resp := makeUserRequest(t, app, "POST", "/users/forgot-password", body, "")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d (should return success to prevent email enumeration)", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if result["status"] != "success" {
		t.Errorf("status = %v, want 'success'", result["status"])
	}
}

func TestForgotPassword_RateLimit(t *testing.T) {
	app := setupUserTestApp(t)

	seedVerifiedUser(t, "rateforgot@example.com")

	body := map[string]string{"email": "rateforgot@example.com"}

	resp1 := makeUserRequest(t, app, "POST", "/users/forgot-password", body, "")
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first request failed: status %d", resp1.StatusCode)
	}

	resp2 := makeUserRequest(t, app, "POST", "/users/forgot-password", body, "")
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("second request status = %d, want %d", resp2.StatusCode, http.StatusBadRequest)
	}

	var result map[string]interface{}
	decodeJSON(t, resp2, &result)

	if result["message"] != "Please wait before requesting a new code" {
		t.Errorf("message = %v, want 'Please wait before requesting a new code'", result["message"])
	}
}

func TestForgotPassword_InvalidEmail(t *testing.T) {
	app := setupUserTestApp(t)

	body := map[string]string{"email": "not-an-email"}
	resp := makeUserRequest(t, app, "POST", "/users/forgot-password", body, "")

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestForgotPassword_MissingEmail(t *testing.T) {
	app := setupUserTestApp(t)

	body := map[string]string{}
	resp := makeUserRequest(t, app, "POST", "/users/forgot-password", body, "")

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// ============================================================
// POST /users/forgot-password/confirm tests
// ============================================================

func TestConfirmForgotPassword_Success(t *testing.T) {
	app := setupUserTestApp(t)

	seedVerifiedUser(t, "confirmfp@example.com")

	reqBody := map[string]string{"email": "confirmfp@example.com"}
	resp1 := makeUserRequest(t, app, "POST", "/users/forgot-password", reqBody, "")
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("forgot-password request failed: status %d", resp1.StatusCode)
	}

	var user models.User
	database.DB.Where("email = ?", "confirmfp@example.com").First(&user)
	code := user.VerificationCode

	confirmBody := map[string]string{
		"email":                 "confirmfp@example.com",
		"code":                  code,
		"new_password":          "newSecurePass123",
		"password_confirmation": "newSecurePass123",
	}
	resp2 := makeUserRequest(t, app, "POST", "/users/forgot-password/confirm", confirmBody, "")
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("confirm status = %d, want %d", resp2.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp2, &result)

	if result["status"] != "success" {
		t.Errorf("status = %v, want 'success'", result["status"])
	}
}

func TestConfirmForgotPassword_WrongCode(t *testing.T) {
	app := setupUserTestApp(t)

	seedVerifiedUser(t, "wrongcodefp@example.com")

	reqBody := map[string]string{"email": "wrongcodefp@example.com"}
	makeUserRequest(t, app, "POST", "/users/forgot-password", reqBody, "")

	confirmBody := map[string]string{
		"email":                 "wrongcodefp@example.com",
		"code":                  "000000",
		"new_password":          "newPass123",
		"password_confirmation": "newPass123",
	}
	resp := makeUserRequest(t, app, "POST", "/users/forgot-password/confirm", confirmBody, "")

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if result["message"] != "Invalid verification code" {
		t.Errorf("message = %v, want 'Invalid verification code'", result["message"])
	}
}

func TestConfirmForgotPassword_ExpiredCode(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedVerifiedUser(t, "expiredfp@example.com")

	database.DB.Session(&gorm.Session{SkipHooks: true}).Model(&user).Updates(map[string]interface{}{
		"VerificationCode":          "123456",
		"VerificationCodeExpiresAt": time.Now().Add(-1 * time.Minute),
	})

	confirmBody := map[string]string{
		"email":                 "expiredfp@example.com",
		"code":                  "123456",
		"new_password":          "newPass123",
		"password_confirmation": "newPass123",
	}
	resp := makeUserRequest(t, app, "POST", "/users/forgot-password/confirm", confirmBody, "")

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if result["message"] != "Verification code expired" {
		t.Errorf("message = %v, want 'Verification code expired'", result["message"])
	}
}

func TestConfirmForgotPassword_UserNotFound(t *testing.T) {
	app := setupUserTestApp(t)

	confirmBody := map[string]string{
		"email":                 "nobody@example.com",
		"code":                  "123456",
		"new_password":          "newPass123",
		"password_confirmation": "newPass123",
	}
	resp := makeUserRequest(t, app, "POST", "/users/forgot-password/confirm", confirmBody, "")

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestConfirmForgotPassword_PasswordMismatch(t *testing.T) {
	app := setupUserTestApp(t)

	seedVerifiedUser(t, "mismatchfp@example.com")

	reqBody := map[string]string{"email": "mismatchfp@example.com"}
	makeUserRequest(t, app, "POST", "/users/forgot-password", reqBody, "")

	var user models.User
	database.DB.Where("email = ?", "mismatchfp@example.com").First(&user)

	confirmBody := map[string]string{
		"email":                 "mismatchfp@example.com",
		"code":                  user.VerificationCode,
		"new_password":          "newPass123",
		"password_confirmation": "differentPass",
	}
	resp := makeUserRequest(t, app, "POST", "/users/forgot-password/confirm", confirmBody, "")

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
}

func TestConfirmForgotPassword_CodeClearedAfterUse(t *testing.T) {
	app := setupUserTestApp(t)

	seedVerifiedUser(t, "clearcodefp@example.com")

	reqBody := map[string]string{"email": "clearcodefp@example.com"}
	makeUserRequest(t, app, "POST", "/users/forgot-password", reqBody, "")

	var user models.User
	database.DB.Where("email = ?", "clearcodefp@example.com").First(&user)
	code := user.VerificationCode

	confirmBody := map[string]string{
		"email":                 "clearcodefp@example.com",
		"code":                  code,
		"new_password":          "newPass123",
		"password_confirmation": "newPass123",
	}
	resp := makeUserRequest(t, app, "POST", "/users/forgot-password/confirm", confirmBody, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("confirm failed: status %d", resp.StatusCode)
	}

	var updated models.User
	database.DB.Where("email = ?", "clearcodefp@example.com").First(&updated)

	if updated.VerificationCode != "" {
		t.Errorf("VerificationCode should be empty after reset, got %q", updated.VerificationCode)
	}
	if updated.VerificationCodeExpiresAt.Valid {
		t.Error("VerificationCodeExpiresAt should be NULL after reset")
	}
}

func TestConfirmForgotPassword_InvalidEmail(t *testing.T) {
	app := setupUserTestApp(t)

	confirmBody := map[string]string{
		"email":                 "not-an-email",
		"code":                  "123456",
		"new_password":          "newPass123",
		"password_confirmation": "newPass123",
	}
	resp := makeUserRequest(t, app, "POST", "/users/forgot-password/confirm", confirmBody, "")

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
}

func TestConfirmForgotPassword_EmptyFields(t *testing.T) {
	app := setupUserTestApp(t)

	confirmBody := map[string]string{}
	resp := makeUserRequest(t, app, "POST", "/users/forgot-password/confirm", confirmBody, "")

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
}

func TestConfirmForgotPassword_CodeCannotBeReused(t *testing.T) {
	app := setupUserTestApp(t)

	seedVerifiedUser(t, "reusecodefp@example.com")

	reqBody := map[string]string{"email": "reusecodefp@example.com"}
	makeUserRequest(t, app, "POST", "/users/forgot-password", reqBody, "")

	var user models.User
	database.DB.Where("email = ?", "reusecodefp@example.com").First(&user)
	code := user.VerificationCode

	confirmBody := map[string]string{
		"email":                 "reusecodefp@example.com",
		"code":                  code,
		"new_password":          "newPass123",
		"password_confirmation": "newPass123",
	}
	resp1 := makeUserRequest(t, app, "POST", "/users/forgot-password/confirm", confirmBody, "")
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first confirm failed: status %d", resp1.StatusCode)
	}

	resp2 := makeUserRequest(t, app, "POST", "/users/forgot-password/confirm", confirmBody, "")
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("reuse status = %d, want %d", resp2.StatusCode, http.StatusBadRequest)
	}

	var result map[string]interface{}
	decodeJSON(t, resp2, &result)

	if result["message"] != "Invalid verification code" {
		t.Errorf("message = %v, want 'Invalid verification code'", result["message"])
	}
}

// ============================================================
// GET /users/:id (GetOne) tests
// ============================================================

func TestGetOne_Success(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedVerifiedUser(t, "getone@example.com")
	token := generateTestToken(t, user.ID, user.Email)

	resp := makeUserRequest(t, app, "GET", "/users/"+strconv.Itoa(int(user.ID)), nil, token)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if result["email"] != "getone@example.com" {
		t.Errorf("email = %v, want 'getone@example.com'", result["email"])
	}
}

func TestGetOne_NotFound(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedVerifiedUser(t, "getonenf@example.com")
	token := generateTestToken(t, user.ID, user.Email)

	resp := makeUserRequest(t, app, "GET", "/users/99999", nil, token)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGetOne_NoJWT(t *testing.T) {
	app := setupUserTestApp(t)

	resp := makeUserRequest(t, app, "GET", "/users/1", nil, "")

	if resp.StatusCode == http.StatusOK {
		t.Error("request without JWT should not return 200")
	}
}

// ============================================================
// DELETE /users/:id (DeleteOne) tests
// ============================================================

func TestDeleteOne_Success(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedVerifiedUser(t, "deleteone@example.com")
	token := generateTestToken(t, user.ID, user.Email)

	resp := makeUserRequest(t, app, "DELETE", "/users/"+strconv.Itoa(int(user.ID)), nil, token)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if result["status"] != "success" {
		t.Errorf("status = %v, want 'success'", result["status"])
	}
}

func TestDeleteOne_ActivityLogged(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedVerifiedUser(t, "deleteactivity@example.com")
	token := generateTestToken(t, user.ID, user.Email)

	resp := makeUserRequest(t, app, "DELETE", "/users/"+strconv.Itoa(int(user.ID)), nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete failed: status %d", resp.StatusCode)
	}

	var activity models.Activity
	result := database.DB.Where("note = ?", "Delete a user").First(&activity)
	if result.Error != nil {
		t.Fatalf("activity not found: %v", result.Error)
	}
}

// ============================================================
// Admin GET /admin/users (GetList) tests
// ============================================================

func TestAdminGetList_Success(t *testing.T) {
	app := setupUserTestApp(t)

	seedVerifiedUser(t, "list1@example.com")
	seedVerifiedUser(t, "list2@example.com")

	adminUser := seedVerifiedUser(t, "admin@example.com")
	database.DB.Session(&gorm.Session{SkipHooks: true}).Model(&adminUser).Update("admin", true)
	token := generateTestToken(t, adminUser.ID, adminUser.Email)

	resp := makeUserRequest(t, app, "GET", "/admin/users", nil, token)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if result["data"] == nil {
		t.Error("response should contain data")
	}
}

// ============================================================
// Admin GET /admin/users/:id (GetOne) tests
// ============================================================

func TestAdminGetOne_Success(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedVerifiedUser(t, "admingetone@example.com")
	adminUser := seedVerifiedUser(t, "admin2@example.com")
	database.DB.Session(&gorm.Session{SkipHooks: true}).Model(&adminUser).Update("admin", true)
	token := generateTestToken(t, adminUser.ID, adminUser.Email)

	resp := makeUserRequest(t, app, "GET", "/admin/users/"+strconv.Itoa(int(user.ID)), nil, token)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if result["email"] != "admingetone@example.com" {
		t.Errorf("email = %v, want 'admingetone@example.com'", result["email"])
	}
}

// ============================================================
// Admin POST /admin/users (Create) tests
// ============================================================

func TestAdminCreate_Success(t *testing.T) {
	app := setupUserTestApp(t)

	adminUser := seedVerifiedUser(t, "admincreate@example.com")
	database.DB.Session(&gorm.Session{SkipHooks: true}).Model(&adminUser).Update("admin", true)
	token := generateTestToken(t, adminUser.ID, adminUser.Email)

	body := map[string]string{
		"email":                 "newuser@example.com",
		"password":              "secret123",
		"password_confirmation": "secret123",
		"active":                "true",
		"admin":                 "false",
	}
	resp := makeUserRequest(t, app, "POST", "/admin/users", body, token)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if result["email"] != "newuser@example.com" {
		t.Errorf("email = %v, want 'newuser@example.com'", result["email"])
	}
}

func TestAdminCreate_InvalidEmail(t *testing.T) {
	app := setupUserTestApp(t)

	adminUser := seedVerifiedUser(t, "admincreate2@example.com")
	database.DB.Session(&gorm.Session{SkipHooks: true}).Model(&adminUser).Update("admin", true)
	token := generateTestToken(t, adminUser.ID, adminUser.Email)

	body := map[string]string{
		"email":                 "not-an-email",
		"password":              "secret123",
		"password_confirmation": "secret123",
		"active":                "true",
		"admin":                 "false",
	}
	resp := makeUserRequest(t, app, "POST", "/admin/users", body, token)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestAdminCreate_RequiresAdmin(t *testing.T) {
	app := setupUserTestApp(t)

	nonAdmin := seedVerifiedUser(t, "nonadmin@example.com")
	token := generateTestToken(t, nonAdmin.ID, nonAdmin.Email)

	body := map[string]string{
		"email":                 "newuser2@example.com",
		"password":              "secret123",
		"password_confirmation": "secret123",
		"active":                "true",
		"admin":                 "false",
	}
	resp := makeUserRequest(t, app, "POST", "/admin/users", body, token)

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// ============================================================
// Admin DELETE /admin/users/:id (DeleteOne) tests
// ============================================================

func TestAdminDeleteOne_Success(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedVerifiedUser(t, "admindelete@example.com")
	adminUser := seedVerifiedUser(t, "admin3@example.com")
	database.DB.Session(&gorm.Session{SkipHooks: true}).Model(&adminUser).Update("admin", true)
	token := generateTestToken(t, adminUser.ID, adminUser.Email)

	resp := makeUserRequest(t, app, "DELETE", "/admin/users/"+strconv.Itoa(int(user.ID)), nil, token)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestAdminDeleteOne_RequiresAdmin(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedVerifiedUser(t, "admindelete2@example.com")
	nonAdmin := seedVerifiedUser(t, "nonadmin2@example.com")
	token := generateTestToken(t, nonAdmin.ID, nonAdmin.Email)

	resp := makeUserRequest(t, app, "DELETE", "/admin/users/"+strconv.Itoa(int(user.ID)), nil, token)

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// ============================================================
// Admin PUT /admin/users/:id (Update) tests
// ============================================================

func TestAdminUpdate_Success(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedVerifiedUser(t, "adminupdate@example.com")
	adminUser := seedVerifiedUser(t, "admin4@example.com")
	database.DB.Session(&gorm.Session{SkipHooks: true}).Model(&adminUser).Update("admin", true)
	token := generateTestToken(t, adminUser.ID, adminUser.Email)

	body := map[string]string{
		"email":  "adminupdate@example.com",
		"active": "true",
		"admin":  "false",
	}
	resp := makeUserRequest(t, app, "PUT", "/admin/users/"+strconv.Itoa(int(user.ID)), body, token)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestAdminUpdate_CanSetAdmin(t *testing.T) {
	app := setupUserTestApp(t)

	user := seedVerifiedUser(t, "adminupdateadmin@example.com")
	adminUser := seedVerifiedUser(t, "admin5@example.com")
	database.DB.Session(&gorm.Session{SkipHooks: true}).Model(&adminUser).Update("admin", true)
	token := generateTestToken(t, adminUser.ID, adminUser.Email)

	body := map[string]string{
		"email":  "adminupdateadmin@example.com",
		"active": "true",
		"admin":  "true",
	}
	resp := makeUserRequest(t, app, "PUT", "/admin/users/"+strconv.Itoa(int(user.ID)), body, token)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("request failed: status %d", resp.StatusCode)
	}

	var updated models.User
	database.DB.Where("id = ?", user.ID).First(&updated)

	if updated.Admin == nil || !*updated.Admin {
		t.Error("Admin should be true after admin update")
	}
}
