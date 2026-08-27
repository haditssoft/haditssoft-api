package opencode

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
)

type translateMockConfig struct {
	failAtCall     int // 1-based call index to fail; 0 = none
	failEveryCall  bool
	replyOverrides map[int]string // 1-based call index -> text to return
}

func setTranslateExecMock(t *testing.T, cfg translateMockConfig) *[][]string {
	t.Helper()

	callCount := 0
	captured := &[][]string{}
	orig := execCommandFunc
	execCommandFunc = func(name string, args ...string) ([]byte, error) {
		callCount++
		*captured = append(*captured, append([]string{name}, args...))

		if cfg.failEveryCall || (cfg.failAtCall > 0 && callCount == cfg.failAtCall) {
			return nil, fmt.Errorf("opencode exec failed at call %d", callCount)
		}

		text := fmt.Sprintf("translated-%d", callCount)
		if override, ok := cfg.replyOverrides[callCount]; ok {
			text = override
		}

		output := fmt.Sprintf(`{"type":"text","part":{"text":"%s"}}`, text)
		return []byte(output), nil
	}
	t.Cleanup(func() { execCommandFunc = orig })
	return captured
}

func setCronKey(t *testing.T, key string) {
	t.Helper()
	os.Setenv("OPENCODE_CRON_KEY", key)
	t.Cleanup(func() { os.Unsetenv("OPENCODE_CRON_KEY") })
}

func seedShahihBukhari(t *testing.T, rows []map[string]interface{}) {
	t.Helper()
	if err := database.DB.Exec(`CREATE TABLE IF NOT EXISTS "ShahihBukhari" ("Nomer" INTEGER PRIMARY KEY, "Arabic" TEXT, "Indonesia" TEXT, "English" TEXT)`).Error; err != nil {
		t.Fatalf("failed to create kitab table: %v", err)
	}
	for _, row := range rows {
		if err := database.DB.Exec(`INSERT INTO "ShahihBukhari" ("Nomer", "Arabic", "Indonesia", "English") VALUES (?, ?, ?, ?)`,
			row["Nomer"], row["Arabic"], row["Indonesia"], row["English"]).Error; err != nil {
			t.Fatalf("failed to seed row %v: %v", row, err)
		}
	}
}

func getKitabEnglish(t *testing.T, nomer uint) *string {
	t.Helper()
	var row struct {
		English *string
	}
	if err := database.DB.Table("ShahihBukhari").Select("English").Where("Nomer = ?", nomer).First(&row).Error; err != nil {
		t.Fatalf("failed to read English for Nomer %d: %v", nomer, err)
	}
	return row.English
}

func assertEnglish(t *testing.T, nomer uint, want string) {
	t.Helper()
	eng := getKitabEnglish(t, nomer)
	if eng == nil {
		t.Errorf("Nomer %d English = nil, want %q", nomer, want)
		return
	}
	if *eng != want {
		t.Errorf("Nomer %d English = %q, want %q", nomer, *eng, want)
	}
}

func assertEnglishEmpty(t *testing.T, nomer uint) {
	t.Helper()
	eng := getKitabEnglish(t, nomer)
	if eng != nil && *eng != "" {
		t.Errorf("Nomer %d English = %q, want empty", nomer, *eng)
	}
}

type translateResponse struct {
	Processed int               `json:"processed"`
	Updated   int               `json:"updated"`
	Failed    []translateResult `json:"failed"`
}

func TestTranslate_MissingKey(t *testing.T) {
	os.Unsetenv("OPENCODE_CRON_KEY")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_CRON_KEY") })

	app := setupTestApp(t)

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	var body map[string]interface{}
	decodeJSON(t, resp, &body)
	if body["status"] != "error" {
		t.Errorf("status = %v, want 'error'", body["status"])
	}
}

func TestTranslate_WrongKey(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=wrong-secret", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestTranslate_NoKeyConfigured(t *testing.T) {
	os.Unsetenv("OPENCODE_CRON_KEY")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_CRON_KEY") })

	app := setupTestApp(t)

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=anything", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (fail closed when key unset)", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestTranslate_WrongMethod(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	resp := makeRequest(t, app, "GET", "/ai/cron/translate/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode == http.StatusOK {
		t.Error("GET should not return 200")
	}
}

func TestTranslate_UnknownKitab(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/NotARealKitab?key=correct-secret", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var body map[string]interface{}
	decodeJSON(t, resp, &body)
	if body["message"] != "unknown kitab: NotARealKitab" {
		t.Errorf("message = %v, want 'unknown kitab: NotARealKitab'", body["message"])
	}
}

func TestTranslate_InvalidLimit(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	for _, raw := range []string{"abc", "0", "-5"} {
		resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret&limit="+raw, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("limit=%q status = %d, want %d", raw, resp.StatusCode, http.StatusBadRequest)
		}
	}
}

func TestTranslate_SuccessUpdatesEnglish(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setTranslateExecMock(t, translateMockConfig{})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one arabic", "Indonesia": "terjemah satu", "English": ""},
		{"Nomer": 2, "Arabic": "hadith two arabic", "Indonesia": "terjemah dua", "English": nil},
		{"Nomer": 3, "Arabic": "hadith three arabic", "Indonesia": "terjemah tiga", "English": "already translated"},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body translateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 2 {
		t.Errorf("processed = %d, want 2", body.Processed)
	}
	if body.Updated != 2 {
		t.Errorf("updated = %d, want 2", body.Updated)
	}
	if len(body.Failed) != 0 {
		t.Errorf("failed = %v, want empty list", body.Failed)
	}

	assertEnglish(t, 1, "translated-1")
	assertEnglish(t, 2, "translated-2")
	if eng := getKitabEnglish(t, 3); eng == nil || *eng != "already translated" {
		t.Errorf("row 3 English = %v, want unchanged 'already translated'", eng)
	}

	if len(*captured) != 2 {
		t.Fatalf("got %d CLI calls, want 2", len(*captured))
	}

	// Verify prompt content in first call
	args := (*captured)[0]
	prompt := args[len(args)-1]
	if !strings.Contains(prompt, "Teks Arab:\nhadith one arabic") {
		t.Errorf("prompt = %q, want it to contain Arabic text", prompt)
	}
	if !strings.Contains(prompt, "Teks Indonesia:\nterjemah satu") {
		t.Errorf("prompt = %q, want it to contain Indonesian text", prompt)
	}
}

func TestTranslate_AgentAlwaysTranslate(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setTranslateExecMock(t, translateMockConfig{})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "terjemah satu", "English": nil},
	})

	os.Setenv("OPENCODE_AGENT", "plan")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_AGENT") })

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if len(*captured) != 1 {
		t.Fatalf("got %d CLI calls, want 1", len(*captured))
	}
	args := (*captured)[0]
	if !containsArg(args, "--agent") || !containsArg(args, "translate") {
		t.Errorf("agent flag not 'translate', got args: %v", args)
	}
}

func TestTranslate_ModelAndAgentFromEnv(t *testing.T) {
	setCronKey(t, "correct-secret")
	os.Setenv("OPENCODE_PROVIDER_ID", "opencode")
	os.Setenv("OPENCODE_MODEL_ID", "deepseek-v4-flash-free")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_PROVIDER_ID")
		os.Unsetenv("OPENCODE_MODEL_ID")
	})

	captured := setTranslateExecMock(t, translateMockConfig{})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "terjemah satu", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret", nil)
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
	if !containsArg(args, "--agent") || !containsArg(args, "translate") {
		t.Errorf("agent should be 'translate', got args: %v", args)
	}
}

func TestTranslate_RespectsLimitAndOrder(t *testing.T) {
	setCronKey(t, "correct-secret")
	setTranslateExecMock(t, translateMockConfig{})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 5, "Arabic": "hadith five", "Indonesia": "lima", "English": ""},
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": ""},
		{"Nomer": 3, "Arabic": "hadith three", "Indonesia": "tiga", "English": ""},
		{"Nomer": 2, "Arabic": "hadith two", "Indonesia": "dua", "English": ""},
		{"Nomer": 4, "Arabic": "hadith four", "Indonesia": "empat", "English": ""},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret&limit=2", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body translateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 2 {
		t.Errorf("processed = %d, want 2", body.Processed)
	}
	if body.Updated != 2 {
		t.Errorf("updated = %d, want 2", body.Updated)
	}

	assertEnglish(t, 1, "translated-1")
	assertEnglish(t, 2, "translated-2")
	assertEnglishEmpty(t, 3)
	assertEnglishEmpty(t, 4)
	assertEnglishEmpty(t, 5)
}

func TestTranslate_CLIFailureContinues(t *testing.T) {
	setCronKey(t, "correct-secret")
	setTranslateExecMock(t, translateMockConfig{failAtCall: 2})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "terjemah satu", "English": nil},
		{"Nomer": 2, "Arabic": "hadith two", "Indonesia": "terjemah dua", "English": nil},
		{"Nomer": 3, "Arabic": "hadith three", "Indonesia": "terjemah tiga", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body translateResponse
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
	if body.Failed[0].Nomer != 2 {
		t.Errorf("failed nomer = %d, want 2", body.Failed[0].Nomer)
	}

	assertEnglish(t, 1, "translated-1")
	assertEnglishEmpty(t, 2)
	assertEnglish(t, 3, "translated-3")
}

func TestTranslate_EveryCLIFailure(t *testing.T) {
	setCronKey(t, "correct-secret")
	setTranslateExecMock(t, translateMockConfig{failEveryCall: true})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "terjemah satu", "English": nil},
		{"Nomer": 2, "Arabic": "hadith two", "Indonesia": "terjemah dua", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body translateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 2 {
		t.Errorf("processed = %d, want 2", body.Processed)
	}
	if body.Updated != 0 {
		t.Errorf("updated = %d, want 0", body.Updated)
	}
	if len(body.Failed) != 2 {
		t.Errorf("failed = %v, want 2 entries", body.Failed)
	}

	assertEnglishEmpty(t, 1)
	assertEnglishEmpty(t, 2)
}

func TestTranslate_EmptyReplyCountedAsFailed(t *testing.T) {
	setCronKey(t, "correct-secret")
	// Empty NDJSON output (no text events) -> "no text response" error -> "failed to get AI response"
	setTranslateExecMock(t, translateMockConfig{replyOverrides: map[int]string{1: ""}})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "terjemah satu", "English": nil},
		{"Nomer": 2, "Arabic": "hadith two", "Indonesia": "terjemah dua", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body translateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 2 {
		t.Errorf("processed = %d, want 2", body.Processed)
	}
	if body.Updated != 1 {
		t.Errorf("updated = %d, want 1", body.Updated)
	}
	if len(body.Failed) != 1 {
		t.Fatalf("failed = %v, want 1 entry", body.Failed)
	}
	if body.Failed[0].Nomer != 1 {
		t.Errorf("failed nomer = %d, want 1", body.Failed[0].Nomer)
	}
	if body.Failed[0].Error != "failed to get AI response" {
		t.Errorf("failed error = %q, want 'failed to get AI response'", body.Failed[0].Error)
	}

	assertEnglishEmpty(t, 1)
	assertEnglish(t, 2, "translated-2")
}

func TestTranslate_WhitespaceReplyCountedAsFailed(t *testing.T) {
	setCronKey(t, "correct-secret")
	setTranslateExecMock(t, translateMockConfig{replyOverrides: map[int]string{1: "   "}})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "terjemah satu", "English": nil},
		{"Nomer": 2, "Arabic": "hadith two", "Indonesia": "terjemah dua", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body translateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 2 {
		t.Errorf("processed = %d, want 2", body.Processed)
	}
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

	assertEnglishEmpty(t, 1)
	assertEnglish(t, 2, "translated-2")
}

func TestTranslate_NoMatchingRows(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setTranslateExecMock(t, translateMockConfig{})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "terjemah satu", "English": "already translated"},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body translateResponse
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

func TestTranslate_RouteRegistered(t *testing.T) {
	app := setupTestApp(t)

	routes := app.Stack()
	found := false
	for _, methodRoutes := range routes {
		for _, route := range methodRoutes {
			if route.Method == "POST" && route.Path == "/ai/cron/translate/:kitabName" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected POST /ai/cron/translate/:kitabName route to be registered")
	}
}
