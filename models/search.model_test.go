package models

import (
	"fmt"
	"strconv"
	"testing"
)

func TestFilterKeywordsModel(t *testing.T) {
	conjunctions := []string{"atau", "dan", "di", "yang", "tentang", "hadits", "hadis", "hadist", "takhrij"}

	tests := []struct {
		name string
		kws  []string
		conj []string
		want []string
	}{
		{
			name: "empty_input",
			kws:  []string{},
			conj: conjunctions,
			want: nil,
		},
		{
			name: "nil_input",
			kws:  nil,
			conj: conjunctions,
			want: nil,
		},
		{
			name: "single_non_conjunction",
			kws:  []string{"sholat"},
			conj: conjunctions,
			want: []string{"sholat"},
		},
		{
			name: "single_conjunction",
			kws:  []string{"dan"},
			conj: conjunctions,
			want: nil,
		},
		{
			name: "removes_duplicates",
			kws:  []string{"sholat", "sholat", "puasa"},
			conj: conjunctions,
			want: []string{"sholat", "puasa"},
		},
		{
			name: "removes_empty_strings",
			kws:  []string{"", "", "sholat"},
			conj: conjunctions,
			want: []string{"sholat"},
		},
		{
			name: "does_not_trim_whitespace",
			kws:  []string{" sholat ", "  puasa  "},
			conj: conjunctions,
			want: []string{" sholat ", "  puasa  "},
		},
		{
			name: "mixed",
			kws:  []string{"dan", "sholat", "", "sholat", "puasa", "di"},
			conj: conjunctions,
			want: []string{"sholat", "puasa"},
		},
		{
			name: "custom_conjunction",
			kws:  []string{"foo", "bar", "baz"},
			conj: []string{"bar"},
			want: []string{"foo", "baz"},
		},
		{
			name: "all_filtered",
			kws:  []string{"dan", "atau", "di"},
			conj: conjunctions,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterKeywords(tt.kws, tt.conj)
			if len(got) != len(tt.want) {
				t.Errorf("filterKeywords() returned %d items, want %d\ngot:  %v\nwant: %v", len(got), len(tt.want), got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("filterKeywords()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestQueryCombinationModel(t *testing.T) {
	tests := []struct {
		name    string
		lengthy int
		wantLen int
		want    []string
	}{
		{"case1", 1, 1, []string{"0"}},
		{"case2", 2, 1, []string{"01"}},
		{"case3", 3, 4, []string{"012", "01", "02", "12"}},
		{"case4", 4, 5, []string{"0123", "012", "013", "023", "123"}},
		{"case5", 5, 10, []string{"01234", "0124", "0134", "0234", "1234", "0123", "012", "013", "023", "123"}},
		{"case6_default", 6, 42, nil},
		{"case7_default", 7, 63, nil},
		{"case10_default", 10, 175, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := queryCombination(tt.lengthy)
			if len(got) != tt.wantLen {
				t.Errorf("queryCombination(%d) returned %d combos, want %d\ngot: %v", tt.lengthy, len(got), tt.wantLen, got)
				return
			}
			if tt.want != nil {
				gotMap := make(map[string]bool, len(got))
				for _, c := range got {
					gotMap[c] = true
				}
				for _, w := range tt.want {
					if !gotMap[w] {
						t.Errorf("missing expected combo %q in result %v", w, got)
					}
				}
			}
			for _, c := range got {
				for _, ch := range c {
					idx, _ := strconv.Atoi(string(ch))
					if idx >= tt.lengthy {
						t.Errorf("combo %q contains index %d >= lengthy %d", c, idx, tt.lengthy)
					}
				}
			}
			seen := make(map[string]bool)
			for _, c := range got {
				if seen[c] {
					t.Errorf("duplicate combo: %q", c)
				}
				seen[c] = true
			}
		})
	}
}

func TestCombineValueModel(t *testing.T) {
	// Reset global state before each test
	origDistance := wordsDistance

	t.Run("two_arrays", func(t *testing.T) {
		wordsDistance = origDistance
		got := combineValue([]string{"sholat*"}, []string{"puasa*"})
		if got == "" {
			t.Error("combineValue() returned empty string")
		}
		// With 2 arrays (c is nil), wordsDistance stays at default "2"
		if wordsDistance != "2" {
			t.Errorf("wordsDistance should remain '2' for 2 arrays, got %q", wordsDistance)
		}
	})

	t.Run("three_arrays_distance_10", func(t *testing.T) {
		wordsDistance = origDistance
		combineValue([]string{"a*"}, []string{"b*"}, []string{"c*"})
		if wordsDistance != "10" {
			t.Errorf("wordsDistance should be '10' for 3 arrays, got %q", wordsDistance)
		}
	})

	t.Run("four_arrays_distance_4", func(t *testing.T) {
		wordsDistance = origDistance
		combineValue([]string{"a*"}, []string{"b*"}, []string{"c*"}, []string{"d*"})
		if wordsDistance != "4" {
			t.Errorf("wordsDistance should be '4' for 4 arrays, got %q", wordsDistance)
		}
	})

	t.Run("result_format", func(t *testing.T) {
		wordsDistance = "2"
		got := combineValue([]string{"a*"}, []string{"b*"})
		expected := "NEAR(a* b*, 2)"
		if got != expected {
			t.Errorf("combineValue() = %q, want %q", got, expected)
		}
	})

	// Restore global state
	wordsDistance = origDistance
}

func TestQueryCombinationModelValidIndices(t *testing.T) {
	for lengthy := 1; lengthy <= 10; lengthy++ {
		t.Run(fmt.Sprintf("length_%d", lengthy), func(t *testing.T) {
			combos := queryCombination(lengthy)
			if len(combos) == 0 {
				t.Errorf("queryCombination(%d) returned empty", lengthy)
				return
			}
			seen := make(map[string]bool)
			for _, c := range combos {
				if seen[c] {
					t.Errorf("duplicate combo: %q", c)
				}
				seen[c] = true
				for _, ch := range c {
					idx, _ := strconv.Atoi(string(ch))
					if idx >= lengthy {
						t.Errorf("combo %q has index %d >= %d", c, idx, lengthy)
					}
				}
			}
		})
	}
}
