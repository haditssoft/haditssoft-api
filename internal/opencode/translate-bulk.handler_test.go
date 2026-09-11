package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
)

// bulkTextEvent builds a valid NDJSON text event whose text field is the given
// reply (properly escaped by json.Marshal).
func bulkTextEvent(reply string) []byte {
	var event struct {
		Type string `json:"type"`
		Part struct {
			Text string `json:"text"`
		} `json:"part"`
	}
	event.Type = "text"
	event.Part.Text = reply
	b, err := json.Marshal(event)
	if err != nil {
		panic(err)
	}
	return b
}

type bulkExecMockConfig struct {
	failAtCall     int
	failEveryCall  bool
	replyOverrides map[int]string
	stderrOnFail   string
}

func setBulkExecMock(t *testing.T, cfg bulkExecMockConfig) *[][]string {
	t.Helper()

	callCount := 0
	captured := &[][]string{}
	orig := execCommandFunc
	execCommandFunc = func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		callCount++
		*captured = append(*captured, append([]string{name}, args...))

		if cfg.failEveryCall || (cfg.failAtCall > 0 && callCount == cfg.failAtCall) {
			return nil, []byte(cfg.stderrOnFail), fmt.Errorf("opencode exec failed at call %d", callCount)
		}

		reply := "{}"
		if override, ok := cfg.replyOverrides[callCount]; ok {
			reply = override
		}
		return bulkTextEvent(reply), nil, nil
	}
	t.Cleanup(func() { execCommandFunc = orig })
	return captured
}

func strPtr(s string) *string {
	return &s
}

type bulkTranslateResponse struct {
	Processed int               `json:"processed"`
	Updated   int               `json:"updated"`
	Failed    []translateResult `json:"failed"`
}

func TestBulkTranslate_MissingKey(t *testing.T) {
	os.Unsetenv("OPENCODE_CRON_KEY")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_CRON_KEY") })

	app := setupTestApp(t)

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	var body map[string]interface{}
	decodeJSON(t, resp, &body)
	if body["status"] != "error" {
		t.Errorf("status = %v, want 'error'", body["status"])
	}
}

func TestBulkTranslate_WrongKey(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=wrong-secret", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestBulkTranslate_NoKeyConfigured(t *testing.T) {
	os.Unsetenv("OPENCODE_CRON_KEY")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_CRON_KEY") })

	app := setupTestApp(t)

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=anything", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (fail closed when key unset)", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestBulkTranslate_WrongMethod(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	resp := makeRequest(t, app, "GET", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode == http.StatusOK {
		t.Error("GET should not return 200")
	}
}

func TestBulkTranslate_UnknownKitab(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/NotARealKitab?key=correct-secret", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var body map[string]interface{}
	decodeJSON(t, resp, &body)
	if body["message"] != "unknown kitab: NotARealKitab" {
		t.Errorf("message = %v, want 'unknown kitab: NotARealKitab'", body["message"])
	}
}

func TestBulkTranslate_InvalidLimit(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	for _, raw := range []string{"abc", "0", "-5"} {
		resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret&limit="+raw, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("limit=%q status = %d, want %d", raw, resp.StatusCode, http.StatusBadRequest)
		}
	}
}

func TestBulkTranslate_SuccessUpdatesEnglish(t *testing.T) {
	setCronKey(t, "correct-secret")
	reply := `{"1":"one english","2":"two english","3":"three english"}`
	captured := setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: reply}})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one arabic", "Indonesia": "terjemah satu", "English": ""},
		{"Nomer": 2, "Arabic": "hadith two arabic", "Indonesia": "terjemah dua", "English": nil},
		{"Nomer": 3, "Arabic": "hadith three arabic", "Indonesia": "terjemah tiga", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body bulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 3 {
		t.Errorf("processed = %d, want 3", body.Processed)
	}
	if body.Updated != 3 {
		t.Errorf("updated = %d, want 3", body.Updated)
	}
	if len(body.Failed) != 0 {
		t.Errorf("failed = %v, want empty list", body.Failed)
	}

	assertEnglish(t, 1, "one english")
	assertEnglish(t, 2, "two english")
	assertEnglish(t, 3, "three english")

	if len(*captured) != 1 {
		t.Fatalf("got %d CLI calls, want 1", len(*captured))
	}
	args := (*captured)[0]
	if !containsArg(args, "--agent") || !containsArg(args, "translate-bulk") {
		t.Errorf("agent should be 'translate-bulk', got args: %v", args)
	}

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
	if e := payload["1"]; e.Arabic != "hadith one arabic" || e.Indonesia != "terjemah satu" {
		t.Errorf("payload[1] = %+v, want arabic 'hadith one arabic' and indonesia 'terjemah satu'", e)
	}
	if e := payload["3"]; e.Arabic != "hadith three arabic" || e.Indonesia != "terjemah tiga" {
		t.Errorf("payload[3] = %+v, want arabic 'hadith three arabic' and indonesia 'terjemah tiga'", e)
	}
}

func TestBulkTranslate_PromptIsKeyedByNomer(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: `{"1":"one","2":"two"}`}})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 5, "Arabic": "hadith five", "Indonesia": "lima", "English": nil},
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": nil},
		{"Nomer": 3, "Arabic": "hadith three", "Indonesia": "tiga", "English": nil},
		{"Nomer": 2, "Arabic": "hadith two", "Indonesia": "dua", "English": nil},
		{"Nomer": 4, "Arabic": "hadith four", "Indonesia": "empat", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret&limit=2", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body bulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 2 {
		t.Errorf("processed = %d, want 2", body.Processed)
	}
	if body.Updated != 2 {
		t.Errorf("updated = %d, want 2", body.Updated)
	}

	assertEnglish(t, 1, "one")
	assertEnglish(t, 2, "two")
	assertEnglishEmpty(t, 3)
	assertEnglishEmpty(t, 4)
	assertEnglishEmpty(t, 5)

	args := (*captured)[0]
	prompt := args[len(args)-1]
	for _, key := range []string{`"3"`, `"4"`, `"5"`} {
		if strings.Contains(prompt, key) {
			t.Errorf("prompt should not contain key %s: %q", key, prompt)
		}
	}
	for _, key := range []string{`"1":`, `"2":`} {
		if !strings.Contains(prompt, key) {
			t.Errorf("prompt should contain key %s: %q", key, prompt)
		}
	}
}

func TestBulkTranslate_MissingKeyCountedAsFailed(t *testing.T) {
	setCronKey(t, "correct-secret")
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: `{"1":"one english"}`}})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": nil},
		{"Nomer": 2, "Arabic": "hadith two", "Indonesia": "dua", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body bulkTranslateResponse
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
	if body.Failed[0].Nomer != 2 {
		t.Errorf("failed nomer = %d, want 2", body.Failed[0].Nomer)
	}
	if body.Failed[0].Error != "missing key in AI response" {
		t.Errorf("failed error = %q, want 'missing key in AI response'", body.Failed[0].Error)
	}

	assertEnglish(t, 1, "one english")
	assertEnglishEmpty(t, 2)
}

func TestBulkTranslate_EmptyValueCountedAsFailed(t *testing.T) {
	setCronKey(t, "correct-secret")
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: `{"1":"","2":"two english"}`}})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": nil},
		{"Nomer": 2, "Arabic": "hadith two", "Indonesia": "dua", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body bulkTranslateResponse
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

	assertEnglishEmpty(t, 1)
	assertEnglish(t, 2, "two english")
}

func TestBulkTranslate_WhitespaceValueCountedAsFailed(t *testing.T) {
	setCronKey(t, "correct-secret")
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: `{"1":"   ","2":"two english"}`}})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": nil},
		{"Nomer": 2, "Arabic": "hadith two", "Indonesia": "dua", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body bulkTranslateResponse
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

	assertEnglishEmpty(t, 1)
	assertEnglish(t, 2, "two english")
}

func TestBulkTranslate_NestedObjectValueRejected(t *testing.T) {
	setCronKey(t, "correct-secret")
	forbidden := `{"1":{"arabic":"hadith one arabic","indonesia":"terjemah satu","english":"should not be written"}}`
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: forbidden}})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one arabic", "Indonesia": "terjemah satu", "English": nil},
		{"Nomer": 2, "Arabic": "hadith two arabic", "Indonesia": "terjemah dua", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body bulkTranslateResponse
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

	assertEnglishEmpty(t, 1)
	assertEnglishEmpty(t, 2)
}

func TestBulkTranslate_InvalidJSONReplyFailsAll(t *testing.T) {
	setCronKey(t, "correct-secret")
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: "not json at all"}})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": nil},
		{"Nomer": 2, "Arabic": "hadith two", "Indonesia": "dua", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body bulkTranslateResponse
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

	assertEnglishEmpty(t, 1)
	assertEnglishEmpty(t, 2)
}

func TestBulkTranslate_NoTextReplyFailsAll(t *testing.T) {
	setCronKey(t, "correct-secret")
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: ""}})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": nil},
		{"Nomer": 2, "Arabic": "hadith two", "Indonesia": "dua", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body bulkTranslateResponse
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

	assertEnglishEmpty(t, 1)
	assertEnglishEmpty(t, 2)
}

func TestBulkTranslate_WhitespaceReplyFailsAll(t *testing.T) {
	setCronKey(t, "correct-secret")
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: "   "}})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": nil},
		{"Nomer": 2, "Arabic": "hadith two", "Indonesia": "dua", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body bulkTranslateResponse
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

	assertEnglishEmpty(t, 1)
	assertEnglishEmpty(t, 2)
}

func TestBulkTranslate_UnknownKeyIgnored(t *testing.T) {
	setCronKey(t, "correct-secret")
	// "999" and "abc" are not among the requested Numers and must be ignored.
	reply := `{"1":"one english","2":"two english","999":"bogus","abc":"nope"}`
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: reply}})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": nil},
		{"Nomer": 2, "Arabic": "hadith two", "Indonesia": "dua", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body bulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Updated != 2 {
		t.Errorf("updated = %d, want 2 (unknown keys must not count or be written)", body.Updated)
	}
	if len(body.Failed) != 0 {
		t.Errorf("failed = %v, want empty", body.Failed)
	}

	assertEnglish(t, 1, "one english")
	assertEnglish(t, 2, "two english")
}

func TestBulkTranslate_KeyOutsideBatchNotOverwritten(t *testing.T) {
	setCronKey(t, "correct-secret")
	// Row 3 is already translated and NOT part of the batch; the reply wrongly
	// includes its key. It must NOT be overwritten.
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: `{"1":"one english","3":"corrupted"}`}})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": nil},
		{"Nomer": 3, "Arabic": "hadith three", "Indonesia": "tiga", "English": "already translated"},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body bulkTranslateResponse
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

	assertEnglish(t, 1, "one english")
	if eng := getKitabEnglish(t, 3); eng == nil || *eng != "already translated" {
		t.Errorf("row 3 English = %v, want unchanged 'already translated'", eng)
	}
}

func TestBulkTranslate_FencedJSONReplyParsed(t *testing.T) {
	setCronKey(t, "correct-secret")
	fenced := "```json\n{\n  \"1\": \"one english\"\n}\n```"
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: fenced}})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body bulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Updated != 1 {
		t.Errorf("updated = %d, want 1", body.Updated)
	}
	if len(body.Failed) != 0 {
		t.Errorf("failed = %v, want empty", body.Failed)
	}

	assertEnglish(t, 1, "one english")
}

func TestBulkTranslate_PreambleAndTrailingTextParsed(t *testing.T) {
	setCronKey(t, "correct-secret")
	noisy := "Here are the translations:\n{\"1\": \"one english\"}\nDone. Have a nice day."
	setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: noisy}})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body bulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Updated != 1 {
		t.Errorf("updated = %d, want 1", body.Updated)
	}

	assertEnglish(t, 1, "one english")
}

func TestBulkTranslate_NoMatchingRows(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setBulkExecMock(t, bulkExecMockConfig{})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": "already translated"},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body bulkTranslateResponse
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

func TestBulkTranslate_CLIFailureAllRowsFail(t *testing.T) {
	setCronKey(t, "correct-secret")
	setBulkExecMock(t, bulkExecMockConfig{failEveryCall: true})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": nil},
		{"Nomer": 2, "Arabic": "hadith two", "Indonesia": "dua", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body bulkTranslateResponse
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

	assertEnglishEmpty(t, 1)
	assertEnglishEmpty(t, 2)
}

func TestBulkTranslate_ServerErrorRetryFailsAll(t *testing.T) {
	setCronKey(t, "correct-secret")
	setTranslateRetryDelay(t, 0)
	setTranslateMaxRetries(t, 2)
	callCount := setServerErrMock(t, "Rate limit exceeded.")

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body bulkTranslateResponse
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

	assertEnglishEmpty(t, 1)
}

func TestBulkTranslate_ServerErrorRetrySucceeds(t *testing.T) {
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
		return bulkTextEvent(`{"1":"one retried"}`), nil, nil
	}
	t.Cleanup(func() { execCommandFunc = orig })

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body bulkTranslateResponse
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

	assertEnglish(t, 1, "one retried")
}

func TestBulkTranslate_ModelFromEnv(t *testing.T) {
	setCronKey(t, "correct-secret")
	os.Setenv("OPENCODE_PROVIDER_ID", "opencode")
	os.Setenv("OPENCODE_MODEL_ID", "deepseek-v4-flash-free")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_PROVIDER_ID")
		os.Unsetenv("OPENCODE_MODEL_ID")
	})

	captured := setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: `{"1":"one"}`}})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
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
	if !containsArg(args, "--agent") || !containsArg(args, "translate-bulk") {
		t.Errorf("agent should be 'translate-bulk', got args: %v", args)
	}
}

func TestBulkTranslate_NoModelWhenUnset(t *testing.T) {
	setCronKey(t, "correct-secret")
	os.Unsetenv("OPENCODE_PROVIDER_ID")
	os.Unsetenv("OPENCODE_MODEL_ID")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_PROVIDER_ID")
		os.Unsetenv("OPENCODE_MODEL_ID")
	})

	captured := setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: `{"1":"one"}`}})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	args := (*captured)[0]
	if containsArg(args, "--model") {
		t.Error("--model flag should not be present when env vars are unset")
	}
}

func TestBulkTranslate_PureFlagPresent(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: `{"1":"one"}`}})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	args := (*captured)[0]
	if !containsArg(args, "--pure") {
		t.Error("--pure flag should be present in bulk translate CLI calls")
	}
}

func TestBulkTranslate_EnvAgentDoesNotOverride(t *testing.T) {
	setCronKey(t, "correct-secret")
	os.Setenv("OPENCODE_AGENT", "plan")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_AGENT") })

	captured := setBulkExecMock(t, bulkExecMockConfig{replyOverrides: map[int]string{1: `{"1":"one"}`}})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	args := (*captured)[0]
	if !containsArg(args, "--agent") || !containsArg(args, "translate-bulk") {
		t.Errorf("agent should always be 'translate-bulk', got args: %v", args)
	}
}

func TestBulkTranslate_RouteRegistered(t *testing.T) {
	app := setupTestApp(t)

	routes := app.Stack()
	found := false
	for _, methodRoutes := range routes {
		for _, route := range methodRoutes {
			if route.Method == "POST" && route.Path == "/ai/cron/translate/bulk/:kitabName" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected POST /ai/cron/translate/bulk/:kitabName route to be registered")
	}
}

func TestBuildBulkTranslatePrompt(t *testing.T) {
	tests := []struct {
		name string
		rows []translationRow
		want string
	}{
		{
			name: "keys sorted and keyed by nomer",
			rows: []translationRow{
				{Nomer: 2, Arabic: strPtr("arabic-two"), Indonesia: strPtr("indonesia-two")},
				{Nomer: 1, Arabic: strPtr("arabic-one"), Indonesia: strPtr("indonesia-one")},
			},
			want: `{"1":{"arabic":"arabic-one","indonesia":"indonesia-one"},"2":{"arabic":"arabic-two","indonesia":"indonesia-two"}}`,
		},
		{
			name: "nil pointers become empty strings",
			rows: []translationRow{
				{Nomer: 7},
			},
			want: `{"7":{"arabic":"","indonesia":""}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildBulkTranslatePrompt(tt.rows)
			if err != nil {
				t.Fatalf("buildBulkTranslatePrompt() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("buildBulkTranslatePrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseBulkTranslationReply(t *testing.T) {
	tests := []struct {
		name    string
		reply   string
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "direct json",
			reply: `{"1":"a","2":"b"}`,
			want:  map[string]string{"1": "a", "2": "b"},
		},
		{
			name:  "single entry",
			reply: `{"123":"translated text"}`,
			want:  map[string]string{"123": "translated text"},
		},
		{
			name:  "surrounding text extracted",
			reply: `Here is the result: {"1":"a"} All done.`,
			want:  map[string]string{"1": "a"},
		},
		{
			name:  "markdown fences",
			reply: "```json\n{\"1\":\"a\"}\n```",
			want:  map[string]string{"1": "a"},
		},
		{
			name:  "multiline value escaped",
			reply: `{"1":"line1\nline2"}`,
			want:  map[string]string{"1": "line1\nline2"},
		},
		{
			name:    "nested object value rejected",
			reply:   `{"1":{"arabic":"a","indonesia":"i","english":"e"}}`,
			wantErr: true,
		},
		{
			name:    "array value rejected",
			reply:   `{"1":["a","b"]}`,
			wantErr: true,
		},
		{
			name:    "number value rejected",
			reply:   `{"1":5}`,
			wantErr: true,
		},
		{
			name:    "invalid",
			reply:   `not json at all`,
			wantErr: true,
		},
		{
			name:    "empty",
			reply:   "",
			wantErr: true,
		},
		{
			name:    "whitespace",
			reply:   "   \n  ",
			wantErr: true,
		},
		{
			name:    "two objects",
			reply:   `{"1":"a"}{"2":"b"}`,
			wantErr: true,
		},
		{
			name:  "trailing text tolerated",
			reply: `{"1":"a"} trailing remark`,
			want:  map[string]string{"1": "a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBulkTranslationReply(tt.reply)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseBulkTranslationReply() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parseBulkTranslationReply() = %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("parseBulkTranslationReply()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

// TestBulkTranslate_DefaultLimitApplied verifies the default limit (10) is used
// when the limit query param is absent.
func TestBulkTranslate_DefaultLimitApplied(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setBulkExecMock(t, bulkExecMockConfig{
		replyOverrides: map[int]string{1: `{"1":"one","2":"two","3":"three","4":"four","5":"five"}`},
	})

	app := setupTestApp(t)
	seedShahihBukhari(t, []map[string]interface{}{
		{"Nomer": 5, "Arabic": "hadith five", "Indonesia": "lima", "English": nil},
		{"Nomer": 1, "Arabic": "hadith one", "Indonesia": "satu", "English": nil},
		{"Nomer": 3, "Arabic": "hadith three", "Indonesia": "tiga", "English": nil},
		{"Nomer": 2, "Arabic": "hadith two", "Indonesia": "dua", "English": nil},
		{"Nomer": 4, "Arabic": "hadith four", "Indonesia": "empat", "English": nil},
	})

	resp := makeRequest(t, app, "POST", "/ai/cron/translate/bulk/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body bulkTranslateResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 5 {
		t.Errorf("processed = %d, want 5 (all 5 rows fit within default limit of 10)", body.Processed)
	}
	if body.Updated != 5 {
		t.Errorf("updated = %d, want 5", body.Updated)
	}
	if len(body.Failed) != 0 {
		t.Errorf("failed = %v, want empty", body.Failed)
	}
	if len(*captured) != 1 {
		t.Errorf("got %d CLI calls, want 1", len(*captured))
	}
}
