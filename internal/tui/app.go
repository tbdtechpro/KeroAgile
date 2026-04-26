package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/git"
	"github.com/tbdtechpro/KeroAgile/internal/tui/forms"
	"github.com/tbdtechpro/KeroAgile/internal/tui/styles"
)

type panelFocus int

const (
	focusSidebar panelFocus = iota
	focusBoard
	focusDetail
)

// App is the root BubbleTea model.
type App struct {
	svc             *domain.Service
	gitClients      map[string]*git.Client
	defaultAssignee string

	sidebar Sidebar
	board   Board
	detail  Detail

	focus              panelFocus
	form               *forms.TaskForm
	sprintForm         *forms.SprintForm
	blockerPicker      *forms.BlockerPicker
	blockerPickerField string

	projects         []*domain.Project
	currentTasks     []*domain.Task
	users            []*domain.User
	sprints          []*domain.Sprint
	sprintSummaries  []domain.SprintSummary
	selectedSprintID *int64

	statusMsg    string
	statusExpiry time.Time

	width  int
	height int

	spinner spinner.Model
	loading bool
}

// New creates the App. Call Run() to start the BubbleTea event loop.
func New(svc *domain.Service, defaultAssignee string) *App {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return &App{
		svc:             svc,
		gitClients:      make(map[string]*git.Client),
		spinner:         sp,
		defaultAssignee: defaultAssignee,
	}
}

func (a *App) Run() error {
	p := tea.NewProgram(a,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}

func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.spinner.Tick,
		a.loadProjects(),
		a.loadUsers(),
		a.tickPRPoll(),
	)
}

func (a App) loadProjects() tea.Cmd {
	return func() tea.Msg {
		projects, err := a.svc.ListProjects()
		if err != nil {
			return statusNotifMsg{fmt.Sprintf("error: %v", err)}
		}
		return projectsLoadedMsg{projects}
	}
}

func (a App) loadUsers() tea.Cmd {
	return func() tea.Msg {
		users, err := a.svc.ListUsers()
		if err != nil {
			return statusNotifMsg{fmt.Sprintf("error loading users: %v", err)}
		}
		return usersLoadedMsg{users}
	}
}

func (a App) loadTasks(projectID string) tea.Cmd {
	return func() tea.Msg {
		tasks, err := a.svc.ListTasks(projectID, domain.TaskFilters{})
		if err != nil {
			return statusNotifMsg{fmt.Sprintf("error: %v", err)}
		}
		return tasksReloadedMsg{tasks}
	}
}

func (a App) loadSprints(projectID string, enterMode bool) tea.Cmd {
	return func() tea.Msg {
		summaries, err := a.svc.ListSprintsWithCounts(projectID)
		if err != nil {
			return statusNotifMsg{fmt.Sprintf("error loading sprints: %v", err)}
		}
		return sprintsLoadedMsg{projectID: projectID, summaries: summaries, enterMode: enterMode}
	}
}

func (a App) loadTasksFiltered(projectID string) tea.Cmd {
	sprintID := a.selectedSprintID
	return func() tea.Msg {
		tasks, err := a.svc.ListTasks(projectID, domain.TaskFilters{SprintID: sprintID})
		if err != nil {
			return statusNotifMsg{fmt.Sprintf("error: %v", err)}
		}
		return tasksReloadedMsg{tasks}
	}
}

func (a App) tickPRPoll() tea.Cmd {
	return tea.Tick(60*time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Picker overlay takes priority when open
	if a.blockerPicker != nil {
		newPicker, cmd := a.blockerPicker.Update(msg)
		bp := newPicker.(forms.BlockerPicker)
		a.blockerPicker = &bp
		return a, cmd
	}

	if a.sprintForm != nil {
		switch msg := msg.(type) {
		case forms.SprintSavedMsg:
			return a.handleSprintFormSaved(msg)
		case forms.SprintCancelledMsg:
			a.sprintForm = nil
			return a, nil
		default:
			m, cmd := a.sprintForm.Update(msg)
			sf := m.(forms.SprintForm)
			a.sprintForm = &sf
			return a, cmd
		}
	}

	if a.form != nil {
		switch msg := msg.(type) {
		case forms.SavedMsg:
			return a.handleFormSaved(msg)
		case forms.CancelledMsg:
			a.form = nil
			return a, nil
		default:
			m, cmd := a.form.Update(msg)
			f := m.(forms.TaskForm)
			a.form = &f
			return a, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.relayout()

	case tea.KeyMsg:
		cmds = append(cmds, a.handleKey(msg)...)

	case tea.MouseMsg:
		cmds = append(cmds, a.handleMouse(msg)...)

	case projectsLoadedMsg:
		a.projects = msg.projects
		counts := a.buildCounts()
		a.sidebar = NewSidebar(a.projects, counts, a.sidebarWidth(), a.height-3)
		for _, p := range a.projects {
			if p.RepoPath != "" {
				a.gitClients[p.ID] = git.New(p.RepoPath)
			}
		}
		if len(a.projects) > 0 {
			cmds = append(cmds, a.loadTasks(a.projects[0].ID))
		}

	case usersLoadedMsg:
		a.users = msg.users
		a.detail = a.detail.SetUsers(msg.users)

	case tasksReloadedMsg:
		a.currentTasks = msg.tasks
		a.board = a.board.SetTasks(msg.tasks)
		counts := a.buildCounts()
		a.sidebar = a.sidebar.SetProjects(a.projects, counts)

	case projectSelectedMsg:
		cmds = append(cmds, a.loadTasks(msg.projectID))

	case sprintsLoadedMsg:
		a.sprintSummaries = msg.summaries
		a.sprints = make([]*domain.Sprint, len(msg.summaries))
		for i, sum := range msg.summaries {
			a.sprints[i] = sum.Sprint
		}
		var m tea.Model
		m, _ = a.sidebar.Update(msg)
		a.sidebar = m.(Sidebar)

	case sprintSelectedMsg:
		a.selectedSprintID = msg.sprintID
		header := ""
		if msg.sprintID != nil {
			for _, sum := range a.sprintSummaries {
				if sum.Sprint.ID == *msg.sprintID {
					header = sprintHeaderLine(sum.Sprint)
					break
				}
			}
		}
		a.board = a.board.SetSprintHeader(header)
		cmds = append(cmds, a.loadTasksFiltered(msg.projectID))

	case openSprintFormMsg:
		sf := forms.NewSprintForm(a.width, a.height)
		a.sprintForm = &sf

	case taskSelectedMsg:
		found := false
		for _, t := range a.currentTasks {
			if t.ID == msg.taskID {
				found = true
				a.detail = a.detail.SetTask(t)
				cmds = append(cmds, a.refreshGit(t))
				break
			}
		}
		if !found {
			a.setStatus("task not found in cache — press r to refresh")
		}

	case taskMovedMsg:
		// Fix 7: wrap MoveTask in a cmd instead of calling synchronously
		pid := a.sidebar.SelectedProjectID()
		cmds = append(cmds, a.doMoveTask(msg.taskID, msg.status, pid))

	case gitRefreshedMsg:
		a.detail = a.detail.SetCommits(msg.commits)

	case prStatusMsg:
		a.detail = a.detail.SetPRStatus(msg.prStatus)
		if msg.prStatus != nil && msg.prStatus.State == "MERGED" {
			taskID := msg.taskID
			cmds = append(cmds, func() tea.Msg { return prMergedMsg{taskID} })
		}

	case prMergedMsg:
		// Fix 9: wrap MarkPRMerged in a cmd instead of calling synchronously
		pid := a.sidebar.SelectedProjectID()
		cmds = append(cmds, a.doMarkPRMerged(msg.taskID, pid))

	case reloadTasksMsg:
		if a.sidebar.InSprintListMode() {
			cmds = append(cmds, a.loadTasksFiltered(msg.projectID))
		} else {
			// "enter" on project in project list → drill to sprint list
			cmds = append(cmds, a.loadTasks(msg.projectID), a.loadSprints(msg.projectID, true))
		}

	case deletedTaskMsg:
		a.setStatus(fmt.Sprintf("deleted %s", msg.taskID))
		cmds = append(cmds, a.loadTasks(msg.projectID))

	case prMergedDoneMsg:
		a.setStatus(fmt.Sprintf("✓ %s auto-closed via merged PR", msg.taskID))
		cmds = append(cmds, a.loadTasks(msg.projectID))

	case forms.OpenBlockerPickerMsg:
		picker := forms.NewBlockerPicker(a.svc, a.width, a.height)
		a.blockerPicker = &picker
		a.blockerPickerField = msg.Field
		return a, picker.Init()

	case forms.BlockerPickedMsg:
		if a.form != nil && a.blockerPicker != nil {
			updated := a.form.AppendToBlocker(a.blockerPickerField, msg.ID)
			a.form = &updated
		}
		a.blockerPicker = nil
		a.blockerPickerField = ""
		return a, nil

	case forms.BlockerPickerCancelledMsg:
		a.blockerPicker = nil
		a.blockerPickerField = ""
		return a, nil

	case addBlockerMsg:
		pid := a.sidebar.SelectedProjectID()
		cmds = append(cmds, a.doAddBlocker(msg.blockerID, msg.blockedID, pid))

	case jumpToTaskMsg:
		a.board = a.board.SetCursorToTask(msg.taskID)
		a.focus = focusBoard
		a.syncFocus()
		for _, t := range a.currentTasks {
			if t.ID == msg.taskID {
				a.detail = a.detail.SetTask(t)
				cmds = append(cmds, a.refreshGit(t))
				break
			}
		}

	case statusNotifMsg:
		a.setStatus(msg.text)

	case tickMsg:
		cmds = append(cmds, a.pollPRs()...)
		cmds = append(cmds, a.tickPRPoll())

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Forward to focused sub-model
	if a.form == nil && a.sprintForm == nil {
		var cmd tea.Cmd
		switch a.focus {
		case focusSidebar:
			var m tea.Model
			m, cmd = a.sidebar.Update(msg)
			a.sidebar = m.(Sidebar)
		case focusBoard:
			var m tea.Model
			m, cmd = a.board.Update(msg)
			a.board = m.(Board)
		case focusDetail:
			var m tea.Model
			m, cmd = a.detail.Update(msg)
			a.detail = m.(Detail)
		}
		cmds = append(cmds, cmd)
	}

	return a, tea.Batch(cmds...)
}

func (a *App) handleKey(msg tea.KeyMsg) []tea.Cmd {
	var cmds []tea.Cmd
	switch msg.String() {
	case "q", "ctrl+c":
		return []tea.Cmd{tea.Quit}
	case "tab":
		a.focus = (a.focus + 1) % 3
		a.syncFocus()
	case "shift+tab":
		if a.focus == 0 {
			a.focus = 2
		} else {
			a.focus--
		}
		a.syncFocus()
	case "n":
		a.openNewForm()
	case "e":
		if id := a.board.SelectedTaskID(); id != "" {
			found := false
			for _, t := range a.currentTasks {
				if t.ID == id {
					found = true
					a.openEditForm(t)
					break
				}
			}
			if !found {
				a.setStatus("task not found in cache — press r to refresh")
			}
		}
	case "m":
		// Fix 6: search cache + wrap MoveTask in cmd
		if id := a.board.SelectedTaskID(); id != "" {
			for _, t := range a.currentTasks {
				if t.ID == id {
					next := t.Status.Next()
					pid := a.sidebar.SelectedProjectID()
					cmds = append(cmds, a.doMoveTask(id, next, pid))
					break
				}
			}
		}
	case "M":
		// Fix 6: search cache + wrap MoveTask in cmd
		if id := a.board.SelectedTaskID(); id != "" {
			for _, t := range a.currentTasks {
				if t.ID == id {
					prev := t.Status.Prev()
					pid := a.sidebar.SelectedProjectID()
					cmds = append(cmds, a.doMoveTask(id, prev, pid))
					break
				}
			}
		}
	case "d":
		// Fix 8: wrap DeleteTask in cmd
		if id := a.board.SelectedTaskID(); id != "" {
			pid := a.sidebar.SelectedProjectID()
			cmds = append(cmds, a.doDeleteTask(id, pid))
		}
	case "r":
		pid := a.sidebar.SelectedProjectID()
		cmds = append(cmds, a.loadTasks(pid), a.pollCurrentTaskGit())
	case "s":
		id := a.board.SelectedTaskID()
		if id == "" {
			break
		}
		if a.selectedSprintID == nil {
			a.setStatus("select a sprint first")
			break
		}
		pid := a.sidebar.SelectedProjectID()
		sprintID := a.selectedSprintID
		sprintFilterID := a.selectedSprintID
		cmds = append(cmds, func() tea.Msg {
			if _, err := a.svc.AssignTaskToSprint(id, sprintID); err != nil {
				return statusNotifMsg{fmt.Sprintf("error: %v", err)}
			}
			tasks, err := a.svc.ListTasks(pid, domain.TaskFilters{SprintID: sprintFilterID})
			if err != nil {
				return statusNotifMsg{fmt.Sprintf("error: %v", err)}
			}
			return tasksReloadedMsg{tasks}
		})
	case "S":
		id := a.board.SelectedTaskID()
		if id == "" {
			break
		}
		pid := a.sidebar.SelectedProjectID()
		sprintFilterID := a.selectedSprintID
		cmds = append(cmds, func() tea.Msg {
			if _, err := a.svc.AssignTaskToSprint(id, nil); err != nil {
				return statusNotifMsg{fmt.Sprintf("error: %v", err)}
			}
			tasks, err := a.svc.ListTasks(pid, domain.TaskFilters{SprintID: sprintFilterID})
			if err != nil {
				return statusNotifMsg{fmt.Sprintf("error: %v", err)}
			}
			return tasksReloadedMsg{tasks}
		})
	}
	return cmds
}

func (a *App) handleMouse(msg tea.MouseMsg) []tea.Cmd {
	if msg.Action == tea.MouseActionPress {
		if msg.X < a.sidebarWidth() {
			a.focus = focusSidebar
		} else if msg.X < a.sidebarWidth()+a.boardWidth() {
			a.focus = focusBoard
		} else {
			a.focus = focusDetail
		}
		a.syncFocus()
	}
	return nil
}

func (a *App) syncFocus() {
	a.sidebar = a.sidebar.SetFocused(a.focus == focusSidebar)
	a.board = a.board.SetFocused(a.focus == focusBoard)
	a.detail = a.detail.SetFocused(a.focus == focusDetail)
}

func (a *App) relayout() {
	sw := a.sidebarWidth()
	bw := a.boardWidth()
	dw := a.width - sw - bw
	h := a.height - 3

	a.sidebar = a.sidebar.SetSize(sw, h)
	// panelTop = 1 header row + 1 top border row
	a.board = a.board.SetSize(bw, h).SetPanelTop(2)
	a.detail = a.detail.SetSize(dw, h)
}

func (a App) sidebarWidth() int { return 18 }
func (a App) boardWidth() int   { return (a.width - a.sidebarWidth()) * 2 / 5 }

func (a *App) buildCounts() map[string]map[domain.Status]int {
	counts := make(map[string]map[domain.Status]int)
	for _, t := range a.currentTasks {
		if counts[t.ProjectID] == nil {
			counts[t.ProjectID] = make(map[domain.Status]int)
		}
		counts[t.ProjectID][t.Status]++
	}
	return counts
}

func (a *App) openNewForm() {
	pid := a.sidebar.SelectedProjectID()
	f := forms.New(pid, a.users, a.defaultAssignee, nil, a.width, a.height, a.sprints)
	a.form = &f
}

func (a *App) openEditForm(t *domain.Task) {
	pid := a.sidebar.SelectedProjectID()
	f := forms.New(pid, a.users, a.defaultAssignee, t, a.width, a.height, a.sprints)
	a.form = &f
}

// Fix 10: handleFormSaved wraps mutations in cmds; uses cache for edit.
func (a App) handleFormSaved(msg forms.SavedMsg) (tea.Model, tea.Cmd) {
	a.form = nil
	pid := a.sidebar.SelectedProjectID()
	if msg.IsNew {
		return a, a.doCreateTask(msg, pid)
	}
	// find task in cache for edit
	for _, t := range a.currentTasks {
		if t.ID == msg.TaskID {
			return a, a.doUpdateTask(msg, t)
		}
	}
	return a, nil
}

func (a App) handleSprintFormSaved(msg forms.SprintSavedMsg) (tea.Model, tea.Cmd) {
	a.sprintForm = nil
	pid := a.sidebar.SelectedProjectID()
	return a, func() tea.Msg {
		if _, err := a.svc.CreateSprint(msg.Name, pid, msg.Start, msg.End); err != nil {
			return statusNotifMsg{fmt.Sprintf("error creating sprint: %v", err)}
		}
		summaries, err := a.svc.ListSprintsWithCounts(pid)
		if err != nil {
			return statusNotifMsg{fmt.Sprintf("error reloading sprints: %v", err)}
		}
		return sprintsLoadedMsg{projectID: pid, summaries: summaries, enterMode: false}
	}
}

func sprintHeaderLine(sp *domain.Sprint) string {
	if sp == nil {
		return ""
	}
	dates := ""
	if sp.StartDate != nil && sp.EndDate != nil {
		dates = fmt.Sprintf("  ·  %s – %s",
			sp.StartDate.Format("Jan 2"),
			sp.EndDate.Format("Jan 2"))
	}
	return fmt.Sprintf("%s%s", sp.Name, dates)
}

func (a App) doCreateTask(msg forms.SavedMsg, projectID string) tea.Cmd {
	return func() tea.Msg {
		opts := domain.TaskCreateOpts{
			AssigneeID: msg.AssigneeID,
			Priority:   msg.Priority,
			Status:     msg.Status,
			Labels:     msg.Labels,
			Points:     msg.Points,
			SprintID:   msg.SprintID,
		}
		if _, err := a.svc.CreateTask(msg.Title, msg.Description, projectID, opts); err != nil {
			return statusNotifMsg{fmt.Sprintf("error: %v", err)}
		}
		return reloadTasksMsg{projectID}
	}
}

func (a App) doUpdateTask(msg forms.SavedMsg, t *domain.Task) tea.Cmd {
	updated := *t
	updated.Title = msg.Title
	updated.Description = msg.Description
	updated.Priority = msg.Priority
	updated.Status = msg.Status
	updated.Labels = msg.Labels
	updated.Points = msg.Points
	updated.SprintID = msg.SprintID
	if msg.AssigneeID != "" {
		s := msg.AssigneeID
		updated.AssigneeID = &s
	} else {
		updated.AssigneeID = nil
	}
	taskID := t.ID
	projectID := t.ProjectID
	oldBlockers := append([]string(nil), t.Blockers...)
	oldBlocking := append([]string(nil), t.Blocking...)
	newBlockedBy := msg.BlockedBy
	newBlocks := msg.Blocks

	return func() tea.Msg {
		if _, err := a.svc.UpdateTask(&updated); err != nil {
			return statusNotifMsg{fmt.Sprintf("error: %v", err)}
		}

		// Reconcile blocked-by (tasks that block this task)
		add, remove := diffBlockers(oldBlockers, newBlockedBy)
		for _, id := range remove {
			if err := a.svc.RemoveDep(id, taskID); err != nil {
				return statusNotifMsg{fmt.Sprintf("error removing blocker %s: %v", id, err)}
			}
		}
		for _, id := range add {
			if err := a.svc.AddDep(id, taskID); err != nil {
				return statusNotifMsg{fmt.Sprintf("error adding blocker %s: %v", id, err)}
			}
		}

		// Reconcile blocks (tasks that this task blocks)
		add, remove = diffBlockers(oldBlocking, newBlocks)
		for _, id := range remove {
			if err := a.svc.RemoveDep(taskID, id); err != nil {
				return statusNotifMsg{fmt.Sprintf("error removing block %s: %v", id, err)}
			}
		}
		for _, id := range add {
			if err := a.svc.AddDep(taskID, id); err != nil {
				return statusNotifMsg{fmt.Sprintf("error adding block %s: %v", id, err)}
			}
		}

		return reloadTasksMsg{projectID}
	}
}

func (a App) doMoveTask(id string, status domain.Status, projectID string) tea.Cmd {
	return func() tea.Msg {
		if _, err := a.svc.MoveTask(id, status); err != nil {
			return statusNotifMsg{fmt.Sprintf("error moving task: %v", err)}
		}
		return reloadTasksMsg{projectID}
	}
}

func (a App) doDeleteTask(id, projectID string) tea.Cmd {
	return func() tea.Msg {
		if err := a.svc.DeleteTask(id); err != nil {
			return statusNotifMsg{fmt.Sprintf("error deleting task: %v", err)}
		}
		return deletedTaskMsg{id, projectID}
	}
}

func (a App) doAddBlocker(blockerID, blockedID, projectID string) tea.Cmd {
	return func() tea.Msg {
		if err := a.svc.AddDep(blockerID, blockedID); err != nil {
			return statusNotifMsg{fmt.Sprintf("error adding blocker: %v", err)}
		}
		return reloadTasksMsg{projectID}
	}
}

func (a App) doMarkPRMerged(id, projectID string) tea.Cmd {
	return func() tea.Msg {
		if err := a.svc.MarkPRMerged(id); err != nil {
			return statusNotifMsg{fmt.Sprintf("error closing task: %v", err)}
		}
		return prMergedDoneMsg{taskID: id, projectID: projectID}
	}
}

// diffBlockers computes which IDs to add and which to remove when reconciling
// a blocker list. oldIDs is the current state; newIDs is the desired state.
func diffBlockers(oldIDs, newIDs []string) (toAdd, toRemove []string) {
	oldSet := make(map[string]bool, len(oldIDs))
	for _, id := range oldIDs {
		oldSet[id] = true
	}
	newSet := make(map[string]bool, len(newIDs))
	for _, id := range newIDs {
		newSet[id] = true
	}
	for id := range newSet {
		if !oldSet[id] {
			toAdd = append(toAdd, id)
		}
	}
	for id := range oldSet {
		if !newSet[id] {
			toRemove = append(toRemove, id)
		}
	}
	return
}

func (a App) refreshGit(t *domain.Task) tea.Cmd {
	client, ok := a.gitClients[t.ProjectID]
	branch := t.Branch
	if !ok || branch == "" {
		return nil
	}
	return func() tea.Msg {
		commits, err := client.CommitLog(branch, 5)
		if err != nil {
			return gitRefreshedMsg{}
		}
		cur, _ := client.CurrentBranch()
		return gitRefreshedMsg{branch: cur, commits: commits}
	}
}

// Fix 3: pollCurrentTaskGit uses cache lookups instead of svc.GetTask/GetProject.
func (a App) pollCurrentTaskGit() tea.Cmd {
	id := a.board.SelectedTaskID()
	if id == "" {
		return nil
	}
	var task *domain.Task
	for _, t := range a.currentTasks {
		if t.ID == id {
			task = t
			break
		}
	}
	if task == nil || task.PRNumber == nil {
		return nil
	}
	var repoPath string
	for _, p := range a.projects {
		if p.ID == task.ProjectID {
			repoPath = p.RepoPath
			break
		}
	}
	if repoPath == "" {
		return nil
	}
	prNum := *task.PRNumber
	taskID := task.ID
	return func() tea.Msg {
		status, err := git.PRView(repoPath, prNum)
		if err != nil {
			return statusNotifMsg{fmt.Sprintf("PR check failed: %v", err)}
		}
		return prStatusMsg{taskID: taskID, prStatus: status}
	}
}

// Fix 2: pollPRs uses cache lookups instead of svc.GetProject.
func (a App) pollPRs() []tea.Cmd {
	var cmds []tea.Cmd
	for _, t := range a.currentTasks {
		if t.Status != domain.StatusReview || t.PRNumber == nil {
			continue
		}
		var repoPath string
		for _, p := range a.projects {
			if p.ID == t.ProjectID {
				repoPath = p.RepoPath
				break
			}
		}
		if repoPath == "" {
			continue
		}
		prNum := *t.PRNumber
		taskID := t.ID
		cmds = append(cmds, func() tea.Msg {
			status, err := git.PRView(repoPath, prNum)
			if err != nil {
				return statusNotifMsg{fmt.Sprintf("PR check failed: %v", err)}
			}
			return prStatusMsg{taskID: taskID, prStatus: status}
		})
	}
	return cmds
}

func (a *App) setStatus(msg string) {
	a.statusMsg = msg
	a.statusExpiry = time.Now().Add(4 * time.Second)
}

func (a App) View() string {
	if a.width == 0 {
		return "loading..."
	}

	// Fix 1: search projects cache instead of calling svc.GetProject
	project := ""
	pid := a.sidebar.SelectedProjectID()
	for _, p := range a.projects {
		if p.ID == pid {
			project = p.Name
			break
		}
	}

	header := styles.Header.Width(a.width).Render(
		styles.Logo.Render("⬡ KeroAgile") + "  " +
			styles.Muted.Render(project),
	)

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		a.sidebar.View(),
		a.board.View(),
		a.detail.View(),
	)

	statusText := "[n]ew  [e]dit  [m]ove  [d]del  [b]lock  [/]filter  [→]jump-blocker  [tab]focus  [r]refresh  [q]quit"
	if a.statusMsg != "" && time.Now().Before(a.statusExpiry) {
		statusText = a.statusMsg
	}
	statusBar := styles.StatusBar.Width(a.width).Render(statusText)

	if a.blockerPicker != nil {
		return a.blockerPicker.View()
	}

	if a.sprintForm != nil {
		return a.sprintForm.View()
	}

	if a.form != nil {
		return a.form.View()
	}

	return header + "\n" + body + "\n" + statusBar
}

// Ensure App implements tea.Model
var _ tea.Model = (*App)(nil)
