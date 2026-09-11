package opencode

import (
	"context"
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

// bulkBatchResult holds the outcome of one AI batch.
type bulkBatchResult struct {
	updated int
	failed  []translateResult
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

// processBulkBatch translates one batch of rows in a single AI call: builds the
// keyed JSON payload, sends it to the translate-bulk agent, parses the JSON
// reply, and writes the English translations back to the database.
func processBulkBatch(ctx context.Context, kitabName string, rows []translationRow, model *openCodeModel) bulkBatchResult {
	allFailed := func(errMsg string) []translateResult {
		failed := make([]translateResult, 0, len(rows))
		for _, row := range rows {
			failed = append(failed, translateResult{Nomer: row.Nomer, Error: errMsg})
		}
		return failed
	}

	if len(rows) == 0 {
		return bulkBatchResult{}
	}

	prompt, err := buildBulkTranslatePrompt(rows)
	if err != nil {
		log.Println("translate bulk prompt error:", err)
		return bulkBatchResult{failed: allFailed(err.Error())}
	}

	reply, err := translateWithRetryAgent(ctx, prompt, bulkTranslateAgent, model)
	if err != nil {
		log.Println("translate bulk opencode error:", err)
		return bulkBatchResult{failed: allFailed(err.Error())}
	}

	translations, err := parseBulkTranslationReply(reply)
	if err != nil {
		log.Println("translate bulk parse error:", err)
		return bulkBatchResult{failed: allFailed(err.Error())}
	}

	requested := make(map[uint]bool, len(rows))
	for _, row := range rows {
		requested[row.Nomer] = true
	}

	result := bulkBatchResult{}

	for _, row := range rows {
		key := strconv.FormatUint(uint64(row.Nomer), 10)
		val, ok := translations[key]
		if !ok {
			log.Println("translate bulk missing key:", key)
			result.failed = append(result.failed, translateResult{Nomer: row.Nomer, Error: "missing key in AI response"})
			continue
		}
		if strings.TrimSpace(val) == "" {
			result.failed = append(result.failed, translateResult{Nomer: row.Nomer, Error: "empty AI response"})
			continue
		}

		if err := database.DB.Table(kitabName).
			Where("Nomer = ?", row.Nomer).
			Update("English", val).Error; err != nil {
			log.Println("translate bulk update error:", err)
			result.failed = append(result.failed, translateResult{Nomer: row.Nomer, Error: err.Error()})
			continue
		}
		result.updated++
	}

	for key := range translations {
		nomer, parseErr := strconv.ParseUint(key, 10, 64)
		if parseErr != nil || !requested[uint(nomer)] {
			log.Println("translate bulk ignoring unknown key in AI response:", key)
		}
	}

	return result
}

// TranslateHadithsBulk translates several hadiths in a single AI call by
// default. With ?all=1 it sweeps the whole table in batches of ?limit= rows
// until no untranslated rows remain. ?maxBatches= caps the number of batches.
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

	sweepAll := false
	if raw := c.Query("all"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status":  "error",
				"message": "all must be a boolean",
				"data":    nil,
			})
		}
		sweepAll = parsed
	}

	maxBatches := 0
	if raw := c.Query("maxBatches"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status":  "error",
				"message": "maxBatches must be a positive integer",
				"data":    nil,
			})
		}
		maxBatches = parsed
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

	attempted := make(map[uint]bool)
	totalProcessed := 0
	totalUpdated := 0
	totalFailed := make([]translateResult, 0)
	batches := 0

	for {
		if maxBatches > 0 && batches >= maxBatches {
			break
		}

		var rows []translationRow
		q := database.DB.Table(kitabName).
			Select("Nomer", "Arabic", "Indonesia").
			Where("English IS NULL OR English = ''")
		if len(attempted) > 0 {
			keys := make([]uint, 0, len(attempted))
			for k := range attempted {
				keys = append(keys, k)
			}
			q = q.Where("Nomer NOT IN ?", keys)
		}
		if err := q.Order("Nomer ASC").Limit(limit).Find(&rows).Error; err != nil {
			log.Println("translate bulk query error:", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": "failed to query hadith records",
				"data":    nil,
			})
		}

		if len(rows) == 0 {
			break
		}

		for _, row := range rows {
			attempted[row.Nomer] = true
		}

		result := processBulkBatch(c.Context(), kitabName, rows, model)
		totalProcessed += len(rows)
		totalUpdated += result.updated
		totalFailed = append(totalFailed, result.failed...)
		batches++

		if !sweepAll {
			break
		}
	}

	return c.JSON(fiber.Map{
		"processed": totalProcessed,
		"updated":   totalUpdated,
		"failed":    totalFailed,
	})
}
