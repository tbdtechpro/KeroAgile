package tui

import (
	"keroagile/internal/domain"
)

// DragState tracks an active mouse drag-and-drop operation.
type DragState struct {
	TaskID       string
	TaskTitle    string
	StartY       int
	CurrentY     int
	TargetStatus domain.Status
}

// Active returns true when a drag is in progress.
func (d *DragState) Active() bool {
	return d != nil && d.TaskID != ""
}

// resolveTargetStatus maps a Y position within the board panel to the status
// section the cursor is hovering over. sectionTops is a map of status → top Y
// (relative to the board panel's inner area).
func resolveTargetStatus(y int, sectionTops map[domain.Status]int) domain.Status {
	best := domain.StatusBacklog
	bestY := -1
	for status, top := range sectionTops {
		if top <= y && top > bestY {
			best = status
			bestY = top
		}
	}
	return best
}
