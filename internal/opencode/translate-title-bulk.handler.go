package opencode

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

// kitabTitleBulkConfig configures the bulk engine for the Kitab<kitabName>
// table: the Arabic source is the NKitabArab column, the Indonesian reference
// is NKitab, and the English result is written to NKitabEng.
func kitabTitleBulkConfig(kitabName string) *bulkTranslateConfig {
	return &bulkTranslateConfig{
		Table:        "Kitab" + kitabName,
		IDColumn:     "VMember",
		TargetColumn: "NKitabEng",
		Agent:        bulkTitleTranslateAgent,
		SelectClause: "VMember AS Nomer, NKitabArab AS Arabic, NKitab AS Indonesia",
	}
}

// babTitleBulkConfig configures the bulk engine for the Bab<kitabName> table:
// the Arabic source is the NBabArab column, the Indonesian reference is NBab,
// and the English result is written to NBabEng.
func babTitleBulkConfig(kitabName string) *bulkTranslateConfig {
	return &bulkTranslateConfig{
		Table:        "Bab" + kitabName,
		IDColumn:     "VMemberBab",
		TargetColumn: "NBabEng",
		Agent:        bulkTitleTranslateAgent,
		SelectClause: "VMemberBab AS Nomer, NBabArab AS Arabic, NBab AS Indonesia",
	}
}

// kitabTitleBulkIDConfig configures the bulk engine for the Kitab<kitabName>
// table for the Indonesian variant: the Arabic source is the NKitabArab column,
// the existing Indonesian reference is NKitab (what gets re-translated), and
// the improved Indonesian result is written to the NKitabBen (Bangla) column.
func kitabTitleBulkIDConfig(kitabName string) *bulkTranslateConfig {
	return &bulkTranslateConfig{
		Table:        "Kitab" + kitabName,
		IDColumn:     "VMember",
		TargetColumn: "NKitabBen",
		Agent:        bulkTitleTranslateIDAgent,
		SelectClause: "VMember AS Nomer, NKitabArab AS Arabic, NKitab AS Indonesia",
	}
}

// babTitleBulkIDConfig configures the bulk engine for the Bab<kitabName> table
// for the Indonesian variant: the Arabic source is the NBabArab column, the
// existing Indonesian reference is NBab (what gets re-translated), and the
// improved Indonesian result is written to the NBabBen (Bangla) column.
func babTitleBulkIDConfig(kitabName string) *bulkTranslateConfig {
	return &bulkTranslateConfig{
		Table:        "Bab" + kitabName,
		IDColumn:     "VMemberBab",
		TargetColumn: "NBabBen",
		Agent:        bulkTitleTranslateIDAgent,
		SelectClause: "VMemberBab AS Nomer, NBabArab AS Arabic, NBab AS Indonesia",
	}
}

// translateTitlesBulk bulk-translates the book (Kitab) and/or chapter (Bab)
// title tables of a kitab in a single request using the given config builders.
// ?type= selects which tables to translate: kitab, bab, or both (default). Each
// selected table is swept independently with the same ?limit=/?all=/?maxBatches=
// rules as the hadith bulk endpoint. The response aggregates both tables and
// also reports a per-table breakdown so failed row ids cannot be misattributed.
func translateTitlesBulk(c *fiber.Ctx, buildKitabConfig, buildBabConfig func(string) *bulkTranslateConfig) error {
	if !authorizeCronKey(c) {
		return nil
	}

	kitabName, ok := resolveKitabName(c)
	if !ok {
		return nil
	}

	titleType := c.Query("type", "both")
	switch titleType {
	case "kitab", "bab", "both":
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "type must be one of: kitab, bab, both",
			"data":    nil,
		})
	}

	limit, sweepAll, maxBatches, valid := parseBulkQueryParams(c)
	if !valid {
		return nil
	}

	loadTranslateConfig()

	model := buildTranslateModel()

	results := make(map[string]bulkSweepResult, 2)

	runTable := func(cfg *bulkTranslateConfig, name string) error {
		if titleType != "both" && titleType != name {
			results[name] = bulkSweepResult{}
			return nil
		}
		result, err := runBulkTranslate(c.Context(), cfg, limit, sweepAll, maxBatches, model)
		if err != nil {
			return err
		}
		results[name] = result
		return nil
	}

	if err := runTable(buildKitabConfig(kitabName), "kitab"); err != nil {
		log.Println("translate title bulk kitab query error:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to query kitab title records",
			"data":    nil,
		})
	}

	if err := runTable(buildBabConfig(kitabName), "bab"); err != nil {
		log.Println("translate title bulk bab query error:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to query bab title records",
			"data":    nil,
		})
	}

	kitab := results["kitab"]
	bab := results["bab"]

	failed := make([]translateResult, 0, len(kitab.Failed)+len(bab.Failed))
	failed = append(failed, kitab.Failed...)
	failed = append(failed, bab.Failed...)

	return c.JSON(fiber.Map{
		"processed": kitab.Processed + bab.Processed,
		"updated":   kitab.Updated + bab.Updated,
		"failed":    failed,
		"kitab":     kitab,
		"bab":       bab,
	})
}

// TranslateTitlesBulk bulk-translates the book (Kitab) and/or chapter (Bab)
// title tables of a kitab into English in a single request. ?type= selects
// which tables to translate: kitab, bab, or both (default). Each selected table
// uses the same ?limit=/?all=/?maxBatches= rules as the hadith bulk endpoint.
func TranslateTitlesBulk(c *fiber.Ctx) error {
	return translateTitlesBulk(c, kitabTitleBulkConfig, babTitleBulkConfig)
}

// TranslateTitlesBulkID bulk-translates the book (Kitab) and/or chapter (Bab)
// title tables of a kitab into improved Indonesian in a single request,
// re-translating the existing Indonesian reference from the Arabic. The results
// are written to the NKitabBen/NBabBen (Bangla) columns. It honors the same
// ?type=/?limit=/?all=/?maxBatches= rules as TranslateTitlesBulk.
func TranslateTitlesBulkID(c *fiber.Ctx) error {
	return translateTitlesBulk(c, kitabTitleBulkIDConfig, babTitleBulkIDConfig)
}
