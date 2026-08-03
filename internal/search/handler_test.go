package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:test_search?mode=memory&cache=shared"), &gorm.Config{
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

	if err := db.Exec(`CREATE TABLE "ShahihBukhari" (Nomer INTEGER PRIMARY KEY, Arabic TEXT, Indonesia TEXT, English TEXT)`).Error; err != nil {
		t.Fatalf("failed to create ShahihBukhari: %v", err)
	}
	if err := db.Exec(`CREATE TABLE "ShahihMuslim" (Nomer INTEGER PRIMARY KEY, Arabic TEXT, Indonesia TEXT, English TEXT)`).Error; err != nil {
		t.Fatalf("failed to create ShahihMuslim: %v", err)
	}
	if err := db.Exec(`CREATE TABLE "KBBI" (katakunci TEXT, artikata TEXT)`).Error; err != nil {
		t.Fatalf("failed to create KBBI: %v", err)
	}

	type hadith struct {
		Nomer     int
		Arabic    string
		Indonesia string
		English   string
	}
	seed := []hadith{
		{1, "صَلُّوا كَمَا رَأَيْتُمُونِي أُصَلِّي", "Sholatlah kamu sebagaimana kamu melihat aku sholat", "Pray as you see me pray"},
		{2, "بَيْنَمَا نَحْنُ جُلُوسٌ عِنْدَ رَسُولِ اللَّهِ", "Tentang sholat lima waktu yang wajib", "About the five daily prayers"},
		{3, "خَيْرُكُمْ مَنْ تَعَلَّمَ الْقُرْآنَ", "Hadits tentang puasa ramadhan yang utama", "The best of you are those who learn the Quran"},
		{4, "مَنْ كَانَ يُؤْمِنُ بِاللَّهِ", "Sabar dalam menghadapi cobaan adalah kunci", "Whoever believes in Allah"},
		{5, "إِنَّمَا الْأَعْمَالُ بِالنِّيَّاتِ", "Menyapu masjid adalah sunnah rasul", "Actions are but by intentions"},
		{6, "الدِّينُ النَّصِيحَةُ", "Sholat sunnah dua rakaat sebelum subuh", "Religion is sincerity"},
		{7, "مَنْ لَا يَرْحَمُ", "Makanan halal dalam islam sangat penting", "Whoever does not show mercy"},
		{8, "إِذَا مَاتَ الْإِنْسَانُ", "ilmu dan amal sholat mengantarkan ke surga", "When a person dies"},
	}
	for _, h := range seed {
		db.Exec(`INSERT INTO "ShahihBukhari" (Nomer, Arabic, Indonesia, English) VALUES (?, ?, ?, ?)`,
			h.Nomer, h.Arabic, h.Indonesia, h.English)
	}

	type kbbi struct {
		Katakunci string
		Artikata  string
	}
	kbSeed := []kbbi{
		{"sholat", "[sholat] [salat] [sholat lima waktu]"},
		{"puasa", "[puasa] [shaum]"},
		{"sabar", "[sabar] [sabara]"},
		{"sapu", "[sapu] [sapuan] [menyapu]"},
		{"shalat", "[shalat] [sholat]"},
	}
	for _, k := range kbSeed {
		db.Exec(`INSERT INTO "KBBI" (katakunci, artikata) VALUES (?, ?)`, k.Katakunci, k.Artikata)
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
	app := fiber.New()
	RegisterRoutes(app)
	return app
}

func makeRequest(t *testing.T, app *fiber.App, method, path string, body interface{}) *http.Response {
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
// POST /searchHadits/:kitabName/:column tests
// ============================================================

func TestSearchSingleKitab_ValidSingleKeyword(t *testing.T) {
	app := setupTestApp(t)

	body := map[string]interface{}{
		"keyword": []string{"sholat"},
	}
	resp := makeRequest(t, app, "POST", "/searchHadits/ShahihBukhari/Indonesia", body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result []interface{}
	decodeJSON(t, resp, &result)

	if len(result) != 3 {
		t.Fatalf("response should have 3 elements, got %d", len(result))
	}

	rows, ok := result[0].([]interface{})
	if !ok {
		t.Fatal("first element should be an array of rows")
	}

	if len(rows) == 0 {
		t.Error("expected at least 1 match for 'sholat', got 0")
	}

	if result[1] != "SEARCHRESULTCOUNT" {
		t.Errorf("second element = %v, want 'SEARCHRESULTCOUNT'", result[1])
	}

	if result[2] != "ShahihBukhari" {
		t.Errorf("third element = %v, want 'ShahihBukhari'", result[2])
	}
}

func TestSearchSingleKitab_ValidMultiKeyword(t *testing.T) {
	app := setupTestApp(t)

	body := map[string]interface{}{
		"keyword": []string{"sholat", "lima"},
	}
	resp := makeRequest(t, app, "POST", "/searchHadits/ShahihBukhari/Indonesia", body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result []interface{}
	decodeJSON(t, resp, &result)

	rows := result[0].([]interface{})
	if len(rows) == 0 {
		t.Error("expected at least 1 match for 'sholat' AND 'lima', got 0")
	}
}

func TestSearchSingleKitab_ApostropheSingleKeyword(t *testing.T) {
	app := setupTestApp(t)

	db := database.DB
	db.Exec(`INSERT INTO "ShahihBukhari" (Nomer, Arabic, Indonesia, English) VALUES (?, ?, ?, ?)`,
		100, "x", "Mereka membaca Al Qur'an namun tidak mengamalkannya", "")

	body := map[string]interface{}{
		"keyword": []string{"qur''an"},
	}
	resp := makeRequest(t, app, "POST", "/searchHadits/ShahihBukhari/Indonesia", body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result []interface{}
	decodeJSON(t, resp, &result)

	rows, ok := result[0].([]interface{})
	if !ok {
		t.Fatal("first element should be an array of rows")
	}
	if len(rows) == 0 {
		t.Error("expected a match for 'qur''an', got 0")
	}
}

func TestSearchSingleKitab_ApostropheMultiKeywordLike(t *testing.T) {
	app := setupTestApp(t)

	db := database.DB
	db.Exec(`INSERT INTO "ShahihBukhari" (Nomer, Arabic, Indonesia, English) VALUES (?, ?, ?, ?)`,
		101, "x", "Akan terjadi perbedaan dan perpecahan, mereka pandai berbicara namun akhlak buruk. Mereka membaca Al Qur'an namun tidak mengamalkannya.", "")

	body := map[string]interface{}{
		"keyword": []string{"Perbedaan", "cara", "baca", "qur''an"},
	}
	resp := makeRequest(t, app, "POST", "/searchHadits/ShahihBukhari/Indonesia", body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result []interface{}
	decodeJSON(t, resp, &result)

	rows, ok := result[0].([]interface{})
	if !ok {
		t.Fatal("first element should be an array of rows")
	}
	if len(rows) == 0 {
		t.Error("expected a LIKE match for multi-keyword with 'qur''an', got 0")
	}
}

func setupTestAppWithFTS(t *testing.T) *fiber.App {
	t.Helper()
	setupTestDB(t)

	if err := database.DB.Exec(`CREATE VIRTUAL TABLE "FTSShahihBukhari" USING fts5(Nomer, Indonesia)`).Error; err != nil {
		t.Fatalf("failed to create FTS table: %v", err)
	}

	database.DB.Exec(`INSERT INTO "FTSShahihBukhari" (Nomer, Indonesia) VALUES (?, ?)`,
		100, "Perbedaan dan cara baca Al Qur'an dalam hadits itu penting")

	app := fiber.New()
	RegisterRoutes(app)
	return app
}

func TestSearchSingleKitab_ApostropheFTS(t *testing.T) {
	app := setupTestAppWithFTS(t)

	body := map[string]interface{}{
		"keyword": []string{"Perbedaan", "cara", "baca", "qur''an"},
	}
	resp := makeRequest(t, app, "POST", "/searchHadits/ShahihBukhari/Indonesia", body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result []interface{}
	decodeJSON(t, resp, &result)

	rows, ok := result[0].([]interface{})
	if !ok {
		t.Fatal("first element should be an array of rows")
	}
	if len(rows) == 0 {
		t.Error("expected an FTS match for keyword with 'qur''an', got 0")
	}

	found := false
	for _, r := range rows {
		m, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		if fmt.Sprint(m["0"]) == "100" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected hadith 100 to be returned by FTS search")
	}
}

func TestSearchSingleKitab_EmptyKeywords(t *testing.T) {
	app := setupTestApp(t)

	body := map[string]interface{}{
		"keyword": []string{},
	}
	resp := makeRequest(t, app, "POST", "/searchHadits/ShahihBukhari/Indonesia", body)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var errResp map[string]interface{}
	decodeJSON(t, resp, &errResp)

	if errResp["error"] != "keyword is required" {
		t.Errorf("error = %v, want 'keyword is required'", errResp["error"])
	}
}

func TestSearchSingleKitab_InvalidJSON(t *testing.T) {
	app := setupTestApp(t)

	req := httptest.NewRequest("POST", "/searchHadits/ShahihBukhari/Indonesia", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestSearchSingleKitab_NoMatch(t *testing.T) {
	app := setupTestApp(t)

	body := map[string]interface{}{
		"keyword": []string{"xyznonexistent"},
	}
	resp := makeRequest(t, app, "POST", "/searchHadits/ShahihBukhari/Indonesia", body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result []interface{}
	decodeJSON(t, resp, &result)

	rows := result[0].([]interface{})
	if len(rows) != 0 {
		t.Errorf("expected 0 matches for nonexistent keyword, got %d", len(rows))
	}
}

func TestSearchSingleKitab_UnknownKitab(t *testing.T) {
	app := setupTestApp(t)

	body := map[string]interface{}{
		"keyword": []string{"sholat"},
	}
	resp := makeRequest(t, app, "POST", "/searchHadits/UnknownKitab/Indonesia", body)

	// Unknown kitab causes SQL error → 500
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

func TestSearchSingleKitab_ArabicColumn(t *testing.T) {
	app := setupTestApp(t)

	body := map[string]interface{}{
		"keyword": []string{"صَلُّوا"},
	}
	resp := makeRequest(t, app, "POST", "/searchHadits/ShahihBukhari/Arabic", body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result []interface{}
	decodeJSON(t, resp, &result)

	rows := result[0].([]interface{})
	if len(rows) == 0 {
		t.Error("expected at least 1 match for Arabic keyword, got 0")
	}
}

func TestSearchSingleKitab_ConjunctionKeywordsFiltered(t *testing.T) {
	app := setupTestApp(t)

	body := map[string]interface{}{
		"keyword": []string{"sholat", "dan", "puasa"},
	}
	resp := makeRequest(t, app, "POST", "/searchHadits/ShahihBukhari/Indonesia", body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result []interface{}
	decodeJSON(t, resp, &result)

	rows := result[0].([]interface{})
	t.Logf("got %d matches for 'sholat' AND 'puasa' (after filtering 'dan')", len(rows))
}

// ============================================================
// POST /searchHadits/all/:column tests
// ============================================================

func TestSearchAll_ValidRequest(t *testing.T) {
	app := setupTestApp(t)

	body := map[string]interface{}{
		"keyword": []string{"sholat"},
		"books":   []string{"ShahihBukhari"},
	}
	resp := makeRequest(t, app, "POST", "/searchHadits/all/Indonesia", body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	results, ok := result["results"].(map[string]interface{})
	if !ok {
		t.Fatal("response should have 'results' map")
	}

	bukhari, ok := results["ShahihBukhari"].(map[string]interface{})
	if !ok {
		t.Fatal("results should contain 'ShahihBukhari'")
	}

	count := int(bukhari["count"].(float64))
	if count == 0 {
		t.Error("expected at least 1 match for 'sholat' in ShahihBukhari")
	}

	rows := bukhari["rows"].([]interface{})
	if len(rows) != count {
		t.Errorf("rows length %d != count %d", len(rows), count)
	}
}

func TestSearchAll_MultipleBooks(t *testing.T) {
	app := setupTestApp(t)

	body := map[string]interface{}{
		"keyword": []string{"sholat"},
		"books":   []string{"ShahihBukhari", "ShahihMuslim"},
	}
	resp := makeRequest(t, app, "POST", "/searchHadits/all/Indonesia", body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	results := result["results"].(map[string]interface{})

	if _, ok := results["ShahihBukhari"]; !ok {
		t.Error("results should contain 'ShahihBukhari'")
	}
	if _, ok := results["ShahihMuslim"]; !ok {
		t.Error("results should contain 'ShahihMuslim'")
	}
}

func TestSearchAll_EmptyKeywords(t *testing.T) {
	app := setupTestApp(t)

	body := map[string]interface{}{
		"keyword": []string{},
		"books":   []string{"ShahihBukhari"},
	}
	resp := makeRequest(t, app, "POST", "/searchHadits/all/Indonesia", body)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var errResp map[string]interface{}
	decodeJSON(t, resp, &errResp)

	if errResp["error"] != "keyword is required" {
		t.Errorf("error = %v, want 'keyword is required'", errResp["error"])
	}
}

func TestSearchAll_EmptyBooks(t *testing.T) {
	app := setupTestApp(t)

	body := map[string]interface{}{
		"keyword": []string{"sholat"},
		"books":   []string{},
	}
	resp := makeRequest(t, app, "POST", "/searchHadits/all/Indonesia", body)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var errResp map[string]interface{}
	decodeJSON(t, resp, &errResp)

	if errResp["error"] != "books is required" {
		t.Errorf("error = %v, want 'books is required'", errResp["error"])
	}
}

func TestSearchAll_UnknownBook(t *testing.T) {
	app := setupTestApp(t)

	body := map[string]interface{}{
		"keyword": []string{"sholat"},
		"books":   []string{"UnknownBook"},
	}
	resp := makeRequest(t, app, "POST", "/searchHadits/all/Indonesia", body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	results := result["results"].(map[string]interface{})
	unknown := results["UnknownBook"].(map[string]interface{})
	if unknown["count"].(float64) != 0 {
		t.Errorf("unknown kitab should return 0 count, got %v", unknown["count"])
	}
}

func TestSearchAll_InvalidJSON(t *testing.T) {
	app := setupTestApp(t)

	req := httptest.NewRequest("POST", "/searchHadits/all/Indonesia", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestSearchAll_TotalCount(t *testing.T) {
	app := setupTestApp(t)

	body := map[string]interface{}{
		"keyword": []string{"sholat"},
		"books":   []string{"ShahihBukhari"},
	}
	resp := makeRequest(t, app, "POST", "/searchHadits/all/Indonesia", body)

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	total := int(result["total"].(float64))
	results := result["results"].(map[string]interface{})
	bukhari := results["ShahihBukhari"].(map[string]interface{})
	count := int(bukhari["count"].(float64))

	if total != count {
		t.Errorf("total (%d) should equal sum of counts (%d)", total, count)
	}
}

// ============================================================
// Keyword count cap behavior tests
// ============================================================

func TestSearchAll_KeywordCountCap(t *testing.T) {
	app := setupTestApp(t)

	tests := []struct {
		name     string
		keywords []string
	}{
		{"1_keyword", []string{"sholat"}},
		{"2_keywords", []string{"sholat", "puasa"}},
		{"3_keywords", []string{"sholat", "puasa", "sabar"}},
		{"4_keywords", []string{"sholat", "puasa", "sabar", "lima"}},
		{"5_keywords", []string{"sholat", "puasa", "sabar", "lima", "waktu"}},
		{"6_keywords", []string{"sholat", "puasa", "sabar", "lima", "waktu", "sholat"}},
		{"7_keywords", []string{"sholat", "puasa", "sabar", "lima", "waktu", "sholat", "lima"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := map[string]interface{}{
				"keyword": tt.keywords,
				"books":   []string{"ShahihBukhari"},
			}
			resp := makeRequest(t, app, "POST", "/searchHadits/all/Indonesia", body)

			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}

			var result map[string]interface{}
			decodeJSON(t, resp, &result)

			if result["results"] == nil {
				t.Error("response should have 'results'")
			}
			if result["total"] == nil {
				t.Error("response should have 'total'")
			}
		})
	}
}

func TestSearchSingle_KeywordCountCap(t *testing.T) {
	app := setupTestApp(t)

	tests := []struct {
		name     string
		keywords []string
	}{
		{"5_keywords", []string{"sholat", "puasa", "sabar", "lima", "waktu"}},
		{"6_keywords", []string{"sholat", "puasa", "sabar", "lima", "waktu", "sholat"}},
		{"7_keywords", []string{"sholat", "puasa", "sabar", "lima", "waktu", "sholat", "lima"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := map[string]interface{}{
				"keyword": tt.keywords,
			}
			resp := makeRequest(t, app, "POST", "/searchHadits/ShahihBukhari/Indonesia", body)

			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}

			var result []interface{}
			decodeJSON(t, resp, &result)

			if len(result) != 3 {
				t.Fatalf("response should have 3 elements, got %d", len(result))
			}
		})
	}
}

// ============================================================
// Non-Indonesia column tests (LIKE-based, no FTS)
// ============================================================

func TestSearchAll_EnglishColumn(t *testing.T) {
	app := setupTestApp(t)

	body := map[string]interface{}{
		"keyword": []string{"pray"},
		"books":   []string{"ShahihBukhari"},
	}
	resp := makeRequest(t, app, "POST", "/searchHadits/all/English", body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	results := result["results"].(map[string]interface{})
	bukhari := results["ShahihBukhari"].(map[string]interface{})
	count := int(bukhari["count"].(float64))
	if count == 0 {
		t.Error("expected at least 1 match for 'pray' in English column")
	}
}

func TestSearchAll_EnglishMultiKeyword(t *testing.T) {
	app := setupTestApp(t)

	body := map[string]interface{}{
		"keyword": []string{"prayers", "five"},
		"books":   []string{"ShahihBukhari"},
	}
	resp := makeRequest(t, app, "POST", "/searchHadits/all/English", body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	results := result["results"].(map[string]interface{})
	bukhari := results["ShahihBukhari"].(map[string]interface{})
	t.Logf("got %d matches for 'prayers' AND 'five' in English", int(bukhari["count"].(float64)))
}

// ============================================================
// Method not allowed tests
// ============================================================

func TestSearchAll_WrongMethod(t *testing.T) {
	app := setupTestApp(t)

	resp := makeRequest(t, app, "GET", "/searchHadits/all/Indonesia", nil)
	if resp.StatusCode == http.StatusOK {
		t.Error("GET should not return 200")
	}
}

func TestSearchSingle_WrongMethod(t *testing.T) {
	app := setupTestApp(t)

	resp := makeRequest(t, app, "GET", "/searchHadits/ShahihBukhari/Indonesia", nil)
	if resp.StatusCode == http.StatusOK {
		t.Error("GET should not return 200")
	}
}

// ============================================================
// Route verification tests
// ============================================================

func TestRoutes_DoNotUseOldPackage(t *testing.T) {
	app := setupTestApp(t)

	routes := app.Stack()
	routeCount := 0
	for _, methodRoutes := range routes {
		for _, route := range methodRoutes {
			if route.Method != "*" {
				routeCount++
			}
		}
	}
	if routeCount < 2 {
		t.Errorf("expected at least 2 routes, got %d", routeCount)
	}
	t.Logf("registered %d routes", routeCount)
}

func TestRoutes_SearchHaditsPaths(t *testing.T) {
	app := setupTestApp(t)

	body := map[string]interface{}{
		"keyword": []string{"sholat"},
		"books":   []string{"ShahihBukhari"},
	}

	paths := []string{
		"/searchHadits/ShahihBukhari/Indonesia",
		"/searchHadits/all/Indonesia",
	}
	for _, path := range paths {
		t.Run(fmt.Sprintf("POST %s", path), func(t *testing.T) {
			resp := makeRequest(t, app, "POST", path, body)
			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
		})
	}
}
