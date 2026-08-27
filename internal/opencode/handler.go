package opencode

import (
	"encoding/json"
	"fmt"
	"io"
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

// execCommandFunc executes an external command and returns (stdout, stderr, error).
// Overridden in tests to avoid calling the real opencode CLI.
var execCommandFunc = execCommand

func execCommand(name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.Command(name, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	out, _ := io.ReadAll(stdout)
	errBytes, _ := io.ReadAll(stderr)
	cmdErr := cmd.Wait()
	return out, errBytes, cmdErr
}

func runOpenCodeCommand(prompt, agent string, model *openCodeModel) (string, error) {
	args := []string{"run", "--format", "json", "--pure", "--agent", agent}
	if model != nil && model.ProviderID != "" && model.ModelID != "" {
		args = append(args, "--model", model.ProviderID+"/"+model.ModelID)
	}
	args = append(args, prompt)

	out, stderr, err := execCommandFunc("opencode", args...)
	if err != nil {
		if len(out) > 0 {
			if ndjsonErr := parseOpenCodeNDJSONError(out); ndjsonErr != "" {
				errMsg := "opencode server error: " + ndjsonErr
				if isGenericServerError(ndjsonErr) && len(stderr) > 0 {
					errMsg += " (" + strings.TrimSpace(string(stderr)) + ")"
				}
				return "", fmt.Errorf(errMsg)
			}
		}
		if len(stderr) > 0 {
			return "", fmt.Errorf("opencode run failed: %s (%s)", err, strings.TrimSpace(string(stderr)))
		}
		return "", fmt.Errorf("opencode run failed: %w", err)
	}

	return parseOpenCodeNDJSON(out)
}

// isGenericServerError returns true if the NDJSON error message is generic
// and unhelpful (e.g. "Unexpected server error. Check server logs for details."),
// meaning the real cause is likely in stderr.
func isGenericServerError(msg string) bool {
	return strings.Contains(msg, "Unexpected server error")
}

func parseOpenCodeNDJSONError(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var event struct {
			Type  string `json:"type"`
			Error struct {
				Data struct {
					Message string `json:"message"`
				} `json:"data"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.Type == "error" && event.Error.Data.Message != "" {
			return event.Error.Data.Message
		}
	}
	return ""
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
