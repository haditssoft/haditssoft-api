package opencode

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
	"github.com/haditssoft/haditssoft-backend/internal/shared/database/entities"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:test_opencode?mode=memory&cache=shared"), &gorm.Config{
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

	if err := db.AutoMigrate(&entities.User{}, &entities.BlacklistToken{}); err != nil {
		t.Fatalf("failed to auto-migrate: %v", err)
	}

	boolTrue := true
	user := entities.User{
		ID:     1,
		Email:  "test@example.com",
		Active: &boolTrue,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
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

	os.Setenv("JWT_SECRET", "test-secret-key-for-opencode")
	t.Cleanup(func() { os.Unsetenv("JWT_SECRET") })

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

func makeRequestWithToken(t *testing.T, app *fiber.App, method, path string, body interface{}, token string) *http.Response {
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
	req.Header.Set("Authorization", "Bearer "+token)

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

// setExecOutput installs a mock execCommandFunc that returns the given output
// for any opencode invocation. It restores the original function on cleanup.
func setExecOutput(t *testing.T, output string, execErr error) {
	t.Helper()
	orig := execCommandFunc
	execCommandFunc = func(name string, args ...string) ([]byte, error) {
		return []byte(output), execErr
	}
	t.Cleanup(func() { execCommandFunc = orig })
}

// setExecCapture installs a mock execCommandFunc that captures the arguments
// and returns the given output. It restores the original function on cleanup.
func setExecCapture(t *testing.T, output string, execErr error) *[][]string {
	t.Helper()
	captured := &[][]string{}
	orig := execCommandFunc
	execCommandFunc = func(name string, args ...string) ([]byte, error) {
		*captured = append(*captured, append([]string{name}, args...))
		return []byte(output), execErr
	}
	t.Cleanup(func() { execCommandFunc = orig })
	return captured
}

func TestRoute_AskNoAuth(t *testing.T) {
	app := setupTestApp(t)

	body := map[string]interface{}{
		"prompt": "What is hadith?",
	}
	resp := makeRequest(t, app, "POST", "/ai/ask", body)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d (no JWT)", resp.StatusCode, http.StatusBadRequest)
	}

	var errResp map[string]interface{}
	decodeJSON(t, resp, &errResp)
	if errResp["status"] != "error" {
		t.Errorf("status = %v, want 'error'", errResp["status"])
	}
}

func TestRoute_AskInvalidJSON(t *testing.T) {
	app := setupTestApp(t)

	jwt, err := auth.GenerateAccessToken(1, "test@example.com")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	req := httptest.NewRequest("POST", "/ai/ask", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

func TestRoute_AskMissingPrompt(t *testing.T) {
	app := setupTestApp(t)

	jwt, err := auth.GenerateAccessToken(1, "test@example.com")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	body := map[string]interface{}{}
	resp := makeRequestWithToken(t, app, "POST", "/ai/ask", body, jwt)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var errResp map[string]interface{}
	decodeJSON(t, resp, &errResp)
	if errResp["errors"] == nil {
		t.Error("expected validation errors")
	}
}

func TestRoute_AskCLIExecutionFailure(t *testing.T) {
	setExecOutput(t, "", fmt.Errorf("opencode not found"))

	app := setupTestApp(t)

	jwt, err := auth.GenerateAccessToken(1, "test@example.com")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	body := map[string]interface{}{
		"prompt": "What is hadith?",
	}
	resp := makeRequestWithToken(t, app, "POST", "/ai/ask", body, jwt)

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadGateway)
	}

	var errResp map[string]interface{}
	decodeJSON(t, resp, &errResp)
	if errResp["error"] != "failed to get AI response" {
		t.Errorf("error = %v, want 'failed to get AI response'", errResp["error"])
	}
}

func TestRoute_AskWrongMethod(t *testing.T) {
	app := setupTestApp(t)

	resp := makeRequest(t, app, "GET", "/ai/ask", nil)
	if resp.StatusCode == http.StatusOK {
		t.Error("GET should not return 200")
	}
}

func TestRoute_AskSuccess(t *testing.T) {
	setExecOutput(t, `{"type":"text","part":{"text":"hadith is a tradition"}}`, nil)

	app := setupTestApp(t)

	jwt, err := auth.GenerateAccessToken(1, "test@example.com")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	body := map[string]interface{}{
		"prompt": "What is hadith?",
	}
	resp := makeRequestWithToken(t, app, "POST", "/ai/ask", body, jwt)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)
	if result["reply"] != "hadith is a tradition" {
		t.Errorf("reply = %v, want 'hadith is a tradition'", result["reply"])
	}
}

func TestRoute_AskDeletedUser(t *testing.T) {
	app := setupTestApp(t)

	boolTrue := true
	deletedUser := entities.User{
		ID:     99,
		Email:  "deleted@example.com",
		Active: &boolTrue,
	}
	if err := database.DB.Create(&deletedUser).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	database.DB.Unscoped().Delete(&entities.User{}, 99)

	jwt, err := auth.GenerateAccessToken(99, "deleted@example.com")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	body := map[string]interface{}{
		"prompt": "What is hadith?",
	}
	resp := makeRequestWithToken(t, app, "POST", "/ai/ask", body, jwt)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (deleted user should be rejected)", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestRoutes_AIPathExists(t *testing.T) {
	app := setupTestApp(t)

	routes := app.Stack()
	postCount := 0
	for _, methodRoutes := range routes {
		for _, route := range methodRoutes {
			if route.Method == "POST" && route.Path == "/ai/ask" {
				postCount++
			}
		}
	}

	if postCount != 1 {
		t.Errorf("expected 1 POST /ai/ask route, got %d", postCount)
	}
}

func TestOpenCodeCLIArgsDefaultAgent(t *testing.T) {
	captured := setExecCapture(t, `{"type":"text","part":{"text":"ok"}}`, nil)

	app := setupTestApp(t)
	os.Unsetenv("OPENCODE_AGENT")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_AGENT") })

	jwt, _ := auth.GenerateAccessToken(1, "test@example.com")

	body := map[string]interface{}{"prompt": "hello"}
	resp := makeRequestWithToken(t, app, "POST", "/ai/ask", body, jwt)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if len(*captured) != 1 {
		t.Fatalf("got %d calls, want 1", len(*captured))
	}
	args := (*captured)[0]
	if args[0] != "opencode" {
		t.Errorf("binary = %v, want 'opencode'", args[0])
	}
	if !containsArg(args, "--format") || !containsArg(args, "json") {
		t.Error("missing --format json flag")
	}
	if !containsArg(args, "--agent") || !containsArg(args, "plan") {
		t.Error("missing --agent plan flag (default)")
	}
	if args[len(args)-1] != "hello" {
		t.Errorf("prompt = %v, want 'hello'", args[len(args)-1])
	}
}

func TestOpenCodeCLIArgsFromEnvAgent(t *testing.T) {
	captured := setExecCapture(t, `{"type":"text","part":{"text":"ok"}}`, nil)

	app := setupTestApp(t)
	os.Setenv("OPENCODE_AGENT", "architect")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_AGENT") })

	jwt, _ := auth.GenerateAccessToken(1, "test@example.com")

	body := map[string]interface{}{"prompt": "hello"}
	resp := makeRequestWithToken(t, app, "POST", "/ai/ask", body, jwt)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	args := (*captured)[0]
	if !containsArg(args, "--agent") || !containsArg(args, "architect") {
		t.Errorf("agent flag not set to 'architect', got args: %v", args)
	}
}

func TestOpenCodeCLIArgsWithModel(t *testing.T) {
	captured := setExecCapture(t, `{"type":"text","part":{"text":"ok"}}`, nil)

	app := setupTestApp(t)
	os.Setenv("OPENCODE_PROVIDER_ID", "opencode")
	os.Setenv("OPENCODE_MODEL_ID", "deepseek-v4-flash-free")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_PROVIDER_ID")
		os.Unsetenv("OPENCODE_MODEL_ID")
	})

	jwt, _ := auth.GenerateAccessToken(1, "test@example.com")

	body := map[string]interface{}{"prompt": "hello"}
	resp := makeRequestWithToken(t, app, "POST", "/ai/ask", body, jwt)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	args := (*captured)[0]
	if !containsArg(args, "--model") {
		t.Error("missing --model flag")
	}
	modelIdx := indexOfArg(args, "--model")
	if modelIdx < 0 || args[modelIdx+1] != "opencode/deepseek-v4-flash-free" {
		t.Errorf("model = %v, want 'opencode/deepseek-v4-flash-free'", args[modelIdx+1])
	}
}

func TestOpenCodeCLIArgsNoModelWhenUnset(t *testing.T) {
	captured := setExecCapture(t, `{"type":"text","part":{"text":"ok"}}`, nil)

	app := setupTestApp(t)
	os.Unsetenv("OPENCODE_PROVIDER_ID")
	os.Unsetenv("OPENCODE_MODEL_ID")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_PROVIDER_ID")
		os.Unsetenv("OPENCODE_MODEL_ID")
	})

	jwt, _ := auth.GenerateAccessToken(1, "test@example.com")

	body := map[string]interface{}{"prompt": "hello"}
	makeRequestWithToken(t, app, "POST", "/ai/ask", body, jwt)

	args := (*captured)[0]
	if containsArg(args, "--model") {
		t.Error("--model flag should not be present when env vars are unset")
	}
}

func TestOpenCodeCLIArgsPartialModelNotSent(t *testing.T) {
	captured := setExecCapture(t, `{"type":"text","part":{"text":"ok"}}`, nil)

	app := setupTestApp(t)
	os.Setenv("OPENCODE_PROVIDER_ID", "opencode")
	os.Unsetenv("OPENCODE_MODEL_ID")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_PROVIDER_ID")
		os.Unsetenv("OPENCODE_MODEL_ID")
	})

	jwt, _ := auth.GenerateAccessToken(1, "test@example.com")

	body := map[string]interface{}{"prompt": "hello"}
	makeRequestWithToken(t, app, "POST", "/ai/ask", body, jwt)

	args := (*captured)[0]
	if containsArg(args, "--model") {
		t.Error("--model flag should not be present when only provider is set")
	}
}

func TestParseOpenCodeNDJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "single text event",
			input: `{"type":"step_start","timestamp":1}` + "\n" + `{"type":"text","part":{"text":"hello world"}}` + "\n" + `{"type":"step_finish"}`,
			want:  "hello world",
		},
		{
			name:  "multiple text events concatenated",
			input: `{"type":"text","part":{"text":"line1"}}` + "\n" + `{"type":"text","part":{"text":"line2"}}`,
			want:  "line1\nline2",
		},
		{
			name:    "no text events",
			input:   `{"type":"step_start"}` + "\n" + `{"type":"step_finish"}`,
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:  "empty text ignored",
			input: `{"type":"text","part":{"text":""}}` + "\n" + `{"type":"text","part":{"text":"actual"}}`,
			want:  "actual",
		},
		{
			name:  "malformed lines skipped",
			input: `not json` + "\n" + `{"type":"text","part":{"text":"ok"}}`,
			want:  "ok",
		},
		{
			name:  "whitespace trimmed",
			input: `{"type":"text","part":{"text":"  trimmed  "}}`,
			want:  "trimmed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOpenCodeNDJSON([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("parseOpenCodeNDJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseOpenCodeNDJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func containsArg(args []string, target string) bool {
	for _, a := range args {
		if a == target {
			return true
		}
	}
	return false
}

func indexOfArg(args []string, target string) int {
	for i, a := range args {
		if a == target {
			return i
		}
	}
	return -1
}
