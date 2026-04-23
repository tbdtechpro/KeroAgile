package domain

import "time"

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
	ID          string
	ProjectID   string
	SprintID    *int64
	Title       string
	Description string
	Status      Status
	Priority    Priority
	Points      *int
	AssigneeID  *string
	Branch      string
	PRNumber    *int
	PRMerged    bool
	Labels      []string
	Blockers    []string // task IDs that block this task
	Blocking    []string // task IDs this task blocks
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
