package hadithdata

import (
	"net/http"
	"testing"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
)

const (
	wantKitabEng = "Book of Faith"
	wantBabEng   = "First Chapter"
)

func kitabTitleOf(t *testing.T, row map[string]interface{}) map[string]interface{} {
	t.Helper()
	v, ok := row["kitabTitle"].([]interface{})
	if !ok || len(v) == 0 {
		t.Fatalf("kitabTitle missing in row: %#v", row)
	}
	title, ok := v[0].(map[string]interface{})
	if !ok {
		t.Fatalf("kitabTitle item is not an object: %#v", v[0])
	}
	return title
}

func babTitleOf(t *testing.T, row map[string]interface{}) map[string]interface{} {
	t.Helper()
	v, ok := row["babTitle"].([]interface{})
	if !ok || len(v) == 0 {
		t.Fatalf("babTitle missing in row: %#v", row)
	}
	title, ok := v[0].(map[string]interface{})
	if !ok {
		t.Fatalf("babTitle item is not an object: %#v", v[0])
	}
	return title
}

func assertExistingKitabKeys(t *testing.T, title map[string]interface{}, vmember, awalan interface{}) {
	t.Helper()
	if title["NKitab"] != "Kitab Iman" {
		t.Errorf("NKitab = %v, want 'Kitab Iman'", title["NKitab"])
	}
	if _, ok := title["VMember"]; !ok {
		t.Error("VMember key missing")
	}
	if _, ok := title["Awalan"]; !ok {
		t.Error("Awalan key missing")
	}
	if title["VMember"] != vmember {
		t.Errorf("VMember = %v, want %v", title["VMember"], vmember)
	}
	if title["Awalan"] != awalan {
		t.Errorf("Awalan = %v, want %v", title["Awalan"], awalan)
	}
}

func assertExistingBabKeys(t *testing.T, title map[string]interface{}, vmemberBab, awalanBab interface{}) {
	t.Helper()
	if title["NBab"] != "Bab Pertama" {
		t.Errorf("NBab = %v, want 'Bab Pertama'", title["NBab"])
	}
	if _, ok := title["VMemberBab"]; !ok {
		t.Error("VMemberBab key missing")
	}
	if _, ok := title["AwalanBab"]; !ok {
		t.Error("AwalanBab key missing")
	}
	if title["VMemberBab"] != vmemberBab {
		t.Errorf("VMemberBab = %v, want %v", title["VMemberBab"], vmemberBab)
	}
	if title["AwalanBab"] != awalanBab {
		t.Errorf("AwalanBab = %v, want %v", title["AwalanBab"], awalanBab)
	}
}

func assertKitabTranslations(t *testing.T, title map[string]interface{}, vmember, awalan interface{}) {
	t.Helper()
	assertExistingKitabKeys(t, title, vmember, awalan)
	if title["NKitabEng"] != wantKitabEng {
		t.Errorf("NKitabEng = %v, want %q", title["NKitabEng"], wantKitabEng)
	}
	if v, ok := title["NKitabUrd"]; !ok {
		t.Error("NKitabUrd key missing")
	} else if v != "" {
		t.Errorf("NKitabUrd = %v, want '' until Urdu column exists", v)
	}
	if v, ok := title["NKitabBen"]; !ok {
		t.Error("NKitabBen key missing")
	} else if v != "" {
		t.Errorf("NKitabBen = %v, want '' until Bengali column exists", v)
	}
}

func assertBabTranslations(t *testing.T, title map[string]interface{}, vmemberBab, awalanBab interface{}) {
	t.Helper()
	assertExistingBabKeys(t, title, vmemberBab, awalanBab)
	if title["NBabEng"] != wantBabEng {
		t.Errorf("NBabEng = %v, want %q", title["NBabEng"], wantBabEng)
	}
	if v, ok := title["NBabUrd"]; !ok {
		t.Error("NBabUrd key missing")
	} else if v != "" {
		t.Errorf("NBabUrd = %v, want '' until Urdu column exists", v)
	}
	if v, ok := title["NBabBen"]; !ok {
		t.Error("NBabBen key missing")
	} else if v != "" {
		t.Errorf("NBabBen = %v, want '' until Bengali column exists", v)
	}
}

// ============================================================
// GET /loadMainData — nested kitabTitle / babTitle translations
// ============================================================

func TestMainData_KitabBabTitlesIncludeTranslations(t *testing.T) {
	app := setupHDRDataApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadMainData/ShahihBukhari/1")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	row := firstRow(t, decodeHDRArray(t, resp))
	assertKitabTranslations(t, kitabTitleOf(t, row), float64(5), float64(1))
	assertBabTranslations(t, babTitleOf(t, row), float64(10), float64(1))
}

// ============================================================
// GET /classificationData — nested kitabTitle / babTitle
// ============================================================

func TestClassificationData_KitabBabTitlesIncludeTranslations(t *testing.T) {
	app := setupHDRDataApp(t)
	resp := makeHDRRequest(t, app, "GET", "/classificationData/ShahihBukhari/1/Tema")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	row := firstRow(t, decodeHDRArray(t, resp))
	assertKitabTranslations(t, kitabTitleOf(t, row), float64(5), float64(1))
	assertBabTranslations(t, babTitleOf(t, row), float64(10), float64(1))
}

// ============================================================
// GET /loadCustomData — nested kitabTitle / babTitle
// ============================================================

func TestCustomData_KitabBabTitlesIncludeTranslations(t *testing.T) {
	app := setupHDRDataApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadCustomData/ShahihBukhari/1/position/actionId")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	row := firstRow(t, decodeHDRArray(t, resp))
	assertKitabTranslations(t, kitabTitleOf(t, row), float64(5), float64(1))
	assertBabTranslations(t, babTitleOf(t, row), float64(10), float64(1))
}

// ============================================================
// GET /loadAllBooks — whole-payload kitab title list
// ============================================================

func TestBook_ReturnsTranslatedKitabFields(t *testing.T) {
	app := setupHDRDataApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadAllBooks/ShahihBukhari")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	arr := decodeHDRArray(t, resp)
	if len(arr) != 2 || arr[1] != "ALLBOOKSRESULT" {
		t.Fatalf("unexpected response shape: %#v", arr)
	}

	rows, ok := arr[0].([]interface{})
	if !ok || len(rows) != 1 {
		t.Fatalf("expected one book row, got %#v", arr[0])
	}
	book, ok := rows[0].(map[string]interface{})
	if !ok {
		t.Fatalf("book row is not an object: %#v", rows[0])
	}

	// Book struct uses string-typed VMember/Awalan
	assertKitabTranslations(t, book, "5", "1")
}

// ============================================================
// GET /loadAllChapters — whole-payload bab title list
// ============================================================

func TestChapterList_ReturnsTranslatedBabFields(t *testing.T) {
	app := setupHDRDataApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadAllChapters/ShahihBukhari/1/10")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	arr := decodeHDRArray(t, resp)
	if len(arr) != 2 || arr[1] != "ALLCHAPTERSRESULT" {
		t.Fatalf("unexpected response shape: %#v", arr)
	}

	rows, ok := arr[0].([]interface{})
	if !ok || len(rows) != 1 {
		t.Fatalf("expected one chapter row, got %#v", arr[0])
	}
	chapter, ok := rows[0].(map[string]interface{})
	if !ok {
		t.Fatalf("chapter row is not an object: %#v", rows[0])
	}

	// Chapter struct uses string-typed VMemberBab/AwalanBab
	assertBabTranslations(t, chapter, "10", "1")
}

// ============================================================
// GET /loadAllChapters/endfirst — whole-payload bab title list
// ============================================================

func TestChapterEndFirst_ReturnsTranslatedBabFields(t *testing.T) {
	app := setupHDRDataApp(t)
	// endfirst resolves the end bound by reading the Awalan of the NEXT kitab
	// (VMember = vSelectedK + 1), so a second kitab row is required.
	if err := database.DB.Exec(
		`INSERT INTO "KitabShahihBukhari" (NKitab, NKitabEng, VMember, Awalan) VALUES ('Kitab Jihad', 'Book of Jihad', 6, '2')`,
	).Error; err != nil {
		t.Fatalf("failed to seed next kitab: %v", err)
	}

	resp := makeHDRRequest(t, app, "GET", "/loadAllChapters/endfirst/ShahihBukhari/1/5")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	arr := decodeHDRArray(t, resp)
	if len(arr) != 2 || arr[1] != "ALLCHAPTERSRESULT" {
		t.Fatalf("unexpected response shape: %#v", arr)
	}

	rows, ok := arr[0].([]interface{})
	if !ok || len(rows) != 1 {
		t.Fatalf("expected one chapter row, got %#v", arr[0])
	}
	chapter, ok := rows[0].(map[string]interface{})
	if !ok {
		t.Fatalf("chapter row is not an object: %#v", rows[0])
	}

	// Chapter struct uses string-typed VMemberBab/AwalanBab
	assertBabTranslations(t, chapter, "10", "1")
}

// ============================================================
// Consistency: both nested and whole-payload lists agree
// ============================================================

func TestTranslatedTitles_ConsistentAcrossEndpoints(t *testing.T) {
	app := setupHDRDataApp(t)

	mainResp := makeHDRRequest(t, app, "GET", "/loadMainData/ShahihBukhari/1")
	if mainResp.StatusCode != http.StatusOK {
		t.Fatalf("loadMainData status = %d", mainResp.StatusCode)
	}
	mainRow := firstRow(t, decodeHDRArray(t, mainResp))
	mainKitab := kitabTitleOf(t, mainRow)
	mainBab := babTitleOf(t, mainRow)

	booksResp := makeHDRRequest(t, app, "GET", "/loadAllBooks/ShahihBukhari")
	if booksResp.StatusCode != http.StatusOK {
		t.Fatalf("loadAllBooks status = %d", booksResp.StatusCode)
	}
	booksArr := decodeHDRArray(t, booksResp)
	books := booksArr[0].([]interface{})[0].(map[string]interface{})

	chaptersResp := makeHDRRequest(t, app, "GET", "/loadAllChapters/ShahihBukhari/1/10")
	if chaptersResp.StatusCode != http.StatusOK {
		t.Fatalf("loadAllChapters status = %d", chaptersResp.StatusCode)
	}
	chapArr := decodeHDRArray(t, chaptersResp)
	chapters := chapArr[0].([]interface{})[0].(map[string]interface{})

	for _, key := range []string{"NKitab", "NKitabEng", "NKitabUrd", "NKitabBen"} {
		if mainKitab[key] != books[key] {
			t.Errorf("kitab key %s differs: mainData=%v books=%v", key, mainKitab[key], books[key])
		}
	}
	for _, key := range []string{"NBab", "NBabEng", "NBabUrd", "NBabBen"} {
		if mainBab[key] != chapters[key] {
			t.Errorf("bab key %s differs: mainData=%v chapters=%v", key, mainBab[key], chapters[key])
		}
	}
}

// ============================================================
// GET /loadTotalHadith — Tema classification, nested kitab/bab titles
// ============================================================

func TestTotalHadith_NestedTitlesIncludeTranslations(t *testing.T) {
	app := setupHDRDataApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadTotalHadith/ShahihBukhari/100")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	arr := decodeHDRArray(t, resp)
	if len(arr) != 3 || arr[1] != "TOTALHADITHROWSRESULT" {
		t.Fatalf("unexpected response shape: %#v", arr)
	}

	// arr[2] = [rows, "TOTALHADITHDATA", 1] -> rows[0] is the ClassificationData row
	inner, ok := arr[2].([]interface{})
	if !ok || len(inner) != 3 {
		t.Fatalf("unexpected nested shape: %#v", arr[2])
	}
	rows, ok := inner[0].([]interface{})
	if !ok || len(rows) == 0 {
		t.Fatalf("expected nested rows, got %#v", inner[0])
	}
	row, ok := rows[0].(map[string]interface{})
	if !ok {
		t.Fatalf("nested row is not an object: %#v", rows[0])
	}

	assertKitabTranslations(t, kitabTitleOf(t, row), float64(5), float64(1))
	assertBabTranslations(t, babTitleOf(t, row), float64(10), float64(1))
}

func TestTotalHadith_EmptyNestedDataHasNoTitle(t *testing.T) {
	app := setupHDRDataApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadTotalHadith/ShahihBukhari/999")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	arr := decodeHDRArray(t, resp)
	inner, ok := arr[2].([]interface{})
	if !ok || len(inner) != 3 {
		t.Fatalf("unexpected nested shape: %#v", arr[2])
	}
	rows, ok := inner[0].([]interface{})
	if !ok || len(rows) != 0 {
		t.Fatalf("expected empty nested rows for unknown narrator, got %#v", inner[0])
	}
}

// ============================================================
// Missing/empty English translations stay safe (blank item)
// ============================================================

func TestTitleTranslations_NullEnglishDoesNotBreak(t *testing.T) {
	app := setupHDRDataApp(t)
	if err := database.DB.Exec(
		`INSERT INTO "KitabShahihBukhari" (NKitab, NKitabEng, VMember, Awalan) VALUES ('Kitab Jihad', NULL, 6, '2')`,
	).Error; err != nil {
		t.Fatalf("failed to seed kitab with null eng: %v", err)
	}
	if err := database.DB.Exec(
		`INSERT INTO "BabShahihBukhari" (NBab, NBabEng, VMemberBab, AwalanBab) VALUES ('Bab Jihad', NULL, 11, '2')`,
	).Error; err != nil {
		t.Fatalf("failed to seed bab with null eng: %v", err)
	}
	if err := database.DB.Exec(
		`INSERT INTO "ShahihBukhari" (Nomer, Arabic, Indonesia, English, Urdu, Bengali, Albani, Darussalam, VSelectedK, VSelectedB) VALUES (3, 'arabic', 'indonesia', 'english', 'urdu', 'bengali', 'shahih', 'sahih', 6, 11)`,
	).Error; err != nil {
		t.Fatalf("failed to seed hadith with null-title refs: %v", err)
	}

	resp := makeHDRRequest(t, app, "GET", "/loadMainData/ShahihBukhari/3")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	row := firstRow(t, decodeHDRArray(t, resp))
	kitab := kitabTitleOf(t, row)
	if kitab["NKitabEng"] != "" {
		t.Errorf("NKitabEng = %v, want '' for NULL column", kitab["NKitabEng"])
	}
	bab := babTitleOf(t, row)
	if bab["NBabEng"] != "" {
		t.Errorf("NBabEng = %v, want '' for NULL column", bab["NBabEng"])
	}
}

// ============================================================
// Validation fields untouched in every row shape
// ============================================================

func TestTitleTranslations_PreservesVMemberAndAwalanValues(t *testing.T) {
	app := setupHDRDataApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadMainData/ShahihBukhari/1")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	row := firstRow(t, decodeHDRArray(t, resp))
	kitab := kitabTitleOf(t, row)
	bab := babTitleOf(t, row)

	if kitab["VMember"] != float64(5) {
		t.Errorf("kitab VMember = %v, want 5 (selection value)", kitab["VMember"])
	}
	if bab["VMemberBab"] != float64(10) {
		t.Errorf("bab VMemberBab = %v, want 10 (selection value)", bab["VMemberBab"])
	}
	if kitab["NKitab"] != "Kitab Iman" {
		t.Errorf("kitab NKitab = %v, want 'Kitab Iman'", kitab["NKitab"])
	}
	if bab["NBab"] != "Bab Pertama" {
		t.Errorf("bab NBab = %v, want 'Bab Pertama'", bab["NBab"])
	}
}