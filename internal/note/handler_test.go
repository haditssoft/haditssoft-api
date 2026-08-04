package note

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
	os.Setenv("JWT_SECRET", "test-secret-for-note-unit-tests")
	validator.RegisterCustomValidations()
	os.Exit(m.Run())
}

var noteTestDBCounter int

func setupNoteTestDB(t *testing.T) {
	t.Helper()
	noteTestDBCounter++
	dbName := fmt.Sprintf("file:test_note_%d?mode=memory&cache=shared", noteTestDBCounter)
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

	db.Exec("CREATE TABLE Muslim (id INTEGER PRIMARY KEY AUTOINCREMENT, Nomer INTEGER)")
	db.Exec("CREATE TABLE MuslimNote (id INTEGER PRIMARY KEY AUTOINCREMENT, hadith_id INTEGER, note TEXT, user_id INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)")

	database.DB = db
	t.Cleanup(func() {
		database.DB = nil
	})
}

func setupNoteTestApp(t *testing.T) *fiber.App {
	t.Helper()
	setupNoteTestDB(t)
	repo := NewRepository()
	svc := NewService(repo)
	h := NewHandler(svc)

	app := fiber.New()
	RegisterRoutes(app, h)

	return app
}

func seedNoteUser(t *testing.T, email string) models.User {
	t.Helper()
	user := models.User{
		Email:    email,
		Password: "hashedpassword",
		Active:   boolPtr(true),
		Admin:    boolPtr(false),
	}
	if err := database.DB.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return user
}

func boolPtr(b bool) *bool { return &b }

func seedNoteHadith(t *testing.T, bookName string, nomer uint) {
	t.Helper()
	database.DB.Exec(fmt.Sprintf("INSERT INTO %s (Nomer) VALUES (?)", bookName), nomer)
}

func seedNote(t *testing.T, bookName string, hadithID uint, note string, userID uint) {
	t.Helper()
	database.DB.Exec(fmt.Sprintf("INSERT INTO %sNote (hadith_id, note, user_id, created_at, updated_at) VALUES (?, ?, ?, datetime('now'), datetime('now'))", bookName), hadithID, note, userID)
}

func createNoteTable(t *testing.T, tableName string) {
	t.Helper()
	createSQL := fmt.Sprintf("CREATE TABLE %s (id INTEGER PRIMARY KEY AUTOINCREMENT, hadith_id INTEGER, note TEXT, user_id INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)", tableName)
	if err := database.DB.Exec(createSQL).Error; err != nil {
		t.Fatalf("failed to create table %s: %v", tableName, err)
	}
}

func createKitabTable(t *testing.T, tableName string) {
	t.Helper()
	createSQL := fmt.Sprintf("CREATE TABLE %s (id INTEGER PRIMARY KEY AUTOINCREMENT, Nomer INTEGER)", tableName)
	if err := database.DB.Exec(createSQL).Error; err != nil {
		t.Fatalf("failed to create table %s: %v", tableName, err)
	}
}

func generateNoteTestToken(t *testing.T, userID uint, email string) string {
	t.Helper()
	token, err := auth.GenerateAccessToken(userID, email)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return token
}

func makeNoteRequest(t *testing.T, app *fiber.App, method, path string, body interface{}, token string) *http.Response {
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

func decodeNoteJSON(t *testing.T, resp *http.Response, dest interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

// ============================================================
// GET /notes/:book_name (GetList) tests
// ============================================================

func decodeNotesResponse(t *testing.T, resp *http.Response) map[string]string {
	t.Helper()
	var result map[string]map[string]string
	decodeNoteJSON(t, resp, &result)
	if result == nil {
		t.Fatal("response body is not a JSON object")
	}
	return result["notes"]
}

func TestNoteGetList_Success(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "notelist@example.com")
	seedNoteHadith(t, "Muslim", 1)
	seedNoteHadith(t, "Muslim", 2)
	seedNote(t, "Muslim", 1, "First note", user.ID)
	seedNote(t, "Muslim", 2, "Second note", user.ID)
	token := generateNoteTestToken(t, user.ID, user.Email)

	resp := makeNoteRequest(t, app, "GET", "/notes/Muslim", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	notes := decodeNotesResponse(t, resp)
	if len(notes) != 2 {
		t.Errorf("got %d notes, want 2", len(notes))
	}
	if notes["1"] != "First note" {
		t.Errorf("notes[1] = %q, want 'First note'", notes["1"])
	}
	if notes["2"] != "Second note" {
		t.Errorf("notes[2] = %q, want 'Second note'", notes["2"])
	}
}

func TestNoteGetList_ResponseShape(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "listshape@example.com")
	seedNoteHadith(t, "Muslim", 1)
	seedNote(t, "Muslim", 1, "Shaped note", user.ID)
	token := generateNoteTestToken(t, user.ID, user.Email)

	resp := makeNoteRequest(t, app, "GET", "/notes/Muslim", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var raw map[string]interface{}
	decodeNoteJSON(t, resp, &raw)
	if len(raw) != 1 {
		t.Errorf("expected exactly 1 top-level key, got %d (%v)", len(raw), raw)
	}
	if _, ok := raw["notes"]; !ok {
		t.Error("response must contain top-level 'notes' key")
	}
}

func TestNoteGetList_EmptyForUserWithNoNotes(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "list_empty@example.com")
	seedNoteHadith(t, "Muslim", 1)
	token := generateNoteTestToken(t, user.ID, user.Email)

	resp := makeNoteRequest(t, app, "GET", "/notes/Muslim", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	notes := decodeNotesResponse(t, resp)
	if notes == nil {
		t.Fatal("expected empty 'notes' object, got null")
	}
	if len(notes) != 0 {
		t.Errorf("got %d notes, want 0", len(notes))
	}
}

func TestNoteGetList_OnlyReturnsOwnNotes(t *testing.T) {
	app := setupNoteTestApp(t)
	userA := seedNoteUser(t, "list_owner_a@example.com")
	userB := seedNoteUser(t, "list_owner_b@example.com")
	seedNoteHadith(t, "Muslim", 1)
	seedNoteHadith(t, "Muslim", 2)
	seedNoteHadith(t, "Muslim", 3)
	seedNote(t, "Muslim", 1, "A's first note", userA.ID)
	seedNote(t, "Muslim", 2, "A's second note", userA.ID)
	seedNote(t, "Muslim", 3, "B's private note", userB.ID)
	tokenA := generateNoteTestToken(t, userA.ID, userA.Email)

	resp := makeNoteRequest(t, app, "GET", "/notes/Muslim", nil, tokenA)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	notes := decodeNotesResponse(t, resp)
	if len(notes) != 2 {
		t.Fatalf("got %d notes, want 2 (other user's note leaked)", len(notes))
	}
	for _, note := range notes {
		if note == "B's private note" {
			t.Error("list leaked another user's note content")
		}
	}
}

func TestNoteGetList_ExcludesSoftDeleted(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "listsd@example.com")
	seedNoteHadith(t, "Muslim", 1)
	seedNoteHadith(t, "Muslim", 2)
	seedNote(t, "Muslim", 1, "Active note", user.ID)
	seedNote(t, "Muslim", 2, "Deleted note", user.ID)
	database.DB.Exec("UPDATE MuslimNote SET deleted_at = datetime('now') WHERE hadith_id = 2 AND user_id = ?", user.ID)
	token := generateNoteTestToken(t, user.ID, user.Email)

	resp := makeNoteRequest(t, app, "GET", "/notes/Muslim", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	notes := decodeNotesResponse(t, resp)
	if len(notes) != 1 {
		t.Fatalf("got %d notes, want 1 (soft-deleted note leaked)", len(notes))
	}
	if notes["1"] != "Active note" {
		t.Errorf("notes[1] = %q, want 'Active note'", notes["1"])
	}
	if _, ok := notes["2"]; ok {
		t.Error("soft-deleted note must not be returned")
	}
}

func TestNoteGetList_BookNameAlias(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "listalias@example.com")
	createNoteTable(t, "SunanDarimiNote")
	seedNote(t, "SunanDarimi", 5, "Alias note", user.ID)
	token := generateNoteTestToken(t, user.ID, user.Email)

	resp := makeNoteRequest(t, app, "GET", "/notes/dariminote", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	notes := decodeNotesResponse(t, resp)
	if len(notes) != 1 {
		t.Fatalf("got %d notes, want 1", len(notes))
	}
	if notes["5"] != "Alias note" {
		t.Errorf("notes[5] = %q, want 'Alias note'", notes["5"])
	}
}

func TestNoteGetList_MissingBookName(t *testing.T) {
	app := setupNoteTestApp(t)

	resp := makeNoteRequest(t, app, "GET", "/notes/", nil, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d (route not matched)", resp.StatusCode, http.StatusNotFound)
	}
}

func TestNoteGetList_NoJWT(t *testing.T) {
	app := setupNoteTestApp(t)

	resp := makeNoteRequest(t, app, "GET", "/notes/Muslim", nil, "")
	if resp.StatusCode == http.StatusOK {
		t.Error("request without JWT should not return 200")
	}
}

func TestNoteGetList_InvalidJWT(t *testing.T) {
	app := setupNoteTestApp(t)

	resp := makeNoteRequest(t, app, "GET", "/notes/Muslim", nil, "bad.token")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestNoteGetList_NonexistentTable(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "listnotbl@example.com")
	token := generateNoteTestToken(t, user.ID, user.Email)

	resp := makeNoteRequest(t, app, "GET", "/notes/DoesNotExist", nil, token)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

func TestNoteTableName(t *testing.T) {
	cases := map[string]string{
		"ShahihBukhari":     "ShahihBukhariNote",
		"bukharinote":       "ShahihBukhariNote",
		"muslimnote":        "ShahihMuslimNote",
		"tirmidzinote":      "SunanTirmidziNote",
		"abudaudnote":       "SunanAbuDaudNote",
		"nasainote":         "SunanNasaiNote",
		"ibnumajahnote":     "SunanIbnuMajahNote",
		"dariminote":        "SunanDarimiNote",
		"ahmadnote":         "MusnadAhmadNote",
		"maliknote":         "MuwathaMalikNote",
		"daruquthninote":    "SunanDaruquthniNote",
		"ibnukhuzaimahnote": "ShahihIbnuKhuzaimahNote",
		"ibnuhibbannote":    "ShahihIbnuHibbanNote",
		"mustadraknote":     "AlMustadrakNote",
		"syafiinote":        "MusnadSyafiiNote",
		"Muslim":            "MuslimNote",
		"SunanDarimi":       "SunanDarimiNote",
	}
	for in, want := range cases {
		if got := noteTableName(in); got != want {
			t.Errorf("noteTableName(%q) = %q, want %q", in, got, want)
		}
	}
}

// ============================================================
// GET /notes/:book_name/:hadith_id (GetOne) tests
// ============================================================

func TestNoteGetOne_Success(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "noteone@example.com")
	seedNoteHadith(t, "Muslim", 42)
	seedNote(t, "Muslim", 42, "My hadith note", user.ID)
	token := generateNoteTestToken(t, user.ID, user.Email)

	resp := makeNoteRequest(t, app, "GET", "/notes/Muslim/42", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeNoteJSON(t, resp, &result)
	if result["note"] != "My hadith note" {
		t.Errorf("note = %v, want 'My hadith note'", result["note"])
	}
}

func TestNoteGetOne_NotFound(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "notfound@example.com")
	token := generateNoteTestToken(t, user.ID, user.Email)

	resp := makeNoteRequest(t, app, "GET", "/notes/Muslim/999", nil, token)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestNoteGetOne_InvalidID(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "invalidid@example.com")
	token := generateNoteTestToken(t, user.ID, user.Email)

	resp := makeNoteRequest(t, app, "GET", "/notes/Muslim/abc", nil, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestNoteGetOne_OtherUsersNote_NotFound(t *testing.T) {
	app := setupNoteTestApp(t)
	userA := seedNoteUser(t, "one_owner_a@example.com")
	userB := seedNoteUser(t, "one_owner_b@example.com")
	seedNoteHadith(t, "Muslim", 42)
	seedNote(t, "Muslim", 42, "A's secret note", userA.ID)
	tokenB := generateNoteTestToken(t, userB.ID, userB.Email)

	resp := makeNoteRequest(t, app, "GET", "/notes/Muslim/42", nil, tokenB)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d (other user's note must be invisible)", resp.StatusCode, http.StatusNotFound)
	}

	var result map[string]interface{}
	decodeNoteJSON(t, resp, &result)
	if result["note"] == "A's secret note" {
		t.Error("GetOne leaked another user's note content")
	}
}

func TestNoteGetOne_SameHadithDistinctUsers(t *testing.T) {
	app := setupNoteTestApp(t)
	userA := seedNoteUser(t, "one_same_a@example.com")
	userB := seedNoteUser(t, "one_same_b@example.com")
	seedNoteHadith(t, "Muslim", 42)
	seedNote(t, "Muslim", 42, "A's note on hadith 42", userA.ID)
	seedNote(t, "Muslim", 42, "B's note on hadith 42", userB.ID)
	tokenA := generateNoteTestToken(t, userA.ID, userA.Email)
	tokenB := generateNoteTestToken(t, userB.ID, userB.Email)

	respA := makeNoteRequest(t, app, "GET", "/notes/Muslim/42", nil, tokenA)
	if respA.StatusCode != http.StatusOK {
		t.Fatalf("user A status = %d, want %d", respA.StatusCode, http.StatusOK)
	}
	var resultA map[string]interface{}
	decodeNoteJSON(t, respA, &resultA)
	if resultA["note"] != "A's note on hadith 42" {
		t.Errorf("user A note = %v, want 'A's note on hadith 42'", resultA["note"])
	}

	respB := makeNoteRequest(t, app, "GET", "/notes/Muslim/42", nil, tokenB)
	if respB.StatusCode != http.StatusOK {
		t.Fatalf("user B status = %d, want %d", respB.StatusCode, http.StatusOK)
	}
	var resultB map[string]interface{}
	decodeNoteJSON(t, respB, &resultB)
	if resultB["note"] != "B's note on hadith 42" {
		t.Errorf("user B note = %v, want 'B's note on hadith 42'", resultB["note"])
	}
}

func TestNoteGetOne_WithBookAlias(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "onealias@example.com")
	createNoteTable(t, "MusnadAhmadNote")
	seedNote(t, "MusnadAhmad", 9, "Alias one note", user.ID)
	token := generateNoteTestToken(t, user.ID, user.Email)

	resp := makeNoteRequest(t, app, "GET", "/notes/ahmadnote/9", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeNoteJSON(t, resp, &result)
	if result["note"] != "Alias one note" {
		t.Errorf("note = %v, want 'Alias one note'", result["note"])
	}
}

// ============================================================
// POST /notes/:book_name/:hadith_id (Create) tests
// ============================================================

func TestNoteCreate_Success(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "notecreate@example.com")
	seedNoteHadith(t, "Muslim", 10)
	token := generateNoteTestToken(t, user.ID, user.Email)

	body := map[string]interface{}{
		"note": "This is a test note",
	}
	resp := makeNoteRequest(t, app, "POST", "/notes/Muslim/10", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestNoteCreate_WithBookAlias(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "createalias@example.com")
	createKitabTable(t, "SunanDarimi")
	seedNoteHadith(t, "SunanDarimi", 7)
	createNoteTable(t, "SunanDarimiNote")
	token := generateNoteTestToken(t, user.ID, user.Email)

	body := map[string]interface{}{"note": "Alias created note"}
	resp := makeNoteRequest(t, app, "POST", "/notes/dariminote/7", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	var count int
	database.DB.Raw("SELECT COUNT(*) FROM SunanDarimiNote WHERE hadith_id = 7 AND deleted_at IS NULL").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 note in SunanDarimiNote, got %d", count)
	}
}

func TestNoteCreate_VerifiesRow(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "noteverify@example.com")
	seedNoteHadith(t, "Muslim", 20)
	token := generateNoteTestToken(t, user.ID, user.Email)

	body := map[string]interface{}{
		"note": "Persisted note",
	}
	resp := makeNoteRequest(t, app, "POST", "/notes/Muslim/20", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("create failed: status %d", resp.StatusCode)
	}

	var count int
	database.DB.Raw("SELECT COUNT(*) FROM MuslimNote WHERE hadith_id = 20 AND deleted_at IS NULL").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 note row, got %d", count)
	}
}

func TestNoteCreate_MissingNote(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "notemissnote@example.com")
	seedNoteHadith(t, "Muslim", 30)
	token := generateNoteTestToken(t, user.ID, user.Email)

	body := map[string]interface{}{}
	resp := makeNoteRequest(t, app, "POST", "/notes/Muslim/30", body, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestNoteCreate_InvalidHadithID(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "noteinvalidhid@example.com")
	token := generateNoteTestToken(t, user.ID, user.Email)

	body := map[string]interface{}{
		"note": "Some note",
	}
	resp := makeNoteRequest(t, app, "POST", "/notes/Muslim/abc", body, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestNoteCreate_NoJWT(t *testing.T) {
	app := setupNoteTestApp(t)

	body := map[string]interface{}{"note": "test"}
	resp := makeNoteRequest(t, app, "POST", "/notes/Muslim/10", body, "")
	if resp.StatusCode == http.StatusNoContent {
		t.Error("request without JWT should not return 204")
	}
}

func TestNoteCreate_CreatesActivity(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "noteact@example.com")
	seedNoteHadith(t, "Muslim", 50)
	token := generateNoteTestToken(t, user.ID, user.Email)

	body := map[string]interface{}{
		"note": "Activity note",
	}
	resp := makeNoteRequest(t, app, "POST", "/notes/Muslim/50", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("create failed: status %d", resp.StatusCode)
	}

	var count int
	database.DB.Raw("SELECT COUNT(*) FROM Activity WHERE UserID = ? AND Note = 'Create new note'", user.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 activity, got %d", count)
	}
}

// ============================================================
// PUT /notes/:book_name/:hadith_id (Update) tests
// ============================================================

func TestNoteUpdate_Success(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "noteupd@example.com")
	seedNoteHadith(t, "Muslim", 1)
	seedNote(t, "Muslim", 1, "Old note", user.ID)

	token := generateNoteTestToken(t, user.ID, user.Email)

	body := map[string]interface{}{
		"note":      "Updated note",
		"book_name": "Muslim",
	}
	resp := makeNoteRequest(t, app, "PUT", "/notes/Muslim/1", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestNoteUpdate_VerifiesContent(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "noteupd2@example.com")
	seedNoteHadith(t, "Muslim", 2)
	seedNote(t, "Muslim", 2, "Original", user.ID)

	token := generateNoteTestToken(t, user.ID, user.Email)

	body := map[string]interface{}{
		"note":      "Changed text",
		"book_name": "Muslim",
	}
	resp := makeNoteRequest(t, app, "PUT", "/notes/Muslim/2", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("update failed: status %d", resp.StatusCode)
	}

	var note string
	database.DB.Raw("SELECT note FROM MuslimNote WHERE hadith_id = 2 AND deleted_at IS NULL").Scan(&note)
	if note != "Changed text" {
		t.Errorf("note = %q, want 'Changed text'", note)
	}
}

func TestNoteUpdate_InvalidID(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "noteupdid@example.com")
	token := generateNoteTestToken(t, user.ID, user.Email)

	body := map[string]interface{}{
		"hadith_id": 1,
		"note":      "test",
		"book_name": "Muslim",
	}
	resp := makeNoteRequest(t, app, "PUT", "/notes/Muslim/abc", body, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestNoteUpdate_NoJWT(t *testing.T) {
	app := setupNoteTestApp(t)

	body := map[string]interface{}{
		"hadith_id": 1,
		"note":      "test",
		"book_name": "Muslim",
	}
	resp := makeNoteRequest(t, app, "PUT", "/notes/Muslim/1", body, "")
	if resp.StatusCode == http.StatusNoContent {
		t.Error("request without JWT should not return 204")
	}
}

func TestNoteUpdate_CreatesActivity(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "noteupact@example.com")
	seedNoteHadith(t, "Muslim", 3)
	seedNote(t, "Muslim", 3, "Act note", user.ID)
	token := generateNoteTestToken(t, user.ID, user.Email)

	body := map[string]interface{}{
		"note":      "Updated act note",
		"book_name": "Muslim",
	}
	resp := makeNoteRequest(t, app, "PUT", "/notes/Muslim/3", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("update failed: status %d", resp.StatusCode)
	}

	var count int
	database.DB.Raw("SELECT COUNT(*) FROM Activity WHERE UserID = ? AND Note = 'Update note'", user.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 activity, got %d", count)
	}
}

// ============================================================
// DELETE /notes/:book_name/:hadith_id (DeleteOne) tests
// ============================================================

func TestNoteDeleteOne_Success(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "notedel@example.com")
	seedNoteHadith(t, "Muslim", 5)
	seedNote(t, "Muslim", 5, "To be deleted", user.ID)
	token := generateNoteTestToken(t, user.ID, user.Email)

	resp := makeNoteRequest(t, app, "DELETE", "/notes/Muslim/5", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeNoteJSON(t, resp, &result)
	if result["status"] != "success" {
		t.Errorf("status = %v, want 'success'", result["status"])
	}
}

func TestNoteDeleteOne_VerifiesSoftDelete(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "notedelsd@example.com")
	seedNoteHadith(t, "Muslim", 6)
	seedNote(t, "Muslim", 6, "Soft delete me", user.ID)
	token := generateNoteTestToken(t, user.ID, user.Email)

	resp := makeNoteRequest(t, app, "DELETE", "/notes/Muslim/6", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete failed: status %d", resp.StatusCode)
	}

	var count int
	database.DB.Raw("SELECT COUNT(*) FROM MuslimNote WHERE hadith_id = 6 AND deleted_at IS NULL").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 active notes, got %d", count)
	}

	var softDeleted int
	database.DB.Raw("SELECT COUNT(*) FROM MuslimNote WHERE hadith_id = 6 AND deleted_at IS NOT NULL").Scan(&softDeleted)
	if softDeleted != 1 {
		t.Errorf("expected 1 soft-deleted note, got %d", softDeleted)
	}
}

func TestNoteDeleteOne_CreatesActivity(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "notedelact@example.com")
	seedNoteHadith(t, "Muslim", 7)
	seedNote(t, "Muslim", 7, "Delete act", user.ID)
	token := generateNoteTestToken(t, user.ID, user.Email)

	resp := makeNoteRequest(t, app, "DELETE", "/notes/Muslim/7", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete failed: status %d", resp.StatusCode)
	}

	var count int
	database.DB.Raw("SELECT COUNT(*) FROM Activity WHERE UserID = ? AND Note = 'Delete note'", user.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 activity, got %d", count)
	}
}

func TestNoteDeleteOne_InvalidID(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "notedelinv@example.com")
	token := generateNoteTestToken(t, user.ID, user.Email)

	resp := makeNoteRequest(t, app, "DELETE", "/notes/Muslim/abc", nil, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestNoteDeleteOne_NoJWT(t *testing.T) {
	app := setupNoteTestApp(t)

	resp := makeNoteRequest(t, app, "DELETE", "/notes/Muslim/1", nil, "")
	if resp.StatusCode == http.StatusOK {
		t.Error("request without JWT should not return 200")
	}
}

// ============================================================
// GET /notes/validate-delete/:book_name/:hadith_id (ValidateDelete) tests
// ============================================================

func TestNoteValidateDelete_Success(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "notevdel@example.com")
	seedNoteHadith(t, "Muslim", 1)
	seedNote(t, "Muslim", 1, "Existing note", user.ID)
	token := generateNoteTestToken(t, user.ID, user.Email)

	resp := makeNoteRequest(t, app, "GET", "/notes/validate-delete/Muslim/1", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeNoteJSON(t, resp, &result)
	if result["status"] != "success" {
		t.Errorf("status = %v, want 'success'", result["status"])
	}
}

func TestNoteValidateDelete_InvalidID(t *testing.T) {
	app := setupNoteTestApp(t)
	user := seedNoteUser(t, "notevdelinv@example.com")
	token := generateNoteTestToken(t, user.ID, user.Email)

	resp := makeNoteRequest(t, app, "GET", "/notes/validate-delete/Muslim/abc", nil, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestNoteValidateDelete_NoJWT(t *testing.T) {
	app := setupNoteTestApp(t)

	resp := makeNoteRequest(t, app, "GET", "/notes/validate-delete/Muslim/1", nil, "")
	if resp.StatusCode == http.StatusOK {
		t.Error("request without JWT should not return 200")
	}
}
