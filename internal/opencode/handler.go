package opencode

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"os"

	"github.com/haditssoft/haditssoft-backend/internal/shared/validator"

	"github.com/gofiber/fiber/v2"
)

const (
	defaultSystemMessage    = "You are a helpful assistant."
	defaultOpenCodeUsername = "opencode"
)

type openCodeModel struct {
	ProviderID string `json:"providerID"`
	ModelID    string `json:"modelID"`
}

type openCodeSessionRequest struct {
	Title string `json:"title,omitempty"`
}

type openCodeSessionResponse struct {
	ID string `json:"id"`
}

type openCodePart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type openCodeMessageRequest struct {
	Parts  []openCodePart `json:"parts"`
	System string         `json:"system"`
	Agent  string         `json:"agent"`
	Model  *openCodeModel `json:"model,omitempty"`
}

type openCodeMessageResponse struct {
	Parts []openCodePart `json:"parts"`
}

func AskOpenCode(c *fiber.Ctx) error {
	modelValidation := new(AskOpenCodeRequest)
	if err := c.BodyParser(modelValidation); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	allErrors := validator.ValidateModel(modelValidation)
	if len(allErrors) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": allErrors})
	}

	baseURL := os.Getenv("OPENCODE_URL")
	if baseURL == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "OPENCODE_URL is not configured",
		})
	}

	// Read model config from env vars
	providerID := os.Getenv("OPENCODE_PROVIDER_ID")
	modelID := os.Getenv("OPENCODE_MODEL_ID")

	// Read agent from env vars
	agent := os.Getenv("OPENCODE_AGENT")
	if agent == "" {
		agent = "plan"
	}

	// Env vars take precedence over request body (request body should NOT override env vars)
	var model *openCodeModel
	if providerID != "" && modelID != "" {
		model = &openCodeModel{
			ProviderID: providerID,
			ModelID:    modelID,
		}
	}

	sessionID, err := createOpenCodeSession(baseURL)
	if err != nil {
		log.Println("opencode session error:", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "failed to create AI session",
		})
	}

	systemMsg := modelValidation.System
	if systemMsg == "" {
		systemMsg = defaultSystemMessage
	}

	reply, err := sendOpenCodeMessage(baseURL, sessionID, modelValidation.Prompt, systemMsg, agent, model)
	if err != nil {
		log.Println("opencode message error:", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "failed to get AI response",
		})
	}

	if err := deleteOpenCodeSession(baseURL, sessionID); err != nil {
		log.Println("opencode session delete error:", err)
	}

	return c.JSON(fiber.Map{
		"reply": reply,
	})
}

func openCodeBasicAuth() string {
	username := os.Getenv("OPENCODE_USERNAME")
	if username == "" {
		username = defaultOpenCodeUsername
	}

	password := os.Getenv("OPENCODE_PASSWORD")
	if password == "" {
		return ""
	}

	token := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	return "Basic " + token
}

func createOpenCodeSession(baseURL string) (string, error) {
	sessionReq := openCodeSessionRequest{Title: "Backend Automation"}
	body, _ := json.Marshal(sessionReq)

	a := fiber.AcquireAgent()
	req := a.Request()
	req.Header.SetMethod(fiber.MethodPost)
	req.Header.Add("Content-Type", "application/json")
	if basicAuth := openCodeBasicAuth(); basicAuth != "" {
		req.Header.Add("Authorization", basicAuth)
	}
	req.SetBodyString(string(body))
	req.SetRequestURI(baseURL + "/session")

	defer fiber.ReleaseAgent(a)

	if err := a.Parse(); err != nil {
		return "", err
	}

	code, resBody, errs := a.Bytes()
	if len(errs) > 0 {
		return "", fiber.ErrBadGateway
	}
	if code != fiber.StatusOK && code != fiber.StatusCreated {
		return "", fiber.ErrBadGateway
	}

	var sessionResp openCodeSessionResponse
	if err := json.Unmarshal(resBody, &sessionResp); err != nil {
		return "", err
	}

	return sessionResp.ID, nil
}

func sendOpenCodeMessage(baseURL, sessionID, prompt, system, agent string, model *openCodeModel) (string, error) {
	messageReq := openCodeMessageRequest{
		Parts:  []openCodePart{{Type: "text", Text: prompt}},
		System: system,
		Agent:  agent,
		Model:  model,
	}
	body, _ := json.Marshal(messageReq)

	a := fiber.AcquireAgent()
	req := a.Request()
	req.Header.SetMethod(fiber.MethodPost)
	req.Header.Add("Content-Type", "application/json")
	if basicAuth := openCodeBasicAuth(); basicAuth != "" {
		req.Header.Add("Authorization", basicAuth)
	}
	req.SetBodyString(string(body))
	req.SetRequestURI(baseURL + "/session/" + sessionID + "/message")

	defer fiber.ReleaseAgent(a)

	if err := a.Parse(); err != nil {
		return "", err
	}

	code, resBody, errs := a.Bytes()
	if len(errs) > 0 {
		return "", fiber.ErrBadGateway
	}
	if code != fiber.StatusOK {
		return "", fiber.ErrBadGateway
	}

	var messageResp openCodeMessageResponse
	if err := json.Unmarshal(resBody, &messageResp); err != nil {
		return "", err
	}

	for _, p := range messageResp.Parts {
		if p.Type == "text" {
			return p.Text, nil
		}
	}

	return "", fiber.ErrBadGateway
}

func deleteOpenCodeSession(baseURL, sessionID string) error {
	a := fiber.AcquireAgent()
	req := a.Request()
	req.Header.SetMethod(fiber.MethodDelete)
	if basicAuth := openCodeBasicAuth(); basicAuth != "" {
		req.Header.Add("Authorization", basicAuth)
	}
	req.SetRequestURI(baseURL + "/session/" + sessionID)

	defer fiber.ReleaseAgent(a)

	if err := a.Parse(); err != nil {
		return err
	}

	code, _, errs := a.Bytes()
	if len(errs) > 0 {
		return fiber.ErrBadGateway
	}
	if code != fiber.StatusOK && code != fiber.StatusNoContent {
		return fiber.ErrBadGateway
	}

	return nil
}
