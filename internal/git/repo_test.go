package git_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"keroagile/internal/git"
)

func TestAutoLinkTaskID(t *testing.T) {
	cases := []struct {
		branch string
		want   string
	}{
		{"feature/KA-001-api-layer", "KA-001"},
		{"feature/KA-012", "KA-012"},
		{"main", ""},
		{"fix/some-bug", ""},
		{"PROJ-42-thing", "PROJ-42"},
	}
	for _, tc := range cases {
		t.Run(tc.branch, func(t *testing.T) {
			assert.Equal(t, tc.want, git.AutoLinkTaskID(tc.branch))
		})
	}
}
