package opencode

import (
	"crypto/subtle"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/models"
)

const (
	defaultTranslateLimit    = 10
	defaultTranslateRetries  = 3
	defaultTranslateDelaySec = 30
)

var (
	translateRetryDelay = time.Duration(defaultTranslateDelaySec) * time.Second
	translateMaxRetries = defaultTranslateRetries
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

// permanentServerErrorPatterns lists substrings in server error messages that
// indicate a non-transient failure. Retrying these is pointless.
var permanentServerErrorPatterns = []string{
	"model not found",
	"Model not found",
	"ProviderModelNotFoundError",
	"unauthorized",
	"Unauthorized",
	"Invalid API key",
	"authentication",
	"Authentication",
	"invalid_api_key",
	"permission denied",
}

// isRetryableServerError returns true if the error is an opencode server error
// that may succeed on retry (transient failures like rate limits, endpoint down).
func isRetryableServerError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "opencode server error:") {
		return false
	}
	return !isPermanentServerError(msg)
}

// isPermanentServerError checks if the error message contains any pattern
// that indicates a non-retryable failure.
func isPermanentServerError(msg string) bool {
	lower := strings.ToLower(msg)
	for _, pattern := range permanentServerErrorPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// loadTranslateConfig reads retry settings from env vars, falling back to defaults.
func loadTranslateConfig() {
	if v := os.Getenv("OPENCODE_TRANSLATE_RETRY_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			translateMaxRetries = n
		}
	}
	if v := os.Getenv("OPENCODE_TRANSLATE_RETRY_DELAY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			translateRetryDelay = time.Duration(n) * time.Second
		}
	}
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

		prompt := buildTranslatePrompt(arabic, indonesia)

		reply, err := translateWithRetry(prompt, model)
		if err != nil {
			log.Println("translate opencode error:", err)
			failed = append(failed, translateResult{Nomer: row.Nomer, Error: err.Error()})
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

// buildTranslatePrompt creates the prompt sent to the translate agent.
// The translate agent instructions say: "You will be given a hadith in Arabic
// and a reference translation in Indonesian."
func buildTranslatePrompt(arabic, indonesia string) string {
	return "Arabic:\n" + arabic + "\n\nIndonesian:\n" + indonesia
}

// translateWithRetry calls opencode and retries on transient server errors
// (e.g. rate limit, endpoint unavailable). Permanent errors (model not found,
// auth failures) are returned immediately without retry.
func translateWithRetry(prompt string, model *openCodeModel) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= translateMaxRetries; attempt++ {
		reply, err := runOpenCodeCommand(prompt, "translate", model)
		if err == nil {
			if attempt > 1 {
				log.Printf("translate succeeded on attempt %d/%d\n", attempt, translateMaxRetries)
			}
			return reply, nil
		}
		lastErr = err
		if !isRetryableServerError(err) {
			return "", err
		}
		if attempt < translateMaxRetries {
			log.Printf("translate attempt %d/%d failed (retryable): %s, retrying in %v\n",
				attempt, translateMaxRetries, err, translateRetryDelay)
			time.Sleep(translateRetryDelay)
		}
	}
	return "", lastErr
}
