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

TASK

Translate the hadith into natural, clear, and accurate English.

The Arabic is the primary and authoritative source of truth. The Indonesian is only a reference for context and may be incorrect. When they conflict, always follow the Arabic.

RULES

1. Translate everything

   - Translate the entire provided Arabic text, including the book/collection name, hadith number, complete isnad/sanad, matan, and all introductory or concluding text.
   - Never summarize, omit, compress, or skip any part.

2. Preserve meaning and sequence

   - Preserve the meaning, sequence, relationships, and substantive information of the Arabic.
   - Do not invent, omit, distort, or rearrange information.
   - Preserve causal, temporal, contrasting, and responsive relationships expressed by the Arabic.

3. Translate contextually, not mechanically

   - Translate each phrase in light of the whole hadith, not word-by-word in isolation.
   - Prefer natural English when literal wording would be confusing, awkward, or misleading.
   - The goal is to preserve the meaning, not necessarily the Arabic word order.

4. Allow minimal contextual clarification

   - Small additions are allowed when strongly implied by the immediate context and necessary for clear English, e.g. "merely," "left unwashed," or "properly."
   - Such additions may clarify existing meaning but must not introduce new substantive information.
   - Do not add tafsir, commentary, historical information, legal conclusions, theological claims, or outside knowledge.

5. Preserve ambiguity

   - Do not turn an interpretation into an explicit fact unless the immediate context strongly supports it.
   - If the Arabic genuinely permits multiple meanings, preserve the ambiguity rather than guessing.

6. Islamic terminology

   - Keep established Islamic terms, names, places, and technical terminology accurate and consistent.
   - If a term cannot be reliably translated without guessing, transliterate it rather than inventing a meaning.

7. No invented rulings or causes

   - Do not add rulings, reasons, causes ("'illah"), or conclusions not communicated by the Arabic.
   - Do not assume that things mentioned together share the same reason or ruling without textual evidence.

8. Uncertainty

   - Use the Arabic context to resolve difficult expressions.
   - Reliable external sources may be consulted when available to verify linguistic or hadith-specific meaning, but external commentary must not be inserted into the translation.
   - If the meaning remains genuinely uncertain, preserve the ambiguity or transliterate rather than guess.
   - Never use the Indonesian translation to override the Arabic.

PRIORITY

When choices conflict, prioritize:

Contextual accuracy → faithful meaning → natural English → literal wording

EXAMPLE

Arabic:

فَجَعَلْنَا نَمْسَحُ عَلَى أَرْجُلِنَا فَنَادَى بِأَعْلَى صَوْتِهِ وَيْلٌ لِلْأَعْقَابِ مِنَ النَّارِ

Prefer:

"So we began merely wiping over our feet. He then called out at the top of his voice: 'Woe to the heels left unwashed, for they will suffer the Fire!'"

Rather than:

"So we began wiping over our feet. He then called out at the top of his voice: 'Woe to the heels from the Fire!'"

"Left unwashed" is permitted because it clarifies the contextual meaning of the warning; it is not claimed to be a word-for-word rendering.

OUTPUT

Output only the English translation. No preamble, commentary, analysis, tafsir, explanation, or headings.

Integrate necessary contextual clarification naturally into the translation. Use brackets only when a clarification cannot be naturally incorporated into the sentence.`

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
