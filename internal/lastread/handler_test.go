package lastread

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
	os.Setenv("JWT_SECRET", "test-secret-for-lastread-unit-tests")
	validator.RegisterCustomValidations()
	os.Exit(m.Run())
}

var lrTestDBCounter int

func setupLRTestDB(t *testing.T) {
	t.Helper()
	lrTestDBCounter++
	dbName := fmt.Sprintf("file:test_lr_%d?mode=memory&cache=shared", lrTestDBCounter)
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
	if err := db.AutoMigrate(&models.User{}, &models.Activity{}, &models.BlacklistToken{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	db.Exec("CREATE TABLE LastRead (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, book_name TEXT, no INTEGER, created_at DATETIME, updated_at DATETIME)")
	db.Exec("CREATE TABLE Muslim (id INTEGER PRIMARY KEY AUTOINCREMENT, Nomer INTEGER)")

	database.DB = db
	t.Cleanup(func() { database.DB = nil })
}

func setupLRTestApp(t *testing.T) *fiber.App {
	t.Helper()
	setupLRTestDB(t)
	repo := NewRepository()
	svc := NewService(repo)
	h := NewHandler(svc)
	app := fiber.New()
	RegisterRoutes(app, h)
	return app
}

func seedLRUser(t *testing.T, email string) models.User {
	t.Helper()
	user := models.User{Email: email, Password: "hashedpassword", Active: boolPtr(true), Admin: boolPtr(false)}
	if err := database.DB.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return user
}

func boolPtr(b bool) *bool { return &b }

func seedLastRead(t *testing.T, userID uint, bookName string, no uint) {
	t.Helper()
	database.DB.Exec("INSERT INTO LastRead (user_id, book_name, no, created_at, updated_at) VALUES (?, ?, ?, datetime('now'), datetime('now'))", userID, bookName, no)
}

func seedLRHadith(t *testing.T, bookName string, nomer uint) {
	t.Helper()
	database.DB.Exec(fmt.Sprintf("INSERT INTO %s (Nomer) VALUES (?)", bookName), nomer)
}

func generateLRToken(t *testing.T, userID uint, email string) string {
	t.Helper()
	token, err := auth.GenerateAccessToken(userID, email)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return token
}

func makeLRRequest(t *testing.T, app *fiber.App, method, path string, body interface{}, token string) *http.Response {
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

func decodeLRJSON(t *testing.T, resp *http.Response, dest interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

// ============================================================
// GET /lastRead/:book_name (GetOne) tests
// ============================================================

func TestLRGetOne_Success(t *testing.T) {
	app := setupLRTestApp(t)
	user := seedLRUser(t, "lrone@example.com")
	seedLastRead(t, user.ID, "Muslim", 42)
	token := generateLRToken(t, user.ID, user.Email)

	resp := makeLRRequest(t, app, "GET", "/lastRead/Muslim", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeLRJSON(t, resp, &result)
	if result["number"] != float64(42) {
		t.Errorf("number = %v, want 42", result["number"])
	}
}

func TestLRGetOne_NotFound(t *testing.T) {
	app := setupLRTestApp(t)
	user := seedLRUser(t, "lrnotfound@example.com")
	token := generateLRToken(t, user.ID, user.Email)

	resp := makeLRRequest(t, app, "GET", "/lastRead/Muslim", nil, token)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestLRGetOne_NoJWT(t *testing.T) {
	app := setupLRTestApp(t)
	resp := makeLRRequest(t, app, "GET", "/lastRead/Muslim", nil, "")
	if resp.StatusCode == http.StatusOK {
		t.Error("request without JWT should not return 200")
	}
}

func TestLRGetOne_InvalidJWT(t *testing.T) {
	app := setupLRTestApp(t)
	resp := makeLRRequest(t, app, "GET", "/lastRead/Muslim", nil, "bad.token")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestLRGetOne_OtherUsersData_NotFound(t *testing.T) {
	app := setupLRTestApp(t)
	userA := seedLRUser(t, "lr_own_a@example.com")
	userB := seedLRUser(t, "lr_own_b@example.com")
	seedLastRead(t, userA.ID, "Muslim", 42)
	tokenB := generateLRToken(t, userB.ID, userB.Email)

	resp := makeLRRequest(t, app, "GET", "/lastRead/Muslim", nil, tokenB)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d (other user's last-read must be invisible)", resp.StatusCode, http.StatusNotFound)
	}

	tokenA := generateLRToken(t, userA.ID, userA.Email)
	respA := makeLRRequest(t, app, "GET", "/lastRead/Muslim", nil, tokenA)
	if respA.StatusCode != http.StatusOK {
		t.Fatalf("owner status = %d, want %d", respA.StatusCode, http.StatusOK)
	}
	var resultA map[string]interface{}
	decodeLRJSON(t, respA, &resultA)
	if resultA["number"] != float64(42) {
		t.Errorf("owner number = %v, want 42", resultA["number"])
	}
}

// ============================================================
// PUT /lastRead (Update) tests
// ============================================================

func TestLRUpdate_CreateNew(t *testing.T) {
	app := setupLRTestApp(t)
	user := seedLRUser(t, "lrupd@example.com")
	seedLRHadith(t, "Muslim", 10)
	token := generateLRToken(t, user.ID, user.Email)

	body := map[string]interface{}{
		"number":    10,
		"book_name": "Muslim",
	}
	resp := makeLRRequest(t, app, "PUT", "/lastRead", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	var count int
	database.DB.Raw("SELECT COUNT(*) FROM LastRead WHERE user_id = ? AND book_name = 'Muslim'", user.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 last read record, got %d", count)
	}
}

func TestLRUpdate_Existing(t *testing.T) {
	app := setupLRTestApp(t)
	user := seedLRUser(t, "lrupd2@example.com")
	seedLastRead(t, user.ID, "Muslim", 5)
	seedLRHadith(t, "Muslim", 20)
	token := generateLRToken(t, user.ID, user.Email)

	body := map[string]interface{}{
		"number":    20,
		"book_name": "Muslim",
	}
	resp := makeLRRequest(t, app, "PUT", "/lastRead", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	var no int
	database.DB.Raw("SELECT no FROM LastRead WHERE user_id = ? AND book_name = 'Muslim'", user.ID).Scan(&no)
	if no != 20 {
		t.Errorf("no = %d, want 20", no)
	}
}

func TestLRUpdate_CreatesActivity(t *testing.T) {
	app := setupLRTestApp(t)
	user := seedLRUser(t, "lract@example.com")
	seedLRHadith(t, "Muslim", 30)
	token := generateLRToken(t, user.ID, user.Email)

	body := map[string]interface{}{
		"number":    30,
		"book_name": "Muslim",
	}
	resp := makeLRRequest(t, app, "PUT", "/lastRead", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("update failed: status %d", resp.StatusCode)
	}

	var count int
	database.DB.Raw("SELECT COUNT(*) FROM Activity WHERE UserID = ? AND Note = 'Update last read'", user.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 activity, got %d", count)
	}
}

func TestLRUpdate_MissingBookName(t *testing.T) {
	app := setupLRTestApp(t)
	user := seedLRUser(t, "lrmiss@example.com")
	token := generateLRToken(t, user.ID, user.Email)

	body := map[string]interface{}{
		"number": 10,
	}
	resp := makeLRRequest(t, app, "PUT", "/lastRead", body, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestLRUpdate_NoJWT(t *testing.T) {
	app := setupLRTestApp(t)
	body := map[string]interface{}{
		"number":    10,
		"book_name": "Muslim",
	}
	resp := makeLRRequest(t, app, "PUT", "/lastRead", body, "")
	if resp.StatusCode == http.StatusNoContent {
		t.Error("request without JWT should not return 204")
	}
}

func TestLRUpdate_DoesNotOverwriteOtherUsersRow(t *testing.T) {
	app := setupLRTestApp(t)
	userA := seedLRUser(t, "lr_upd_a@example.com")
	userB := seedLRUser(t, "lr_upd_b@example.com")
	seedLastRead(t, userA.ID, "Muslim", 42)
	seedLastRead(t, userB.ID, "Muslim", 7)
	seedLRHadith(t, "Muslim", 99)
	tokenB := generateLRToken(t, userB.ID, userB.Email)

	body := map[string]interface{}{
		"number":    99,
		"book_name": "Muslim",
	}
	resp := makeLRRequest(t, app, "PUT", "/lastRead", body, tokenB)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("update failed: status %d", resp.StatusCode)
	}

	var aNo, bNo int
	database.DB.Raw("SELECT no FROM LastRead WHERE user_id = ? AND book_name = 'Muslim'", userA.ID).Scan(&aNo)
	database.DB.Raw("SELECT no FROM LastRead WHERE user_id = ? AND book_name = 'Muslim'", userB.ID).Scan(&bNo)
	if aNo != 42 {
		t.Errorf("user A number = %d, want 42 (overwritten by user B)", aNo)
	}
	if bNo != 99 {
		t.Errorf("user B number = %d, want 99", bNo)
	}
}

func TestLRUpdate_CreatesOwnRow_NotOthers(t *testing.T) {
	app := setupLRTestApp(t)
	userA := seedLRUser(t, "lr_new_a@example.com")
	userB := seedLRUser(t, "lr_new_b@example.com")
	seedLastRead(t, userA.ID, "Muslim", 42)
	seedLRHadith(t, "Muslim", 15)
	tokenB := generateLRToken(t, userB.ID, userB.Email)

	body := map[string]interface{}{
		"number":    15,
		"book_name": "Muslim",
	}
	resp := makeLRRequest(t, app, "PUT", "/lastRead", body, tokenB)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("update failed: status %d", resp.StatusCode)
	}

	var count int
	database.DB.Raw("SELECT COUNT(*) FROM LastRead WHERE user_id = ? AND book_name = 'Muslim'", userB.ID).Scan(&count)
	if count != 1 {
		t.Errorf("user B rows = %d, want 1", count)
	}

	var aNo int
	database.DB.Raw("SELECT no FROM LastRead WHERE user_id = ? AND book_name = 'Muslim'", userA.ID).Scan(&aNo)
	if aNo != 42 {
		t.Errorf("user A number = %d, want 42 (untouched)", aNo)
	}
}
