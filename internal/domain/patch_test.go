package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

func TestPatchTaskChangesTheTicketAndReturnsIt(t *testing.T) {
	svc, _ := boardWith(t, "a ticket")

	got, err := svc.PatchTask("ka-001", domain.TaskPatch{Title: domain.To("renamed"), Status: domain.To(domain.StatusTodo)})

	require.NoError(t, err)
	assert.Equal(t, "KA-001", got.ID, "the ID is accepted in any letter case")
	assert.Equal(t, "renamed", got.Title)
	assert.Equal(t, domain.StatusTodo, got.Status)
}

// The same rules as creating a ticket, applied to whichever fields are named.
func TestPatchTaskChecksTheFieldsItChanges(t *testing.T) {
	nobody := "nobody"
	otherSprint := func(svc *domain.Service) *int64 {
		require.NoError(t, svc.CreateProject("ZZ", "Other", ""))
		sp, err := svc.CreateSprint("Sprint 1", "ZZ", nil, nil)
		require.NoError(t, err)
		return &sp.ID
	}
	cases := []struct {
		name  string
		patch func(svc *domain.Service) domain.TaskPatch
		want  error
	}{
		{"unknown status", func(*domain.Service) domain.TaskPatch {
			return domain.TaskPatch{Status: domain.To(domain.Status("doing"))}
		}, domain.ErrInvalid},
		{"unknown priority", func(*domain.Service) domain.TaskPatch {
			return domain.TaskPatch{Priority: domain.To(domain.Priority("urgent"))}
		}, domain.ErrInvalid},
		{"blank title", func(*domain.Service) domain.TaskPatch { return domain.TaskPatch{Title: domain.To("   ")} }, domain.ErrInvalid},
		{"unknown assignee", func(*domain.Service) domain.TaskPatch { return domain.TaskPatch{AssigneeID: domain.To(&nobody)} }, domain.ErrNotFound},
		{"another project's sprint", func(svc *domain.Service) domain.TaskPatch {
			return domain.TaskPatch{SprintID: domain.To(otherSprint(svc))}
		}, domain.ErrInvalid},
	}
	for _, c := range cases {
		svc, m := boardWith(t, "a ticket")

		_, err := svc.PatchTask("KA-001", c.patch(svc))

		require.ErrorIs(t, err, c.want, c.name)
		assert.Empty(t, m.patches, "%s: nothing reached the store", c.name)
	}
}

func TestPatchTaskOnAMissingTicketNamesIt(t *testing.T) {
	svc, _ := boardWith(t)

	_, err := svc.PatchTask("KA-999", domain.TaskPatch{Title: domain.To("x")})

	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Contains(t, err.Error(), `ticket "KA-999"`)
}

// Front ends send "" for "no assignee".
func TestPatchTaskTreatsAnEmptyAssigneeAsNone(t *testing.T) {
	svc, m := boardWith(t, "a ticket")
	claude, none := "claude", ""
	_, err := svc.PatchTask("KA-001", domain.TaskPatch{AssigneeID: domain.To(&claude)})
	require.NoError(t, err)

	got, err := svc.PatchTask("KA-001", domain.TaskPatch{AssigneeID: domain.To(&none)})

	require.NoError(t, err)
	assert.Nil(t, got.AssigneeID)
	assert.Nil(t, m.tasks["KA-001"].AssigneeID)
}

func TestPatchTaskWithNothingToChangeWritesNothing(t *testing.T) {
	svc, m := boardWith(t, "a ticket")
	before := m.tasks["KA-001"].UpdatedAt

	got, err := svc.PatchTask("KA-001", domain.TaskPatch{})

	require.NoError(t, err)
	assert.Equal(t, "a ticket", got.Title, "the ticket is returned as it is")
	assert.Empty(t, m.patches)
	assert.True(t, m.tasks["KA-001"].UpdatedAt.Equal(before), "the update time does not move")
}

func TestPatchTaskRefusesAStaleConditionalChange(t *testing.T) {
	svc, m := boardWith(t, "a ticket")
	readAt := m.tasks["KA-001"].UpdatedAt
	_, err := svc.MoveTask("KA-001", domain.StatusTodo) // someone else changes it
	require.NoError(t, err)
	require.False(t, m.tasks["KA-001"].UpdatedAt.Equal(readAt), "the move advanced the update time")

	_, err = svc.PatchTask("KA-001", domain.TaskPatch{Title: domain.To("mine"), IfUpdatedAt: &readAt})

	require.ErrorIs(t, err, domain.ErrConflict)
	assert.Equal(t, "a ticket", m.tasks["KA-001"].Title)
}

// Each of these used to read the whole ticket, change one field in memory and
// write every column back, reverting whatever another writer had changed in
// between. Each must now send the store only its own field.
func TestSingleFieldOperationsSendOnlyTheirOwnField(t *testing.T) {
	sprintIn := func(svc *domain.Service) *int64 {
		sp, err := svc.CreateSprint("Sprint 1", "KA", nil, nil)
		require.NoError(t, err)
		return &sp.ID
	}
	pr := 42
	cases := []struct {
		name string
		do   func(svc *domain.Service) error
		want func(svc *domain.Service) domain.TaskPatch
	}{
		{"move",
			func(svc *domain.Service) error { _, err := svc.MoveTask("KA-001", domain.StatusReview); return err },
			func(*domain.Service) domain.TaskPatch {
				return domain.TaskPatch{Status: domain.To(domain.StatusReview)}
			}},
		{"link branch",
			func(svc *domain.Service) error { return svc.LinkBranch("KA-001", "fix/x") },
			func(*domain.Service) domain.TaskPatch { return domain.TaskPatch{Branch: domain.To("fix/x")} }},
		{"link PR",
			func(svc *domain.Service) error { return svc.LinkPR("KA-001", 42) },
			func(*domain.Service) domain.TaskPatch { return domain.TaskPatch{PRNumber: domain.To(&pr)} }},
		{"mark PR merged",
			func(svc *domain.Service) error { return svc.MarkPRMerged("KA-001") },
			func(*domain.Service) domain.TaskPatch {
				return domain.TaskPatch{PRMerged: domain.To(true), Status: domain.To(domain.StatusDone)}
			}},
		{"take out of sprint",
			func(svc *domain.Service) error { _, err := svc.AssignTaskToSprint("KA-001", nil); return err },
			func(*domain.Service) domain.TaskPatch { return domain.TaskPatch{SprintID: domain.Clear[int64]()} }},
	}
	for _, c := range cases {
		svc, m := boardWith(t, "a ticket")

		require.NoError(t, c.do(svc), c.name)

		require.Len(t, m.patches, 1, c.name)
		assert.Equal(t, c.want(svc), m.patches[0], c.name)
	}

	// Assigning to a sprint needs the sprint to exist first.
	svc, m := boardWith(t, "a ticket")
	id := sprintIn(svc)
	_, err := svc.AssignTaskToSprint("KA-001", id)
	require.NoError(t, err)
	require.Len(t, m.patches, 1)
	assert.Equal(t, domain.TaskPatch{SprintID: domain.To(id)}, m.patches[0])
}

// Assigning to a sprint did not move the update time, so nothing could tell
// the ticket had changed.
func TestEveryChangeAdvancesTheUpdateTime(t *testing.T) {
	svc, m := boardWith(t, "a ticket")
	sp, err := svc.CreateSprint("Sprint 1", "KA", nil, nil)
	require.NoError(t, err)
	before := m.tasks["KA-001"].UpdatedAt
	time.Sleep(time.Millisecond)

	_, err = svc.AssignTaskToSprint("KA-001", &sp.ID)

	require.NoError(t, err)
	assert.True(t, m.tasks["KA-001"].UpdatedAt.After(before))
}
