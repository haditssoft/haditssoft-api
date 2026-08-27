package opencode

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
)

type translateMockConfig struct {
	failAtCall     int // 1-based call index to fail; 0 = none
	failEveryCall  bool
	replyOverrides map[int]string // 1-based call index -> text to return
	stderrOnFail   string         // stderr content to return on failure
}

func setTranslateExecMock(t *testing.T, cfg translateMockConfig) *[][]string {
	t.Helper()

	callCount := 0
	captured := &[][]string{}
	orig := execCommandFunc
	execCommandFunc = func(name string, args ...string) ([]byte, []byte, error) {
		callCount++
		*captured = append(*captured, append([]string{name}, args...))

		if cfg.failEveryCall || (cfg.failAtCall > 0 && callCount == cfg.failAtCall) {
			stderr := []byte(cfg.stderrOnFail)
			return nil, stderr, fmt.Errorf("opencode exec failed at call %d", callCount)
		}

		text := fmt.Sprintf("translated-%d", callCount)
		if override, ok := cfg.replyOverrides[callCount]; ok {
			text = override
		}

		output := fmt.Sprintf(`{"type":"text","part":{"text":"%s"}}`, text)
		return []byte(output), nil, nil
	}
	t.Cleanup(func() { execCommandFunc = orig })
	return captured
}

// setServerErrMock installs a mock that always returns a server error event.
func setServerErrMock(t *testing.T, errMsg string) *int {
	t.Helper()
	callCount := 0
	orig := execCommandFunc
	execCommandFunc = func(name string, args ...string) ([]byte, []byte, error) {
		callCount++
		output := fmt.Sprintf(`{"type":"error","error":{"data":{"message":"%s"}}}`, errMsg)
		return []byte(output), nil, fmt.Errorf("exit status 1")
	}
	t.Cleanup(func() { execCommandFunc = orig })
	return &callCount
}

// setTranslateRetryDelay overrides the retry delay for tests and restores on cleanup.
func setTranslateRetryDelay(t *testing.T, d time.Duration) {
	t.Helper()
	orig := translateRetryDelay
	translateRetryDelay = d
	t.Cleanup(func() { translateRetryDelay = orig })
}

// setTranslateMaxRetries overrides the max retry count for tests and restores on cleanup.
func setTranslateMaxRetries(t *testing.T, n int) {
	t.Helper()
	orig := translateMaxRetries
	translateMaxRetries = n
	t.Cleanup(func() { translateMaxRetries = orig })
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

	args := (*captured)[0]
	prompt := args[len(args)-1]
	if !strings.Contains(prompt, "Arabic:\nhadith one arabic") {
		t.Errorf("prompt = %q, want it to contain Arabic text", prompt)
	}
	if !strings.Contains(prompt, "Indonesian:\nterjemah satu") {
		t.Errorf("prompt = %q, want it to contain Indonesian text", prompt)
	}
}

func TestTranslate_PureFlagPresent(t *testing.T) {
	setCronKey(t, "correct-secret")
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
	if !containsArg(args, "--pure") {
		t.Error("--pure flag should be present in translate CLI calls")
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
	if body.Failed[0].Error != "opencode run failed: opencode exec failed at call 2" {
		t.Errorf("failed error = %q, want 'opencode run failed: opencode exec failed at call 2'", body.Failed[0].Error)
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
	if body.Failed[0].Error != "no text response in opencode output" {
		t.Errorf("failed error = %q, want 'no text response in opencode output'", body.Failed[0].Error)
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

func TestTranslate_ServerErrorSurfaced(t *testing.T) {
	setCronKey(t, "correct-secret")
	setTranslateRetryDelay(t, 0)
	setTranslateMaxRetries(t, 2)
	callCount := setServerErrMock(t, "Rate limit exceeded. Please try again later.")

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "terjemah satu", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body translateResponse
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

	wantErr := "opencode server error: Rate limit exceeded. Please try again later."
	if body.Failed[0].Error != wantErr {
		t.Errorf("failed error = %q, want %q", body.Failed[0].Error, wantErr)
	}

	// Server error triggers retry, so 2 calls total (initial + 1 retry)
	if *callCount != 2 {
		t.Errorf("call count = %d, want 2 (initial + 1 retry)", *callCount)
	}

	assertEnglishEmpty(t, 1)
}

func TestTranslate_ServerErrorRetrySucceeds(t *testing.T) {
	setCronKey(t, "correct-secret")
	setTranslateRetryDelay(t, 0)

	callCount := 0
	orig := execCommandFunc
	execCommandFunc = func(name string, args ...string) ([]byte, []byte, error) {
		callCount++
		if callCount == 1 {
			output := `{"type":"error","error":{"data":{"message":"Rate limit exceeded"}}}`
			return []byte(output), nil, fmt.Errorf("exit status 1")
		}
		return []byte(`{"type":"text","part":{"text":"translated successfully"}}`), nil, nil
	}
	t.Cleanup(func() { execCommandFunc = orig })

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "terjemah satu", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body translateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 1 {
		t.Errorf("processed = %d, want 1", body.Processed)
	}
	if body.Updated != 1 {
		t.Errorf("updated = %d, want 1 (retry succeeded)", body.Updated)
	}
	if len(body.Failed) != 0 {
		t.Errorf("failed = %v, want empty (retry succeeded)", body.Failed)
	}
	if callCount != 2 {
		t.Errorf("call count = %d, want 2 (initial fail + retry success)", callCount)
	}

	assertEnglish(t, 1, "translated successfully")
}

func TestTranslate_NonRetryableExecErrorNoRetry(t *testing.T) {
	setCronKey(t, "correct-secret")
	setTranslateRetryDelay(t, 0)

	callCount := 0
	orig := execCommandFunc
	execCommandFunc = func(name string, args ...string) ([]byte, []byte, error) {
		callCount++
		return nil, nil, fmt.Errorf("executable not found")
	}
	t.Cleanup(func() { execCommandFunc = orig })

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "terjemah satu", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body translateResponse
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

	wantErr := "opencode run failed: executable not found"
	if body.Failed[0].Error != wantErr {
		t.Errorf("failed error = %q, want %q", body.Failed[0].Error, wantErr)
	}

	if callCount != 1 {
		t.Errorf("call count = %d, want 1 (no retry for exec failures)", callCount)
	}
}

func TestTranslate_PermanentModelErrorNoRetry(t *testing.T) {
	setCronKey(t, "correct-secret")
	setTranslateRetryDelay(t, 0)

	callCount := 0
	orig := execCommandFunc
	execCommandFunc = func(name string, args ...string) ([]byte, []byte, error) {
		callCount++
		output := `{"type":"error","error":{"data":{"message":"Model not found: opencode/deepseek-v4-flash-free. Did you mean: hy3-free?"}}}`
		return []byte(output), nil, fmt.Errorf("exit status 1")
	}
	t.Cleanup(func() { execCommandFunc = orig })

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "terjemah satu", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body translateResponse
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

	if !strings.Contains(body.Failed[0].Error, "Model not found") {
		t.Errorf("failed error should contain 'Model not found': %q", body.Failed[0].Error)
	}

	// Permanent error: no retry
	if callCount != 1 {
		t.Errorf("call count = %d, want 1 (no retry for model not found)", callCount)
	}
}

func TestTranslate_PermanentAuthErrorNoRetry(t *testing.T) {
	setCronKey(t, "correct-secret")
	setTranslateRetryDelay(t, 0)

	callCount := 0
	orig := execCommandFunc
	execCommandFunc = func(name string, args ...string) ([]byte, []byte, error) {
		callCount++
		output := `{"type":"error","error":{"data":{"message":"Unauthorized: Invalid API key"}}}`
		return []byte(output), nil, fmt.Errorf("exit status 1")
	}
	t.Cleanup(func() { execCommandFunc = orig })

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "terjemah satu", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body translateResponse
	decodeJSON(t, resp, &body)
	if len(body.Failed) != 1 {
		t.Fatalf("failed = %v, want 1 entry", body.Failed)
	}

	if !strings.Contains(body.Failed[0].Error, "Unauthorized") {
		t.Errorf("failed error should contain 'Unauthorized': %q", body.Failed[0].Error)
	}

	if callCount != 1 {
		t.Errorf("call count = %d, want 1 (no retry for auth errors)", callCount)
	}
}

func TestTranslate_RetryPreservesOtherRows(t *testing.T) {
	setCronKey(t, "correct-secret")
	setTranslateRetryDelay(t, 0)

	callCount := 0
	orig := execCommandFunc
	execCommandFunc = func(name string, args ...string) ([]byte, []byte, error) {
		callCount++
		switch callCount {
		case 1:
			return []byte(`{"type":"error","error":{"data":{"message":"rate limit"}}}`), nil, fmt.Errorf("exit status 1")
		case 2:
			return []byte(`{"type":"text","part":{"text":"row1 translated"}}`), nil, nil
		case 3:
			return []byte(`{"type":"text","part":{"text":"row2 translated"}}`), nil, nil
		}
		return nil, nil, fmt.Errorf("unexpected call %d", callCount)
	}
	t.Cleanup(func() { execCommandFunc = orig })

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
	if body.Updated != 2 {
		t.Errorf("updated = %d, want 2", body.Updated)
	}
	if len(body.Failed) != 0 {
		t.Errorf("failed = %v, want empty", body.Failed)
	}

	assertEnglish(t, 1, "row1 translated")
	assertEnglish(t, 2, "row2 translated")
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

func TestBuildTranslatePrompt(t *testing.T) {
	tests := []struct {
		name       string
		arabic     string
		indonesian string
		want       string
	}{
		{
			name:       "basic prompt",
			arabic:     "بِسْمِ اللَّهِ الرَّحْمَنِ الرَّحِيمِ",
			indonesian: "Dengan nama Allah Yang Maha Pengasih lagi Maha Penyayang",
			want:       "Arabic:\nبِسْمِ اللَّهِ الرَّحْمَنِ الرَّحِيمِ\n\nIndonesian:\nDengan nama Allah Yang Maha Pengasih lagi Maha Penyayang",
		},
		{
			name:       "empty arabic",
			arabic:     "",
			indonesian: "test",
			want:       "Arabic:\n\n\nIndonesian:\ntest",
		},
		{
			name:       "empty both",
			arabic:     "",
			indonesian: "",
			want:       "Arabic:\n\n\nIndonesian:\n",
		},
		{
			name:       "multiline text",
			arabic:     "line1\nline2",
			indonesian: "garis1\ngaris2",
			want:       "Arabic:\nline1\nline2\n\nIndonesian:\ngaris1\ngaris2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTranslatePrompt(tt.arabic, tt.indonesian)
			if got != tt.want {
				t.Errorf("buildTranslatePrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsPermanentServerError(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{"model not found lowercase", "opencode server error: model not found: xyz", true},
		{"model not found mixed case", "opencode server error: Model not found: xyz", true},
		{"ProviderModelNotFoundError", "opencode server error: ProviderModelNotFoundError: Model not found", true},
		{"unauthorized", "opencode server error: unauthorized access", true},
		{"Unauthorized", "opencode server error: Unauthorized: Invalid API key", true},
		{"invalid API key", "opencode server error: Invalid API key provided", true},
		{"authentication", "opencode server error: authentication failed", true},
		{"permission denied", "opencode server error: permission denied", true},
		{"rate limit - not permanent", "opencode server error: Rate limit exceeded", false},
		{"endpoint unavailable - not permanent", "opencode server error: Endpoint is unavailable", false},
		{"generic server error - not permanent", "opencode server error: Unexpected server error", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPermanentServerError(tt.msg)
			if got != tt.want {
				t.Errorf("isPermanentServerError(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

func TestIsRetryableServerError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"exec failure", fmt.Errorf("opencode run failed: executable not found"), false},
		{"rate limit - retryable", fmt.Errorf("opencode server error: Rate limit exceeded"), true},
		{"endpoint unavailable - retryable", fmt.Errorf("opencode server error: Endpoint is unavailable"), true},
		{"generic error - retryable", fmt.Errorf("opencode server error: Unexpected server error"), true},
		{"model not found - permanent", fmt.Errorf("opencode server error: Model not found: xyz"), false},
		{"unauthorized - permanent", fmt.Errorf("opencode server error: Unauthorized"), false},
		{"auth failure - permanent", fmt.Errorf("opencode server error: Invalid API key"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableServerError(tt.err)
			if got != tt.want {
				t.Errorf("isRetryableServerError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestTranslate_MultipleRetriesExhausted(t *testing.T) {
	setCronKey(t, "correct-secret")
	setTranslateRetryDelay(t, 0)
	setTranslateMaxRetries(t, 3)

	callCount := setServerErrMock(t, "Endpoint is unavailable")

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "terjemah satu", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body translateResponse
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
	if !strings.Contains(body.Failed[0].Error, "Endpoint is unavailable") {
		t.Errorf("failed error should contain 'Endpoint is unavailable': %q", body.Failed[0].Error)
	}

	// 3 attempts: initial + 2 retries
	if *callCount != 3 {
		t.Errorf("call count = %d, want 3 (initial + 2 retries)", *callCount)
	}

	assertEnglishEmpty(t, 1)
}

func TestTranslate_RetryConfigFromEnv(t *testing.T) {
	setCronKey(t, "correct-secret")
	setTranslateRetryDelay(t, 0)
	setTranslateMaxRetries(t, 2)

	// Set env vars — loadTranslateConfig() will override the vars
	os.Setenv("OPENCODE_TRANSLATE_RETRY_COUNT", "5")
	os.Setenv("OPENCODE_TRANSLATE_RETRY_DELAY", "1")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_TRANSLATE_RETRY_COUNT")
		os.Unsetenv("OPENCODE_TRANSLATE_RETRY_DELAY")
	})

	// Reset to known defaults before loadTranslateConfig reads env
	origRetries := translateMaxRetries
	origDelay := translateRetryDelay
	translateMaxRetries = defaultTranslateRetries
	translateRetryDelay = time.Duration(defaultTranslateDelaySec) * time.Second
	t.Cleanup(func() {
		translateMaxRetries = origRetries
		translateRetryDelay = origDelay
	})

	loadTranslateConfig()

	if translateMaxRetries != 5 {
		t.Errorf("translateMaxRetries = %d, want 5 (from env)", translateMaxRetries)
	}
	if translateRetryDelay != 1*time.Second {
		t.Errorf("translateRetryDelay = %v, want 1s (from env)", translateRetryDelay)
	}
}

func TestTranslate_LoadConfigDefaults(t *testing.T) {
	// Ensure env vars are absent
	os.Unsetenv("OPENCODE_TRANSLATE_RETRY_COUNT")
	os.Unsetenv("OPENCODE_TRANSLATE_RETRY_DELAY")

	// Reset to defaults
	translateMaxRetries = defaultTranslateRetries
	translateRetryDelay = time.Duration(defaultTranslateDelaySec) * time.Second

	loadTranslateConfig()

	if translateMaxRetries != defaultTranslateRetries {
		t.Errorf("translateMaxRetries = %d, want %d (default)", translateMaxRetries, defaultTranslateRetries)
	}
	if translateRetryDelay != time.Duration(defaultTranslateDelaySec)*time.Second {
		t.Errorf("translateRetryDelay = %v, want %ds (default)", translateRetryDelay, defaultTranslateDelaySec)
	}
}
