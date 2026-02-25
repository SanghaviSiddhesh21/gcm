package ui

import "testing"

func TestCountDiffLines(t *testing.T) {
	tests := []struct {
		name string
		diff string
		want int
	}{
		{
			name: "empty diff",
			diff: "",
			want: 0,
		},
		{
			name: "only context lines",
			diff: " unchanged\n unchanged\n",
			want: 0,
		},
		{
			name: "added and removed lines",
			diff: "+added\n-removed\n unchanged\n",
			want: 2,
		},
		{
			name: "file headers not counted",
			diff: "--- a/foo.go\n+++ b/foo.go\n+added\n",
			want: 1,
		},
		{
			name: "hunk header not counted",
			diff: "@@ -1,3 +1,4 @@\n+added\n-removed\n",
			want: 2,
		},
		{
			name: "large diff over threshold",
			diff: func() string {
				var s string
				for i := 0; i < 600; i++ {
					s += "+line\n"
				}
				return s
			}(),
			want: 600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countDiffLines(tt.diff)
			if got != tt.want {
				t.Errorf("countDiffLines() = %d, want %d", got, tt.want)
			}
		})
	}
}
