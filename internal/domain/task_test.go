package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

func TestStatusNext(t *testing.T) {
	assert.Equal(t, domain.StatusTodo, domain.StatusBacklog.Next())
	assert.Equal(t, domain.StatusInProgress, domain.StatusTodo.Next())
	assert.Equal(t, domain.StatusReview, domain.StatusInProgress.Next())
	assert.Equal(t, domain.StatusDone, domain.StatusReview.Next())
	assert.Equal(t, domain.StatusDone, domain.StatusDone.Next()) // no-op at end
}

func TestStatusPrev(t *testing.T) {
	assert.Equal(t, domain.StatusBacklog, domain.StatusBacklog.Prev()) // no-op at start
	assert.Equal(t, domain.StatusBacklog, domain.StatusTodo.Prev())
	assert.Equal(t, domain.StatusTodo, domain.StatusInProgress.Prev())
	assert.Equal(t, domain.StatusInProgress, domain.StatusReview.Prev())
	assert.Equal(t, domain.StatusReview, domain.StatusDone.Prev())
}

func TestStatusLabel(t *testing.T) {
	assert.Equal(t, "In Progress", domain.StatusInProgress.Label())
	assert.Equal(t, "Backlog", domain.StatusBacklog.Label())
}

func TestPriorityColor(t *testing.T) {
	assert.Equal(t, "#6B7280", domain.PriorityLow.Color())
	assert.Equal(t, "#EAB308", domain.PriorityMedium.Color())
	assert.Equal(t, "#F97316", domain.PriorityHigh.Color())
	assert.Equal(t, "#EF4444", domain.PriorityCritical.Color())
}
