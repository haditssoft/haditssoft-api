package opencode

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
)

const (
	bulkTranslateAgent = "translate-bulk"
)

type bulkHadithInput struct {
	Arabic    string `json:"arabic"`
	Indonesia string `json:"indonesia"`
}

// buildBulkTranslatePrompt marshals the rows into the JSON payload fed to the
// translate-bulk agent. Each key is the hadith number (Nomer column) and each
// value holds the Arabic text plus the Indonesian reference translation.
func buildBulkTranslatePrompt(rows []translationRow) (string, error) {
	payload := make(map[string]bulkHadithInput, len(rows))
	for _, row := range rows {
		arabic := ""
		if row.Arabic != nil {
			arabic = *row.Arabic
		}
		indonesia := ""
		if row.Indonesia != nil {
			indonesia = *row.Indonesia
		}
		payload[strconv.FormatUint(uint64(row.Nomer), 10)] = bulkHadithInput{
			Arabic:    arabic,
			Indonesia: indonesia,
		}
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// parseBulkTranslationReply parses the agent's reply into a map keyed by the
// hadith number. It tolerates markdown code fences, leading/trailing text, and
// other noise by extracting the first { ... } block when a direct parse fails.
// The forbidden shape ({"Nomer": {"arabic": ..., "english": ...}}) fails
// parsing because the values must be plain strings.
func parseBulkTranslationReply(reply string) (map[string]string, error) {
	raw := strings.TrimSpace(reply)
	if raw == "" {
		return nil, errors.New("empty AI response")
	}

	var out map[string]string
	if err := json.Unmarshal([]byte(raw), &out); err == nil {
		return out, nil
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		var extracted map[string]string
		if err := json.Unmarshal([]byte(raw[start:end+1]), &extracted); err == nil {
			return extracted, nil
		}
	}

	return nil, errors.New("invalid JSON in AI response")
}

// TranslateHadithsBulk translates several hadiths in a single AI call. It reads
// the same untranslated rows as TranslateHadiths, feeds them to the agent as one
// JSON payload keyed by Nomer, then writes the returned translations back.
func TranslateHadithsBulk(c *fiber.Ctx) error {
	if !authorizeCronKey(c) {
		return nil
	}

	kitabName, ok := resolveKitabName(c)
	if !ok {
		return nil
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
		log.Println("translate bulk query error:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to query hadith records",
			"data":    nil,
		})
	}

	if len(rows) == 0 {
		return c.JSON(fiber.Map{
			"processed": 0,
			"updated":   0,
			"failed":    []translateResult{},
		})
	}

	prompt, err := buildBulkTranslatePrompt(rows)
	if err != nil {
		log.Println("translate bulk prompt error:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to build translation payload",
			"data":    nil,
		})
	}

	loadTranslateConfig()

	providerID := os.Getenv("OPENCODE_PROVIDER_ID")
	modelID := os.Getenv("OPENCODE_MODEL_ID")
	var model *openCodeModel
	if providerID != "" && modelID != "" {
		model = &openCodeModel{
			ProviderID: providerID,
			ModelID:    modelID,
		}
	}

	allFailed := func(errMsg string) []translateResult {
		failed := make([]translateResult, 0, len(rows))
		for _, row := range rows {
			failed = append(failed, translateResult{Nomer: row.Nomer, Error: errMsg})
		}
		return failed
	}

	reply, err := translateWithRetryAgent(c.Context(), prompt, bulkTranslateAgent, model)
	if err != nil {
		log.Println("translate bulk opencode error:", err)
		return c.JSON(fiber.Map{
			"processed": len(rows),
			"updated":   0,
			"failed":    allFailed(err.Error()),
		})
	}

	translations, err := parseBulkTranslationReply(reply)
	if err != nil {
		log.Println("translate bulk parse error:", err)
		return c.JSON(fiber.Map{
			"processed": len(rows),
			"updated":   0,
			"failed":    allFailed(err.Error()),
		})
	}

	requested := make(map[uint]bool, len(rows))
	for _, row := range rows {
		requested[row.Nomer] = true
	}

	updated := 0
	failed := make([]translateResult, 0)

	for _, row := range rows {
		key := strconv.FormatUint(uint64(row.Nomer), 10)
		val, ok := translations[key]
		if !ok {
			log.Println("translate bulk missing key:", key)
			failed = append(failed, translateResult{Nomer: row.Nomer, Error: "missing key in AI response"})
			continue
		}
		if strings.TrimSpace(val) == "" {
			failed = append(failed, translateResult{Nomer: row.Nomer, Error: "empty AI response"})
			continue
		}

		if err := database.DB.Table(kitabName).
			Where("Nomer = ?", row.Nomer).
			Update("English", val).Error; err != nil {
			log.Println("translate bulk update error:", err)
			failed = append(failed, translateResult{Nomer: row.Nomer, Error: err.Error()})
			continue
		}
		updated++
	}

	for key := range translations {
		nomer, parseErr := strconv.ParseUint(key, 10, 64)
		if parseErr != nil || !requested[uint(nomer)] {
			log.Println("translate bulk ignoring unknown key in AI response:", key)
		}
	}

	return c.JSON(fiber.Map{
		"processed": len(rows),
		"updated":   updated,
		"failed":    failed,
	})
}
