package utils

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

var wordVariantRegex = regexp.MustCompile(`\[(\D*?)\]`)
var nonAlnumRegex = regexp.MustCompile(`[^A-Za-z0-9]`)

type kbbiRow struct {
	Katakunci string
	Artikata  string
}

type kbbiQuery struct {
	sql  string
	args []interface{}
}

func buildStringQueries(keywrd string) []kbbiQuery {
	base := `SELECT katakunci, artikata FROM KBBI WHERE `
	return []kbbiQuery{
		{sql: base + `katakunci = ?`, args: []interface{}{keywrd}},
		{sql: base + `katakunci LIKE ? AND artikata LIKE ?`, args: []interface{}{"%" + keywrd + "]%", "%" + keywrd + "]%"}},
		{sql: base + `artikata LIKE ?`, args: []interface{}{"%" + keywrd + "]%"}},
	}
}

func buildArrayQueries(keywrd []string) []kbbiQuery {
	out := make([]kbbiQuery, 0, 6)
	base := `SELECT katakunci, artikata FROM KBBI WHERE `
	idx := []int{0, 1}
	if len(keywrd) == 1 {
		idx = []int{0, 0}
	}
	for _, i := range idx {
		out = append(out, kbbiQuery{sql: base + `katakunci = ?`, args: []interface{}{keywrd[i]}})
	}
	for _, i := range idx {
		out = append(out, kbbiQuery{
			sql:  base + `katakunci LIKE ? AND artikata LIKE ?`,
			args: []interface{}{"%" + keywrd[i] + "]%", "%" + keywrd[i] + "]%"},
		})
	}
	for _, i := range idx {
		out = append(out, kbbiQuery{sql: base + `artikata LIKE ?`, args: []interface{}{"%" + keywrd[i] + "]%"}})
	}
	return out
}

func fetchKeywords(reqID int64, queries []kbbiQuery, DB *gorm.DB) ([]kbbiRow, error) {
	for i, q := range queries {
		tag := fmt.Sprintf("KBBI_QUERY_%d", i)
		start := time.Now()
		var rows []kbbiRow
		err := DB.Raw(q.sql, q.args...).Scan(&rows).Error
		rowCount := len(rows)
		LogQuery(reqID, tag, q.sql, q.args, rowCount, err, start)
		if err != nil {
			continue
		}
		if len(rows) > 0 {
			return rows, nil
		}
	}
	return nil, nil
}

func GetOtherKeywords(reqID int64, initialKeywords []string, DB *gorm.DB) (map[string]string, error) {
	combinedWords := make(map[string]string)

	for _, kw := range initialKeywords {
		base := strings.ToLower(kw)

		var queries []kbbiQuery

		switch {
		case strings.HasPrefix(base, "di"):
			base = base[2:]
		case strings.HasPrefix(base, "mem"):
			base = base[3:]
		case strings.HasPrefix(base, "meng"):
			base = base[4:]
		case strings.HasPrefix(base, "meny"):
			suffix := base[4:]
			arr := []string{"s" + suffix, "c" + suffix}
			doLoop := make([]string, 2)
			copy(doLoop, arr)

			processed := make([]string, 0, 2)
			for _, wv := range arr {
				if strings.HasSuffix(wv, "hi") {
					processed = append(processed, wv[:len(wv)-1])
				}
			}
			if len(processed) == 0 {
				processed = doLoop
			}
			if len(processed) < 2 {
				processed = append(processed, processed[0])
			}

			queries = buildArrayQueries(processed)
		case strings.HasPrefix(base, "men"):
			base = base[3:]
		case strings.HasPrefix(base, "me"):
			base = base[2:]
		case strings.HasPrefix(base, "ber"):
			base = base[3:]
		case strings.HasPrefix(base, "ter"):
			base = base[3:]
		}

		if queries == nil {
			if strings.HasSuffix(base, "hi") {
				base = base[:len(base)-1]
			}
			queries = buildStringQueries(base)
		}

		rows, err := fetchKeywords(reqID, queries, DB)
		if err != nil {
			continue
		}

		for _, row := range rows {
			if existing, ok := combinedWords[kw]; ok {
				combinedWords[kw] = existing + " " + row.Artikata
			} else {
				combinedWords[kw] = row.Artikata
			}
		}
	}

	return combinedWords, nil
}

func ExtractWordVariants(keyName []string, combinedWords map[string]string) [][]string {
	var words [][]string

	for _, kw := range keyName {
		wordData, ok := combinedWords[kw]
		if !ok || wordData == "" {
			wordData = "[" + kw + "]"
		}

		matches := wordVariantRegex.FindAllString(wordData, -1)
		if matches == nil {
			matches = []string{"[" + kw + "]"}
		}

		var batch []string
		for _, raw := range matches {
			eachWord := strings.NewReplacer("[", "", "]", "").Replace(raw)
			if len(eachWord) == 1 || strings.Contains(eachWord, ")") ||
				strings.HasPrefix(eachWord, " ") || strings.HasPrefix(eachWord, "-") {
				continue
			}
			if nonAlnumRegex.MatchString(eachWord) {
				eachWord = fmt.Sprintf(`"%s"`, eachWord)
			}
			batch = append(batch, escapeFTSKeyword(eachWord)+"*")
		}

		if len(batch) > 0 {
			addKey := kw + "*"
			if strings.Contains(addKey, "-") {
				parts := strings.Split(addKey, "-")
				addKey = parts[len(parts)-1]
			}
			if strings.HasSuffix(addKey, "*") && nonAlnumRegex.MatchString(strings.TrimSuffix(addKey, "*")) {
				addKey = fmt.Sprintf(`"%s"*`, escapeFTSKeyword(strings.TrimSuffix(addKey, "*")))
			}
			found := false
			for _, b := range batch {
				if b == addKey {
					found = true
					break
				}
			}
			if !found {
				batch = append([]string{addKey}, batch...)
			}
			words = append(words, batch)
		}
	}

	return words
}
