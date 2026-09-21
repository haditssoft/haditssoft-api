package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	aiProviderOpenAI        = "openai"
	openAIServerErrorPrefix = "AI server error: "
	defaultOpenAITimeout    = 90 * time.Second
	genericSystemPrompt     = "You are a helpful assistant. Always answer concisely and clearly."
)

// opencodeAgentsDir is where agent instruction files live relative to the
// working directory. The file for the requested agent is read and passed as
// the system message on the OpenAI provider path. Overridden in tests.
var opencodeAgentsDir = ".opencode/agents"

// chatCompletionFunc posts a chat completion request and returns the assistant
// reply. Overridden in tests to avoid real network calls.
var chatCompletionFunc = openAICompletion

// openAIHandlerRequest mirrors the OpenAI-compatible chat completions request.
type openAIHandlerRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIHandlerResponse mirrors the OpenAI-compatible chat completions response.
type openAIHandlerResponse struct {
	Choices []openAIChoice `json:"choices"`
}

type openAIChoice struct {
	Message openAIMessage `json:"message"`
}

// useOpenAIProvider reports whether the OpenAI provider is enabled: AI_PROVIDER
// must be "openai" (case-insensitive) with both OPENAI_API_KEY and
// OPENAI_BASE_URL set. Anything missing falls back to the opencode CLI path.
func useOpenAIProvider() bool {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("AI_PROVIDER")), aiProviderOpenAI) {
		return false
	}
	return strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != "" &&
		strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")) != ""
}

// runAIProvider dispatches to the OpenAI-compatible HTTP API when the provider
// is enabled, otherwise to the opencode CLI exactly as before.
func runAIProvider(ctx context.Context, prompt, agent string, model *openCodeModel) (string, error) {
	if useOpenAIProvider() {
		return chatCompletionFunc(ctx, agent, prompt)
	}
	return runOpenCodeCommand(ctx, prompt, agent, model)
}

// openAICompletion sends the prompt to {OPENAI_BASE_URL}/chat/completions with
// the agent instructions as the system message and returns choices[0] content.
func openAICompletion(ctx context.Context, agent, prompt string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")), "/")
	apiKey := os.Getenv("OPENAI_API_KEY")
	model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	if model == "" {
		return "", fmt.Errorf("OPENAI_MODEL is not set")
	}

	payload, err := json.Marshal(openAIHandlerRequest{
		Model: model,
		Messages: []openAIMessage{
			{Role: "system", Content: loadAgentSystemPrompt(agent)},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: defaultOpenAITimeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = resp.Status
		}
		return "", fmt.Errorf("%sHTTP %d: %s", openAIServerErrorPrefix, resp.StatusCode, msg)
	}

	var parsed openAIHandlerResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("%sinvalid response JSON: %w", openAIServerErrorPrefix, err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("%sno choices in response", openAIServerErrorPrefix)
	}

	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

// loadAgentSystemPrompt returns the instructions for the given agent from
// .opencode/agents/<agent>.md, falling back to a generic helpful-assistant
// prompt when the file is missing or unreadable.
func loadAgentSystemPrompt(agent string) string {
	if agent == "" {
		return genericSystemPrompt
	}
	data, err := os.ReadFile(filepath.Join(opencodeAgentsDir, agent+".md"))
	if err != nil {
		return genericSystemPrompt
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return genericSystemPrompt
	}
	return content
}
