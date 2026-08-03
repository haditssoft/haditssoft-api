package opencode

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

	body := map[string]interface{}{
		"system": "You are a helpful assistant.",
	}
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

func TestRoute_AskMissingOpenCodeURL(t *testing.T) {
	app := setupTestApp(t)

	os.Unsetenv("OPENCODE_URL")

	jwt, err := auth.GenerateAccessToken(1, "test@example.com")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	body := map[string]interface{}{
		"prompt": "What is hadith?",
	}
	resp := makeRequestWithToken(t, app, "POST", "/ai/ask", body, jwt)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	var errResp map[string]interface{}
	decodeJSON(t, resp, &errResp)
	if errResp["error"] != "OPENCODE_URL is not configured" {
		t.Errorf("error = %v, want 'OPENCODE_URL is not configured'", errResp["error"])
	}
}

func TestRoute_AskExternalAPIFailure(t *testing.T) {
	app := setupTestApp(t)

	os.Setenv("OPENCODE_URL", "http://127.0.0.1:19999")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_URL") })

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
	if errResp["error"] != "failed to create AI session" {
		t.Errorf("error = %v, want 'failed to create AI session'", errResp["error"])
	}
}

func TestRoute_AskWrongMethod(t *testing.T) {
	app := setupTestApp(t)

	resp := makeRequest(t, app, "GET", "/ai/ask", nil)
	if resp.StatusCode == http.StatusOK {
		t.Error("GET should not return 200")
	}
}

func TestRoute_AskWithSystemMessage(t *testing.T) {
	app := setupTestApp(t)

	os.Setenv("OPENCODE_URL", "http://127.0.0.1:19999")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_URL") })

	jwt, err := auth.GenerateAccessToken(1, "test@example.com")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	body := map[string]interface{}{
		"prompt": "What is hadith?",
		"system": "You are a Quran scholar.",
	}
	resp := makeRequestWithToken(t, app, "POST", "/ai/ask", body, jwt)

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadGateway)
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

// setupModelTestApp creates a test app with a mock opencode server that captures requests
func setupModelTestApp(t *testing.T) (*fiber.App, **http.Request, *[]byte, string) {
	t.Helper()

	var capturedMsgReq *http.Request
	var capturedMsgBody []byte
	sessionID := "test-session-123"

	msgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/session" && r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"id": sessionID})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/session/") && strings.HasSuffix(r.URL.Path, "/message") && r.Method == "POST" {
			capturedMsgReq = r
			capturedMsgBody, _ = io.ReadAll(r.Body)
			r.Body.Close()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"parts": []map[string]string{{"type": "text", "text": "test response"}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(msgServer.Close)

	app := setupTestApp(t)
	return app, &capturedMsgReq, &capturedMsgBody, msgServer.URL
}

// TestOpenCodeModelFieldBothEnvVarsSet tests that model field is sent when both env vars are set
func TestOpenCodeModelFieldBothEnvVarsSet(t *testing.T) {
	os.Setenv("OPENCODE_URL", "http://placeholder")
	os.Setenv("OPENCODE_PROVIDER_ID", "opencode")
	os.Setenv("OPENCODE_MODEL_ID", "deepseek-v4-flash-free")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_URL")
		os.Unsetenv("OPENCODE_PROVIDER_ID")
		os.Unsetenv("OPENCODE_MODEL_ID")
	})

	app, capturedMsgReq, capturedMsgBody, testURL := setupModelTestApp(t)
	os.Setenv("OPENCODE_URL", testURL)
	jwt, _ := auth.GenerateAccessToken(1, "test@example.com")

	reqBody := map[string]interface{}{
		"prompt":      "What is hadith?",
		"provider_id": "ignored-provider",
		"model_id":    "ignored-model",
	}
	resp := makeRequestWithToken(t, app, "POST", "/ai/ask", reqBody, jwt)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if *capturedMsgReq == nil {
		t.Fatal("no message request captured")
	}

	var msgReqBody map[string]interface{}
	if err := json.Unmarshal(*capturedMsgBody, &msgReqBody); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	model, ok := msgReqBody["model"].(map[string]interface{})
	if !ok {
		t.Fatal("model field missing from request body")
	}
	if model["providerID"] != "opencode" {
		t.Errorf("providerID = %v, want 'opencode'", model["providerID"])
	}
	if model["modelID"] != "deepseek-v4-flash-free" {
		t.Errorf("modelID = %v, want 'deepseek-v4-flash-free'", model["modelID"])
	}

	if msgReqBody["provider_id"] != nil {
		t.Error("provider_id from request body should not be in request")
	}
	if msgReqBody["model_id"] != nil {
		t.Error("model_id from request body should not be in request")
	}
}

// TestOpenCodeModelFieldOnlyProviderIDSet tests that model field is NOT sent when only provider is set
func TestOpenCodeModelFieldOnlyProviderIDSet(t *testing.T) {
	os.Setenv("OPENCODE_URL", "http://placeholder")
	os.Setenv("OPENCODE_PROVIDER_ID", "opencode")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_URL")
		os.Unsetenv("OPENCODE_PROVIDER_ID")
	})

	app, capturedMsgReq, capturedMsgBody, testURL := setupModelTestApp(t)
	os.Setenv("OPENCODE_URL", testURL)
	jwt, _ := auth.GenerateAccessToken(1, "test@example.com")

	reqBody := map[string]interface{}{
		"prompt": "What is hadith?",
	}
	makeRequestWithToken(t, app, "POST", "/ai/ask", reqBody, jwt)

	if *capturedMsgReq == nil {
		t.Fatal("no message request captured")
	}

	var msgReqBody map[string]interface{}
	if err := json.Unmarshal(*capturedMsgBody, &msgReqBody); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	if _, ok := msgReqBody["model"]; ok {
		t.Error("model field should not be present when only OPENCODE_PROVIDER_ID is set")
	}
}

// TestOpenCodeModelFieldOnlyModelIDSet tests that model field is NOT sent when only model is set
func TestOpenCodeModelFieldOnlyModelIDSet(t *testing.T) {
	os.Setenv("OPENCODE_URL", "http://placeholder")
	os.Setenv("OPENCODE_MODEL_ID", "deepseek-v4-flash-free")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_URL")
		os.Unsetenv("OPENCODE_MODEL_ID")
	})

	app, capturedMsgReq, capturedMsgBody, testURL := setupModelTestApp(t)
	os.Setenv("OPENCODE_URL", testURL)
	jwt, _ := auth.GenerateAccessToken(1, "test@example.com")

	reqBody := map[string]interface{}{
		"prompt": "What is hadith?",
	}
	makeRequestWithToken(t, app, "POST", "/ai/ask", reqBody, jwt)

	if *capturedMsgReq == nil {
		t.Fatal("no message request captured")
	}

	var msgReqBody map[string]interface{}
	if err := json.Unmarshal(*capturedMsgBody, &msgReqBody); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	if _, ok := msgReqBody["model"]; ok {
		t.Error("model field should not be present when only OPENCODE_MODEL_ID is set")
	}
}

// TestOpenCodeModelFieldNeitherEnvVarSet tests that model field is NOT sent when neither env var is set
func TestOpenCodeModelFieldNeitherEnvVarSet(t *testing.T) {
	os.Setenv("OPENCODE_URL", "http://placeholder")
	t.Cleanup(func() { os.Unsetenv("OPENCODE_URL") })

	app, capturedMsgReq, capturedMsgBody, testURL := setupModelTestApp(t)
	os.Setenv("OPENCODE_URL", testURL)
	jwt, _ := auth.GenerateAccessToken(1, "test@example.com")

	reqBody := map[string]interface{}{
		"prompt": "What is hadith?",
	}
	makeRequestWithToken(t, app, "POST", "/ai/ask", reqBody, jwt)

	if *capturedMsgReq == nil {
		t.Fatal("no message request captured")
	}

	var msgReqBody map[string]interface{}
	if err := json.Unmarshal(*capturedMsgBody, &msgReqBody); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	if _, ok := msgReqBody["model"]; ok {
		t.Error("model field should not be present when neither env var is set")
	}
}

// TestOpenCodeModelFieldRequestBodyIgnored tests that request body provider_id/model_id are ignored
func TestOpenCodeModelFieldRequestBodyIgnored(t *testing.T) {
	os.Setenv("OPENCODE_URL", "http://placeholder")
	os.Setenv("OPENCODE_PROVIDER_ID", "opencode")
	os.Setenv("OPENCODE_MODEL_ID", "deepseek-v4-flash-free")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_URL")
		os.Unsetenv("OPENCODE_PROVIDER_ID")
		os.Unsetenv("OPENCODE_MODEL_ID")
	})

	app, capturedMsgReq, capturedMsgBody, testURL := setupModelTestApp(t)
	os.Setenv("OPENCODE_URL", testURL)
	jwt, _ := auth.GenerateAccessToken(1, "test@example.com")

	reqBody := map[string]interface{}{
		"prompt":      "What is hadith?",
		"provider_id": "anthropic",
		"model_id":    "claude-3-opus",
	}
	makeRequestWithToken(t, app, "POST", "/ai/ask", reqBody, jwt)

	if *capturedMsgReq == nil {
		t.Fatal("no message request captured")
	}

	var msgReqBody map[string]interface{}
	if err := json.Unmarshal(*capturedMsgBody, &msgReqBody); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	model, ok := msgReqBody["model"].(map[string]interface{})
	if !ok {
		t.Fatal("model field missing from request body")
	}
	if model["providerID"] != "opencode" {
		t.Errorf("providerID = %v, want 'opencode' (from env, not request body)", model["providerID"])
	}
	if model["modelID"] != "deepseek-v4-flash-free" {
		t.Errorf("modelID = %v, want 'deepseek-v4-flash-free' (from env, not request body)", model["modelID"])
	}

	if msgReqBody["provider_id"] != nil {
		t.Error("provider_id from request body should not be in request to opencode")
	}
	if msgReqBody["model_id"] != nil {
		t.Error("model_id from request body should not be in request to opencode")
	}
}

// TestOpenCodeAgentFieldDefaultIsPlan tests that agent defaults to "plan" when OPENCODE_AGENT is unset
func TestOpenCodeAgentFieldDefaultIsPlan(t *testing.T) {
	os.Unsetenv("OPENCODE_AGENT")
	os.Setenv("OPENCODE_URL", "http://placeholder")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_URL")
		os.Unsetenv("OPENCODE_AGENT")
	})

	app, capturedMsgReq, capturedMsgBody, testURL := setupModelTestApp(t)
	os.Setenv("OPENCODE_URL", testURL)
	jwt, _ := auth.GenerateAccessToken(1, "test@example.com")

	reqBody := map[string]interface{}{
		"prompt": "What is hadith?",
	}
	resp := makeRequestWithToken(t, app, "POST", "/ai/ask", reqBody, jwt)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if *capturedMsgReq == nil {
		t.Fatal("no message request captured")
	}

	var msgReqBody map[string]interface{}
	if err := json.Unmarshal(*capturedMsgBody, &msgReqBody); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	agent, ok := msgReqBody["agent"].(string)
	if !ok {
		t.Fatal("agent field missing from request body")
	}
	if agent != "plan" {
		t.Errorf("agent = %v, want 'plan' (default)", agent)
	}
}

// TestOpenCodeAgentFieldFromEnv tests that agent uses the value from OPENCODE_AGENT
func TestOpenCodeAgentFieldFromEnv(t *testing.T) {
	os.Setenv("OPENCODE_AGENT", "architect")
	os.Setenv("OPENCODE_URL", "http://placeholder")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_URL")
		os.Unsetenv("OPENCODE_AGENT")
	})

	app, capturedMsgReq, capturedMsgBody, testURL := setupModelTestApp(t)
	os.Setenv("OPENCODE_URL", testURL)
	jwt, _ := auth.GenerateAccessToken(1, "test@example.com")

	reqBody := map[string]interface{}{
		"prompt": "What is hadith?",
	}
	resp := makeRequestWithToken(t, app, "POST", "/ai/ask", reqBody, jwt)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if *capturedMsgReq == nil {
		t.Fatal("no message request captured")
	}

	var msgReqBody map[string]interface{}
	if err := json.Unmarshal(*capturedMsgBody, &msgReqBody); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	agent, ok := msgReqBody["agent"].(string)
	if !ok {
		t.Fatal("agent field missing from request body")
	}
	if agent != "architect" {
		t.Errorf("agent = %v, want 'architect' (from env)", agent)
	}
}

// TestOpenCodeAgentFieldEmptyEnvFallsBackToPlan tests that empty OPENCODE_AGENT falls back to "plan"
func TestOpenCodeAgentFieldEmptyEnvFallsBackToPlan(t *testing.T) {
	os.Setenv("OPENCODE_AGENT", "")
	os.Setenv("OPENCODE_URL", "http://placeholder")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_URL")
		os.Unsetenv("OPENCODE_AGENT")
	})

	app, capturedMsgReq, capturedMsgBody, testURL := setupModelTestApp(t)
	os.Setenv("OPENCODE_URL", testURL)
	jwt, _ := auth.GenerateAccessToken(1, "test@example.com")

	reqBody := map[string]interface{}{
		"prompt": "What is hadith?",
	}
	resp := makeRequestWithToken(t, app, "POST", "/ai/ask", reqBody, jwt)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if *capturedMsgReq == nil {
		t.Fatal("no message request captured")
	}

	var msgReqBody map[string]interface{}
	if err := json.Unmarshal(*capturedMsgBody, &msgReqBody); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	agent, ok := msgReqBody["agent"].(string)
	if !ok {
		t.Fatal("agent field missing from request body")
	}
	if agent != "plan" {
		t.Errorf("agent = %v, want 'plan' (fallback for empty env)", agent)
	}
}

// setupBasicAuthTestApp creates a test app with a mock opencode server that captures
// the Authorization header on every outbound request (session, message, delete).
func setupBasicAuthTestApp(t *testing.T) (*fiber.App, *string, *string, *string, string) {
	t.Helper()

	var sessionAuth, msgAuth, deleteAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		switch {
		case r.URL.Path == "/session" && r.Method == http.MethodPost:
			sessionAuth = auth
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"id": "test-session-123"})
		case strings.HasPrefix(r.URL.Path, "/session/") && strings.HasSuffix(r.URL.Path, "/message") && r.Method == http.MethodPost:
			msgAuth = auth
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"parts": []map[string]string{{"type": "text", "text": "test response"}},
			})
		case strings.HasPrefix(r.URL.Path, "/session/") && r.Method == http.MethodDelete:
			deleteAuth = auth
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	app := setupTestApp(t)
	return app, &sessionAuth, &msgAuth, &deleteAuth, srv.URL
}

// TestOpenCodeBasicAuthDefaultUsername tests that all outbound requests send
// Basic auth with the default "opencode" username when OPENCODE_USERNAME is unset.
func TestOpenCodeBasicAuthDefaultUsername(t *testing.T) {
	os.Unsetenv("OPENCODE_USERNAME")
	os.Setenv("OPENCODE_PASSWORD", "test-password")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_USERNAME")
		os.Unsetenv("OPENCODE_PASSWORD")
	})

	app, sessionAuth, msgAuth, deleteAuth, testURL := setupBasicAuthTestApp(t)
	os.Setenv("OPENCODE_URL", testURL)
	t.Cleanup(func() { os.Unsetenv("OPENCODE_URL") })

	jwt, err := auth.GenerateAccessToken(1, "test@example.com")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	body := map[string]interface{}{"prompt": "What is hadith?"}
	resp := makeRequestWithToken(t, app, "POST", "/ai/ask", body, jwt)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("opencode:test-password"))
	if *sessionAuth != want {
		t.Errorf("session auth = %q, want %q", *sessionAuth, want)
	}
	if *msgAuth != want {
		t.Errorf("message auth = %q, want %q", *msgAuth, want)
	}
	if *deleteAuth != want {
		t.Errorf("delete auth = %q, want %q", *deleteAuth, want)
	}
}

// TestOpenCodeBasicAuthCustomUsername tests that OPENCODE_USERNAME overrides the default.
func TestOpenCodeBasicAuthCustomUsername(t *testing.T) {
	os.Setenv("OPENCODE_USERNAME", "automation")
	os.Setenv("OPENCODE_PASSWORD", "secret-pw")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_USERNAME")
		os.Unsetenv("OPENCODE_PASSWORD")
	})

	app, sessionAuth, msgAuth, deleteAuth, testURL := setupBasicAuthTestApp(t)
	os.Setenv("OPENCODE_URL", testURL)
	t.Cleanup(func() { os.Unsetenv("OPENCODE_URL") })

	jwt, err := auth.GenerateAccessToken(1, "test@example.com")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	body := map[string]interface{}{"prompt": "What is hadith?"}
	resp := makeRequestWithToken(t, app, "POST", "/ai/ask", body, jwt)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("automation:secret-pw"))
	if *sessionAuth != want {
		t.Errorf("session auth = %q, want %q", *sessionAuth, want)
	}
	if *msgAuth != want {
		t.Errorf("message auth = %q, want %q", *msgAuth, want)
	}
	if *deleteAuth != want {
		t.Errorf("delete auth = %q, want %q", *deleteAuth, want)
	}
}

// TestOpenCodeBasicAuthNoPassword tests that no Authorization header is sent
// when OPENCODE_PASSWORD is unset (backwards compatible).
func TestOpenCodeBasicAuthNoPassword(t *testing.T) {
	os.Setenv("OPENCODE_USERNAME", "opencode")
	os.Unsetenv("OPENCODE_PASSWORD")
	t.Cleanup(func() {
		os.Unsetenv("OPENCODE_USERNAME")
		os.Unsetenv("OPENCODE_PASSWORD")
	})

	app, sessionAuth, msgAuth, deleteAuth, testURL := setupBasicAuthTestApp(t)
	os.Setenv("OPENCODE_URL", testURL)
	t.Cleanup(func() { os.Unsetenv("OPENCODE_URL") })

	jwt, err := auth.GenerateAccessToken(1, "test@example.com")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	body := map[string]interface{}{"prompt": "What is hadith?"}
	resp := makeRequestWithToken(t, app, "POST", "/ai/ask", body, jwt)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if *sessionAuth != "" {
		t.Errorf("session auth = %q, want empty (no password configured)", *sessionAuth)
	}
	if *msgAuth != "" {
		t.Errorf("message auth = %q, want empty (no password configured)", *msgAuth)
	}
	if *deleteAuth != "" {
		t.Errorf("delete auth = %q, want empty (no password configured)", *deleteAuth)
	}
}

// TestOpenCodeBasicAuthHelper covers the credential builder directly.
func TestOpenCodeBasicAuthHelper(t *testing.T) {
	t.Run("default username", func(t *testing.T) {
		os.Unsetenv("OPENCODE_USERNAME")
		os.Setenv("OPENCODE_PASSWORD", "pw")
		t.Cleanup(func() {
			os.Unsetenv("OPENCODE_USERNAME")
			os.Unsetenv("OPENCODE_PASSWORD")
		})

		want := "Basic " + base64.StdEncoding.EncodeToString([]byte("opencode:pw"))
		if got := openCodeBasicAuth(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("custom username", func(t *testing.T) {
		os.Setenv("OPENCODE_USERNAME", "admin")
		os.Setenv("OPENCODE_PASSWORD", "pw")
		t.Cleanup(func() {
			os.Unsetenv("OPENCODE_USERNAME")
			os.Unsetenv("OPENCODE_PASSWORD")
		})

		want := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:pw"))
		if got := openCodeBasicAuth(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("empty password", func(t *testing.T) {
		os.Setenv("OPENCODE_USERNAME", "opencode")
		os.Unsetenv("OPENCODE_PASSWORD")
		t.Cleanup(func() {
			os.Unsetenv("OPENCODE_USERNAME")
			os.Unsetenv("OPENCODE_PASSWORD")
		})

		if got := openCodeBasicAuth(); got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("empty username falls back to default", func(t *testing.T) {
		os.Setenv("OPENCODE_USERNAME", "")
		os.Setenv("OPENCODE_PASSWORD", "pw")
		t.Cleanup(func() {
			os.Unsetenv("OPENCODE_USERNAME")
			os.Unsetenv("OPENCODE_PASSWORD")
		})

		want := "Basic " + base64.StdEncoding.EncodeToString([]byte("opencode:pw"))
		if got := openCodeBasicAuth(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
