package opencode

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
)

type summarizeResponse struct {
	Processed int               `json:"processed"`
	Updated   int               `json:"updated"`
	Failed    []translateResult `json:"failed"`
}

// setSummarizeExecMock installs a mock execCommandFunc that returns the given
// text replies (as NDJSON text events), one per call, wrapping around the last
// reply once exhausted. When execErr is non-nil every call fails.
func setSummarizeExecMock(t *testing.T, replies []string, execErr error) *[][]string {
	t.Helper()

	callCount := 0
	captured := &[][]string{}
	orig := execCommandFunc
	execCommandFunc = func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		*captured = append(*captured, append([]string{name}, args...))

		if execErr != nil {
			return nil, []byte("boom"), execErr
		}

		reply := "default summary"
		if len(replies) > 0 {
			if callCount < len(replies) {
				reply = replies[callCount]
			} else {
				reply = replies[len(replies)-1]
			}
		}
		callCount++
		return bulkTextEvent(reply), nil, nil
	}
	t.Cleanup(func() { execCommandFunc = orig })
	return captured
}

// getSummaryColumn reads one language summary column from the shared
// ShahihBukhariSummary test table.
func getSummaryColumn(t *testing.T, nomer uint, column string) *string {
	t.Helper()
	var row struct {
		Value *string
	}
	if err := database.DB.Table("ShahihBukhariSummary").
		Select(column+" AS Value").
		Where("Nomer = ?", nomer).
		Limit(1).
		Find(&row).Error; err != nil {
		t.Fatalf("failed to read summary column: %v", err)
	}
	return row.Value
}

// === buildSummaryPrompt ===

func TestBuildSummaryPrompt(t *testing.T) {
	tests := []struct {
		name        string
		kitab       string
		nomer       uint
		language    string
		arabic      string
		translation string
		wantParts   []string
	}{
		{
			name:        "contains all fields",
			kitab:       "ShahihBukhari",
			nomer:       42,
			language:    "Indonesia",
			arabic:      "النص العربي",
			translation: "terjemahan indonesia",
			wantParts:   []string{"Kitab: ShahihBukhari", "Nomer: 42", "النص العربي", "Indonesia:", "terjemahan indonesia"},
		},
		{
			name:        "english language",
			kitab:       "ShahihMuslim",
			nomer:       7,
			language:    "English",
			arabic:      "arabic",
			translation: "english translation",
			wantParts:   []string{"Kitab: ShahihMuslim", "Nomer: 7", "English:", "english translation"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSummaryPrompt(tt.kitab, tt.nomer, tt.language, tt.arabic, tt.translation)
			for _, part := range tt.wantParts {
				if !strings.Contains(got, part) {
					t.Errorf("prompt = %q, missing %q", got, part)
				}
			}
		})
	}
}

// === summarizeTimeoutDuration ===

func TestSummarizeTimeoutDuration(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want time.Duration
	}{
		{name: "default when unset", env: "", want: 90 * time.Second},
		{name: "custom value", env: "30", want: 30 * time.Second},
		{name: "zero falls back", env: "0", want: 90 * time.Second},
		{name: "negative falls back", env: "-5", want: 90 * time.Second},
		{name: "invalid falls back", env: "abc", want: 90 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("OPENCODE_ASK_TIMEOUT_SEC")
			if tt.env != "" {
				os.Setenv("OPENCODE_ASK_TIMEOUT_SEC", tt.env)
			}
			t.Cleanup(func() { os.Unsetenv("OPENCODE_ASK_TIMEOUT_SEC") })

			if got := summarizeTimeoutDuration(); got != tt.want {
				t.Errorf("summarizeTimeoutDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

// === route guards ===

func TestRoute_SummarizeMissingKey(t *testing.T) {
	os.Unsetenv("OPENCODE_CRON_KEY")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_CRON_KEY") })

	app := setupTestApp(t)

	resp := makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?language=Indonesia", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestRoute_SummarizeWrongKey(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	resp := makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=wrong&language=Indonesia", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestRoute_SummarizeUnknownKitab(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	resp := makeRequest(t, app, "POST", "/ai/cron/summarize/NotARealKitab?key=correct-secret&language=Indonesia", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestRoute_SummarizeMissingLanguage(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	resp := makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var body map[string]interface{}
	decodeJSON(t, resp, &body)
	if body["message"] != "language is required" {
		t.Errorf("message = %v, want 'language is required'", body["message"])
	}
}

func TestRoute_SummarizeInvalidLanguage(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	resp := makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=Klingon", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestRoute_SummarizeWrongMethod(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	resp := makeRequest(t, app, "GET", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=Indonesia", nil)
	if resp.StatusCode == http.StatusOK {
		t.Error("GET should not return 200")
	}
}

func TestRoute_SummarizeInvalidQueryParams(t *testing.T) {
	setCronKey(t, "correct-secret")

	app := setupTestApp(t)

	cases := []string{"limit=abc", "limit=0", "limit=-1", "all=notbool", "maxBatches=0", "maxBatches=abc"}
	for _, q := range cases {
		resp := makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=Indonesia&"+q, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("query %q status = %d, want %d", q, resp.StatusCode, http.StatusBadRequest)
		}
	}
}

// === generation ===

func TestRoute_SummarizeSuccess(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setSummarizeExecMock(t, []string{"# summary one", "# summary two", "# summary three"}, nil)

	app := setupTestApp(t)
	seedHadith(t, 1, "arabic one", "indonesia one", "", "", "")
	seedHadith(t, 2, "arabic two", "indonesia two", "", "", "")
	seedHadith(t, 3, "arabic three", "indonesia three", "", "", "")

	resp := makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=Indonesia", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body summarizeResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 3 || body.Updated != 3 {
		t.Errorf("processed/updated = %d/%d, want 3/3", body.Processed, body.Updated)
	}
	if len(body.Failed) != 0 {
		t.Errorf("failed = %v, want none", body.Failed)
	}

	if got := getSummaryColumn(t, 1, "Indonesia"); got == nil || *got != "# summary one" {
		t.Errorf("summary[1] = %v, want '# summary one'", got)
	}
	if got := getSummaryColumn(t, 3, "Indonesia"); got == nil || *got != "# summary three" {
		t.Errorf("summary[3] = %v, want '# summary three'", got)
	}

	if len(*captured) != 3 {
		t.Fatalf("exec calls = %d, want 3", len(*captured))
	}
	args := (*captured)[0]
	if !containsArg(args, "--agent") || !containsArg(args, "summarize") {
		t.Errorf("missing --agent summarize flag, args: %v", args)
	}
	prompt := args[len(args)-1]
	for _, part := range []string{"arabic one", "indonesia one", "ShahihBukhari"} {
		if !strings.Contains(prompt, part) {
			t.Errorf("prompt = %q, missing %q", prompt, part)
		}
	}
}

func TestRoute_SummarizeSkipsExistingSummary(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setSummarizeExecMock(t, []string{"new summary"}, nil)

	app := setupTestApp(t)
	seedHadith(t, 1, "arabic one", "indonesia one", "", "", "")
	seedHadith(t, 2, "arabic two", "indonesia two", "", "", "")
	seedSummary(t, 1, "Indonesia", "already summarized")

	resp := makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=Indonesia", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body summarizeResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 1 || body.Updated != 1 {
		t.Errorf("processed/updated = %d/%d, want 1/1 (skip already summarized)", body.Processed, body.Updated)
	}
	if len(*captured) != 1 {
		t.Errorf("exec calls = %d, want 1", len(*captured))
	}
	if got := getSummaryColumn(t, 1, "Indonesia"); got == nil || *got != "already summarized" {
		t.Errorf("summary[1] = %v, want untouched 'already summarized'", got)
	}
	if got := getSummaryColumn(t, 2, "Indonesia"); got == nil || *got != "new summary" {
		t.Errorf("summary[2] = %v, want 'new summary'", got)
	}
}

func TestRoute_SummarizeEmptySourceSkipped(t *testing.T) {
	setCronKey(t, "correct-secret")
	captured := setSummarizeExecMock(t, []string{"summary"}, nil)

	app := setupTestApp(t)
	seedHadith(t, 1, "arabic one", "", "", "", "")
	seedHadith(t, 2, "arabic two", "indonesia two", "", "", "")

	resp := makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=Indonesia", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body summarizeResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 1 || body.Updated != 1 {
		t.Errorf("processed/updated = %d/%d, want 1/1 (empty source skipped)", body.Processed, body.Updated)
	}
	if len(*captured) != 1 {
		t.Errorf("exec calls = %d, want 1", len(*captured))
	}
}

func TestRoute_SummarizeDifferentLanguageColumns(t *testing.T) {
	setCronKey(t, "correct-secret")
	setSummarizeExecMock(t, []string{"english summary"}, nil)

	app := setupTestApp(t)
	seedHadith(t, 1, "arabic one", "", "english source", "", "")

	resp := makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=English", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if got := getSummaryColumn(t, 1, "English"); got == nil || *got != "english summary" {
		t.Errorf("summary English = %v, want 'english summary'", got)
	}
	if got := getSummaryColumn(t, 1, "Indonesia"); got != nil {
		t.Errorf("Indonesia column = %v, want nil (untouched)", *got)
	}
}

func TestRoute_SummarizeExecError(t *testing.T) {
	setCronKey(t, "correct-secret")
	setSummarizeExecMock(t, nil, fmt.Errorf("opencode not found"))

	app := setupTestApp(t)
	seedHadith(t, 1, "arabic one", "indonesia one", "", "", "")

	resp := makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=Indonesia", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body summarizeResponse
	decodeJSON(t, resp, &body)
	if body.Updated != 0 {
		t.Errorf("updated = %d, want 0", body.Updated)
	}
	if len(body.Failed) != 1 || body.Failed[0].Nomer != 1 {
		t.Errorf("failed = %v, want one entry for nomer 1", body.Failed)
	}
	if got := getSummaryColumn(t, 1, "Indonesia"); got != nil {
		t.Errorf("summary should not be written on exec error, got %v", *got)
	}
}

func TestRoute_SummarizeEmptyAIResponse(t *testing.T) {
	setCronKey(t, "correct-secret")
	setAIProviderEnv(t, "openai", "key", "https://api.example.com", "model")
	setChatCompletion(t, func(ctx context.Context, agent, prompt string) (string, error) {
		return "   ", nil
	})

	app := setupTestApp(t)
	seedHadith(t, 1, "arabic one", "indonesia one", "", "", "")

	resp := makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=Indonesia", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body summarizeResponse
	decodeJSON(t, resp, &body)
	if body.Updated != 0 {
		t.Errorf("updated = %d, want 0", body.Updated)
	}
	if len(body.Failed) != 1 || body.Failed[0].Error != "empty AI response" {
		t.Errorf("failed = %v, want 'empty AI response'", body.Failed)
	}
	if got := getSummaryColumn(t, 1, "Indonesia"); got != nil {
		t.Errorf("summary should not be written on empty reply, got %v", *got)
	}
}

func TestRoute_SummarizeOpenAIProvider(t *testing.T) {
	setCronKey(t, "correct-secret")
	setAIProviderEnv(t, "openai", "key", "https://api.example.com", "model")

	agentUsed := ""
	setChatCompletion(t, func(ctx context.Context, agent, prompt string) (string, error) {
		agentUsed = agent
		return "ai summary", nil
	})
	captured := setExecCapture(t, "", nil)

	app := setupTestApp(t)
	seedHadith(t, 1, "arabic one", "indonesia one", "", "", "")

	resp := makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=Indonesia", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if agentUsed != "summarize" {
		t.Errorf("agent = %q, want 'summarize'", agentUsed)
	}
	if got := getSummaryColumn(t, 1, "Indonesia"); got == nil || *got != "ai summary" {
		t.Errorf("summary = %v, want 'ai summary'", got)
	}
	if len(*captured) != 0 {
		t.Errorf("execCommandFunc called %d time(s), want 0 on OpenAI path", len(*captured))
	}
}

// === sweep ===

func TestRoute_SummarizeSweepAll(t *testing.T) {
	setCronKey(t, "correct-secret")
	setSummarizeExecMock(t, []string{"s1", "s2", "s3"}, nil)

	app := setupTestApp(t)
	for i := uint(1); i <= 3; i++ {
		seedHadith(t, i, fmt.Sprintf("arabic %d", i), fmt.Sprintf("indonesia %d", i), "", "", "")
	}

	resp := makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=Indonesia&all=1&limit=1", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body summarizeResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 3 || body.Updated != 3 {
		t.Errorf("processed/updated = %d/%d, want 3/3", body.Processed, body.Updated)
	}
}

func TestRoute_SummarizeMaxBatches(t *testing.T) {
	setCronKey(t, "correct-secret")
	setSummarizeExecMock(t, []string{"s1", "s2"}, nil)

	app := setupTestApp(t)
	for i := uint(1); i <= 3; i++ {
		seedHadith(t, i, fmt.Sprintf("arabic %d", i), fmt.Sprintf("indonesia %d", i), "", "", "")
	}

	resp := makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=Indonesia&all=1&limit=1&maxBatches=2", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body summarizeResponse
	decodeJSON(t, resp, &body)
	if body.Processed != 2 || body.Updated != 2 {
		t.Errorf("processed/updated = %d/%d, want 2/2 (capped by maxBatches)", body.Processed, body.Updated)
	}
}

// === CLI args / agent selection / model ===

func TestRoute_SummarizeAgentAlwaysSummarize(t *testing.T) {
	setCronKey(t, "correct-secret")
	os.Setenv("OPENCODE_AGENT", "architect")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_AGENT") })

	captured := setSummarizeExecMock(t, []string{"s"}, nil)

	app := setupTestApp(t)
	seedHadith(t, 1, "arabic one", "indonesia one", "", "", "")

	makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=Indonesia", nil)

	if len(*captured) != 1 {
		t.Fatalf("exec calls = %d, want 1", len(*captured))
	}
	args := (*captured)[0]
	if !containsArg(args, "--agent") || !containsArg(args, "summarize") {
		t.Errorf("expected fixed --agent summarize regardless of OPENCODE_AGENT, args: %v", args)
	}
	if containsArg(args, "architect") {
		t.Errorf("OPENCODE_AGENT must not leak into summarize args: %v", args)
	}
}

func TestRoute_SummarizeModelFlags(t *testing.T) {
	setCronKey(t, "correct-secret")
	os.Setenv("OPENCODE_PROVIDER_ID", "opencode")
	os.Setenv("OPENCODE_MODEL_ID", "deepseek-v4-flash-free")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_PROVIDER_ID")
		os.Unsetenv("OPENCODE_MODEL_ID")
	})

	captured := setSummarizeExecMock(t, []string{"s"}, nil)

	app := setupTestApp(t)
	seedHadith(t, 1, "arabic one", "indonesia one", "", "", "")

	makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=Indonesia", nil)

	args := (*captured)[0]
	if !containsArg(args, "--model") {
		t.Error("missing --model flag")
	}
	modelIdx := indexOfArg(args, "--model")
	if modelIdx < 0 || args[modelIdx+1] != "opencode/deepseek-v4-flash-free" {
		t.Errorf("model = %v, want 'opencode/deepseek-v4-flash-free'", args[modelIdx+1])
	}
}

func TestRoute_SummarizeNoModelWhenUnset(t *testing.T) {
	setCronKey(t, "correct-secret")
	os.Unsetenv("OPENCODE_PROVIDER_ID")
	os.Unsetenv("OPENCODE_MODEL_ID")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_PROVIDER_ID")
		os.Unsetenv("OPENCODE_MODEL_ID")
	})

	captured := setSummarizeExecMock(t, []string{"s"}, nil)

	app := setupTestApp(t)
	seedHadith(t, 1, "arabic one", "indonesia one", "", "", "")

	makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=Indonesia", nil)

	if containsArg((*captured)[0], "--model") {
		t.Error("--model flag should not be present when env vars are unset")
	}
}

func TestRoute_SummarizePartialModelNotSent(t *testing.T) {
	setCronKey(t, "correct-secret")
	os.Setenv("OPENCODE_PROVIDER_ID", "opencode")
	os.Unsetenv("OPENCODE_MODEL_ID")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_PROVIDER_ID")
		os.Unsetenv("OPENCODE_MODEL_ID")
	})

	captured := setSummarizeExecMock(t, []string{"s"}, nil)

	app := setupTestApp(t)
	seedHadith(t, 1, "arabic one", "indonesia one", "", "", "")

	makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=Indonesia", nil)

	if containsArg((*captured)[0], "--model") {
		t.Error("--model flag should not be present when only provider is set")
	}
}

// === timeout ===

func TestRoute_SummarizeTimeoutKillsSlowCommand(t *testing.T) {
	setCronKey(t, "correct-secret")
	os.Setenv("OPENCODE_ASK_TIMEOUT_SEC", "1")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_ASK_TIMEOUT_SEC") })

	orig := execCommandFunc
	execCommandFunc = func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}
	t.Cleanup(func() { execCommandFunc = orig })

	app := setupTestApp(t)
	seedHadith(t, 1, "arabic one", "indonesia one", "", "", "")

	resp := makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=Indonesia", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d (timeout is collected per-record)", resp.StatusCode, http.StatusOK)
	}

	var body summarizeResponse
	decodeJSON(t, resp, &body)
	if body.Updated != 0 {
		t.Errorf("updated = %d, want 0", body.Updated)
	}
	if len(body.Failed) != 1 {
		t.Errorf("failed = %v, want one entry", body.Failed)
	}
}

func TestRoute_SummarizeDefaultTimeoutApplied(t *testing.T) {
	setCronKey(t, "correct-secret")
	os.Unsetenv("OPENCODE_ASK_TIMEOUT_SEC")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_ASK_TIMEOUT_SEC") })

	var capturedCtx context.Context
	orig := execCommandFunc
	execCommandFunc = func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		capturedCtx = ctx
		return bulkTextEvent("s"), nil, nil
	}
	t.Cleanup(func() { execCommandFunc = orig })

	app := setupTestApp(t)
	seedHadith(t, 1, "arabic one", "indonesia one", "", "", "")

	makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=Indonesia", nil)

	deadline, ok := capturedCtx.Deadline()
	if !ok {
		t.Fatal("expected context to have a deadline")
	}
	if remaining := time.Until(deadline); remaining < 80*time.Second || remaining > 95*time.Second {
		t.Errorf("deadline remaining = %v, want ~90s", remaining)
	}
}

func TestRoute_SummarizeCustomTimeoutFromEnv(t *testing.T) {
	setCronKey(t, "correct-secret")
	os.Setenv("OPENCODE_ASK_TIMEOUT_SEC", "30")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_ASK_TIMEOUT_SEC") })

	var capturedCtx context.Context
	orig := execCommandFunc
	execCommandFunc = func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		capturedCtx = ctx
		return bulkTextEvent("s"), nil, nil
	}
	t.Cleanup(func() { execCommandFunc = orig })

	app := setupTestApp(t)
	seedHadith(t, 1, "arabic one", "indonesia one", "", "", "")

	makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=Indonesia", nil)

	deadline, ok := capturedCtx.Deadline()
	if !ok {
		t.Fatal("expected context to have a deadline")
	}
	if remaining := time.Until(deadline); remaining < 25*time.Second || remaining > 35*time.Second {
		t.Errorf("deadline remaining = %v, want ~30s", remaining)
	}
}
