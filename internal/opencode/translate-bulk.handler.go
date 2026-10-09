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
	bulkTranslateAgent        = "translate-bulk"
	bulkTitleTranslateAgent   = "translate-title-bulk"
	bulkTranslateIDAgent      = "translate-bulk-id"
	bulkTitleTranslateIDAgent = "translate-title-bulk-id"
)

type bulkHadithInput struct {
	Arabic    string `json:"arabic"`
	Indonesia string `json:"indonesia"`
}

// bulkTranslateConfig describes a bulk translation target table: the columns to
// SELECT (a comma-separated list, aliased to Nomer/Arabic/Indonesia so it scans
// into translationRow), the row id column, the column the translation result is
// written back to, and the opencode agent to invoke. Both the hadith bulk
// endpoint and the Kitab/Bab title bulk endpoint share this engine. The target
// column is the "TargetColumn" (e.g. English or, for the Indonesian variant,
// the Bangla column Bengali/NKitabBen/NBabBen) used both as the sweep filter
// and the write-back destination.
type bulkTranslateConfig struct {
	Table        string
	IDColumn     string
	TargetColumn string
	Agent        string
	SelectClause string
}

// bulkBatchResult holds the outcome of one AI batch.
type bulkBatchResult struct {
	updated int
	failed  []translateResult
}

// bulkSweepResult holds the aggregated outcome of a full sweep over one table.
type bulkSweepResult struct {
	Processed int               `json:"processed"`
	Updated   int               `json:"updated"`
	Failed    []translateResult `json:"failed"`
}

// buildBulkTranslatePrompt marshals the rows into the JSON payload fed to the
// bulk agent. Each key is the hadith number (Nomer column) and each value holds
// the Arabic text plus the Indonesian reference translation (which the
// Indonesian variant re-translates into improved Indonesian).
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
// keyed JSON payload, sends it to the configured bulk agent, parses the JSON
// reply, and writes the translations back to the target table.
func processBulkBatch(ctx context.Context, cfg *bulkTranslateConfig, rows []translationRow, model *openCodeModel) bulkBatchResult {
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

	reply, err := translateWithRetryAgent(ctx, prompt, cfg.Agent, model)
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

		if err := database.DB.Table(cfg.Table).
			Where(cfg.IDColumn+" = ?", row.Nomer).
			Update(cfg.TargetColumn, val).Error; err != nil {
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

// runBulkTranslate sweeps cfg.Table in batches of limit untranslated rows. By
// default a single batch is processed; with sweepAll it keeps draining until no
// untranslated rows remain (batches advance by IDColumn ASC, never resending an
// already-attempted id). maxBatches caps the number of batches (0 = unbounded).
func runBulkTranslate(ctx context.Context, cfg *bulkTranslateConfig, limit int, sweepAll bool, maxBatches int, model *openCodeModel) (bulkSweepResult, error) {
	result := bulkSweepResult{}
	attempted := make(map[uint]bool)
	batches := 0

	for {
		if maxBatches > 0 && batches >= maxBatches {
			break
		}

		var rows []translationRow
		q := database.DB.Table(cfg.Table).
			Select(cfg.SelectClause).
			Where(cfg.TargetColumn + " IS NULL OR " + cfg.TargetColumn + " = ''")
		if len(attempted) > 0 {
			keys := make([]uint, 0, len(attempted))
			for k := range attempted {
				keys = append(keys, k)
			}
			q = q.Where(cfg.IDColumn+" NOT IN ?", keys)
		}
		if err := q.Order(cfg.IDColumn + " ASC").Limit(limit).Find(&rows).Error; err != nil {
			return bulkSweepResult{}, err
		}

		if len(rows) == 0 {
			break
		}

		for _, row := range rows {
			attempted[row.Nomer] = true
		}

		batch := processBulkBatch(ctx, cfg, rows, model)
		result.Processed += len(rows)
		result.Updated += batch.updated
		result.Failed = append(result.Failed, batch.failed...)
		batches++

		if !sweepAll {
			break
		}
	}

	return result, nil
}

// parseBulkQueryParams parses the limit/all/maxBatches query parameters shared
// by the bulk translate endpoints. On invalid input it writes the 400 response
// and returns ok=false.
func parseBulkQueryParams(c *fiber.Ctx) (limit int, sweepAll bool, maxBatches int, ok bool) {
	limit = defaultTranslateLimit
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status":  "error",
				"message": "limit must be a positive integer",
				"data":    nil,
			})
			return 0, false, 0, false
		}
		limit = parsed
	}

	sweepAll = false
	if raw := c.Query("all"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status":  "error",
				"message": "all must be a boolean",
				"data":    nil,
			})
			return 0, false, 0, false
		}
		sweepAll = parsed
	}

	maxBatches = 0
	if raw := c.Query("maxBatches"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status":  "error",
				"message": "maxBatches must be a positive integer",
				"data":    nil,
			})
			return 0, false, 0, false
		}
		maxBatches = parsed
	}

	return limit, sweepAll, maxBatches, true
}

// buildTranslateModel returns the opencode model from env when both provider
// and model id are set, or nil otherwise.
func buildTranslateModel() *openCodeModel {
	providerID := os.Getenv("OPENCODE_PROVIDER_ID")
	modelID := os.Getenv("OPENCODE_MODEL_ID")
	if providerID == "" || modelID == "" {
		return nil
	}
	return &openCodeModel{
		ProviderID: providerID,
		ModelID:    modelID,
	}
}

// hadithBulkConfig configures the bulk engine for a hadith table
// (Nomer/Arabic/Indonesia -> the English column).
func hadithBulkConfig(kitabName string) *bulkTranslateConfig {
	return &bulkTranslateConfig{
		Table:        kitabName,
		IDColumn:     "Nomer",
		TargetColumn: "English",
		Agent:        bulkTranslateAgent,
		SelectClause: "Nomer, Arabic, Indonesia",
	}
}

// hadithBulkIDConfig configures the bulk engine for a hadith table for the
// Indonesian variant: the Arabic is authoritative and the existing Indonesian
// is the reference to re-translate. The improved Indonesian result is written
// to the Bengali (Bangla) column for now, so the frontend Bangla language
// switch shows it while real Urdu/Bangla translations are not ready.
func hadithBulkIDConfig(kitabName string) *bulkTranslateConfig {
	return &bulkTranslateConfig{
		Table:        kitabName,
		IDColumn:     "Nomer",
		TargetColumn: "Bengali",
		Agent:        bulkTranslateIDAgent,
		SelectClause: "Nomer, Arabic, Indonesia",
	}
}

// translateHadithsBulk runs the shared bulk engine for the hadith table of a
// kitab using the given config builder. With ?all=1 it sweeps the whole table
// in batches of ?limit= rows until no rows with an empty target column remain.
// ?maxBatches= caps the number of batches.
func translateHadithsBulk(c *fiber.Ctx, buildConfig func(string) *bulkTranslateConfig) error {
	if !authorizeCronKey(c) {
		return nil
	}

	kitabName, ok := resolveKitabName(c)
	if !ok {
		return nil
	}

	limit, sweepAll, maxBatches, valid := parseBulkQueryParams(c)
	if !valid {
		return nil
	}

	loadTranslateConfig()

	model := buildTranslateModel()

	result, err := runBulkTranslate(c.Context(), buildConfig(kitabName), limit, sweepAll, maxBatches, model)
	if err != nil {
		log.Println("translate bulk query error:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to query hadith records",
			"data":    nil,
		})
	}

	return c.JSON(fiber.Map{
		"processed": result.Processed,
		"updated":   result.Updated,
		"failed":    result.Failed,
	})
}

// TranslateHadithsBulk translates several hadiths into English in a single AI
// call by default. With ?all=1 it sweeps the whole table in batches of ?limit=
// rows until no untranslated rows remain. ?maxBatches= caps the number of
// batches.
func TranslateHadithsBulk(c *fiber.Ctx) error {
	return translateHadithsBulk(c, hadithBulkConfig)
}

// TranslateHadithsBulkID translates several hadiths into improved Indonesian in
// a single AI call by default, re-translating the existing Indonesian reference
// from the Arabic. The result is written to the Bengali (Bangla) column. It
// honors the same ?all=1/?limit=/?maxBatches= rules as TranslateHadithsBulk.
func TranslateHadithsBulkID(c *fiber.Ctx) error {
	return translateHadithsBulk(c, hadithBulkIDConfig)
}
