package styles_test

import (
	"testing"

	"github.com/tbdtechpro/KeroAgile/internal/tui/styles"
)

func TestTruncateRune(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"short string unchanged", "Career", 20, "Career"},
		{"exact fit unchanged", "Career", 6, "Career"},
		{"long string truncated with ellipsis", "Career Research", 10, "Career Re…"},
		{"very narrow width", "Career Research", 4, "Car…"},
		{"width 1 returns ellipsis only", "Career Research", 1, "…"},
		{"width 0 returns empty", "Career Research", 0, ""},
		{"negative width returns empty", "Career Research", -3, ""},
		{"empty string unchanged", "", 10, ""},
		{"wide rune (emoji) counted by display width", "abc🎉def", 6, "abc🎉…"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := styles.TruncateRune(tc.in, tc.width)
			if got != tc.want {
				t.Fatalf("TruncateRune(%q, %d) = %q; want %q", tc.in, tc.width, got, tc.want)
			}
		})
	}
}
