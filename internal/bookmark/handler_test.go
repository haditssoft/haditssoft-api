package bookmark

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
	"github.com/haditssoft/haditssoft-backend/internal/shared/middleware"
	"github.com/haditssoft/haditssoft-backend/internal/shared/validator"
	"github.com/haditssoft/haditssoft-backend/models"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestMain(m *testing.M) {
	os.Setenv("JWT_SECRET", "test-secret-for-bookmark-unit-tests")
	validator.RegisterCustomValidations()
	os.Exit(m.Run())
}

var testDBCounter int

func setupBookmarkTestDB(t *testing.T) {
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
	if err := db.AutoMigrate(&models.User{}, &models.Bookmark{}, &models.BookmarkItem{}, &models.Activity{}, &models.BlacklistToken{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	database.DB = db
	t.Cleanup(func() {
		database.DB = nil
	})
}

func setupBookmarkTestApp(t *testing.T) *fiber.App {
	t.Helper()
	setupBookmarkTestDB(t)
	repo := NewRepository()
	svc := NewService(repo)
	h := NewHandler(svc)

	app := fiber.New()
	RegisterRoutes(app, h, middleware.Protected())

	return app
}

func seedBookmarkUser(t *testing.T, email string) models.User {
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

func seedBookmark(t *testing.T, userID uint, title string) models.Bookmark {
	t.Helper()
	bookmark := models.Bookmark{
		UserID: userID,
		Title:  title,
	}
	if err := database.DB.Create(&bookmark).Error; err != nil {
		t.Fatalf("failed to seed bookmark: %v", err)
	}
	return bookmark
}

func seedBookmarkItem(t *testing.T, bookmarkID uint, bookName string, bookNumber string) models.BookmarkItem {
	t.Helper()
	item := models.BookmarkItem{
		BookmarkID: bookmarkID,
		BookName:   bookName,
		BookNumber: bookNumber,
	}
	if err := database.DB.Create(&item).Error; err != nil {
		t.Fatalf("failed to seed bookmark item: %v", err)
	}
	return item
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

func makeRequest(t *testing.T, app *fiber.App, method, path string, body interface{}, token string) *http.Response {
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
// GET /bookmarks (GetList) tests
// ============================================================

func TestGetList_Success(t *testing.T) {
	app := setupBookmarkTestApp(t)
	user := seedBookmarkUser(t, "list@example.com")
	seedBookmark(t, user.ID, "Favorites")
	seedBookmark(t, user.ID, "Read Later")
	token := generateTestToken(t, user.ID, user.Email)

	resp := makeRequest(t, app, "GET", "/bookmarks", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result []string
	decodeJSON(t, resp, &result)
	if len(result) != 2 {
		t.Errorf("got %d titles, want 2", len(result))
	}
	if result[0] != "Favorites" || result[1] != "Read Later" {
		t.Errorf("titles = %v, want [Favorites, Read Later]", result)
	}
}

func TestGetList_NotFound(t *testing.T) {
	app := setupBookmarkTestApp(t)
	user := seedBookmarkUser(t, "empty@example.com")
	token := generateTestToken(t, user.ID, user.Email)

	resp := makeRequest(t, app, "GET", "/bookmarks", nil, token)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGetList_NoJWT(t *testing.T) {
	app := setupBookmarkTestApp(t)

	resp := makeRequest(t, app, "GET", "/bookmarks", nil, "")
	if resp.StatusCode == http.StatusOK {
		t.Error("request without JWT should not return 200")
	}
}

func TestGetList_InvalidJWT(t *testing.T) {
	app := setupBookmarkTestApp(t)

	resp := makeRequest(t, app, "GET", "/bookmarks", nil, "bad.token")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestGetList_OnlyReturnsOwnTitles(t *testing.T) {
	app := setupBookmarkTestApp(t)
	userA := seedBookmarkUser(t, "list_own_a@example.com")
	userB := seedBookmarkUser(t, "list_own_b@example.com")
	seedBookmark(t, userA.ID, "A's private list")
	tokenB := generateTestToken(t, userB.ID, userB.Email)

	resp := makeRequest(t, app, "GET", "/bookmarks", nil, tokenB)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d (other user's titles must be invisible)", resp.StatusCode, http.StatusNotFound)
	}

	tokenA := generateTestToken(t, userA.ID, userA.Email)
	respA := makeRequest(t, app, "GET", "/bookmarks", nil, tokenA)
	if respA.StatusCode != http.StatusOK {
		t.Errorf("owner status = %d, want %d", respA.StatusCode, http.StatusOK)
	}
	var result []string
	decodeJSON(t, respA, &result)
	if len(result) != 1 || result[0] != "A's private list" {
		t.Errorf("titles = %v, want ['A's private list']", result)
	}
}

// ============================================================
// POST /bookmarks (Create) tests
// ============================================================

func TestCreate_Success(t *testing.T) {
	app := setupBookmarkTestApp(t)
	user := seedBookmarkUser(t, "create@example.com")
	token := generateTestToken(t, user.ID, user.Email)

	body := map[string]interface{}{
		"title": "My Bookmarks",
		"items": map[string]interface{}{
			"book_name":   "ShahihBukhari",
			"book_number": 1,
		},
	}
	resp := makeRequest(t, app, "POST", "/bookmarks", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestCreate_StoresBookmark(t *testing.T) {
	app := setupBookmarkTestApp(t)
	user := seedBookmarkUser(t, "createbm@example.com")
	token := generateTestToken(t, user.ID, user.Email)

	body := map[string]interface{}{
		"title": "Test Bookmark",
		"items": map[string]interface{}{
			"book_name":   "ShahihBukhari",
			"book_number": 42,
		},
	}
	resp := makeRequest(t, app, "POST", "/bookmarks", body, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("create failed: status %d", resp.StatusCode)
	}

	var bookmark models.Bookmark
	result := database.DB.Where("user_id = ? AND title = ?", user.ID, "Test Bookmark").First(&bookmark)
	if result.Error != nil {
		t.Fatalf("bookmark not found: %v", result.Error)
	}

	var item models.BookmarkItem
	result = database.DB.Where("bookmark_id = ?", bookmark.ID).First(&item)
	if result.Error != nil {
		t.Fatalf("bookmark item not found: %v", result.Error)
	}
	if item.BookName != "ShahihBukhari" {
		t.Errorf("book_name = %q, want 'ShahihBukhari'", item.BookName)
	}
}

func TestCreate_MissingTitle(t *testing.T) {
	app := setupBookmarkTestApp(t)
	user := seedBookmarkUser(t, "notitle@example.com")
	token := generateTestToken(t, user.ID, user.Email)

	body := map[string]interface{}{
		"items": map[string]interface{}{
			"book_name":   "ShahihBukhari",
			"book_number": 1,
		},
	}
	resp := makeRequest(t, app, "POST", "/bookmarks", body, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestCreate_NoJWT(t *testing.T) {
	app := setupBookmarkTestApp(t)

	body := map[string]interface{}{
		"title": "Test",
		"items": map[string]interface{}{"book_name": "Test", "book_number": 1},
	}
	resp := makeRequest(t, app, "POST", "/bookmarks", body, "")
	if resp.StatusCode == http.StatusNoContent {
		t.Error("request without JWT should not return 204")
	}
}

func TestCreate_IgnoresSpoofedUserID(t *testing.T) {
	app := setupBookmarkTestApp(t)
	userA := seedBookmarkUser(t, "spoof_a@example.com")
	userB := seedBookmarkUser(t, "spoof_b@example.com")
	tokenB := generateTestToken(t, userB.ID, userB.Email)

	body := map[string]interface{}{
		"user_id": userA.ID,
		"title":   "Spoofed List",
		"items": map[string]interface{}{
			"book_name":   "ShahihBukhari",
			"book_number": 1,
		},
	}
	resp := makeRequest(t, app, "POST", "/bookmarks", body, tokenB)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("create failed: status %d", resp.StatusCode)
	}

	var bookmark models.Bookmark
	if err := database.DB.Where("title = ?", "Spoofed List").First(&bookmark).Error; err != nil {
		t.Fatalf("bookmark not found: %v", err)
	}
	if bookmark.UserID != userB.ID {
		t.Errorf("bookmark stored under user_id = %d, want %d (spoofed user_id accepted)", bookmark.UserID, userB.ID)
	}
}

// ============================================================
// GET /bookmarks/:title (GetOne) tests
// ============================================================

func TestGetOne_Success(t *testing.T) {
	app := setupBookmarkTestApp(t)
	user := seedBookmarkUser(t, "getone@example.com")
	bm := seedBookmark(t, user.ID, "Favorites")
	seedBookmarkItem(t, bm.ID, "ShahihBukhari", "1")
	seedBookmarkItem(t, bm.ID, "ShahihBukhari", "2")
	seedBookmarkItem(t, bm.ID, "ShahihMuslim", "5")
	token := generateTestToken(t, user.ID, user.Email)

	resp := makeRequest(t, app, "GET", "/bookmarks/Favorites", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string][]int
	decodeJSON(t, resp, &result)
	if len(result["ShahihBukhari"]) != 2 {
		t.Errorf("ShahihBukhari count = %d, want 2", len(result["ShahihBukhari"]))
	}
	if len(result["ShahihMuslim"]) != 1 {
		t.Errorf("ShahihMuslim count = %d, want 1", len(result["ShahihMuslim"]))
	}
}

func TestGetOne_EmptyTitle(t *testing.T) {
	app := setupBookmarkTestApp(t)
	user := seedBookmarkUser(t, "emptytitle@example.com")
	token := generateTestToken(t, user.ID, user.Email)

	resp := makeRequest(t, app, "GET", "/bookmarks/", nil, token)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d (should 404 for unmatched route)", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGetOne_OtherUsersTitle_NotExposed(t *testing.T) {
	app := setupBookmarkTestApp(t)
	userA := seedBookmarkUser(t, "one_own_a@example.com")
	userB := seedBookmarkUser(t, "one_own_b@example.com")
	bm := seedBookmark(t, userA.ID, "SecretList")
	seedBookmarkItem(t, bm.ID, "ShahihBukhari", "1")
	tokenB := generateTestToken(t, userB.ID, userB.Email)

	resp := makeRequest(t, app, "GET", "/bookmarks/SecretList", nil, tokenB)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string][]int
	decodeJSON(t, resp, &result)
	if len(result) != 0 {
		t.Errorf("got %d book entries, want 0 (other user's bookmarks leaked)", len(result))
	}

	tokenA := generateTestToken(t, userA.ID, userA.Email)
	respA := makeRequest(t, app, "GET", "/bookmarks/SecretList", nil, tokenA)
	if respA.StatusCode != http.StatusOK {
		t.Fatalf("owner status = %d, want %d", respA.StatusCode, http.StatusOK)
	}
	var resultA map[string][]int
	decodeJSON(t, respA, &resultA)
	if len(resultA["ShahihBukhari"]) != 1 {
		t.Errorf("owner got %d numbers, want 1", len(resultA["ShahihBukhari"]))
	}
}

// ============================================================
// GET /bookmarks/:title/:book_name (GetSome) tests
// ============================================================

func TestGetSome_Success(t *testing.T) {
	app := setupBookmarkTestApp(t)
	user := seedBookmarkUser(t, "getsome@example.com")
	bm := seedBookmark(t, user.ID, "Favorites")
	seedBookmarkItem(t, bm.ID, "ShahihBukhari", "1")
	seedBookmarkItem(t, bm.ID, "ShahihBukhari", "5")
	seedBookmarkItem(t, bm.ID, "ShahihMuslim", "10")
	token := generateTestToken(t, user.ID, user.Email)

	resp := makeRequest(t, app, "GET", "/bookmarks/Favorites/ShahihBukhari", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result []int64
	decodeJSON(t, resp, &result)
	if len(result) != 2 {
		t.Errorf("got %d numbers, want 2", len(result))
	}
}

func TestGetSome_EmptyBookName(t *testing.T) {
	app := setupBookmarkTestApp(t)
	user := seedBookmarkUser(t, "emptybn@example.com")
	token := generateTestToken(t, user.ID, user.Email)

	resp := makeRequest(t, app, "GET", "/bookmarks/Favorites/", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d (matches /:title route)", resp.StatusCode, http.StatusOK)
	}
}

func TestGetSome_OtherUsersBook_Empty(t *testing.T) {
	app := setupBookmarkTestApp(t)
	userA := seedBookmarkUser(t, "some_own_a@example.com")
	userB := seedBookmarkUser(t, "some_own_b@example.com")
	bm := seedBookmark(t, userA.ID, "SecretList")
	seedBookmarkItem(t, bm.ID, "ShahihBukhari", "1")
	tokenB := generateTestToken(t, userB.ID, userB.Email)

	resp := makeRequest(t, app, "GET", "/bookmarks/SecretList/ShahihBukhari", nil, tokenB)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result []int64
	decodeJSON(t, resp, &result)
	if len(result) != 0 {
		t.Errorf("got %d numbers, want 0 (other user's book numbers leaked)", len(result))
	}
}

// ============================================================
// PUT /bookmarks/:title/:book_name (UpdateAll) tests
// ============================================================

func TestUpdateAll_Success(t *testing.T) {
	app := setupBookmarkTestApp(t)
	user := seedBookmarkUser(t, "updateall@example.com")
	bm := seedBookmark(t, user.ID, "Favorites")
	item := seedBookmarkItem(t, bm.ID, "ShahihBukhari", "5")
	token := generateTestToken(t, user.ID, user.Email)

	payload := []uint{1, 2, 3}
	resp := makeRequest(t, app, "PUT", "/bookmarks/Favorites/ShahihBukhari", payload, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	var updated models.BookmarkItem
	database.DB.Unscoped().Where("id = ?", item.ID).First(&updated)
	if updated.DeletedAt.Time.IsZero() {
		t.Error("bookmark item should be soft-deleted")
	}
}

func TestUpdateAll_EmptyPayload(t *testing.T) {
	app := setupBookmarkTestApp(t)
	user := seedBookmarkUser(t, "emptyupdate@example.com")
	token := generateTestToken(t, user.ID, user.Email)

	payload := []uint{}
	resp := makeRequest(t, app, "PUT", "/bookmarks/Favorites/ShahihBukhari", payload, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestUpdateAll_NoMatchingItem(t *testing.T) {
	app := setupBookmarkTestApp(t)
	user := seedBookmarkUser(t, "nomatch@example.com")
	bm := seedBookmark(t, user.ID, "Favorites")
	seedBookmarkItem(t, bm.ID, "ShahihBukhari", "1")
	token := generateTestToken(t, user.ID, user.Email)

	payload := []uint{1}
	resp := makeRequest(t, app, "PUT", "/bookmarks/Favorites/ShahihBukhari", payload, token)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestUpdateAll_OtherUsersItem_NotFound(t *testing.T) {
	app := setupBookmarkTestApp(t)
	userA := seedBookmarkUser(t, "upd_own_a@example.com")
	userB := seedBookmarkUser(t, "upd_own_b@example.com")
	bm := seedBookmark(t, userA.ID, "SecretList")
	item := seedBookmarkItem(t, bm.ID, "ShahihBukhari", "1")
	tokenB := generateTestToken(t, userB.ID, userB.Email)

	payload := []uint{999}
	resp := makeRequest(t, app, "PUT", "/bookmarks/SecretList/ShahihBukhari", payload, tokenB)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d (must not modify other user's item)", resp.StatusCode, http.StatusNotFound)
	}

	var stored models.BookmarkItem
	database.DB.Where("id = ?", item.ID).First(&stored)
	if stored.DeletedAt.Valid {
		t.Error("other user's bookmark item was soft-deleted")
	}
}

// ============================================================
// DELETE /bookmarks/:title/:book_name (DeleteParent) tests
// ============================================================

func TestDeleteParent_Success(t *testing.T) {
	app := setupBookmarkTestApp(t)
	user := seedBookmarkUser(t, "delparent@example.com")
	bm := seedBookmark(t, user.ID, "Favorites")
	item := seedBookmarkItem(t, bm.ID, "ShahihBukhari", "1")
	token := generateTestToken(t, user.ID, user.Email)

	resp := makeRequest(t, app, "DELETE", "/bookmarks/Favorites/ShahihBukhari", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)
	if result["status"] != "success" {
		t.Errorf("status = %v, want 'success'", result["status"])
	}

	var deleted models.BookmarkItem
	database.DB.Unscoped().Where("id = ?", item.ID).First(&deleted)
	if deleted.DeletedAt.Time.IsZero() {
		t.Error("bookmark item should be soft-deleted")
	}
}

func TestDeleteParent_NotFound(t *testing.T) {
	app := setupBookmarkTestApp(t)
	user := seedBookmarkUser(t, "delnotfound@example.com")
	token := generateTestToken(t, user.ID, user.Email)

	resp := makeRequest(t, app, "DELETE", "/bookmarks/Favorites/ShahihBukhari", nil, token)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

func TestDeleteParent_NoJWT(t *testing.T) {
	app := setupBookmarkTestApp(t)

	resp := makeRequest(t, app, "DELETE", "/bookmarks/Favorites/ShahihBukhari", nil, "")
	if resp.StatusCode == http.StatusOK {
		t.Error("request without JWT should not return 200")
	}
}

func TestDeleteParent_OtherUsersItem_NotDeleted(t *testing.T) {
	app := setupBookmarkTestApp(t)
	userA := seedBookmarkUser(t, "del_own_a@example.com")
	userB := seedBookmarkUser(t, "del_own_b@example.com")
	bm := seedBookmark(t, userA.ID, "SecretList")
	item := seedBookmarkItem(t, bm.ID, "ShahihBukhari", "1")
	tokenB := generateTestToken(t, userB.ID, userB.Email)

	resp := makeRequest(t, app, "DELETE", "/bookmarks/SecretList/ShahihBukhari", nil, tokenB)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d (must not delete other user's item)", resp.StatusCode, http.StatusInternalServerError)
	}

	var stored models.BookmarkItem
	database.DB.Where("id = ?", item.ID).First(&stored)
	if stored.DeletedAt.Valid {
		t.Error("other user's bookmark item was soft-deleted")
	}
}
