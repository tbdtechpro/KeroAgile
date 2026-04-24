package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

func TestSuggestAssignee(t *testing.T) {
	human := &domain.User{ID: "matt", IsAgent: false}
	agent := &domain.User{ID: "claude", IsAgent: true}
	users := []*domain.User{human, agent}

	tests := []struct {
		title     string
		defaultID string
		want      string
	}{
		{"Implement OAuth login", "matt", "claude"},
		{"Build payment API", "matt", "claude"},
		{"Refactor auth middleware", "matt", "claude"},
		{"Fix null pointer in parser", "matt", "claude"},
		{"Design the onboarding flow", "matt", "matt"},
		{"Research competitor pricing", "matt", "matt"},
		{"QA sprint 3 features", "matt", "matt"},
		{"Review PR for login page", "matt", "matt"},
		{"Write tests for auth service", "matt", "claude"},
		{"", "matt", "matt"},
		{"Some vague task", "matt", "matt"},
	}

	for _, tc := range tests {
		t.Run(tc.title, func(t *testing.T) {
			got := domain.SuggestAssignee(tc.title, users, tc.defaultID)
			assert.Equal(t, tc.want, got, "title=%q", tc.title)
		})
	}
}

func TestSuggestAssignee_NoAgentUsers(t *testing.T) {
	human := &domain.User{ID: "matt", IsAgent: false}
	got := domain.SuggestAssignee("Implement OAuth", []*domain.User{human}, "matt")
	assert.Equal(t, "matt", got)
}

func TestSuggestAssignee_EmptyUsers(t *testing.T) {
	got := domain.SuggestAssignee("Build API", []*domain.User{}, "matt")
	assert.Equal(t, "matt", got)
}
