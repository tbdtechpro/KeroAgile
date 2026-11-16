package styles

import "github.com/charmbracelet/lipgloss"

// TruncateRune returns s truncated to fit displayWidth columns, appending "…"
// if truncation occurred. Uses lipgloss.Width for display-width measurement so
// wide runes (CJK, emoji) are counted by visual columns rather than byte/rune
// count.
//
// Edge cases:
//   - displayWidth <= 0 returns "".
//   - displayWidth == 1 returns "…" (the one column we have).
//   - Strings that already fit are returned unchanged.
func TruncateRune(s string, displayWidth int) string {
	if displayWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= displayWidth {
		return s
	}
	if displayWidth == 1 {
		return "…"
	}
	// Reserve 1 column for the ellipsis.
	budget := displayWidth - 1
	var b []rune
	used := 0
	for _, r := range s {
		w := lipgloss.Width(string(r))
		if used+w > budget {
			break
		}
		b = append(b, r)
		used += w
	}
	return string(b) + "…"
}
