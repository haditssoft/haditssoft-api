package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// setAIProviderEnv sets the AI provider env vars and restores them on cleanup.
func setAIProviderEnv(t *testing.T, provider, apiKey, baseURL, model string) {
	t.Helper()
	os.Setenv("AI_PROVIDER", provider)
	os.Setenv("OPENAI_API_KEY", apiKey)
	os.Setenv("OPENAI_BASE_URL", baseURL)
	os.Setenv("OPENAI_MODEL", model)
	t.Cleanup(func() {
		os.Unsetenv("AI_PROVIDER")
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("OPENAI_BASE_URL")
		os.Unsetenv("OPENAI_MODEL")
	})
}

// setChatCompletion installs a mock chatCompletionFunc and restores the
// original on cleanup.
func setChatCompletion(t *testing.T, fn func(ctx context.Context, agent, prompt string) (string, error)) {
	t.Helper()
	orig := chatCompletionFunc
	chatCompletionFunc = fn
	t.Cleanup(func() { chatCompletionFunc = orig })
}

func TestUseOpenAIProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		apiKey   string
		baseURL  string
		want     bool
	}{
		{"unset provider", "", "key", "https://api.example.com", false},
		{"opencode value", "opencode", "key", "https://api.example.com", false},
		{"openai with key and url", "openai", "key", "https://api.example.com", true},
		{"openai missing key", "openai", "", "https://api.example.com", false},
		{"openai missing url", "openai", "key", "", false},
		{"openai whitespace key", "openai", "   ", "https://api.example.com", false},
		{"openai whitespace url", "openai", "key", "   ", false},
		{"case insensitive", "OPENAI", "key", "https://api.example.com", true},
		{"trimmed provider", " openai ", "key", "https://api.example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setAIProviderEnv(t, tt.provider, tt.apiKey, tt.baseURL, "model")
			got := useOpenAIProvider()
			if got != tt.want {
				t.Errorf("useOpenAIProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}

// capturedCompletion holds the request details received by the httptest server.
type capturedCompletion struct {
	method        string
	path          string
	contentType   string
	authorization string
	model         string
	system        string
	user          string
}

// newCompletionServer returns an httptest server that records the incoming
// chat completion request and replies with the given status and body.
func newCompletionServer(t *testing.T, got *capturedCompletion, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method = r.Method
		got.path = r.URL.Path
		got.contentType = r.Header.Get("Content-Type")
		got.authorization = r.Header.Get("Authorization")

		var req openAIHandlerRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		got.model = req.Model
		for _, msg := range req.Messages {
			switch msg.Role {
			case "system":
				got.system = msg.Content
			case "user":
				got.user = msg.Content
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRunOpenAICompletion_Success(t *testing.T) {
	// Force the generic system prompt regardless of test working directory.
	origDir := opencodeAgentsDir
	opencodeAgentsDir = ".nonexistent-agents-dir"
	t.Cleanup(func() { opencodeAgentsDir = origDir })

	var got capturedCompletion
	srv := newCompletionServer(t, &got, http.StatusOK,
		`{"choices":[{"message":{"role":"assistant","content":"  hello world  "}}]}`)
	setAIProviderEnv(t, "openai", "test-key", srv.URL, "claude-sonnet-4-5")

	reply, err := openAICompletion(context.Background(), "translate", "the prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "hello world" {
		t.Errorf("reply = %q, want %q", reply, "hello world")
	}

	if got.method != http.MethodPost {
		t.Errorf("method = %q, want POST", got.method)
	}
	if got.path != "/chat/completions" {
		t.Errorf("path = %q, want /chat/completions", got.path)
	}
	if !strings.Contains(got.contentType, "application/json") {
		t.Errorf("content-type = %q, want application/json", got.contentType)
	}
	if got.authorization != "Bearer test-key" {
		t.Errorf("authorization = %q, want %q", got.authorization, "Bearer test-key")
	}
	if got.model != "claude-sonnet-4-5" {
		t.Errorf("model = %q, want %q", got.model, "claude-sonnet-4-5")
	}
	if got.system != genericSystemPrompt {
		t.Errorf("system = %q, want generic fallback prompt", got.system)
	}
	if got.user != "the prompt" {
		t.Errorf("user = %q, want %q", got.user, "the prompt")
	}
}

func TestRunOpenAICompletion_NonOKStatus(t *testing.T) {
	var got capturedCompletion
	srv := newCompletionServer(t, &got, http.StatusTooManyRequests, `{"error":"rate limit exceeded"}`)
	setAIProviderEnv(t, "openai", "key", srv.URL, "model")

	_, err := openAICompletion(context.Background(), "translate", "prompt")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "AI server error: HTTP 429") {
		t.Errorf("error = %q, want HTTP 429 status", err.Error())
	}
	if !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Errorf("error = %q, want response body included", err.Error())
	}
}

func TestRunOpenAICompletion_EmptyErrorBody(t *testing.T) {
	var got capturedCompletion
	srv := newCompletionServer(t, &got, http.StatusBadRequest, "")
	setAIProviderEnv(t, "openai", "key", srv.URL, "model")

	_, err := openAICompletion(context.Background(), "translate", "prompt")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "AI server error: HTTP 400") {
		t.Errorf("error = %q, want HTTP 400 status", err.Error())
	}
	if !strings.Contains(err.Error(), "400 Bad Request") {
		t.Errorf("error = %q, want status text fallback", err.Error())
	}
}

func TestRunOpenAICompletion_InvalidJSON(t *testing.T) {
	var got capturedCompletion
	srv := newCompletionServer(t, &got, http.StatusOK, `not json`)
	setAIProviderEnv(t, "openai", "key", srv.URL, "model")

	_, err := openAICompletion(context.Background(), "translate", "prompt")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "AI server error: invalid response JSON") {
		t.Errorf("error = %q, want invalid JSON error", err.Error())
	}
}

func TestRunOpenAICompletion_MissingChoices(t *testing.T) {
	var got capturedCompletion
	srv := newCompletionServer(t, &got, http.StatusOK, `{"choices":[]}`)
	setAIProviderEnv(t, "openai", "key", srv.URL, "model")

	_, err := openAICompletion(context.Background(), "translate", "prompt")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no choices in response") {
		t.Errorf("error = %q, want missing choices error", err.Error())
	}
}

func TestRunOpenAICompletion_MissingModel(t *testing.T) {
	setAIProviderEnv(t, "openai", "key", "https://api.example.com", "")

	_, err := openAICompletion(context.Background(), "translate", "prompt")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "OPENAI_MODEL is not set") {
		t.Errorf("error = %q, want missing model error", err.Error())
	}
}

func TestRunOpenAICompletion_ContextCancellation(t *testing.T) {
	var got capturedCompletion
	srv := newCompletionServer(t, &got, http.StatusOK,
		`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
	setAIProviderEnv(t, "openai", "key", srv.URL, "model")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := openAICompletion(ctx, "translate", "prompt")
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestRunAIProvider_DispatchesOpenAI(t *testing.T) {
	setAIProviderEnv(t, "openai", "key", "https://api.example.com", "model")

	var chatCalled bool
	var gotAgent, gotPrompt string
	setChatCompletion(t, func(ctx context.Context, agent, prompt string) (string, error) {
		chatCalled = true
		gotAgent = agent
		gotPrompt = prompt
		return "ai reply", nil
	})
	captured := setExecCapture(t, "", nil)

	reply, err := runAIProvider(context.Background(), "prompt text", "translate", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "ai reply" {
		t.Errorf("reply = %q, want %q", reply, "ai reply")
	}
	if !chatCalled {
		t.Error("chatCompletionFunc was not called")
	}
	if gotAgent != "translate" {
		t.Errorf("agent = %q, want %q", gotAgent, "translate")
	}
	if gotPrompt != "prompt text" {
		t.Errorf("prompt = %q, want %q", gotPrompt, "prompt text")
	}
	if len(*captured) != 0 {
		t.Errorf("execCommandFunc was called %d time(s), want 0", len(*captured))
	}
}

func TestRunAIProvider_DispatchesOpenCode(t *testing.T) {
	os.Unsetenv("AI_PROVIDER")
	t.Cleanup(func() { os.Unsetenv("AI_PROVIDER") })

	chatCalled := false
	setChatCompletion(t, func(ctx context.Context, agent, prompt string) (string, error) {
		chatCalled = true
		return "", nil
	})
	captured := setExecCapture(t, `{"type":"text","part":{"text":"opencode reply"}}`, nil)

	reply, err := runAIProvider(context.Background(), "prompt text", "translate", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "opencode reply" {
		t.Errorf("reply = %q, want %q", reply, "opencode reply")
	}
	if chatCalled {
		t.Error("chatCompletionFunc should not be called on the opencode path")
	}
	if len(*captured) != 1 {
		t.Errorf("execCommandFunc called %d time(s), want 1", len(*captured))
	}
}

func TestRunAIProvider_OpenAIMissingKeyFallsBackToOpenCode(t *testing.T) {
	setAIProviderEnv(t, "openai", "", "", "model")

	chatCalled := false
	setChatCompletion(t, func(ctx context.Context, agent, prompt string) (string, error) {
		chatCalled = true
		return "", nil
	})
	captured := setExecCapture(t, `{"type":"text","part":{"text":"opencode reply"}}`, nil)

	reply, err := runAIProvider(context.Background(), "prompt text", "translate", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "opencode reply" {
		t.Errorf("reply = %q, want %q", reply, "opencode reply")
	}
	if chatCalled {
		t.Error("chatCompletionFunc should not be called when the API key is missing")
	}
	if len(*captured) != 1 {
		t.Errorf("execCommandFunc called %d time(s), want 1 (fallback to opencode)", len(*captured))
	}
}

func TestLoadAgentSystemPrompt(t *testing.T) {
	origDir := opencodeAgentsDir
	opencodeAgentsDir = "../../.opencode/agents"
	t.Cleanup(func() { opencodeAgentsDir = origDir })

	t.Run("existing translate agent", func(t *testing.T) {
		got := loadAgentSystemPrompt("translate")
		if got == genericSystemPrompt {
			t.Error("expected agent instructions, got generic fallback")
		}
		if !strings.Contains(got, "expert Islamic scholar") {
			t.Errorf("system prompt missing agent instructions, got %q", got)
		}
	})

	t.Run("existing summarize agent", func(t *testing.T) {
		got := loadAgentSystemPrompt("summarize")
		if got == genericSystemPrompt {
			t.Error("expected agent instructions, got generic fallback")
		}
		if !strings.Contains(got, "Ringkasan Hadits") {
			t.Errorf("system prompt missing summarize instructions, got %q", got)
		}
	})

	t.Run("unknown agent falls back", func(t *testing.T) {
		got := loadAgentSystemPrompt("architect")
		if got != genericSystemPrompt {
			t.Errorf("system prompt = %q, want generic fallback", got)
		}
	})

	t.Run("empty agent falls back", func(t *testing.T) {
		if got := loadAgentSystemPrompt(""); got != genericSystemPrompt {
			t.Errorf("system prompt = %q, want generic fallback", got)
		}
	})
}

func TestRoute_SummarizeOpenAIProviderNeverUsesExecCLI(t *testing.T) {
	setCronKey(t, "correct-secret")
	setAIProviderEnv(t, "openai", "key", "https://api.example.com", "model")

	chatCalled := false
	setChatCompletion(t, func(ctx context.Context, agent, prompt string) (string, error) {
		chatCalled = true
		return "ai route summary", nil
	})
	captured := setExecCapture(t, "", nil)

	app := setupTestApp(t)
	seedHadith(t, 1, "arabic one", "indonesia one", "", "", "")

	resp := makeRequest(t, app, "POST", "/ai/cron/summarize/ShahihBukhari?key=correct-secret&language=Indonesia", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if !chatCalled {
		t.Error("chatCompletionFunc was not called for summarize")
	}
	if len(*captured) != 0 {
		t.Errorf("execCommandFunc called %d time(s), want 0", len(*captured))
	}
	if got := getSummaryColumn(t, 1, "Indonesia"); got == nil || *got != "ai route summary" {
		t.Errorf("summary = %v, want 'ai route summary'", got)
	}
}

func TestRunOpenAICompletion_SuccessWithAgentInstructions(t *testing.T) {
	origDir := opencodeAgentsDir
	opencodeAgentsDir = "../../.opencode/agents"
	t.Cleanup(func() { opencodeAgentsDir = origDir })

	var got capturedCompletion
	srv := newCompletionServer(t, &got, http.StatusOK,
		`{"choices":[{"message":{"role":"assistant","content":"translated"}}]}`)
	setAIProviderEnv(t, "openai", "key", srv.URL, "model")

	reply, err := openAICompletion(context.Background(), "translate", "Arabic:\n...")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "translated" {
		t.Errorf("reply = %q, want %q", reply, "translated")
	}
	if !strings.Contains(got.system, "expert Islamic scholar") {
		t.Errorf("system = %q, want translate agent instructions", got.system)
	}
}
