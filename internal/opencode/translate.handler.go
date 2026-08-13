package opencode

import (
	"crypto/subtle"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/models"
)

const (
	defaultTranslateLimit = 10
)

const (
	defaultTranslationPromptFile = "translation_system_prompt.txt"
)

type translationRow struct {
	Nomer     uint
	Arabic    *string
	Indonesia *string
}

type translateResult struct {
	Nomer uint   `json:"nomer"`
	Error string `json:"error"`
}

func loadTranslationSystemMessage() (string, error) {
	path := os.Getenv("TRANSLATION_SYSTEM_PROMPT_FILE")
	if path == "" {
		path = defaultTranslationPromptFile
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read translation system prompt file %q: %w", path, err)
	}

	prompt := strings.TrimSpace(string(data))
	if prompt == "" {
		return "", fmt.Errorf("translation system prompt file %q is empty", path)
	}

	return prompt, nil
}

func TranslateHadiths(c *fiber.Ctx) error {
	expectedKey := os.Getenv("OPENCODE_CRON_KEY")
	providedKey := c.Query("key")
	if expectedKey == "" || subtle.ConstantTimeCompare([]byte(providedKey), []byte(expectedKey)) != 1 {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "invalid or missing cron key",
			"data":    nil,
		})
	}

	kitabName := c.Params("kitabName")
	if _, ok := models.GetIndexOfKitab[kitabName]; !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "unknown kitab: " + kitabName,
			"data":    nil,
		})
	}

	limit := defaultTranslateLimit
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status":  "error",
				"message": "limit must be a positive integer",
				"data":    nil,
			})
		}
		limit = parsed
	}

	var rows []translationRow
	if err := database.DB.Table(kitabName).
		Select("Nomer", "Arabic", "Indonesia").
		Where("English IS NULL OR English = ''").
		Order("Nomer ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		log.Println("translate query error:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to query hadith records",
			"data":    nil,
		})
	}

	baseURL := os.Getenv("OPENCODE_URL")
	if baseURL == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "OPENCODE_URL is not configured",
			"data":    nil,
		})
	}

	systemMessage, err := loadTranslationSystemMessage()
	if err != nil {
		log.Println("translate system prompt error:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to load translation system prompt",
			"data":    nil,
		})
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
		agent = "plan"
	}

	updated := 0
	failed := make([]translateResult, 0)

	for _, row := range rows {
		arabic := ""
		if row.Arabic != nil {
			arabic = *row.Arabic
		}
		indonesia := ""
		if row.Indonesia != nil {
			indonesia = *row.Indonesia
		}

		prompt := "Teks Arab:\n" + arabic + "\n\nTeks Indonesia:\n" + indonesia

		sessionID, err := createOpenCodeSession(baseURL)
		if err != nil {
			log.Println("translate session error:", err)
			failed = append(failed, translateResult{Nomer: row.Nomer, Error: "failed to create AI session"})
			continue
		}

		reply, err := sendOpenCodeMessage(baseURL, sessionID, prompt, systemMessage, agent, model)
		if err != nil {
			log.Println("translate message error:", err)
			failed = append(failed, translateResult{Nomer: row.Nomer, Error: "failed to get AI response"})
			_ = deleteOpenCodeSession(baseURL, sessionID)
			continue
		}

		if err := deleteOpenCodeSession(baseURL, sessionID); err != nil {
			log.Println("translate session delete error:", err)
		}

		if strings.TrimSpace(reply) == "" {
			log.Println("translate empty reply:", row.Nomer)
			failed = append(failed, translateResult{Nomer: row.Nomer, Error: "empty AI response"})
			continue
		}

		if err := database.DB.Table(kitabName).
			Where("Nomer = ?", row.Nomer).
			Update("English", reply).Error; err != nil {
			log.Println("translate update error:", err)
			failed = append(failed, translateResult{Nomer: row.Nomer, Error: err.Error()})
			continue
		}
		updated++
	}

	return c.JSON(fiber.Map{
		"processed": len(rows),
		"updated":   updated,
		"failed":    failed,
	})
}
