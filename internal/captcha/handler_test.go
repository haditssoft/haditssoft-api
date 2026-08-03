package captcha

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func setupTestApp(t *testing.T) *fiber.App {
	t.Helper()
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

func decodeJSON(t *testing.T, resp *http.Response, dest interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

func TestRoute_CaptchaPOST(t *testing.T) {
	app := setupTestApp(t)

	body := map[string]interface{}{
		"token": "fake-token",
	}
	resp := makeRequest(t, app, "POST", "/verifyreCaptcha/", body)

	// Google reCAPTCHA returns 200 with success:false for invalid tokens
	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if success, ok := result["success"].(bool); ok && success {
		t.Error("expected success to be false for fake token")
	}
}

func TestRoute_CaptchaInvalidJSON(t *testing.T) {
	app := setupTestApp(t)

	req := httptest.NewRequest("POST", "/verifyreCaptcha/", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestRoute_CaptchaWrongMethod(t *testing.T) {
	app := setupTestApp(t)

	resp := makeRequest(t, app, "GET", "/verifyreCaptcha/", nil)
	if resp.StatusCode == http.StatusOK {
		t.Error("GET should not return 200")
	}
}

func TestRoute_CaptchaRouteExists(t *testing.T) {
	app := setupTestApp(t)

	routes := app.Stack()
	routeCount := 0
	for _, methodRoutes := range routes {
		for _, route := range methodRoutes {
			if route.Method == "POST" && route.Path == "/verifyreCaptcha/" {
				routeCount++
			}
		}
	}

	if routeCount != 1 {
		t.Errorf("expected 1 POST /verifyreCaptcha/ route, got %d", routeCount)
	}
}

func TestRoute_CaptchaBody(t *testing.T) {
	app := setupTestApp(t)

	body := map[string]interface{}{
		"bookInfo":   "Shahih Bukhari",
		"reportText": "Typo in hadith number 1",
	}
	resp := makeRequest(t, app, "POST", "/verifyreCaptcha/", body)

	// Google reCAPTCHA returns 200 with success:false for invalid tokens
	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	if success, ok := result["success"].(bool); ok && success {
		t.Error("expected success to be false for fake token")
	}
}
