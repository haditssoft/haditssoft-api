package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
)

type titleBulkTranslateResponse struct {
	Processed int               `json:"processed"`
	Updated   int               `json:"updated"`
	Failed    []translateResult `json:"failed"`
	Kitab     bulkSweepResult   `json:"kitab"`
	Bab       bulkSweepResult   `json:"bab"`
}

func seedKitabShahihBukhari(t *testing.T, rows []map[string]interface{}) {
	t.Helper()
	if err := database.DB.Exec(`CREATE TABLE IF NOT EXISTS "KitabShahihBukhari" ("VMember" INTEGER PRIMARY KEY, "NKitabArab" TEXT, "NKitab" TEXT, "NKitabEng" TEXT)`).Error; err != nil {
		t.Fatalf("failed to create kitab title table: %v", err)
	}
	for _, row := range rows {
		if err := database.DB.Exec(`INSERT INTO "KitabShahihBukhari" ("VMember", "NKitabArab", "NKitab", "NKitabEng") VALUES (?, ?, ?, ?)`,
			row["VMember"], row["NKitabArab"], row["NKitab"], row["NKitabEng"]).Error; err != nil {
			t.Fatalf("failed to seed kitab title row %v: %v", row, err)
		}
	}
}

func seedBabShahihBukhari(t *testing.T, rows []map[string]interface{}) {
	t.Helper()
	if err := database.DB.Exec(`CREATE TABLE IF NOT EXISTS "BabShahihBukhari" ("VMemberBab" INTEGER PRIMARY KEY, "NBabArab" TEXT, "NBab" TEXT, "NBabEng" TEXT)`).Error; err != nil {
		t.Fatalf("failed to create bab title table: %v", err)
	}
	for _, row := range rows {
		if err := database.DB.Exec(`INSERT INTO "BabShahihBukhari" ("VMemberBab", "NBabArab", "NBab", "NBabEng") VALUES (?, ?, ?, ?)`,
			row["VMemberBab"], row["NBabArab"], row["NBab"], row["NBabEng"]).Error; err != nil {
			t.Fatalf("failed to seed bab title row %v: %v", row, err)
		}
	}
}

func getKitabTitleEng(t *testing.T, vmember uint) *string {
	t.Helper()
	var row struct {
		NKitabEng *string
	}
	if err := database.DB.Table("KitabShahihBukhari").Select("NKitabEng").Where("VMember = ?", vmember).First(&row).Error; err != nil {
		t.Fatalf("failed to read NKitabEng for VMember %d: %v", vmember, err)
	}
	return row.NKitabEng
}

func getBabTitleEng(t *testing.T, vmemberBab uint) *string {
	t.Helper()
	var row struct {
		NBabEng *string
	}
	if err := database.DB.Table("BabShahihBukhari").Select("NBabEng").Where("VMemberBab = ?", vmemberBab).First(&row).Error; err != nil {
		t.Fatalf("failed to read NBabEng for VMemberBab %d: %v", vmemberBab, err)
	}
	return row.NBabEng
}

func assertKitabTitleEng(t *testing.T, vmember uint, want string) {
	t.Helper()
	eng := getKitabTitleEng(t, vmember)
	if eng == nil {
		t.Errorf("VMember %d NKitabEng = nil, want %q", vmember, want)
		return
	}
	if *eng != want {
		t.Errorf("VMember %d NKitabEng = %q, want %q", vmember, *eng, want)
	}
}

func assertBabTitleEng(t *testing.T, vmemberBab uint, want string) {
	t.Helper()
	eng := getBabTitleEng(t, vmemberBab)
	if eng == nil {
		t.Errorf("VMemberBab %d NBabEng = nil, want %q", vmemberBab, want)
		return
	}
	if *eng != want {
		t.Errorf("VMemberBab %d NBabEng = %q, want %q", vmemberBab, *eng, want)
	}
}

func assertKitabTitleEmpty(t *testing.T, vmember uint) {
	t.Helper()
	eng := getKitabTitleEng(t, vmember)
	if eng != nil && *eng != "" {
		t.Errorf("VMember %d NKitabEng = %q, want empty", vmember, *eng)
	}
}

func assertBabTitleEmpty(t *testing.T, vmemberBab uint) {
	t.Helper()
	eng := getBabTitleEng(t, vmemberBab)
	if eng != nil && *eng != "" {
		t.Errorf("VMemberBab %d NBabEng = %q, want empty", vmemberBab, *eng)
	}
}

func TestTitleBulkTranslate_MissingKey(t *testing.T) {
	os.Unsetenv("OPENCODE_CRON_KEY")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_CRON_KEY") })

	app := setupTestApp(t)

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	var body map[string]interface{}
	decodeJSON(t, resp, &body)
	if body["status"] != "error" {
		t.Errorf("status = %v, want 'error'", body["status"])
	}
}

func TestTitleBulkTranslate_WrongKey(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=wrong-secret", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestTitleBulkTranslate_NoKeyConfigured(t *testing.T) {
	os.Unsetenv("OPENCODE_CRON_KEY")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_CRON_KEY") })

	app := setupTestApp(t)

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=anything", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (fail closed when key unset)", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestTitleBulkTranslate_WrongMethod(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	resp := makeRequest(t, app, "GET", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode == http.StatusOK {
		t.Error("GET should not return 200")
	}
}

func TestTitleBulkTranslate_UnknownKitab(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/NotARealKitab?key=correct-secret", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var body map[string]interface{}
	decodeJSON(t, resp, &body)
	if body["message"] != "unknown kitab: NotARealKitab" {
		t.Errorf("message = %v, want 'unknown kitab: NotARealKitab'", body["message"])
	}
}

func TestTitleBulkTranslate_InvalidLimit(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	for _, raw := range []string{"abc", "0", "-5"} {
		resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&limit="+raw, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("limit=%q status = %d, want %d", raw, resp.StatusCode, http.StatusBadRequest)
		}
	}
}

func TestTitleBulkTranslate_InvalidAllParam(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	for _, raw := range []string{"notabool", "maybe"} {
		resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&all="+raw, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("all=%q status = %d, want %d", raw, resp.StatusCode, http.StatusBadRequest)
		}
	}
}

func TestTitleBulkTranslate_InvalidMaxBatches(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	for _, raw := range []string{"abc", "0", "-5"} {
		resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&maxBatches="+raw, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("maxBatches=%q status = %d, want %d", raw, resp.StatusCode, http.StatusBadRequest)
		}
	}
}

func TestTitleBulkTranslate_InvalidType(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=chapters", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var body map[string]interface{}
	decodeJSON(t, resp, &body)
	if body["message"] != "type must be one of: kitab, bab, both" {
		t.Errorf("message = %v, want 'type must be one of: kitab, bab, both'", body["message"])
	}
}

func TestTitleBulkTranslate_KitabSuccessUpdatesNKitabEng(t *testing.T) {
	setCronKey(t, "correct-secret")
	reply := `{"1":"Book of Faith","2":"Book of Prayer"}`
	captured := setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: reply}})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "كتاب الإيمان", "NKitab": "Kitab Iman", "NKitabEng": ""},
		{"VMember": 2, "NKitabArab": "كتاب الصلاة", "NKitab": "Kitab Sholat", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 2 {
		t.Errorf("processed = %d, want 2", body.Processed)
	}
	if body.Updated != 2 {
		t.Errorf("updated = %d, want 2", body.Updated)
	}
	if body.Kitab.Processed != 2 || body.Kitab.Updated != 2 {
		t.Errorf("kitab processed/updated = %d/%d, want 2/2", body.Kitab.Processed, body.Kitab.Updated)
	}
	if body.Bab.Processed != 0 || body.Bab.Updated != 0 {
		t.Errorf("bab processed/updated = %d/%d, want 0/0 (skipped)", body.Bab.Processed, body.Bab.Updated)
	}
	if len(body.Failed) != 0 {
		t.Errorf("failed = %v, want empty list", body.Failed)
	}

	assertKitabTitleEng(t, 1, "Book of Faith")
	assertKitabTitleEng(t, 2, "Book of Prayer")

	if len(*captured) != 1 {
		t.Fatalf("got %d CLI calls, want 1", len(*captured))
	}
	args := (*captured)[0]
	if !containsArg(args, "--agent") || !containsArg(args, "translate-title-bulk") {
		t.Errorf("agent should be 'translate-title-bulk', got args: %v", args)
	}
}

func TestTitleBulkTranslate_BabSuccessUpdatesNBabEng(t *testing.T) {
	setCronKey(t, "correct-secret")
	reply := `{"10":"Chapter on Ablution","20":"Chapter on Prayer"}`
	captured := setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: reply}})

	app := setupTestApp(t)
	seedBabShahihBukhari(t, []map[string]interface{}{
		{"VMemberBab": 10, "NBabArab": "باب الوضوء", "NBab": "Bab Wudhu", "NBabEng": nil},
		{"VMemberBab": 20, "NBabArab": "باب الصلاة", "NBab": "Bab Sholat", "NBabEng": ""},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=bab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 2 {
		t.Errorf("processed = %d, want 2", body.Processed)
	}
	if body.Updated != 2 {
		t.Errorf("updated = %d, want 2", body.Updated)
	}
	if body.Bab.Processed != 2 || body.Bab.Updated != 2 {
		t.Errorf("bab processed/updated = %d/%d, want 2/2", body.Bab.Processed, body.Bab.Updated)
	}
	if body.Kitab.Processed != 0 {
		t.Errorf("kitab processed = %d, want 0 (skipped)", body.Kitab.Processed)
	}
	if len(body.Failed) != 0 {
		t.Errorf("failed = %v, want empty list", body.Failed)
	}

	assertBabTitleEng(t, 10, "Chapter on Ablution")
	assertBabTitleEng(t, 20, "Chapter on Prayer")

	if len(*captured) != 1 {
		t.Fatalf("got %d CLI calls, want 1", len(*captured))
	}
	args := (*captured)[0]
	if !containsArg(args, "--agent") || !containsArg(args, "translate-title-bulk") {
		t.Errorf("agent should be 'translate-title-bulk', got args: %v", args)
	}
}

func TestTitleBulkTranslate_BothTablesByDefault(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setBulkExecMock(t, bulkExecMockConfig{
		replyOverrides: map[int]string{
			1: `{"1":"Book of Faith"}`,
			2: `{"10":"Chapter of Prayer"}`,
		},
	})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "كتاب الإيمان", "NKitab": "Kitab Iman", "NKitabEng": nil},
	})
	seedBabShahihBukhari(t, []map[string]interface{}{
		{"VMemberBab": 10, "NBabArab": "باب الصلاة", "NBab": "Bab Sholat", "NBabEng": nil},
	})

	// No ?type= present: both tables are translated by default.
	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 2 {
		t.Errorf("processed = %d, want 2", body.Processed)
	}
	if body.Updated != 2 {
		t.Errorf("updated = %d, want 2", body.Updated)
	}
	if body.Kitab.Processed != 1 || body.Bab.Processed != 1 {
		t.Errorf("kitab/bab processed = %d/%d, want 1/1", body.Kitab.Processed, body.Bab.Processed)
	}
	if len(body.Failed) != 0 {
		t.Errorf("failed = %v, want empty", body.Failed)
	}
	if len(*captured) != 2 {
		t.Fatalf("got %d CLI calls, want 2 (one per table)", len(*captured))
	}
	for _, args := range *captured {
		if !containsArg(args, "--agent") || !containsArg(args, "translate-title-bulk") {
			t.Errorf("agent should be 'translate-title-bulk', got args: %v", args)
		}
	}

	assertKitabTitleEng(t, 1, "Book of Faith")
	assertBabTitleEng(t, 10, "Chapter of Prayer")
}

func TestTitleBulkTranslate_ExplicitTypeBoth(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setBulkExecMock(t, bulkExecMockConfig{
		replyOverrides: map[int]string{
			1: `{"1":"Book of Faith"}`,
			2: `{"10":"Chapter of Prayer"}`,
		},
	})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "كتاب الإيمان", "NKitab": "Kitab Iman", "NKitabEng": nil},
	})
	seedBabShahihBukhari(t, []map[string]interface{}{
		{"VMemberBab": 10, "NBabArab": "باب الصلاة", "NBab": "Bab Sholat", "NBabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=both", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 2 || body.Updated != 2 {
		t.Errorf("processed/updated = %d/%d, want 2/2", body.Processed, body.Updated)
	}
	if len(*captured) != 2 {
		t.Fatalf("got %d CLI calls, want 2", len(*captured))
	}
}

func TestTitleBulkTranslate_TypeKitabOnlyLeavesBabUntouched(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: `{"1":"Book of Faith"}`}})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "كتاب الإيمان", "NKitab": "Kitab Iman", "NKitabEng": nil},
	})
	seedBabShahihBukhari(t, []map[string]interface{}{
		{"VMemberBab": 10, "NBabArab": "باب الصلاة", "NBab": "Bab Sholat", "NBabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 1 || body.Updated != 1 {
		t.Errorf("processed/updated = %d/%d, want 1/1", body.Processed, body.Updated)
	}
	if len(*captured) != 1 {
		t.Errorf("got %d CLI calls, want 1", len(*captured))
	}

	assertKitabTitleEng(t, 1, "Book of Faith")
	assertBabTitleEmpty(t, 10)
}

func TestTitleBulkTranslate_TypeBabOnlyLeavesKitabUntouched(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: `{"10":"Chapter of Prayer"}`}})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "كتاب الإيمان", "NKitab": "Kitab Iman", "NKitabEng": nil},
	})
	seedBabShahihBukhari(t, []map[string]interface{}{
		{"VMemberBab": 10, "NBabArab": "باب الصلاة", "NBab": "Bab Sholat", "NBabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=bab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 1 || body.Updated != 1 {
		t.Errorf("processed/updated = %d/%d, want 1/1", body.Processed, body.Updated)
	}
	if len(*captured) != 1 {
		t.Errorf("got %d CLI calls, want 1", len(*captured))
	}

	assertBabTitleEng(t, 10, "Chapter of Prayer")
	assertKitabTitleEmpty(t, 1)
}

func TestTitleBulkTranslate_PromptKeyedByID(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: `{"1":"a","2":"b"}`}})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 2, "NKitabArab": "كتاب الصلاة", "NKitab": "Kitab Sholat", "NKitabEng": nil},
		{"VMember": 1, "NKitabArab": "كتاب الإيمان", "NKitab": "Kitab Iman", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	args := (*captured)[0]
	prompt := args[len(args)-1]
	if !json.Valid([]byte(prompt)) {
		t.Fatalf("prompt is not valid JSON: %q", prompt)
	}
	var payload map[string]struct {
		Arabic    string `json:"arabic"`
		Indonesia string `json:"indonesia"`
	}
	if err := json.Unmarshal([]byte(prompt), &payload); err != nil {
		t.Fatalf("failed to decode prompt payload: %v", err)
	}
	if e := payload["1"]; e.Arabic != "كتاب الإيمان" || e.Indonesia != "Kitab Iman" {
		t.Errorf("payload[1] = %+v, want arabic 'كتاب الإيمان' and indonesia 'Kitab Iman'", e)
	}
	if e := payload["2"]; e.Arabic != "كتاب الصلاة" || e.Indonesia != "Kitab Sholat" {
		t.Errorf("payload[2] = %+v, want arabic 'كتاب الصلاة' and indonesia 'Kitab Sholat'", e)
	}
}

func TestTitleBulkTranslate_MissingKeyCountedAsFailed(t *testing.T) {
	setCronKey(t, "correct-secret")
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: `{"1":"Book of Faith"}`}})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": nil},
		{"VMember": 2, "NKitabArab": "a2", "NKitab": "dua", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Updated != 1 {
		t.Errorf("updated = %d, want 1", body.Updated)
	}
	if len(body.Failed) != 1 {
		t.Fatalf("failed = %v, want 1 entry", body.Failed)
	}
	if body.Failed[0].Nomer != 2 {
		t.Errorf("failed nomer = %d, want 2", body.Failed[0].Nomer)
	}
	if body.Failed[0].Error != "missing key in AI response" {
		t.Errorf("failed error = %q, want 'missing key in AI response'", body.Failed[0].Error)
	}

	assertKitabTitleEng(t, 1, "Book of Faith")
	assertKitabTitleEmpty(t, 2)
}

func TestTitleBulkTranslate_EmptyValueCountedAsFailed(t *testing.T) {
	setCronKey(t, "correct-secret")
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: `{"1":"","2":"Book of Prayer"}`}})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": nil},
		{"VMember": 2, "NKitabArab": "a2", "NKitab": "dua", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Updated != 1 {
		t.Errorf("updated = %d, want 1", body.Updated)
	}
	if len(body.Failed) != 1 {
		t.Fatalf("failed = %v, want 1 entry", body.Failed)
	}
	if body.Failed[0].Nomer != 1 {
		t.Errorf("failed nomer = %d, want 1", body.Failed[0].Nomer)
	}
	if body.Failed[0].Error != "empty AI response" {
		t.Errorf("failed error = %q, want 'empty AI response'", body.Failed[0].Error)
	}

	assertKitabTitleEmpty(t, 1)
	assertKitabTitleEng(t, 2, "Book of Prayer")
}

func TestTitleBulkTranslate_NestedObjectValueRejected(t *testing.T) {
	setCronKey(t, "correct-secret")
	forbidden := `{"1":{"arabic":"كتاب الإيمان","indonesia":"Kitab Iman","english":"should not be written"}}`
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: forbidden}})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": nil},
		{"VMember": 2, "NKitabArab": "a2", "NKitab": "dua", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Updated != 0 {
		t.Errorf("updated = %d, want 0 (nested-object reply must not be written)", body.Updated)
	}
	if len(body.Failed) != 2 {
		t.Fatalf("failed = %v, want 2 entries", body.Failed)
	}
	for _, f := range body.Failed {
		if f.Error != "invalid JSON in AI response" {
			t.Errorf("failed error = %q, want 'invalid JSON in AI response'", f.Error)
		}
	}

	assertKitabTitleEmpty(t, 1)
	assertKitabTitleEmpty(t, 2)
}

func TestTitleBulkTranslate_InvalidJSONReplyFailsAll(t *testing.T) {
	setCronKey(t, "correct-secret")
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: "not json at all"}})

	app := setupTestApp(t)
	seedBabShahihBukhari(t, []map[string]interface{}{
		{"VMemberBab": 1, "NBabArab": "a1", "NBab": "satu", "NBabEng": nil},
		{"VMemberBab": 2, "NBabArab": "a2", "NBab": "dua", "NBabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=bab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Updated != 0 {
		t.Errorf("updated = %d, want 0", body.Updated)
	}
	if len(body.Failed) != 2 {
		t.Fatalf("failed = %v, want 2 entries", body.Failed)
	}
	if body.Failed[0].Error != "invalid JSON in AI response" {
		t.Errorf("failed error = %q, want 'invalid JSON in AI response'", body.Failed[0].Error)
	}

	assertBabTitleEmpty(t, 1)
	assertBabTitleEmpty(t, 2)
}

func TestTitleBulkTranslate_NoTextReplyFailsAll(t *testing.T) {
	setCronKey(t, "correct-secret")
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: ""}})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": nil},
		{"VMember": 2, "NKitabArab": "a2", "NKitab": "dua", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Updated != 0 {
		t.Errorf("updated = %d, want 0", body.Updated)
	}
	if len(body.Failed) != 2 {
		t.Fatalf("failed = %v, want 2 entries", body.Failed)
	}
	if body.Failed[0].Error != "no text response in opencode output" {
		t.Errorf("failed error = %q, want 'no text response in opencode output'", body.Failed[0].Error)
	}

	assertKitabTitleEmpty(t, 1)
	assertKitabTitleEmpty(t, 2)
}

func TestTitleBulkTranslate_WhitespaceReplyFailsAll(t *testing.T) {
	setCronKey(t, "correct-secret")
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: "   "}})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": nil},
		{"VMember": 2, "NKitabArab": "a2", "NKitab": "dua", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Updated != 0 {
		t.Errorf("updated = %d, want 0", body.Updated)
	}
	if len(body.Failed) != 2 {
		t.Fatalf("failed = %v, want 2 entries", body.Failed)
	}
	if body.Failed[0].Error != "empty AI response" {
		t.Errorf("failed error = %q, want 'empty AI response'", body.Failed[0].Error)
	}

	assertKitabTitleEmpty(t, 1)
	assertKitabTitleEmpty(t, 2)
}

func TestTitleBulkTranslate_FencedJSONReplyParsed(t *testing.T) {
	setCronKey(t, "correct-secret")
	fenced := "```json\n{\n  \"1\": \"Book of Faith\"\n}\n```"
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: fenced}})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "كتاب الإيمان", "NKitab": "Kitab Iman", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Updated != 1 {
		t.Errorf("updated = %d, want 1", body.Updated)
	}
	if len(body.Failed) != 0 {
		t.Errorf("failed = %v, want empty", body.Failed)
	}

	assertKitabTitleEng(t, 1, "Book of Faith")
}

func TestTitleBulkTranslate_PreambleAndTrailingTextParsed(t *testing.T) {
	setCronKey(t, "correct-secret")
	noisy := "Here are the translations:\n{\"1\": \"Book of Faith\"}\nDone."
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: noisy}})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "كتاب الإيمان", "NKitab": "Kitab Iman", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Updated != 1 {
		t.Errorf("updated = %d, want 1", body.Updated)
	}

	assertKitabTitleEng(t, 1, "Book of Faith")
}

func TestTitleBulkTranslate_UnknownKeyIgnored(t *testing.T) {
	setCronKey(t, "correct-secret")
	// "999" and "abc" are not among the requested ids and must be ignored.
	reply := `{"1":"Book of Faith","2":"Book of Prayer","999":"bogus","abc":"nope"}`
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: reply}})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": nil},
		{"VMember": 2, "NKitabArab": "a2", "NKitab": "dua", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Updated != 2 {
		t.Errorf("updated = %d, want 2 (unknown keys must not count or be written)", body.Updated)
	}
	if len(body.Failed) != 0 {
		t.Errorf("failed = %v, want empty", body.Failed)
	}

	assertKitabTitleEng(t, 1, "Book of Faith")
	assertKitabTitleEng(t, 2, "Book of Prayer")
}

func TestTitleBulkTranslate_KeyOutsideBatchNotOverwritten(t *testing.T) {
	setCronKey(t, "correct-secret")
	// Row 3 is already translated and NOT part of the batch; the reply wrongly
	// includes its key. It must NOT be overwritten.
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: `{"1":"Book of Faith","3":"corrupted"}`}})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": nil},
		{"VMember": 3, "NKitabArab": "a3", "NKitab": "tiga", "NKitabEng": "already translated"},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 1 {
		t.Errorf("processed = %d, want 1", body.Processed)
	}
	if body.Updated != 1 {
		t.Errorf("updated = %d, want 1", body.Updated)
	}
	if len(body.Failed) != 0 {
		t.Errorf("failed = %v, want empty", body.Failed)
	}

	assertKitabTitleEng(t, 1, "Book of Faith")
	if eng := getKitabTitleEng(t, 3); eng == nil || *eng != "already translated" {
		t.Errorf("VMember 3 NKitabEng = %v, want unchanged 'already translated'", eng)
	}
}

func TestTitleBulkTranslate_NoMatchingRows(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setBulkExecMock(t, bulkExecMockConfig{})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": "already translated"},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 0 {
		t.Errorf("processed = %d, want 0", body.Processed)
	}
	if body.Updated != 0 {
		t.Errorf("updated = %d, want 0", body.Updated)
	}
	if len(*captured) != 0 {
		t.Errorf("got %d CLI calls, want 0", len(*captured))
	}
}

func TestTitleBulkTranslate_CLIFailureAllRowsFail(t *testing.T) {
	setCronKey(t, "correct-secret")
	setBulkExecMock(t, bulkExecMockConfig{failEveryCall: true})

	app := setupTestApp(t)
	seedBabShahihBukhari(t, []map[string]interface{}{
		{"VMemberBab": 1, "NBabArab": "a1", "NBab": "satu", "NBabEng": nil},
		{"VMemberBab": 2, "NBabArab": "a2", "NBab": "dua", "NBabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=bab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 2 {
		t.Errorf("processed = %d, want 2", body.Processed)
	}
	if body.Updated != 0 {
		t.Errorf("updated = %d, want 0", body.Updated)
	}
	if len(body.Failed) != 2 {
		t.Fatalf("failed = %v, want 2 entries", body.Failed)
	}
	wantErr := "opencode run failed: opencode exec failed at call 1"
	for _, f := range body.Failed {
		if f.Error != wantErr {
			t.Errorf("failed error = %q, want %q", f.Error, wantErr)
		}
	}

	assertBabTitleEmpty(t, 1)
	assertBabTitleEmpty(t, 2)
}

func TestTitleBulkTranslate_ServerErrorRetryFailsAll(t *testing.T) {
	setCronKey(t, "correct-secret")
	setTranslateRetryDelay(t, 0)
	setTranslateMaxRetries(t, 2)
	callCount := setServerErrMock(t, "Rate limit exceeded.")

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 1 {
		t.Errorf("processed = %d, want 1", body.Processed)
	}
	if body.Updated != 0 {
		t.Errorf("updated = %d, want 0", body.Updated)
	}
	if len(body.Failed) != 1 {
		t.Fatalf("failed = %v, want 1 entry", body.Failed)
	}
	wantErr := "opencode server error: Rate limit exceeded."
	if body.Failed[0].Error != wantErr {
		t.Errorf("failed error = %q, want %q", body.Failed[0].Error, wantErr)
	}
	// initial + 1 retry (max 2)
	if *callCount != 2 {
		t.Errorf("call count = %d, want 2 (initial + 1 retry)", *callCount)
	}

	assertKitabTitleEmpty(t, 1)
}

func TestTitleBulkTranslate_ServerErrorRetrySucceeds(t *testing.T) {
	setCronKey(t, "correct-secret")
	setTranslateRetryDelay(t, 0)

	callCount := 0
	orig := execCommandFunc
	execCommandFunc = func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		callCount++
		if callCount == 1 {
			output := `{"type":"error","error":{"data":{"message":"Rate limit exceeded"}}}`
			return []byte(output), nil, fmt.Errorf("exit status 1")
		}
		return bulkTextEvent(`{"1":"Book of Faith"}`), nil, nil
	}
	t.Cleanup(func() { execCommandFunc = orig })

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Updated != 1 {
		t.Errorf("updated = %d, want 1 (retry succeeded)", body.Updated)
	}
	if len(body.Failed) != 0 {
		t.Errorf("failed = %v, want empty (retry succeeded)", body.Failed)
	}
	if callCount != 2 {
		t.Errorf("call count = %d, want 2 (initial fail + retry success)", callCount)
	}

	assertKitabTitleEng(t, 1, "Book of Faith")
}

func TestTitleBulkTranslate_ModelFromEnv(t *testing.T) {
	setCronKey(t, "correct-secret")
	os.Setenv("OPENCODE_PROVIDER_ID", "opencode")
	os.Setenv("OPENCODE_MODEL_ID", "deepseek-v4-flash-free")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_PROVIDER_ID")
		os.Unsetenv("OPENCODE_MODEL_ID")
	})

	captured := setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: `{"1":"Book of Faith"}`}})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if len(*captured) != 1 {
		t.Fatalf("got %d CLI calls, want 1", len(*captured))
	}
	args := (*captured)[0]
	if !containsArg(args, "--model") {
		t.Error("missing --model flag")
	}
	modelIdx := indexOfArg(args, "--model")
	if modelIdx < 0 || args[modelIdx+1] != "opencode/deepseek-v4-flash-free" {
		t.Errorf("model = %v, want 'opencode/deepseek-v4-flash-free'", args[modelIdx+1])
	}
	if !containsArg(args, "--agent") || !containsArg(args, "translate-title-bulk") {
		t.Errorf("agent should be 'translate-title-bulk', got args: %v", args)
	}
}

func TestTitleBulkTranslate_NoModelWhenUnset(t *testing.T) {
	setCronKey(t, "correct-secret")
	os.Unsetenv("OPENCODE_PROVIDER_ID")
	os.Unsetenv("OPENCODE_MODEL_ID")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_PROVIDER_ID")
		os.Unsetenv("OPENCODE_MODEL_ID")
	})

	captured := setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: `{"1":"Book of Faith"}`}})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	args := (*captured)[0]
	if containsArg(args, "--model") {
		t.Error("--model flag should not be present when env vars are unset")
	}
}

func TestTitleBulkTranslate_PureFlagPresent(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: `{"1":"Book of Faith"}`}})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	args := (*captured)[0]
	if !containsArg(args, "--pure") {
		t.Error("--pure flag should be present in bulk translate CLI calls")
	}
}

func TestTitleBulkTranslate_EnvAgentDoesNotOverride(t *testing.T) {
	setCronKey(t, "correct-secret")
	os.Setenv("OPENCODE_AGENT", "plan")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_AGENT") })

	captured := setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: `{"1":"Book of Faith"}`}})

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	args := (*captured)[0]
	if !containsArg(args, "--agent") || !containsArg(args, "translate-title-bulk") {
		t.Errorf("agent should always be 'translate-title-bulk', got args: %v", args)
	}
}

func TestTitleBulkTranslate_RouteRegistered(t *testing.T) {
	app := setupTestApp(t)

	routes := app.Stack()
	found := false
	for _, methodRoutes := range routes {
		for _, route := range methodRoutes {
			if route.Method == "POST" && route.Path == "/ai/cron/translate/title/bulk/:kitabName" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected POST /ai/cron/translate/title/bulk/:kitabName route to be registered")
	}
}

func TestTitleBulkTranslate_SingleBatchRemainsDefault(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setSweepExecMock(t, 0)

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": nil},
		{"VMember": 2, "NKitabArab": "a2", "NKitab": "dua", "NKitabEng": nil},
		{"VMember": 3, "NKitabArab": "a3", "NKitab": "tiga", "NKitabEng": nil},
		{"VMember": 4, "NKitabArab": "a4", "NKitab": "empat", "NKitabEng": nil},
	})

	// No ?all= flag: exactly one batch of 1 row runs, even though more remain.
	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab&limit=1", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 1 {
		t.Errorf("processed = %d, want 1", body.Processed)
	}
	if body.Updated != 1 {
		t.Errorf("updated = %d, want 1", body.Updated)
	}
	if len(*captured) != 1 {
		t.Errorf("got %d CLI calls, want 1", len(*captured))
	}

	assertKitabTitleEng(t, 1, "english-1")
	assertKitabTitleEmpty(t, 2)
	assertKitabTitleEmpty(t, 3)
	assertKitabTitleEmpty(t, 4)
}

func TestTitleBulkTranslate_SweepProcessesAllBatches(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setSweepExecMock(t, 0)

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 5, "NKitabArab": "a5", "NKitab": "lima", "NKitabEng": nil},
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": nil},
		{"VMember": 3, "NKitabArab": "a3", "NKitab": "tiga", "NKitabEng": nil},
		{"VMember": 2, "NKitabArab": "a2", "NKitab": "dua", "NKitabEng": nil},
		{"VMember": 4, "NKitabArab": "a4", "NKitab": "empat", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab&all=1&limit=2", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 5 {
		t.Errorf("processed = %d, want 5", body.Processed)
	}
	if body.Updated != 5 {
		t.Errorf("updated = %d, want 5", body.Updated)
	}
	if len(body.Failed) != 0 {
		t.Errorf("failed = %v, want empty", body.Failed)
	}
	if len(*captured) != 3 {
		t.Errorf("got %d CLI calls, want 3 (5 rows / limit 2)", len(*captured))
	}

	assertKitabTitleEng(t, 1, "english-1")
	assertKitabTitleEng(t, 2, "english-2")
	assertKitabTitleEng(t, 3, "english-3")
	assertKitabTitleEng(t, 4, "english-4")
	assertKitabTitleEng(t, 5, "english-5")
}

func TestTitleBulkTranslate_SweepOrderedByID(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setSweepExecMock(t, 0)

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 100, "NKitabArab": "a100", "NKitab": "seratus", "NKitabEng": nil},
		{"VMember": 2, "NKitabArab": "a2", "NKitab": "dua", "NKitabEng": nil},
		{"VMember": 50, "NKitabArab": "a50", "NKitab": "lima puluh", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab&all=1&limit=1", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 3 {
		t.Errorf("processed = %d, want 3", body.Processed)
	}
	if body.Updated != 3 {
		t.Errorf("updated = %d, want 3", body.Updated)
	}

	// Batches must advance VMember ASC: 2 first, then 50, then 100.
	for i, want := range []uint{2, 50, 100} {
		args := (*captured)[i]
		prompt := args[len(args)-1]
		if !strings.Contains(prompt, fmt.Sprintf(`"%d":`, want)) {
			t.Errorf("batch %d prompt should contain key %d: %q", i, want, prompt)
		}
	}
}

func TestTitleBulkTranslate_SweepNoRowsNoCalls(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setSweepExecMock(t, 0)

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": "done"},
		{"VMember": 2, "NKitabArab": "a2", "NKitab": "dua", "NKitabEng": "done"},
	})
	seedBabShahihBukhari(t, []map[string]interface{}{
		{"VMemberBab": 1, "NBabArab": "b1", "NBab": "satu", "NBabEng": "done"},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&all=1", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 0 {
		t.Errorf("processed = %d, want 0", body.Processed)
	}
	if body.Updated != 0 {
		t.Errorf("updated = %d, want 0", body.Updated)
	}
	if len(*captured) != 0 {
		t.Errorf("got %d CLI calls, want 0", len(*captured))
	}
}

func TestTitleBulkTranslate_SweepMaxBatchesBounds(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setSweepExecMock(t, 0)

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": nil},
		{"VMember": 2, "NKitabArab": "a2", "NKitab": "dua", "NKitabEng": nil},
		{"VMember": 3, "NKitabArab": "a3", "NKitab": "tiga", "NKitabEng": nil},
		{"VMember": 4, "NKitabArab": "a4", "NKitab": "empat", "NKitabEng": nil},
		{"VMember": 5, "NKitabArab": "a5", "NKitab": "lima", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab&all=1&limit=1&maxBatches=2", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 2 {
		t.Errorf("processed = %d, want 2 (capped by maxBatches)", body.Processed)
	}
	if body.Updated != 2 {
		t.Errorf("updated = %d, want 2", body.Updated)
	}
	if len(body.Failed) != 0 {
		t.Errorf("failed = %v, want empty", body.Failed)
	}
	if len(*captured) != 2 {
		t.Errorf("got %d CLI calls, want 2", len(*captured))
	}

	assertKitabTitleEng(t, 1, "english-1")
	assertKitabTitleEng(t, 2, "english-2")
	assertKitabTitleEmpty(t, 3)
	assertKitabTitleEmpty(t, 4)
	assertKitabTitleEmpty(t, 5)
}

func TestTitleBulkTranslate_SweepFirstBatchFailsContinues(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setSweepExecMock(t, 1)

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": nil},
		{"VMember": 2, "NKitabArab": "a2", "NKitab": "dua", "NKitabEng": nil},
		{"VMember": 3, "NKitabArab": "a3", "NKitab": "tiga", "NKitabEng": nil},
	})

	// The first batch (VMember 1) fails; the sweep must continue with 2 and 3.
	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab&all=1&limit=1", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 3 {
		t.Errorf("processed = %d, want 3", body.Processed)
	}
	if body.Updated != 2 {
		t.Errorf("updated = %d, want 2", body.Updated)
	}
	if len(body.Failed) != 1 {
		t.Fatalf("failed = %v, want 1 entry", body.Failed)
	}
	if body.Failed[0].Nomer != 1 {
		t.Errorf("failed nomer = %d, want 1", body.Failed[0].Nomer)
	}
	wantErr := "opencode run failed: opencode exec failed at call 1"
	if body.Failed[0].Error != wantErr {
		t.Errorf("failed error = %q, want %q", body.Failed[0].Error, wantErr)
	}
	if len(*captured) != 3 {
		t.Errorf("got %d CLI calls, want 3 (failed row must be skipped to avoid an infinite loop)", len(*captured))
	}

	assertKitabTitleEmpty(t, 1)
	assertKitabTitleEng(t, 2, "english-2")
	assertKitabTitleEng(t, 3, "english-3")
}

func TestTitleBulkTranslate_SweepAllBatchesFailTerminates(t *testing.T) {
	setCronKey(t, "correct-secret")

	callCount := 0
	orig := execCommandFunc
	execCommandFunc = func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		callCount++
		return nil, nil, fmt.Errorf("opencode exec failed at call %d", callCount)
	}
	t.Cleanup(func() { execCommandFunc = orig })

	app := setupTestApp(t)
	seedKitabShahihBukhari(t, []map[string]interface{}{
		{"VMember": 1, "NKitabArab": "a1", "NKitab": "satu", "NKitabEng": nil},
		{"VMember": 2, "NKitabArab": "a2", "NKitab": "dua", "NKitabEng": nil},
		{"VMember": 3, "NKitabArab": "a3", "NKitab": "tiga", "NKitabEng": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/title/bulk/ShahihBukhari?key=correct-secret&type=kitab&all=1&limit=1", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body titleBulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 3 {
		t.Errorf("processed = %d, want 3", body.Processed)
	}
	if body.Updated != 0 {
		t.Errorf("updated = %d, want 0", body.Updated)
	}
	if len(body.Failed) != 3 {
		t.Errorf("failed = %v, want 3 entries", body.Failed)
	}
	// Even though every batch fails, the loop must terminate after 3 attempts.
	if callCount != 3 {
		t.Errorf("call count = %d, want 3 (must not loop forever)", callCount)
	}

	assertKitabTitleEmpty(t, 1)
	assertKitabTitleEmpty(t, 2)
	assertKitabTitleEmpty(t, 3)
}
