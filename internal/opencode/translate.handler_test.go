package opencode

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
)

type translateMockConfig struct {
	failSessionAt    int // 1-based exact session call index to fail; 0 = none
	failMessageAt    int // 1-based exact message call index to fail; 0 = none
	failEverySession bool
	failEveryMessage bool
	replyOverrides   map[int]string // 1-based message index -> text to return instead of "translated-N"
}

// setupTranslateMock spins up a mock opencode serve that records every message
// request body. Failures are controlled by cfg (see translateMockConfig).
func setupTranslateMock(t *testing.T, cfg translateMockConfig) (*httptest.Server, *[][]byte) {
	t.Helper()

	setTranslationPromptFile(t)

	var msgBodies [][]byte
	var sessionCount, msgCount int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/session" && r.Method == http.MethodPost:
			sessionCount++
			if cfg.failEverySession || (cfg.failSessionAt > 0 && sessionCount == cfg.failSessionAt) {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"id": fmt.Sprintf("sess-%d", sessionCount)})

		case strings.HasPrefix(r.URL.Path, "/session/") && strings.HasSuffix(r.URL.Path, "/message") && r.Method == http.MethodPost:
			msgCount++
			body, _ := io.ReadAll(r.Body)
			r.Body.Close()
			msgBodies = append(msgBodies, body)
			if cfg.failEveryMessage || (cfg.failMessageAt > 0 && msgCount == cfg.failMessageAt) {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			text := fmt.Sprintf("translated-%d", msgCount)
			if override, ok := cfg.replyOverrides[msgCount]; ok {
				text = override
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"parts": []map[string]string{{"type": "text", "text": text}},
			})

		case strings.HasPrefix(r.URL.Path, "/session/") && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusOK)

		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	return srv, &msgBodies
}

// extractPromptText pulls the first text part out of a captured message request
// body (the outbound opencode payload carries the user text under parts).
func extractPromptText(t *testing.T, body []byte) string {
	t.Helper()

	var msg map[string]interface{}
	if err := json.Unmarshal(body, &msg); err != nil {
		t.Fatalf("failed to decode captured body: %v", err)
	}
	parts, ok := msg["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		t.Fatalf("no parts in captured message body")
	}
	part, ok := parts[0].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected part shape: %v", parts[0])
	}
	prompt, _ := part["text"].(string)
	return prompt
}

func setCronKey(t *testing.T, key string) {
	t.Helper()
	os.Setenv("OPENCODE_CRON_KEY", key)
	t.Cleanup(func() { os.Unsetenv("OPENCODE_CRON_KEY") })
}

// repoRootPromptPath resolves the repo-root translation_system_prompt.txt
// relative to this package dir (tests always run with cwd = package dir).
func repoRootPromptPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "translation_system_prompt.txt")
}

func setTranslationPromptFile(t *testing.T) {
	t.Helper()
	os.Setenv("TRANSLATION_SYSTEM_PROMPT_FILE", repoRootPromptPath(t))
	t.Cleanup(func() { os.Unsetenv("TRANSLATION_SYSTEM_PROMPT_FILE") })
}

// loadPromptForTest reads and trims the repo-root prompt file, mirroring how
// the production loader normalizes the file contents.
func loadPromptForTest(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(repoRootPromptPath(t))
	if err != nil {
		t.Fatalf("failed to read repo-root prompt file: %v", err)
	}
	return strings.TrimSpace(string(data))
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

func TestTranslate_MissingOpenCodeURL(t *testing.T) {
	os.Unsetenv("OPENCODE_URL")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_URL") })
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "terjemah satu", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	var body map[string]interface{}
	decodeJSON(t, resp, &body)
	if body["message"] != "OPENCODE_URL is not configured" {
		t.Errorf("message = %v, want 'OPENCODE_URL is not configured'", body["message"])
	}
}

func TestTranslate_SuccessUpdatesEnglish(t *testing.T) {
	setCronKey(t, "correct-secret")
	srv, msgBodies := setupTranslateMock(t, translateMockConfig{})
	os.Setenv("OPENCODE_URL", srv.URL)
	t.Cleanup(func() { os.Unsetenv("OPENCODE_URL") })

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one arabic", "Indonesia": "terjemah satu", "English": ""},
		{"Nomer": 2, "Arabic": "hadith two arabic", "Indonesia": "terjemah dua", "English": nil},
		{"Nomer": 3, "Arabic": "hadith three arabic", "Indonesia": "terjemah tiga", "English": "already translated"},
	})

	// No Authorization header: cron endpoint must not require JWT
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

	bodies := *msgBodies
	if len(bodies) != 2 {
		t.Fatalf("got %d message requests, want 2", len(bodies))
	}

	var first map[string]interface{}
	if err := json.Unmarshal(bodies[0], &first); err != nil {
		t.Fatalf("failed to decode captured body: %v", err)
	}
	if sys, _ := first["system"].(string); sys != loadPromptForTest(t) {
		t.Error("system prompt sent to opencode does not match the translation prompt file")
	}
	prompt := extractPromptText(t, bodies[0])
	if !strings.Contains(prompt, "Teks Arab:\nhadith one arabic") {
		t.Errorf("prompt = %q, want it to contain Arabic text", prompt)
	}
	if !strings.Contains(prompt, "Teks Indonesia:\nterjemah satu") {
		t.Errorf("prompt = %q, want it to contain Indonesian text", prompt)
	}
	if agent, _ := first["agent"].(string); agent != "plan" {
		t.Errorf("agent = %v, want 'plan' (default)", agent)
	}
}

func TestTranslate_ModelAndAgentFromEnv(t *testing.T) {
	setCronKey(t, "correct-secret")
	os.Setenv("OPENCODE_PROVIDER_ID", "opencode")
	os.Setenv("OPENCODE_MODEL_ID", "deepseek-v4-flash-free")
	os.Setenv("OPENCODE_AGENT", "build")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_PROVIDER_ID")
		os.Unsetenv("OPENCODE_MODEL_ID")
		os.Unsetenv("OPENCODE_AGENT")
	})

	srv, msgBodies := setupTranslateMock(t, translateMockConfig{})
	os.Setenv("OPENCODE_URL", srv.URL)
	t.Cleanup(func() { os.Unsetenv("OPENCODE_URL") })

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "terjemah satu", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	bodies := *msgBodies
	if len(bodies) != 1 {
		t.Fatalf("got %d message requests, want 1", len(bodies))
	}

	var body map[string]interface{}
	if err := json.Unmarshal(bodies[0], &body); err != nil {
		t.Fatalf("failed to decode captured body: %v", err)
	}
	model, ok := body["model"].(map[string]interface{})
	if !ok {
		t.Fatal("model field missing from request body")
	}
	if model["providerID"] != "opencode" {
		t.Errorf("providerID = %v, want 'opencode'", model["providerID"])
	}
	if model["modelID"] != "deepseek-v4-flash-free" {
		t.Errorf("modelID = %v, want 'deepseek-v4-flash-free'", model["modelID"])
	}
	if agent, _ := body["agent"].(string); agent != "build" {
		t.Errorf("agent = %v, want 'build'", agent)
	}
}

func TestTranslate_RespectsLimitAndOrder(t *testing.T) {
	setCronKey(t, "correct-secret")
	srv, _ := setupTranslateMock(t, translateMockConfig{})
	os.Setenv("OPENCODE_URL", srv.URL)
	t.Cleanup(func() { os.Unsetenv("OPENCODE_URL") })

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

func TestTranslate_LLMFailureContinues(t *testing.T) {
	setCronKey(t, "correct-secret")
	srv, _ := setupTranslateMock(t, translateMockConfig{failSessionAt: 2}) // 2nd session creation fails
	os.Setenv("OPENCODE_URL", srv.URL)
	t.Cleanup(func() { os.Unsetenv("OPENCODE_URL") })

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
	assertEnglish(t, 3, "translated-2")
}

func TestTranslate_EmptyReplyCountedAsFailed(t *testing.T) {
	setCronKey(t, "correct-secret")
	srv, _ := setupTranslateMock(t, translateMockConfig{replyOverrides: map[int]string{1: ""}})
	os.Setenv("OPENCODE_URL", srv.URL)
	t.Cleanup(func() { os.Unsetenv("OPENCODE_URL") })

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

func TestTranslate_WhitespaceReplyCountedAsFailed(t *testing.T) {
	setCronKey(t, "correct-secret")
	srv, _ := setupTranslateMock(t, translateMockConfig{replyOverrides: map[int]string{1: "   "}})
	os.Setenv("OPENCODE_URL", srv.URL)
	t.Cleanup(func() { os.Unsetenv("OPENCODE_URL") })

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

func TestTranslate_SessionCreationFailure(t *testing.T) {
	setCronKey(t, "correct-secret")
	srv, msgBodies := setupTranslateMock(t, translateMockConfig{failEverySession: true}) // every session creation fails
	os.Setenv("OPENCODE_URL", srv.URL)
	t.Cleanup(func() { os.Unsetenv("OPENCODE_URL") })

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

	if len(*msgBodies) != 0 {
		t.Errorf("got %d message requests, want 0 (sessions never created)", len(*msgBodies))
	}
}

func TestTranslate_NoMatchingRows(t *testing.T) {
	setCronKey(t, "correct-secret")
	srv, msgBodies := setupTranslateMock(t, translateMockConfig{})
	os.Setenv("OPENCODE_URL", srv.URL)
	t.Cleanup(func() { os.Unsetenv("OPENCODE_URL") })

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
	if len(*msgBodies) != 0 {
		t.Errorf("got %d message requests, want 0", len(*msgBodies))
	}
}

func TestTranslate_SystemPromptBoundaries(t *testing.T) {
	phrases := []string{
		"primary and authoritative source of truth",
		"Indonesia",
		"reference",
		"bracket",
		"Output only the English translation",
		"Arabic",
		"isnad",
		"sanad",
		"book/collection name",
		"hadith number",
		"never summarize",
		"entire",
		"matan",
		"transliterate",
		"external",
		"ambiguity",
		"contextual accuracy",
	}
	for _, phrase := range phrases {
		if !strings.Contains(strings.ToLower(loadPromptForTest(t)), strings.ToLower(phrase)) {
			t.Errorf("system prompt missing boundary phrase %q", phrase)
		}
	}
}

func TestTranslate_MissingPromptFile(t *testing.T) {
	setCronKey(t, "correct-secret")
	srv, _ := setupTranslateMock(t, translateMockConfig{})
	os.Setenv("OPENCODE_URL", srv.URL)
	t.Cleanup(func() { os.Unsetenv("OPENCODE_URL") })

	os.Setenv("TRANSLATION_SYSTEM_PROMPT_FILE", filepath.Join(t.TempDir(), "does-not-exist.txt"))
	t.Cleanup(func() { os.Unsetenv("TRANSLATION_SYSTEM_PROMPT_FILE") })

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "terjemah satu", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	var body map[string]interface{}
	decodeJSON(t, resp, &body)
	if body["status"] != "error" {
		t.Errorf("status = %v, want 'error'", body["status"])
	}
	if body["message"] != "failed to load translation system prompt" {
		t.Errorf("message = %v, want 'failed to load translation system prompt'", body["message"])
	}
}

func TestTranslate_EmptyPromptFile(t *testing.T) {
	setCronKey(t, "correct-secret")
	promptPath := filepath.Join(t.TempDir(), "empty.txt")
	if err := os.WriteFile(promptPath, []byte("   \n\t \n"), 0o644); err != nil {
		t.Fatalf("failed to write empty prompt file: %v", err)
	}

	srv, _ := setupTranslateMock(t, translateMockConfig{})
	os.Setenv("OPENCODE_URL", srv.URL)
	t.Cleanup(func() { os.Unsetenv("OPENCODE_URL") })

	os.Setenv("TRANSLATION_SYSTEM_PROMPT_FILE", promptPath)
	t.Cleanup(func() { os.Unsetenv("TRANSLATION_SYSTEM_PROMPT_FILE") })

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "terjemah satu", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	var body map[string]interface{}
	decodeJSON(t, resp, &body)
	if body["message"] != "failed to load translation system prompt" {
		t.Errorf("message = %v, want 'failed to load translation system prompt'", body["message"])
	}
}

func TestTranslate_PromptReloadedEachRun(t *testing.T) {
	setCronKey(t, "correct-secret")

	promptPath := filepath.Join(t.TempDir(), "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("PROMPT-VERSION-ONE"), 0o644); err != nil {
		t.Fatalf("failed to write prompt file: %v", err)
	}

	srv, msgBodies := setupTranslateMock(t, translateMockConfig{})
	os.Setenv("OPENCODE_URL", srv.URL)
	t.Cleanup(func() { os.Unsetenv("OPENCODE_URL") })

	os.Setenv("TRANSLATION_SYSTEM_PROMPT_FILE", promptPath)
	t.Cleanup(func() { os.Unsetenv("TRANSLATION_SYSTEM_PROMPT_FILE") })

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "terjemah satu", "English": nil},
		{"Nomer": 2, "Arabic": "hadith two", "Indonesia": "terjemah dua", "English": nil},
	})

	if resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret&limit=1", nil); resp.StatusCode != http.StatusOK {
		t.Errorf("first run status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if err := os.WriteFile(promptPath, []byte("PROMPT-VERSION-TWO"), 0o644); err != nil {
		t.Fatalf("failed to update prompt file: %v", err)
	}

	if resp := makeRequest(t, app, "POST", "/ai/cron/translate/ShahihBukhari?key=correct-secret&limit=1", nil); resp.StatusCode != http.StatusOK {
		t.Errorf("second run status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	bodies := *msgBodies
	if len(bodies) != 2 {
		t.Fatalf("got %d message requests, want 2", len(bodies))
	}
	for i, want := range []string{"PROMPT-VERSION-ONE", "PROMPT-VERSION-TWO"} {
		var msg map[string]interface{}
		if err := json.Unmarshal(bodies[i], &msg); err != nil {
			t.Fatalf("failed to decode captured body %d: %v", i, err)
		}
		if sys, _ := msg["system"].(string); sys != want {
			t.Errorf("message %d system = %q, want %q (prompt must reload per run)", i, sys, want)
		}
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
