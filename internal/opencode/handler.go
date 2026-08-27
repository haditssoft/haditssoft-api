package opencode

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/haditssoft/haditssoft-backend/internal/shared/validator"

	"github.com/gofiber/fiber/v2"
)

const (
	defaultOpenCodeAgent = "plan"
)

type openCodeModel struct {
	ProviderID string `json:"providerID"`
	ModelID    string `json:"modelID"`
}

// execCommandFunc is the function used to execute external commands.
// Overridden in tests to avoid calling the real opencode CLI.
var execCommandFunc = execCommand

func execCommand(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

func runOpenCodeCommand(prompt, agent string, model *openCodeModel) (string, error) {
	args := []string{"run", "--format", "json", "--agent", agent}
	if model != nil && model.ProviderID != "" && model.ModelID != "" {
		args = append(args, "--model", model.ProviderID+"/"+model.ModelID)
	}
	args = append(args, prompt)

	out, err := execCommandFunc("opencode", args...)
	if err != nil {
		return "", fmt.Errorf("opencode run failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	return parseOpenCodeNDJSON(out)
}

func parseOpenCodeNDJSON(data []byte) (string, error) {
	var texts []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var event struct {
			Type string `json:"type"`
			Part struct {
				Text string `json:"text"`
			} `json:"part"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.Type == "text" && event.Part.Text != "" {
			texts = append(texts, event.Part.Text)
		}
	}

	if len(texts) == 0 {
		return "", fmt.Errorf("no text response in opencode output")
	}

	return strings.TrimSpace(strings.Join(texts, "\n")), nil
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

	providerID := os.Getenv("OPENCODE_PROVIDER_ID")
	modelID := os.Getenv("OPENCODE_MODEL_ID")

	var model *openCodeModel
	if providerID != "" && modelID != "" {
		model = &openCodeModel{
			ProviderID: providerID,
			ModelID:    modelID,
		}
	}

	agent := os.Getenv("OPENCODE_AGENT")
	if agent == "" {
		agent = defaultOpenCodeAgent
	}

	reply, err := runOpenCodeCommand(modelValidation.Prompt, agent, model)
	if err != nil {
		log.Println("opencode run error:", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "failed to get AI response",
		})
	}

	return c.JSON(fiber.Map{
		"reply": reply,
	})
}
