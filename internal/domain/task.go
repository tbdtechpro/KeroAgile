package domain

import (
	"slices"
	"time"
)

type Status string

const (
	StatusBacklog    Status = "backlog"
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusReview     Status = "review"
	StatusDone       Status = "done"
)

var statusOrder = []Status{
	StatusBacklog, StatusTodo, StatusInProgress, StatusReview, StatusDone,
}

// Valid reports whether s is one of the five statuses every board shows.
func (s Status) Valid() bool { return slices.Contains(statusOrder, s) }

func (s Status) Next() Status {
	for i, st := range statusOrder {
		if st == s && i < len(statusOrder)-1 {
			return statusOrder[i+1]
		}
	}
	return s
}

func (s Status) Prev() Status {
	for i, st := range statusOrder {
		if st == s && i > 0 {
			return statusOrder[i-1]
		}
	}
	return s
}

func (s Status) Label() string {
	switch s {
	case StatusBacklog:
		return "Backlog"
	case StatusTodo:
		return "Todo"
	case StatusInProgress:
		return "In Progress"
	case StatusReview:
		return "Review"
	case StatusDone:
		return "Done"
	}
	return string(s)
}

type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

// Valid reports whether p is a known priority.
func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityCritical:
		return true
	}
	return false
}

func (p Priority) Color() string {
	switch p {
	case PriorityLow:
		return "#6B7280"
	case PriorityMedium:
		return "#EAB308"
	case PriorityHigh:
		return "#F97316"
	case PriorityCritical:
		return "#EF4444"
	}
	return "#6B7280"
}

func (p Priority) Label() string {
	switch p {
	case PriorityLow:
		return "LOW"
	case PriorityMedium:
		return "MEDIUM"
	case PriorityHigh:
		return "HIGH"
	case PriorityCritical:
		return "CRITICAL"
	}
	return string(p)
}

type Task struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	SprintID    *int64    `json:"sprint_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	Priority    Priority  `json:"priority"`
	Points      *int      `json:"points"`
	AssigneeID  *string   `json:"assignee_id"`
	Branch      string    `json:"branch"`
	PRNumber    *int      `json:"pr_number"`
	PRMerged    bool      `json:"pr_merged"`
	Labels      []string  `json:"labels"`
	Blockers        []string       `json:"blockers"` // task IDs that block this task
	Blocking        []string       `json:"blocking"` // task IDs this task blocks
	BlockerDetails  []*TaskSummary `json:"blocker_details,omitempty"`
	BlockingDetails []*TaskSummary `json:"blocking_details,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// TaskSummary is a lightweight task representation used in search results
// and blocker enrichment. It does not include body fields like description,
// labels, or sprint.
type TaskSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	ProjectID string `json:"project_id"`
	Status    Status `json:"status"`
}

// Change says whether a field should change, and to what. The zero value
// leaves the field alone.
type Change[T any] struct {
	Set   bool
	Value T
}

// To returns a Change that sets a field to v.
func To[T any](v T) Change[T] { return Change[T]{Set: true, Value: v} }

// Clear returns a Change that empties an optional field.
func Clear[T any]() Change[*T] { return Change[*T]{Set: true} }

// TaskPatch changes some fields of a ticket and leaves every other field
// exactly as it is in the store at that moment, including changes other
// writers have made since the caller last read the ticket.
type TaskPatch struct {
	Title       Change[string]
	Description Change[string]
	Status      Change[Status]
	Priority    Change[Priority]
	AssigneeID  Change[*string] // nil clears
	Points      Change[*int]    // nil clears
	Labels      Change[[]string]
	SprintID    Change[*int64] // nil takes the ticket out of its sprint
	Branch      Change[string]
	PRNumber    Change[*int]
	PRMerged    Change[bool]

	// IfUpdatedAt makes the patch conditional: it is applied only if the
	// ticket's UpdatedAt still equals this value, which the caller read
	// earlier. Otherwise the patch fails with ErrConflict and changes nothing.
	IfUpdatedAt *time.Time
}

// Empty reports whether the patch names no field to change.
func (p TaskPatch) Empty() bool {
	return !(p.Title.Set || p.Description.Set || p.Status.Set || p.Priority.Set ||
		p.AssigneeID.Set || p.Points.Set || p.Labels.Set || p.SprintID.Set ||
		p.Branch.Set || p.PRNumber.Set || p.PRMerged.Set)
}
