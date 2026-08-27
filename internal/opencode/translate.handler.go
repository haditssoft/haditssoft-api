package opencode

import (
	"crypto/subtle"
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

type translationRow struct {
	Nomer     uint
	Arabic    *string
	Indonesia *string
}

type translateResult struct {
	Nomer uint   `json:"nomer"`
	Error string `json:"error"`
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

	providerID := os.Getenv("OPENCODE_PROVIDER_ID")
	modelID := os.Getenv("OPENCODE_MODEL_ID")
	var model *openCodeModel
	if providerID != "" && modelID != "" {
		model = &openCodeModel{
			ProviderID: providerID,
			ModelID:    modelID,
		}
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

		reply, err := runOpenCodeCommand(prompt, "translate", model)
		if err != nil {
			log.Println("translate opencode error:", err)
			failed = append(failed, translateResult{Nomer: row.Nomer, Error: "failed to get AI response"})
			continue
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
