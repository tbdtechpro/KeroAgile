package tui

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"keroagile/internal/domain"
	"keroagile/internal/git"
	"keroagile/internal/tui/forms"
	"keroagile/internal/tui/styles"
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
		return struct{ projects []*domain.Project }{projects}
	}
}

func (a App) loadUsers() tea.Cmd {
	return func() tea.Msg {
		users, err := a.svc.ListUsers()
		if err != nil {
			return statusNotifMsg{fmt.Sprintf("error loading users: %v", err)}
		}
		return struct{ users []*domain.User }{users}
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

	case struct{ projects []*domain.Project }:
		a.projects = msg.projects
		counts := a.buildCounts()
		a.sidebar = NewSidebar(a.projects, counts, a.sidebarWidth(), a.height-3)
		if len(a.projects) > 0 {
			cmds = append(cmds, a.loadTasks(a.projects[0].ID))
			if p := a.projects[0]; p.RepoPath != "" {
				a.gitClients[p.ID] = git.New(p.RepoPath)
			}
		}

	case struct{ users []*domain.User }:
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
		t, err := a.svc.GetTask(msg.taskID)
		if err == nil {
			a.detail = a.detail.SetTask(t)
			cmds = append(cmds, a.refreshGit(t))
		}

	case taskMovedMsg:
		if _, err := a.svc.MoveTask(msg.taskID, msg.status); err == nil {
			pid := a.sidebar.SelectedProjectID()
			cmds = append(cmds, a.loadTasks(pid))
		}

	case gitRefreshedMsg:
		a.detail = a.detail.SetCommits(msg.commits)

	case prStatusMsg:
		a.detail = a.detail.SetPRStatus(msg.prStatus)
		if msg.prStatus != nil && msg.prStatus.State == "MERGED" {
			taskID := msg.taskID
			cmds = append(cmds, func() tea.Msg { return prMergedMsg{taskID} })
		}

	case prMergedMsg:
		if err := a.svc.MarkPRMerged(msg.taskID); err == nil {
			pid := a.sidebar.SelectedProjectID()
			a.setStatus(fmt.Sprintf("✓ %s auto-closed via merged PR", msg.taskID))
			cmds = append(cmds, a.loadTasks(pid))
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
			if t, err := a.svc.GetTask(id); err == nil {
				a.openEditForm(t)
			}
		}
	case "m":
		if id := a.board.SelectedTaskID(); id != "" {
			if t, err := a.svc.GetTask(id); err == nil {
				next := t.Status.Next()
				if _, err := a.svc.MoveTask(id, next); err == nil {
					pid := a.sidebar.SelectedProjectID()
					return []tea.Cmd{a.loadTasks(pid)}
				}
			}
		}
	case "M":
		if id := a.board.SelectedTaskID(); id != "" {
			if t, err := a.svc.GetTask(id); err == nil {
				prev := t.Status.Prev()
				if _, err := a.svc.MoveTask(id, prev); err == nil {
					pid := a.sidebar.SelectedProjectID()
					return []tea.Cmd{a.loadTasks(pid)}
				}
			}
		}
	case "d":
		if id := a.board.SelectedTaskID(); id != "" {
			if err := a.svc.DeleteTask(id); err == nil {
				pid := a.sidebar.SelectedProjectID()
				a.setStatus(fmt.Sprintf("deleted %s", id))
				return []tea.Cmd{a.loadTasks(pid)}
			}
		}
	case "r":
		pid := a.sidebar.SelectedProjectID()
		return []tea.Cmd{a.loadTasks(pid), a.pollCurrentTaskGit()}
	}
	return nil
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
	a.board = a.board.SetSize(bw, h)
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

func (a App) handleFormSaved(msg forms.SavedMsg) (tea.Model, tea.Cmd) {
	a.form = nil
	var err error
	if msg.IsNew {
		opts := domain.TaskCreateOpts{
			AssigneeID: msg.AssigneeID,
			Priority:   msg.Priority,
			Status:     msg.Status,
			Labels:     msg.Labels,
			Points:     msg.Points, // both *int
		}
		_, err = a.svc.CreateTask(msg.Title, msg.Description, a.sidebar.SelectedProjectID(), opts)
	} else {
		t, gerr := a.svc.GetTask(msg.TaskID)
		if gerr == nil {
			t.Title = msg.Title
			t.Description = msg.Description
			t.Priority = msg.Priority
			t.Status = msg.Status
			t.Labels = msg.Labels
			if msg.AssigneeID != "" {
				t.AssigneeID = &msg.AssigneeID
			}
			t.Points = msg.Points
			_, err = a.svc.UpdateTask(t)
		}
	}
	if err != nil {
		a.setStatus(fmt.Sprintf("error: %v", err))
		return a, nil
	}
	return a, a.loadTasks(a.sidebar.SelectedProjectID())
}

func (a App) refreshGit(t *domain.Task) tea.Cmd {
	client, ok := a.gitClients[t.ProjectID]
	if !ok || t.Branch == "" {
		return nil
	}
	return func() tea.Msg {
		commits, err := client.CommitLog(t.Branch, 5)
		if err != nil {
			return gitRefreshedMsg{}
		}
		branch, _ := client.CurrentBranch()
		return gitRefreshedMsg{branch: branch, commits: commits}
	}
}

func (a App) pollCurrentTaskGit() tea.Cmd {
	id := a.board.SelectedTaskID()
	if id == "" {
		return nil
	}
	t, err := a.svc.GetTask(id)
	if err != nil || t.PRNumber == nil {
		return nil
	}
	p, err := a.svc.GetProject(t.ProjectID)
	if err != nil || p.RepoPath == "" {
		return nil
	}
	prNum := *t.PRNumber
	taskID := t.ID
	repoPath := p.RepoPath
	return func() tea.Msg {
		status, err := git.PRView(repoPath, prNum)
		if err != nil {
			return nil
		}
		return prStatusMsg{taskID: taskID, prStatus: status}
	}
}

func (a App) pollPRs() []tea.Cmd {
	var cmds []tea.Cmd
	for _, t := range a.currentTasks {
		if t.Status == domain.StatusReview && t.PRNumber != nil {
			t := t
			p, err := a.svc.GetProject(t.ProjectID)
			if err != nil || p.RepoPath == "" {
				continue
			}
			prNum := *t.PRNumber
			taskID := t.ID
			repoPath := p.RepoPath
			cmds = append(cmds, func() tea.Msg {
				status, err := git.PRView(repoPath, prNum)
				if err != nil {
					return nil
				}
				return prStatusMsg{taskID: taskID, prStatus: status}
			})
		}
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

	project := ""
	if pid := a.sidebar.SelectedProjectID(); pid != "" {
		if p, err := a.svc.GetProject(pid); err == nil {
			project = p.Name
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

func logErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
