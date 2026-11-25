package domain

import (
	"slices"
	"testing"
)

func TestSplitAcrossChapters(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		n     int
		sizes []int
		want  []int
	}{
		"even sizes":   {10, []int{10, 10}, []int{5, 5}},
		"single":       {5, []int{20}, []int{5}},
		"whole pool":   {5, []int{3, 2}, []int{3, 2}},
		"uneven sizes": {10, []int{5, 10, 15}, []int{2, 3, 5}}, // quotas 1.67, 3.33, 5
		// A remainder tie goes to the earlier chapter.
		"tie": {5, []int{7, 3}, []int{4, 1}}, // quotas 3.5, 1.5
		// Chapters smaller than an even share get at most their own size.
		"chapter smaller than its share": {5, []int{1, 9}, []int{1, 4}},             // quotas 0.5, 4.5
		"several tiny chapters":          {5, []int{1, 1, 1, 7}, []int{1, 1, 0, 3}}, // quotas 0.5, 0.5, 0.5, 3.5
		"tiny chapter gets nothing":      {5, []int{1, 19}, []int{0, 5}},            // quotas 0.25, 4.75
	}
	for name, tt := range tests {
		got := SplitAcrossChapters(tt.n, tt.sizes)
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s: SplitAcrossChapters(%d, %v) = %v, want %v", name, tt.n, tt.sizes, got, tt.want)
		}
	}
}
