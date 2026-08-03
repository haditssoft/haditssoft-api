package models

import (
	"fmt"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"regexp"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

var GetIndexOfKitab = map[string]string{
	"ShahihBukhari":       "0",
	"ShahihMuslim":        "1",
	"SunanTirmidzi":       "2",
	"SunanAbuDaud":        "3",
	"SunanNasai":          "4",
	"SunanIbnuMajah":      "5",
	"SunanDarimi":         "6",
	"MusnadAhmad":         "7",
	"MuwathaMalik":        "8",
	"SunanDaruquthni":     "9",
	"ShahihIbnuKhuzaimah": "10",
	"ShahihIbnuHibban":    "11",
	"AlMustadrak":         "12",
	"MusnadSyafii":        "13",
}

var Conjunction []string = []string{"atau", "dan", "di", "yang", "tentang", "hadits", "hadis", "hadist", "takhrij"}

func GetSearchResult(kitab, column string, keywords []string) (Result, error) {
	qb := database.DB.Table(kitab)
	col := column
	/** aliasName is like '5' */
	aliasName := GetIndexOfKitab[kitab]

	if len(keywords) <= 1 {
		w := ""
		if len(keywords) == 1 {
			w = keywords[0]
		}
		qb = qb.Where(fmt.Sprintf("%s LIKE ?", col), "%"+w+"%")
	} else {
		keyName := filterKeywords(keywords, Conjunction)
		howManyWords := len(keyName)
		if howManyWords > 1 {
			if strings.ToLower(col) == "indonesia" {
				if len(keywords) > 7 {
					keywords = keywords[:7]
				}
				limitChars := 1001
				switch howManyWords {
				case 3:
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

				/** bookName is like FTSSunanIbnuMajah */
				bookName := ("FTS" + kitab)

				row, err := getOtherKeywords(database.DB, keyName)
				if err != nil {
					fmt.Println(err.Error())
					qb = qb.Select("Nomer AS " + aliasName).Order("Nomer")
					var out Result
					rows, err := qb.Rows()
					if err != nil {
						return nil, err
					}
					defer rows.Close()
					for rows.Next() {
						var number int64
						rows.Scan(&number)
						out = append(out, map[string]interface{}{aliasName: number})
					}
					return out, nil
				}

				var words [][]string
				var arrayOfMatch []string

				for _, v := range keyName {
					wordData := fmt.Sprintf("[%s]", v)
					if row[v] != "" {
						wordData = row[v]
					}
					re := regexp.MustCompile(`\[(\D*?)\]`)
					arrayOfMatch = re.FindAllString(wordData, -1)
					if len(arrayOfMatch) == 0 {
						arrayOfMatch = []string{fmt.Sprintf("[%s]", v)}
					}
					if len(arrayOfMatch) > 0 {
						var batch []string
						for _, vl := range arrayOfMatch {
							// Compile regex to match [ or ]
							re := regexp.MustCompile(`(\[|\])`)
							// Replace with empty string
							eachWord := re.ReplaceAllString(vl, "")

							// Equivalent of: if (eachWord.length === 1 || eachWord.includes(')') || eachWord.startsWith(' ') || eachWord.startsWith('-'))
							if len([]rune(eachWord)) == 1 || strings.Contains(eachWord, ")") ||
								strings.HasPrefix(eachWord, " ") || strings.HasPrefix(eachWord, "-") {
								continue
							}

							// Equivalent of: if (eachWord.includes('-') || eachWord.includes(' '))
							if strings.Contains(eachWord, "-") || strings.Contains(eachWord, " ") {
								eachWord = `"` + eachWord + `"`
							}

							// Equivalent of: batch.push(eachWord + '*')
							batch = append(batch, eachWord+"*")

						}

						if len(batch) != 0 {
							addKeyFromClient := (v + "*")

							if !isSliceContains(batch, addKeyFromClient) {
								if strings.Contains(addKeyFromClient, "-") {
									// addKeyFromClient = ('"' + addKeyFromClient + '"');
									// example word: macam-macam*
									// become [macam, macam*]
									parts := strings.Split(addKeyFromClient, "-")
									// get the last one: "macam*"
									addKeyFromClient = parts[len(parts)-1]
								}
								// Equivalent of batch.unshift(addKeyFromClient)
								batch = append([]string{addKeyFromClient}, batch...)
							}
							words = append(words, batch)
						}
					}
				}
				getCombination := queryCombination(len(keyName))
				joinedQuery := ""
				var firstNSecondRank Result
				for inx := -1; inx < len(getCombination); inx++ { // notice start from -1 instead of 0
					if inx < 0 {
						firstNSecondRank, err = getFirstNSecondRank(database.DB, kitab, column, keywords, aliasName, bookName)
						if err != nil {
							continue
						}
					} else {
						var arrayToBeCombined [][]string // [[arr idx 0],[arr idx 1],[arr idx 2]] become -> [[idx 0],[idx 2]]
						combination := getCombination[inx]
						for ix, arr := range words {
							if strings.Contains(combination, strconv.Itoa(ix)) {
								arrayToBeCombined = append(arrayToBeCombined, arr)
							}
						}
						firKeyVar := arrayToBeCombined[0]
						secKeyVar := arrayToBeCombined[1]
						restKeyVar := arrayToBeCombined[2:]
						if joinedQuery != "" {
							joinedQuery = joinedQuery + " OR "
						}
						joinedQuery += combineValue(firKeyVar, secKeyVar, restKeyVar...)
						// const oneByOne = await combineValue(firKeyVar, secKeyVar, ...restKeyVar);
					}
				}

				FTSQuery := (column + " MATCH '" + joinedQuery + "' AND length(" + column + ") < " + strconv.Itoa(limitChars))
				// whateverRank data is like [{ '5': 78 },   { '5': 3961 }]
				var whateverRank Result
				query := fmt.Sprintf(`
					SELECT Nomer as '%s'
					FROM %s
					WHERE %s
					ORDER BY rank
				`, aliasName, bookName, FTSQuery)

				rows, err := database.DB.Raw(query).Rows()
				if err != nil {
					return nil, err
				}
				defer rows.Close()
				for rows.Next() {
					var number int64
					rows.Scan(&number)
					whateverRank = append(whateverRank, map[string]interface{}{aliasName: number})
				}

				// resultContainer ata is like [{ '5': 78 },   { '5': 3961 }]
				resultContainer := uniqueByAlias(aliasName, firstNSecondRank, whateverRank)
				return resultContainer, err
			} else {
				joiner := `%' And ` + column + ` Like '%`
				joinedKeywords := strings.Join(keywords, joiner)
				qwery := column + " Like '%" + joinedKeywords + "%'"
				qb = qb.Where(qwery)
			}
		} else {
			// if about to search in multi-keywords mode but the words are all the same
			// remove the other words except one then search in one-keyword mode
			qb = qb.Where(col+" LIKE ?", "'%"+keyName[0]+"%'")
		}
	}

	qb = qb.Select("Nomer AS '" + aliasName + "'").Order("Nomer")
	var out Result
	rows, err := qb.Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var number int64
		rows.Scan(&number)
		out = append(out, map[string]interface{}{aliasName: number})
	}

	return out, nil
}

// FilterKeywords removes duplicates, empty strings, and words in conjunction.
func filterKeywords(keyWords, conjunction []string) []string {
	// convert conjunction slice to a set for fast lookup
	conjSet := make(map[string]bool, len(conjunction))
	for _, v := range conjunction {
		conjSet[v] = true
	}

	seen := make(map[string]bool)
	var result []string

	for _, word := range keyWords {
		if word == "" { // skip empty
			continue
		}
		if conjSet[word] { // skip conjunction
			continue
		}
		if seen[word] { // skip duplicate
			continue
		}
		seen[word] = true
		result = append(result, word)
	}

	return result
}

type HadithData struct {
	BookName string
	Language string
	Keywords []string
}

// Result holds the DB rows we fetch
type Result []map[string]interface{}

func getFirstNSecondRank(db *gorm.DB, kitab, column string, keywords []string, aliasName string, bookName string) (Result, error) {
	var firstRank Result
	var secondRank Result

	// === First Rank ===
	// e.g. WHERE <column> MATCH '"keyword1 keyword2"*'
	thisKeyword := fmt.Sprintf("\"%s\"*", strings.Join(keywords, " "))

	query := fmt.Sprintf(`
		SELECT Nomer as '%s'
		FROM %s
		WHERE %s MATCH ?
		ORDER BY rank
	`, aliasName, bookName, column)

	rows, err := db.Raw(query, thisKeyword).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var number int64
		rows.Scan(&number)
		firstRank = append(firstRank, map[string]interface{}{aliasName: number})
	}

	// === Second Rank ===
	// e.g. WHERE <column> MATCH 'NEAR("kw1"* "kw2"*, 2)'
	var quoted []string
	for _, wrd := range keywords {
		quoted = append(quoted, fmt.Sprintf("\"%s\"*", wrd))
	}
	thisKeyword = strings.Join(quoted, " ")

	query = fmt.Sprintf(`
		SELECT Nomer as '%s'
		FROM %s
		WHERE %s MATCH 'NEAR(?, 2)'
		ORDER BY rank
	`, aliasName, bookName, column)

	rowsx, errx := db.Raw(query, thisKeyword).Rows()
	if errx != nil {
		return nil, errx
	}
	defer rowsx.Close()
	for rowsx.Next() {
		var number int64
		rowsx.Scan(&number)
		secondRank = append(secondRank, map[string]interface{}{aliasName: number})
	}

	// === Merge & Deduplicate ===
	allResults := append(firstRank, secondRank...)
	seen := make(map[int64]bool)
	var resultContainer Result

	for _, r := range allResults {
		if !seen[r[aliasName].(int64)] {
			seen[r[aliasName].(int64)] = true
			resultContainer = append(resultContainer, r)
		}
	}

	return resultContainer, nil
}

// KBBI table model
type KBBI struct {
	KataKunci string `gorm:"column:katakunci"`
	ArtiKata  string `gorm:"column:artikata"`
}

func fetchKeywords(db *gorm.DB, queries []string) ([]KBBI, error) {
	var results []KBBI
	for _, query := range queries {
		var rows []KBBI
		if err := db.Raw("SELECT katakunci, artikata FROM KBBI WHERE " + query).Scan(&rows).Error; err != nil {
			return nil, err
		}
		if len(rows) > 0 {
			results = rows
			break
		}
	}
	return results, nil
}

func getOtherKeywords(db *gorm.DB, initialKeywords []string) (map[string]string, error) {
	combinedWords := make(map[string]string)

	for _, kw := range initialKeywords {
		keywrd := strings.ToLower(kw)

		// prefix stripping
		if strings.HasPrefix(keywrd, "di") {
			keywrd = keywrd[2:]
		} else if strings.HasPrefix(keywrd, "mem") {
			keywrd = keywrd[3:]
		} else if strings.HasPrefix(keywrd, "meng") {
			keywrd = keywrd[4:]
		} else if strings.HasPrefix(keywrd, "meny") {
			keywrd = "" // special case → handled as []string below
		} else if strings.HasPrefix(keywrd, "men") {
			keywrd = keywrd[3:]
		} else if strings.HasPrefix(keywrd, "me") {
			keywrd = keywrd[2:]
		} else if strings.HasPrefix(keywrd, "ber") {
			keywrd = keywrd[3:]
		} else if strings.HasPrefix(keywrd, "ter") {
			keywrd = keywrd[3:]
		}

		var mainWord []string
		if kw != "" && !strings.HasPrefix(strings.ToLower(kw), "meny") {
			// suffix adjustments
			if strings.HasSuffix(keywrd, "hi") {
				keywrd = keywrd[:len(keywrd)-1]
			}
			mainWord = []string{
				fmt.Sprintf("katakunci = '%s'", keywrd),
				fmt.Sprintf("katakunci LIKE '%%%s]%%' AND artikata LIKE '%%%s]%%'", keywrd, keywrd),
				fmt.Sprintf("artikata LIKE '%%%s]%%'", keywrd),
			}
		} else {
			// meny → two variants
			doLoop := []string{"s" + kw[4:], "c" + kw[4:]}
			var keywrds []string
			for _, wv := range doLoop {
				if strings.HasSuffix(wv, "hi") {
					keywrds = append(keywrds, wv[:len(wv)-1])
				}
			}
			if len(keywrds) == 0 {
				keywrds = doLoop
			}
			mainWord = []string{
				fmt.Sprintf("katakunci = '%s'", keywrds[0]),
				fmt.Sprintf("katakunci = '%s'", keywrds[1]),
				fmt.Sprintf("katakunci LIKE '%%%s]%%' AND artikata LIKE '%%%s]%%'", keywrds[0], keywrds[0]),
				fmt.Sprintf("katakunci LIKE '%%%s]%%' AND artikata LIKE '%%%s]%%'", keywrds[1], keywrds[1]),
				fmt.Sprintf("artikata LIKE '%%%s]%%'", keywrds[0]),
				fmt.Sprintf("artikata LIKE '%%%s]%%'", keywrds[1]),
			}
		}

		// fetch rows
		rows, err := fetchKeywords(db, mainWord)
		if err != nil {
			return nil, err
		}

		// combine results
		for _, obj := range rows {
			if val, exists := combinedWords[kw]; exists {
				combinedWords[kw] = val + " " + obj.ArtiKata
			} else {
				combinedWords[kw] = obj.ArtiKata
			}
		}
	}

	return combinedWords, nil
}

func queryCombination(lengthy int) []string {
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

func isSliceContains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}

func uniqueByAlias(aliasName string, slices ...Result) Result {
	seen := make(map[int64]bool) // track seen values for aliasName
	var result Result

	// merge slices
	for _, slice := range slices {
		for _, item := range slice {
			if val, ok := item[aliasName]; ok {
				if !seen[val.(int64)] {
					seen[val.(int64)] = true
					result = append(result, item)
				}
			}
		}
	}
	return result
}

// =======================================================
var wordsDistance = "2"

// Cartesian product generator
func cartesian(a []string, b ...[]string) []string {
	if len(b) == 0 {
		return a
	}

	result := []string{}
	for _, x := range a {
		for _, y := range b[0] {
			result = append(result, x+" "+y)
		}
	}

	// recurse if more slices
	if len(b) > 1 {
		return cartesian(result, b[1:]...)
	}
	return result
}

// Joins with NEAR operator
func joiner(arr []string) string {
	separator := ", " + wordsDistance + ") OR NEAR("
	middle := strings.Join(arr, separator)
	return "NEAR(" + middle + ", " + wordsDistance + ")"
}

// Main combiner
func combineValue(a []string, b []string, c ...[]string) string {
	var queryWords []string
	if len(b) > 0 {
		// prepend b to c
		newCombine := append([][]string{b}, c...)
		queryWords = cartesian(a, newCombine...)
	} else {
		queryWords = a
	}

	if len(c) > 0 {
		switch len(c) {
		case 0, 1:
			wordsDistance = "10"
		default:
			wordsDistance = "4"
		}
	}

	return joiner(queryWords)
}
