package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

func TestAddNote_TrimsAndRefusesEmpty(t *testing.T) {
	svc, _ := boardWith(t, "a ticket")

	n, err := svc.AddNote("ka-001", "matt", "  found the cause  \n")
	require.NoError(t, err)
	assert.Equal(t, "found the cause", n.Body)
	assert.Equal(t, "KA-001", n.TaskID)
	assert.Equal(t, "matt", n.AuthorID)
	assert.False(t, n.CreatedAt.IsZero())

	_, err = svc.AddNote("KA-001", "matt", " \n\t ")
	require.ErrorIs(t, err, domain.ErrInvalid)
}

func TestAddNote_UnknownTicketOrAuthor(t *testing.T) {
	svc, _ := boardWith(t, "a ticket")

	_, err := svc.AddNote("KA-999", "matt", "x")
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = svc.AddNote("KA-001", "nobody", "x")
	require.ErrorIs(t, err, domain.ErrNotFound)

	n, err := svc.AddNote("KA-001", "", "unsigned is allowed")
	require.NoError(t, err)
	assert.Equal(t, "", n.AuthorID)
}

func TestAddNote_TooLong(t *testing.T) {
	svc, _ := boardWith(t, "a ticket")
	_, err := svc.AddNote("KA-001", "matt", strings.Repeat("x", domain.MaxNoteLen+1))
	require.ErrorIs(t, err, domain.ErrInvalid)
	_, err = svc.AddNote("KA-001", "matt", strings.Repeat("x", domain.MaxNoteLen))
	require.NoError(t, err)
}

func TestGetTask_IncludesNotes(t *testing.T) {
	svc, _ := boardWith(t, "a ticket")
	_, err := svc.AddNote("KA-001", "matt", "first")
	require.NoError(t, err)
	_, err = svc.AddNote("KA-001", "claude", "second")
	require.NoError(t, err)

	got, err := svc.GetTask("KA-001")
	require.NoError(t, err)
	require.Len(t, got.Notes, 2)
	assert.Equal(t, "first", got.Notes[0].Body)
	assert.Equal(t, "second", got.Notes[1].Body)
}

// Records stay out of what a change returns: the API logs PatchTask's result
// as a sync event, and the sync must not carry notes.
func TestPatchTask_ResultCarriesNoRecords(t *testing.T) {
	svc, _ := boardWith(t, "a ticket")
	_, err := svc.AddNote("KA-001", "matt", "private")
	require.NoError(t, err)
	got, err := svc.PatchTask("KA-001", domain.TaskPatch{Title: domain.To("renamed")})
	require.NoError(t, err)
	assert.Equal(t, "renamed", got.Title)
	assert.Nil(t, got.Notes)
}

// A note is shown in terminals; escape sequences in it would be obeyed.
func TestAddNote_DropsControlCharacters(t *testing.T) {
	svc, _ := boardWith(t, "a ticket")
	n, err := svc.AddNote("KA-001", "matt", "red \x1b[31malert\x1b[0m\r\nline two\tok\x07\u009b2J")
	require.NoError(t, err)
	assert.Equal(t, "red [31malert[0m\nline two\tok2J", n.Body)
}
