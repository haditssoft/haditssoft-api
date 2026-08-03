package utils

import (
	"testing"
)

func TestQueryCombination(t *testing.T) {
	tests := []struct {
		name    string
		lengthy int
		wantLen int
		want    []string
	}{
		{"n=1", 1, 1, []string{"0"}},
		{"n=2", 2, 1, []string{"01"}},
		{"n=3", 3, 4, []string{"012", "01", "02", "12"}},
		{"n=4", 4, 5, []string{"0123", "012", "013", "023", "123"}},
		{"n=5", 5, 10, []string{"01234", "0124", "0134", "0234", "1234", "0123", "012", "013", "023", "123"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := QueryCombination(tt.lengthy)
			if len(got) != tt.wantLen {
				t.Errorf("QueryCombination(%d) returned %d combos, want %d\ngot: %v\nwant: %v", tt.lengthy, len(got), tt.wantLen, got, tt.want)
				return
			}

			// Check all expected combos are present
			gotMap := make(map[string]bool, len(got))
			for _, c := range got {
				gotMap[c] = true
			}
			for _, w := range tt.want {
				if !gotMap[w] {
					t.Errorf("missing expected combo %q in result %v", w, got)
				}
			}

			// Check no duplicates
			seen := make(map[string]bool)
			for _, c := range got {
				if seen[c] {
					t.Errorf("duplicate combo: %q", c)
				}
				seen[c] = true
			}

			// Check valid indices
			for _, c := range got {
				for _, ch := range c {
					idx := int(ch - '0')
					if idx >= tt.lengthy {
						t.Errorf("combo %q contains index %d >= lengthy %d", c, idx, tt.lengthy)
					}
				}
			}
		})
	}
}
