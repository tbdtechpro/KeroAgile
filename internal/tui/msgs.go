package tui

import (
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/git"
)

// projectSelectedMsg is sent when the user selects a different project in the sidebar.
type projectSelectedMsg struct{ projectID string }

// taskSelectedMsg is sent when the board cursor moves to a different task.
type taskSelectedMsg struct{ taskID string }

// tasksReloadedMsg carries a fresh task list after a create/update/move.
type tasksReloadedMsg struct{ tasks []*domain.Task }

// gitRefreshedMsg carries fresh git data for the detail panel.
type gitRefreshedMsg struct {
	branch  string
	commits []git.Commit
}

// prStatusMsg carries a PR status update for a single task.
type prStatusMsg struct {
	taskID   string
	prStatus *git.PRStatus
}

// prMergedMsg triggers auto-transition of a task to done.
type prMergedMsg struct{ taskID string }

// tickMsg is sent by the 60-second PR polling ticker.
type tickMsg struct{}

// statusNotifMsg shows a transient notification in the status bar.
type statusNotifMsg struct{ text string }

// reloadTasksMsg triggers a fresh task list load for the given project.
type reloadTasksMsg struct{ projectID string }

// deletedTaskMsg is sent after a task has been successfully deleted.
type deletedTaskMsg struct{ taskID, projectID string }

// prMergedDoneMsg is sent after MarkPRMerged succeeds.
type prMergedDoneMsg struct{ taskID, projectID string }

// projectsLoadedMsg carries a fresh project list after initial load.
type projectsLoadedMsg struct{ projects []*domain.Project }

// usersLoadedMsg carries a fresh user list after initial load.
type usersLoadedMsg struct{ users []*domain.User }

// sprintSelectedMsg is sent when the user selects a sprint (or "All tasks") in the sidebar.
// sprintID == nil means "all tasks for this project".
type sprintSelectedMsg struct {
	projectID string
	sprintID  *int64
}

// sprintsLoadedMsg carries a fresh sprint list for the sidebar sprint view.
// enterMode == true tells the sidebar to switch into sprint list mode.
type sprintsLoadedMsg struct {
	projectID string
	summaries []domain.SprintSummary
	enterMode bool
}

// openSprintFormMsg tells App to open the sprint creation modal.
type openSprintFormMsg struct{}
