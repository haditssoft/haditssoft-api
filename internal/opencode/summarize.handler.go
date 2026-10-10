package opencode

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/models"
)

// summarizeAgent produces a Markdown summary from an Arabic text plus a
// translation. Its instructions live in .opencode/agents/summarize.md.
const summarizeAgent = "summarize"

// summarySourceRow is the hadith row scanned for one summary batch: the id, the
// Arabic text and the source-language translation (aliased to Translation).
type summarySourceRow struct {
	Nomer       uint
	Arabic      *string
	Translation *string
}

// resolveSummaryLanguage reads and validates the ?language= query parameter
// against the summary languages, writing the 400 response itself on failure.
func resolveSummaryLanguage(c *fiber.Ctx) (string, bool) {
	raw := c.Query("language")
	if strings.TrimSpace(raw) == "" {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "language is required",
			"data":    nil,
		})
		return "", false
	}

	language, ok := models.IsSummaryLanguage(raw)
	if !ok {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "language must be one of: " + strings.Join(models.SummaryLanguages, ", "),
			"data":    nil,
		})
		return "", false
	}
	return language, true
}

// buildSummaryPrompt creates the prompt sent to the summarize agent. It gives
// the agent the hadith reference (so the Markdown heading is correct), the
// Arabic text, and the translation in the target language.
func buildSummaryPrompt(kitab string, nomer uint, language, arabic, translation string) string {
	var b strings.Builder
	b.WriteString("Kitab: ")
	b.WriteString(kitab)
	b.WriteString("\nNomer: ")
	b.WriteString(strconv.FormatUint(uint64(nomer), 10))
	b.WriteString("\n\nArabic:\n")
	b.WriteString(arabic)
	b.WriteString("\n\n")
	b.WriteString(language)
	b.WriteString(":\n")
	b.WriteString(translation)
	return b.String()
}

// summarizeTimeoutDuration returns the hard timeout applied to each AI
// summarization call. It reads OPENCODE_ASK_TIMEOUT_SEC (falling back to
// defaultAskTimeoutSec) and ignores invalid/zero values.
func summarizeTimeoutDuration() time.Duration {
	timeoutSec := defaultAskTimeoutSec
	if envSec := os.Getenv("OPENCODE_ASK_TIMEOUT_SEC"); envSec != "" {
		if parsed, err := strconv.Atoi(envSec); err == nil && parsed >= 1 {
			timeoutSec = parsed
		}
	}
	return time.Duration(timeoutSec) * time.Second
}

// runSummarizeSweep generates and stores summaries for one kitab in batches of
// limit records, one AI call per record. By default a single batch is
// processed; with sweepAll it keeps draining until no records with an empty
// target-language summary remain (batches advance by Nomer ASC and never resend
// an already-attempted number). maxBatches caps the number of batches
// (0 = unbounded).
func runSummarizeSweep(c *fiber.Ctx, kitab, language string, limit int, sweepAll bool, maxBatches int, model *openCodeModel) (bulkSweepResult, error) {
	result := bulkSweepResult{}
	attempted := make(map[uint]bool)
	batches := 0

	for {
		if maxBatches > 0 && batches >= maxBatches {
			break
		}

		var rows []summarySourceRow
		summaryTable := models.SummaryTable(kitab)
		q := database.DB.Table(kitab).
			Select("Nomer, Arabic, " + language + " AS Translation").
			Where("(" + language + " IS NOT NULL AND " + language + " != '')").
			Where("NOT EXISTS (SELECT 1 FROM \"" + summaryTable + "\" s WHERE s.Nomer = \"" + kitab + "\".Nomer AND s.\"" + language + "\" IS NOT NULL AND s.\"" + language + "\" != '')")
		if len(attempted) > 0 {
			keys := make([]uint, 0, len(attempted))
			for k := range attempted {
				keys = append(keys, k)
			}
			q = q.Where("Nomer NOT IN ?", keys)
		}
		if err := q.Order("Nomer ASC").Limit(limit).Find(&rows).Error; err != nil {
			return bulkSweepResult{}, err
		}

		if len(rows) == 0 {
			break
		}

		for _, row := range rows {
			attempted[row.Nomer] = true
		}

		for _, row := range rows {
			arabic := ""
			if row.Arabic != nil {
				arabic = *row.Arabic
			}
			translation := ""
			if row.Translation != nil {
				translation = *row.Translation
			}

			prompt := buildSummaryPrompt(kitab, row.Nomer, language, arabic, translation)

			ctx, cancel := context.WithTimeout(c.Context(), summarizeTimeoutDuration())
			reply, err := translateWithRetryAgent(ctx, prompt, summarizeAgent, model)
			cancel()
			if err != nil {
				log.Println("summarize opencode error:", err)
				result.Failed = append(result.Failed, translateResult{Nomer: row.Nomer, Error: err.Error()})
				continue
			}

			if strings.TrimSpace(reply) == "" {
				log.Println("summarize empty reply:", row.Nomer)
				result.Failed = append(result.Failed, translateResult{Nomer: row.Nomer, Error: "empty AI response"})
				continue
			}

			if err := models.SaveHadithSummary(kitab, row.Nomer, language, reply); err != nil {
				log.Println("summarize save error:", err)
				result.Failed = append(result.Failed, translateResult{Nomer: row.Nomer, Error: err.Error()})
				continue
			}
			result.Updated++
		}

		result.Processed += len(rows)
		batches++

		if !sweepAll {
			break
		}
	}

	return result, nil
}

// SummarizeHadiths generates and stores hadith summaries for one kitab into its
// dedicated <kitabName>Summary table, in the language given by ?language=. It
// is cron-only (guarded by OPENCODE_CRON_KEY) and processes one record per AI
// call. ?limit= controls the batch size; ?all=1 sweeps the whole table;
// ?maxBatches= caps the number of batches.
func SummarizeHadiths(c *fiber.Ctx) error {
	if !authorizeCronKey(c) {
		return nil
	}

	kitabName, ok := resolveKitabName(c)
	if !ok {
		return nil
	}

	language, ok := resolveSummaryLanguage(c)
	if !ok {
		return nil
	}

	limit, sweepAll, maxBatches, valid := parseBulkQueryParams(c)
	if !valid {
		return nil
	}

	loadTranslateConfig()

	model := buildTranslateModel()

	result, err := runSummarizeSweep(c, kitabName, language, limit, sweepAll, maxBatches, model)
	if err != nil {
		log.Println("summarize query error:", err)
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
