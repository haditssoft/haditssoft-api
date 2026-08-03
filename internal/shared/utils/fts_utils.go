package utils

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

func QueryCombination(lengthy int) []string {
	switch lengthy {
	case 1:
		return []string{"0"}
	case 2:
		return []string{"01"}
	case 3:
		return []string{"012", "01", "02", "12"}
	case 4:
		return []string{"0123", "012", "013", "023", "123"}
	case 5:
		return []string{
			"01234", "0124", "0134", "0234", "1234",
			"0123", "012", "013", "023", "123",
		}
	default:
		return generateCombos(lengthy)
	}
}

func generateCombos(n int) []string {
	digits := make([]byte, n)
	for i := 0; i < n; i++ {
		digits[i] = byte('0' + i)
	}

	var result []string

	if n <= 6 {
		result = append(result, string(digits))
	}

	for drop := 1; drop <= 3; drop++ {
		generateDrops(digits, drop, 0, []byte{}, &result)
	}

	return result
}

func generateDrops(digits []byte, toDrop, start int, current []byte, result *[]string) {
	if toDrop == 0 {
		var combo []byte
		j := 0
		for i := 0; i < len(digits); i++ {
			if j < len(current) && current[j] == digits[i] {
				j++
				continue
			}
			combo = append(combo, digits[i])
		}
		*result = append(*result, string(combo))
		return
	}
	for i := start; i < len(digits); i++ {
		generateDrops(digits, toDrop-1, i+1, append(current, digits[i]), result)
	}
}

func cartesianProduct(arrays ...[]string) []string {
	if len(arrays) == 0 {
		return nil
	}
	result := arrays[0]
	for i := 1; i < len(arrays); i++ {
		var next []string
		for _, a := range result {
			for _, b := range arrays[i] {
				next = append(next, a+" "+b)
			}
		}
		result = next
	}
	return result
}

func joiner(arr []string, distance string) string {
	if len(arr) == 0 {
		return ""
	}
	middle := strings.Join(arr, ", "+distance+") OR NEAR(")
	return "NEAR(" + middle + ", " + distance + ")"
}

func CombineValue(arrays ...[]string) string {
	var queryWords []string
	switch len(arrays) {
	case 0:
		return ""
	case 1:
		queryWords = arrays[0]
	default:
		queryWords = cartesianProduct(arrays...)
	}

	restCount := len(arrays) - 2
	distance := "4"
	if restCount <= 1 {
		distance = "10"
	}
	return joiner(queryWords, distance)
}

func escapeFTSKeyword(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "''", "'"), "'", "''")
}

func GetFirstNSecondRank(reqID int64, column, aliasName, ftsTable string, originalKeywords []string, DB *gorm.DB) ([]map[string]interface{}, error) {
	exactPhrase := escapeFTSKeyword(fmt.Sprintf(`"%s"*`, strings.Join(originalKeywords, " ")))
	firstRankSQL := fmt.Sprintf(
		`SELECT Nomer AS "%s" FROM "%s" WHERE %s MATCH '%s' ORDER BY rank`,
		aliasName, ftsTable, column, exactPhrase,
	)
	firstRank, err := LoggedQueryNoParams(reqID, "FIRST_RANK", firstRankSQL, DB)
	if err != nil {
		return nil, err
	}

	var quoted []string
	for _, w := range originalKeywords {
		quoted = append(quoted, escapeFTSKeyword(fmt.Sprintf(`"%s"*`, w)))
	}
	nearExpr := fmt.Sprintf("NEAR(%s, 2)", strings.Join(quoted, " "))
	secondRankSQL := fmt.Sprintf(
		`SELECT Nomer AS "%s" FROM "%s" WHERE %s MATCH '%s' ORDER BY rank`,
		aliasName, ftsTable, column, nearExpr,
	)
	secondRank, err := LoggedQueryNoParams(reqID, "SECOND_RANK", secondRankSQL, DB)
	if err != nil {
		return nil, err
	}

	merged := append(firstRank, secondRank...)
	return DedupByAlias(merged, aliasName), nil
}

func ExecSimpleLike(reqID int64, tableName, aliasName, column string, keywords []string, DB *gorm.DB) ([]map[string]interface{}, error) {
	conditions := make([]string, len(keywords))
	args := make([]interface{}, len(keywords))
	for i, kw := range keywords {
		conditions[i] = fmt.Sprintf(`"%s" LIKE ?`, column)
		args[i] = "%" + strings.ReplaceAll(kw, "''", "'") + "%"
	}
	sql := fmt.Sprintf(
		`SELECT Nomer AS "%s" FROM "%s" WHERE %s ORDER BY "%s" LIMIT 50`,
		aliasName, tableName, strings.Join(conditions, " AND "), aliasName,
	)
	result, err := LoggedQuery(reqID, "LIKE_FALLBACK", sql, args, DB)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DedupByAlias(rows []map[string]interface{}, aliasName string) []map[string]interface{} {
	seen := make(map[string]bool)
	var result []map[string]interface{}
	for _, row := range rows {
		key := fmt.Sprint(row[aliasName])
		if !seen[key] {
			seen[key] = true
			result = append(result, row)
		}
	}
	return result
}
