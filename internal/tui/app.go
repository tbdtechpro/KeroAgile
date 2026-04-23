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
	svc        *domain.Service
	gitClients map[string]*git.Client

	sidebar Sidebar
	board   Board
	detail  Detail

	focus panelFocus
	form  *forms.TaskForm

	projects     []*domain.Project
	currentTasks []*domain.Task
	users        []*domain.User

	statusMsg    string
	statusExpiry time.Time

	width  int
	height int

	spinner spinner.Model
	loading bool
}

// New creates the App. Call Run() to start the BubbleTea event loop.
func New(svc *domain.Service) *App {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return &App{
		svc:        svc,
		gitClients: make(map[string]*git.Client),
		spinner:    sp,
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

func (a App) tickPRPoll() tea.Cmd {
	return tea.Tick(60*time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

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
		cmds = append(cmds, a.loadTasks(msg.projectID))

	case deletedTaskMsg:
		a.setStatus(fmt.Sprintf("deleted %s", msg.taskID))
		cmds = append(cmds, a.loadTasks(msg.projectID))

	case prMergedDoneMsg:
		a.setStatus(fmt.Sprintf("✓ %s auto-closed via merged PR", msg.taskID))
		cmds = append(cmds, a.loadTasks(msg.projectID))

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
	if a.form == nil {
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
	f := forms.New(pid, a.users, nil, a.width, a.height)
	a.form = &f
}

func (a *App) openEditForm(t *domain.Task) {
	pid := a.sidebar.SelectedProjectID()
	f := forms.New(pid, a.users, t, a.width, a.height)
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

func (a App) doCreateTask(msg forms.SavedMsg, projectID string) tea.Cmd {
	return func() tea.Msg {
		opts := domain.TaskCreateOpts{
			AssigneeID: msg.AssigneeID,
			Priority:   msg.Priority,
			Status:     msg.Status,
			Labels:     msg.Labels,
			Points:     msg.Points,
		}
		if _, err := a.svc.CreateTask(msg.Title, msg.Description, projectID, opts); err != nil {
			return statusNotifMsg{fmt.Sprintf("error: %v", err)}
		}
		return reloadTasksMsg{projectID}
	}
}

func (a App) doUpdateTask(msg forms.SavedMsg, t *domain.Task) tea.Cmd {
	// copy to avoid mutation of shared pointer
	updated := *t
	updated.Title = msg.Title
	updated.Description = msg.Description
	updated.Priority = msg.Priority
	updated.Status = msg.Status
	updated.Labels = msg.Labels
	updated.Points = msg.Points
	if msg.AssigneeID != "" {
		s := msg.AssigneeID
		updated.AssigneeID = &s
	} else {
		updated.AssigneeID = nil
	}
	projectID := t.ProjectID
	return func() tea.Msg {
		if _, err := a.svc.UpdateTask(&updated); err != nil {
			return statusNotifMsg{fmt.Sprintf("error: %v", err)}
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

func (a App) doMarkPRMerged(id, projectID string) tea.Cmd {
	return func() tea.Msg {
		if err := a.svc.MarkPRMerged(id); err != nil {
			return statusNotifMsg{fmt.Sprintf("error closing task: %v", err)}
		}
		return prMergedDoneMsg{taskID: id, projectID: projectID}
	}
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

	statusText := "[n]ew  [e]dit  [m]ove  [d]del  [tab]focus  [r]refresh  [q]quit"
	if a.statusMsg != "" && time.Now().Before(a.statusExpiry) {
		statusText = a.statusMsg
	}
	statusBar := styles.StatusBar.Width(a.width).Render(statusText)

	if a.form != nil {
		return a.form.View()
	}

	return header + "\n" + body + "\n" + statusBar
}

// Ensure App implements tea.Model
var _ tea.Model = (*App)(nil)
