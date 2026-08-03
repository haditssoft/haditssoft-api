package search

import (
	"fmt"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/internal/shared/utils"
	"github.com/haditssoft/haditssoft-backend/models"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type searchRequest struct {
	Keyword []string `json:"keyword"`
	Books   []string `json:"books"`
}

func GetSearchHadith(c *fiber.Ctx) error {
	reqID := utils.NextRequestID()
	clientIP := c.IP()

	var req searchRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}
	if len(req.Keyword) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "keyword is required",
		})
	}

	originalKeywords := req.Keyword
	qwery := strings.Join(originalKeywords, "-")

	if entry, exists := utils.GetIPEntry(clientIP); exists {
		if entry.Keywords != qwery {
			entry.Keywords = qwery
			entry.Attempt++
			utils.SetIPEntry(clientIP, entry)
		}
	} else {
		utils.SetIPEntry(clientIP, utils.IpEntry{Keywords: qwery, Attempt: 1})
	}

	kitabName := c.Params("kitabName")
	column := c.Params("column")

	aliasName := models.GetIndexOfKitab[kitabName]
	if aliasName == "" {
		aliasName = "0"
	}

	filtered := filterKeywords(originalKeywords)
	if len(filtered) == 0 {
		filtered = originalKeywords
	}
	utils.LogSearchStart(reqID, kitabName, column, originalKeywords, filtered)

	var rows []map[string]interface{}
	var err error

	switch {
	case len(originalKeywords) == 1:
		err = singleKeywordSearch(reqID, kitabName, column, aliasName, originalKeywords[0], &rows)

	default:
		if column == "Indonesia" && len(filtered) > 1 {
			err = indonesiaFTSearch(reqID, kitabName, column, aliasName, originalKeywords, filtered, &rows)
		} else if len(filtered) <= 1 {
			kw := originalKeywords[0]
			if len(filtered) > 0 {
				kw = filtered[0]
			}
			err = singleKeywordSearch(reqID, kitabName, column, aliasName, kw, &rows)
		} else {
			err = multiKeywordLikeSearch(reqID, kitabName, column, aliasName, originalKeywords, &rows)
		}
	}

	if err != nil {
		utils.LogSearchResult(reqID, 0)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if len(rows) == 0 {
		rows = []map[string]interface{}{}
	}

	utils.LogSearchResult(reqID, len(rows))
	return c.JSON([]interface{}{rows, "SEARCHRESULTCOUNT", kitabName})
}

func filterKeywords(keywords []string) []string {
	seen := make(map[string]bool)
	var filtered []string
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		if kw == "" || seen[kw] {
			continue
		}
		seen[kw] = true
		isCon := false
		for _, c := range models.Conjunction {
			if kw == c {
				isCon = true
				break
			}
		}
		if !isCon {
			filtered = append(filtered, kw)
		}
	}
	return filtered
}

func normalizeKeyword(kw string) string {
	return strings.ReplaceAll(kw, "''", "'")
}

func singleKeywordSearch(reqID int64, kitabName, column, aliasName, keyword string, rows *[]map[string]interface{}) error {
	sql := fmt.Sprintf(`SELECT Nomer AS "%s" FROM "%s" WHERE "%s" LIKE ? ORDER BY "%s"`, aliasName, kitabName, column, aliasName)
	result, err := utils.LoggedQuery(reqID, "LIKE_SINGLE", sql, []interface{}{"%" + normalizeKeyword(keyword) + "%"}, database.DB)
	if err != nil {
		return err
	}
	*rows = result
	return nil
}

func multiKeywordLikeSearch(reqID int64, kitabName, column, aliasName string, keywords []string, rows *[]map[string]interface{}) error {
	conditions := make([]string, len(keywords))
	args := make([]interface{}, len(keywords))
	for i, kw := range keywords {
		conditions[i] = fmt.Sprintf(`"%s" LIKE ?`, column)
		args[i] = "%" + normalizeKeyword(kw) + "%"
	}
	sql := fmt.Sprintf(`SELECT Nomer AS "%s" FROM "%s" WHERE %s ORDER BY "%s"`, aliasName, kitabName, strings.Join(conditions, " AND "), aliasName)
	result, err := utils.LoggedQuery(reqID, "LIKE_MULTI", sql, args, database.DB)
	if err != nil {
		return err
	}
	*rows = result
	return nil
}

func indonesiaFTSearch(reqID int64, kitabName, column, aliasName string, originalKeywords, filtered []string, rows *[]map[string]interface{}) error {
	if len(filtered) > 7 {
		filtered = filtered[:7]
	}

	limitChars := 1001
	switch len(filtered) {
	case 2, 3:
		limitChars = 1001
	case 4:
		limitChars = 1501
	case 5:
		limitChars = 2001
	case 6:
		limitChars = 2501
	case 7:
		limitChars = 3001
	default:
		limitChars = 999999
	}

	ftsTable := "FTS" + kitabName
	tagTblCheck := "FTS_TABLE_CHECK"
	_ = tagTblCheck
	start := time.Now()
	var tblCount int64
	database.DB.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", ftsTable).Scan(&tblCount)
	utils.LogQuery(reqID, "FTS_TABLE_CHECK", "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", []interface{}{ftsTable}, 0, nil, start)

	if tblCount == 0 {
		return multiKeywordLikeSearch(reqID, kitabName, column, aliasName, originalKeywords, rows)
	}

	firstNSecondRank, err := utils.GetFirstNSecondRank(reqID, column, aliasName, ftsTable, originalKeywords, database.DB)
	if err != nil {
		return err
	}

	queryToKey := strings.Join(filtered, "")
	var whateverRank []map[string]interface{}

	if cached, ok := utils.FtsCache.Get(queryToKey); ok {
		tag := "FTS_WHATEVER_CACHED"
		sql := fmt.Sprintf(`SELECT Nomer AS "%s" FROM "%s" WHERE %s ORDER BY rank`, aliasName, ftsTable, cached.(string))
		whateverRank, err = utils.LoggedQueryNoParams(reqID, tag, sql, database.DB)
		if err != nil {
			whateverRank = nil
		}
	}

	if whateverRank == nil {
		var joinedQuery string
		var builtFTSQuery string
		var words [][]string

		combinedWords, kwErr := utils.GetOtherKeywords(reqID, filtered, database.DB)
		if kwErr == nil && combinedWords != nil {
			words = utils.ExtractWordVariants(filtered, combinedWords)
			if len(words) > 0 {
				for _, combo := range utils.QueryCombination(len(filtered)) {
					var selected [][]string
					for _, ch := range combo {
						idx := int(ch - '0')
						if idx < len(words) {
							selected = append(selected, words[idx])
						}
					}
					if len(selected) < 2 {
						continue
					}
					if joinedQuery != "" {
						joinedQuery += " OR "
					}
					joinedQuery += utils.CombineValue(selected...)
				}
			}
		}

		if joinedQuery != "" {
			ftsQuery := fmt.Sprintf(`%s MATCH '%s' AND length(%s) < %d`, column, joinedQuery, column, limitChars)
			builtFTSQuery = ftsQuery
			utils.FtsCache.Set(queryToKey, ftsQuery)

			sql := fmt.Sprintf(`SELECT Nomer AS "%s" FROM "%s" WHERE %s ORDER BY rank`, aliasName, ftsTable, ftsQuery)
			whateverRank, err = utils.LoggedQueryNoParams(reqID, "FTS_WHATEVER", sql, database.DB)
			if err != nil {
				whateverRank = nil
			}
		}

		if joinedQuery != "" {
			logSearchFTSBuilt(reqID, builtFTSQuery, len(words))
		}
	}

	if whateverRank == nil {
		whateverRank = []map[string]interface{}{}
	}

	merged := append(firstNSecondRank, whateverRank...)
	*rows = utils.DedupByAlias(merged, aliasName)

	const threshold = 10
	if len(*rows) < threshold {
		fallback, fbErr := utils.ExecSimpleLike(reqID, kitabName, aliasName, "Indonesia", filtered, database.DB)
		if fbErr == nil && len(fallback) > 0 {
			if len(fallback) > 50 {
				fallback = fallback[:50]
			}
			merged := append(*rows, fallback...)
			*rows = utils.DedupByAlias(merged, aliasName)
		}
	}

	return nil
}

func searchOneKitab(reqID int64, kitabName, column, aliasName string, originalKeywords, filtered []string) ([]map[string]interface{}, error) {
	var rows []map[string]interface{}
	var err error

	switch {
	case len(originalKeywords) == 1:
		err = singleKeywordSearch(reqID, kitabName, column, aliasName, originalKeywords[0], &rows)

	default:
		if column == "Indonesia" && len(filtered) > 1 {
			err = indonesiaFTSearch(reqID, kitabName, column, aliasName, originalKeywords, filtered, &rows)
		} else if len(filtered) <= 1 {
			kw := originalKeywords[0]
			if len(filtered) > 0 {
				kw = filtered[0]
			}
			err = singleKeywordSearch(reqID, kitabName, column, aliasName, kw, &rows)
		} else {
			err = multiKeywordLikeSearch(reqID, kitabName, column, aliasName, originalKeywords, &rows)
		}
	}

	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		rows = []map[string]interface{}{}
	}

	return rows, nil
}

func SearchHadithAll(c *fiber.Ctx) error {
	reqID := utils.NextRequestID()
	clientIP := c.IP()

	var req searchRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}
	if len(req.Keyword) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "keyword is required",
		})
	}
	if len(req.Books) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "books is required",
		})
	}

	originalKeywords := req.Keyword
	qwery := strings.Join(originalKeywords, "-")

	if entry, exists := utils.GetIPEntry(clientIP); exists {
		if entry.Keywords != qwery {
			entry.Keywords = qwery
			entry.Attempt++
			utils.SetIPEntry(clientIP, entry)
		}
	} else {
		utils.SetIPEntry(clientIP, utils.IpEntry{Keywords: qwery, Attempt: 1})
	}

	column := c.Params("column")

	filtered := filterKeywords(originalKeywords)
	if len(filtered) == 0 {
		filtered = originalKeywords
	}

	kitabList := req.Books

	type kitabResult struct {
		KitabName string
		AliasName string
		Rows      []map[string]interface{}
		Err       error
	}

	ch := make(chan kitabResult, len(kitabList))

	for _, kitabName := range kitabList {
		aliasName, isValid := models.GetIndexOfKitab[kitabName]
		if !isValid {
			go func(kn string) {
				ch <- kitabResult{KitabName: kn, AliasName: "0", Rows: []map[string]interface{}{}, Err: fmt.Errorf("unknown kitab: %s", kn)}
			}(kitabName)
			continue
		}

		utils.LogSearchStart(reqID, kitabName, column, originalKeywords, filtered)

		go func(kn, an string) {
			rows, err := searchOneKitab(reqID, kn, column, an, originalKeywords, filtered)
			ch <- kitabResult{KitabName: kn, AliasName: an, Rows: rows, Err: err}
		}(kitabName, aliasName)
	}

	results := make(map[string]interface{}, len(kitabList))
	total := 0

	for range kitabList {
		res := <-ch
		if res.Err != nil {
			utils.LogSearchResult(reqID, 0)
			results[res.KitabName] = fiber.Map{"rows": []map[string]interface{}{}, "count": 0}
			continue
		}
		count := len(res.Rows)
		utils.LogSearchResult(reqID, count)
		total += count
		results[res.KitabName] = fiber.Map{"rows": res.Rows, "count": count}
	}

	utils.LogSearchResult(reqID, total)
	return c.JSON(fiber.Map{
		"results": results,
		"total":   total,
	})
}

func logSearchFTSBuilt(reqID int64, ftsQuery string, wordCount int) {
	start := time.Now()
	utils.LogMu.Lock()
	if utils.LogWriter != nil {
		utils.LogWriter.Printf(
			"%s\t%d\t%s\t%s\t%s\t%d\t%d\t%s",
			start.Format(time.RFC3339Nano),
			reqID,
			"FTS_QUERY_BUILT",
			ftsQuery,
			fmt.Sprintf("words=%d", wordCount),
			0, 0, "",
		)
	}
	utils.LogMu.Unlock()
}
