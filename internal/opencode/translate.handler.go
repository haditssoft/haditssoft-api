package opencode

import (
	"crypto/subtle"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/models"

	"github.com/gofiber/fiber/v2"
)

const translationSystemMessage = `You are an expert Islamic scholar (ulama) specializing in the Qur'an, hadith, and fiqh. You will be given a hadith in Arabic and a reference translation in Indonesian.

TASK:
Translate the hadith into English. The ARABIC text is the main source and the only source of truth — translate it faithfully and accurately.

TRANSLATE EVERYTHING:
- Translate the ENTIRE hadith — every single word — including the book name, the hadith number, and the full chain of narrators (isnad/sanad), not just the main text (matan).
- This is a complete translation, not a summary. Do not summarize, compress, or skip any part of the text.

IMPORTANT RULES:
- The Indonesian text is ONLY a reference to help you understand the context. It is NOT the source of truth and it may be incorrect. Always prioritize the meaning of the Arabic text over the Indonesian text.
- Your translation must match the meaning of the Arabic text exactly and must not deviate from it.
- Do NOT add information that has no basis (no dalil). Do not add, omit, or change anything that is not mentioned in the Arabic text.
- You may add brief clarifications ONLY to explain context (e.g., a historical or linguistic note), and only when the meaning would otherwise be unclear. Clearly mark such clarifications in brackets so the reader can distinguish them from the actual translation.
- If you do not know the context or meaning of a word or phrase, try to search for it first using reliable hadith sources. If still unsure, you may cautiously use the Indonesian text as a reference — but always remember it might be wrong, and cross-check it against the Arabic text.
- Do not invent reasons, rulings, or causes ('illah) unless they are supported by a dalil from the Qur'an or authentic hadith.
- Do not assume that things mentioned together share the same reason unless there is valid evidence.
- If you genuinely cannot determine the meaning of a term, keep it transliterated rather than guessing.
- Output ONLY the English translation of the hadith. No preamble, no commentary, no headers, no analysis — unless it is a brief bracketed clarification for context.`

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

	baseURL := os.Getenv("OPENCODE_URL")
	if baseURL == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "OPENCODE_URL is not configured",
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

		reply, err := sendOpenCodeMessage(baseURL, sessionID, prompt, translationSystemMessage, agent, model)
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
