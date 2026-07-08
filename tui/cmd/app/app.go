package main

import (
	"fmt"
	"os"
	"strings"

	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ignorant05/control/shared"
	"github.com/ignorant05/control/tui/pkgs/client"
	"github.com/ignorant05/control/tui/styles"
	"github.com/ignorant05/control/tui/types"
	"github.com/ignorant05/control/tui/views"
)

// appState represents the current high-level state of the application
type appState int

const (
	stateLogin appState = iota
	stateLoading
	stateMain
)

// appView represents the active tab/view
type appView int

const (
	viewFlags appView = iota
	viewUsers
	viewProjects
)

// Messages
type loginSuccessMsg struct {
	client   *client.APIClient
	token    string
	user     shared.User
	projects []*shared.Project
	users    []*shared.User
	flags    []*shared.FeatureFlag
}

type loginErrMsg struct{ err error }
type refreshDataMsg struct {
	projects []*shared.Project
	users    []*shared.User
	flags    []*shared.FeatureFlag
	auditLog []*shared.AuditEntry
}
type refreshErrMsg struct{ err error }
type wsUpdateMsg struct{ updateType string }
type apiOpDoneMsg struct{ err error }
type tickMsg struct{}

// appModel represents all views properties
type appModel struct {
	state      appState
	activeView appView

	apiClient *client.APIClient

	program *tea.Program

	urlInput   textinput.Model
	userInput  textinput.Model
	passInput  textinput.Model
	loginFocus int
	loginErr   string

	session  types.SessionUser
	user     shared.User
	users    []*shared.User
	projects []*shared.Project
	flags    []*shared.FeatureFlag

	flagsView    views.FlagsView
	usersView    views.UsersView
	projectsView views.ProjectsView

	height int
	width  int
}

func newAppModel() *appModel {
	urlTi := textinput.New()
	urlTi.Placeholder = "http://localhost:8080"
	urlTi.CharLimit = 128
	urlTi.SetValue("http://localhost:8080")
	urlTi.Width = 40

	userTi := textinput.New()
	userTi.Placeholder = "username"
	userTi.CharLimit = 48
	userTi.Width = 40

	passTi := textinput.New()
	passTi.Placeholder = "password"
	passTi.EchoMode = textinput.EchoPassword
	passTi.EchoCharacter = '*'
	passTi.CharLimit = 48
	passTi.Width = 40

	urlTi.Focus()

	return &appModel{
		state:      stateLogin,
		urlInput:   urlTi,
		userInput:  userTi,
		passInput:  passTi,
		loginFocus: 0,
	}
}

// setProgram gives the model access to program.Send() for WS callbacks
func (m *appModel) setProgram(p *tea.Program) {
	m.program = p
}

// Init initialize the ead cmd rendering
func (m *appModel) Init() tea.Cmd {
	return nil
}

// tickCmd returns a command that sends a tickMsg every 5 seconds
// This ensures the TUI stays up-to-date even if WebSocket messages
// are missed or the connection is unstable.
func tickCmd() tea.Cmd {
	return tea.Every(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

// Update updates the view based on current user behaviour (position)
func (m *appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height - 2
		if m.state == stateMain {
			m.resizeViews()
		}
		return m, nil

	case loginSuccessMsg:
		m.state = stateMain
		m.apiClient = msg.client
		m.apiClient.SetToken(msg.token)
		m.user = msg.user
		m.session = types.SessionUser{
			Name:       msg.user.Name,
			Role:       msg.user.Role,
			Project:    msg.user.Project,
			Privileges: shared.PrivilegesString(msg.user.Role.Privileges()),
		}
		m.projects = msg.projects
		m.users = msg.users
		m.flags = msg.flags

		m.apiClient.OnFlagChange = func(msg shared.WSMessage) {
			if m.program != nil {
				m.program.Send(wsUpdateMsg{updateType: "flag"})
			}
		}
		m.apiClient.OnUserChange = func(msg shared.WSMessage) {
			if m.program != nil {
				m.program.Send(wsUpdateMsg{updateType: "user"})
			}
		}
		m.apiClient.OnAudit = func(msg shared.WSMessage) {
			if m.program != nil {
				m.program.Send(wsUpdateMsg{updateType: "audit"})
			}
		}

		go func() {
			if err := m.apiClient.ConnectWebSocket(); err != nil {
				fmt.Fprintf(os.Stderr, "WebSocket connection failed: %v\n", err)
			}
		}()

		m.initViews()
		m.resizeViews()
		return m, tickCmd()

	case loginErrMsg:
		m.state = stateLogin
		m.loginErr = msg.err.Error()
		return m, nil

	case refreshDataMsg:
		if msg.projects != nil {
			m.projects = msg.projects
			m.projectsView.SetProjects(msg.projects)
		}
		if msg.users != nil {
			m.users = msg.users
			m.projectsView.SetUsers(msg.users)
			m.usersView = m.usersView.SetUsers(msg.users)
		}
		if msg.flags != nil {
			m.flags = msg.flags
			m.flagsView = m.flagsView.SetFlags(msg.flags)
		}
		if msg.auditLog != nil {
			m.flagsView = m.flagsView.SetAuditLog(msg.auditLog)
		}
		return m, nil

	case refreshErrMsg:
		switch m.activeView {
		case viewFlags:
			m.flagsView = m.flagsView.SetStatus("Error: "+msg.err.Error(), true)
		case viewUsers:
			m.usersView = m.usersView.SetStatus("Error: "+msg.err.Error(), true)
		}
		return m, nil

	case wsUpdateMsg:
		if msg.updateType == "audit" {
			return m, m.apiFetchAuditLog()
		}
		return m, m.refreshAllData()

	case tickMsg:
		if m.state == stateMain && m.apiClient != nil {
			return m, tea.Batch(m.refreshAllData(), tickCmd())
		}
		return m, tickCmd()

	case apiOpDoneMsg:
		if msg.err != nil {
			switch m.activeView {
			case viewFlags:
				m.flagsView = m.flagsView.SetStatus("Error: "+msg.err.Error(), true)
			case viewUsers:
				m.usersView = m.usersView.SetStatus("Error: "+msg.err.Error(), true)
			}
		}
		return m, m.refreshAllData()

	case tea.KeyMsg:
		if m.state == stateLogin {
			model, cmd := m.handleLoginKey(msg)
			return model, cmd
		}
		if m.state == stateMain {
			model, cmd := m.handleMainKey(msg)
			return model, cmd
		}
	}

	return m, nil
}

// Login Handling
func (m *appModel) handleLoginKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "down":
		m.loginFocus = (m.loginFocus + 1) % 3
		m.updateLoginFocus()
		return m, nil
	case "shift+tab", "up":
		m.loginFocus = (m.loginFocus - 1 + 3) % 3
		m.updateLoginFocus()
		return m, nil
	case "enter":
		if m.loginFocus < 2 {
			m.loginFocus++
			m.updateLoginFocus()
			return m, nil
		}
		return m, m.doLogin()
	case "esc", "q", "ctrl+c":
		return m, tea.Quit
	}

	var cmd tea.Cmd
	switch m.loginFocus {
	case 0:
		m.urlInput, cmd = m.urlInput.Update(msg)
	case 1:
		m.userInput, cmd = m.userInput.Update(msg)
	case 2:
		m.passInput, cmd = m.passInput.Update(msg)
	}
	return m, cmd
}

func (m *appModel) updateLoginFocus() {
	m.urlInput.Blur()
	m.userInput.Blur()
	m.passInput.Blur()
	switch m.loginFocus {
	case 0:
		m.urlInput.Focus()
	case 1:
		m.userInput.Focus()
	case 2:
		m.passInput.Focus()
	}
}

func (m *appModel) doLogin() tea.Cmd {
	return func() tea.Msg {
		c := client.NewAPIClient(m.urlInput.Value())
		resp, err := c.Login(m.userInput.Value(), m.passInput.Value())
		if err != nil {
			return loginErrMsg{err: err}
		}
		c.SetToken(resp.Token)

		projects, err := c.ListProjects()
		if err != nil {
			return loginErrMsg{err: fmt.Errorf("list projects: %w", err)}
		}

		users, err := c.ListUsers()
		if err != nil {
			return loginErrMsg{err: fmt.Errorf("list users: %w", err)}
		}

		flags, err := c.ListFlags()
		if err != nil {
			return loginErrMsg{err: fmt.Errorf("list flags: %w", err)}
		}

		return loginSuccessMsg{
			client:   c,
			token:    resp.Token,
			user:     resp.User,
			projects: projects,
			users:    users,
			flags:    flags,
		}
	}
}

// Main App Handling
func (m *appModel) handleMainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "q" || msg.String() == "ctrl+c" {
		if m.activeView == viewFlags && views.IsFocused(&m.flagsView) {
		} else if m.activeView == viewUsers && m.usersView.InInputMode() {
		} else {
			return m, tea.Quit
		}
	}

	if m.activeView != viewFlags || !m.flagsView.BlocksArrowNav() {
		if m.activeView != viewUsers || !m.usersView.InInputMode() {
			switch msg.String() {
			case "left", "right":
				forward := msg.String() == "right"
				m.activeView = nextView(m.activeView, forward)
				return m, nil
			}
		}
	}

	if m.activeView == viewProjects && msg.String() == "enter" {
		if p := m.projectsView.SelectedProject(); p != nil {
			var projectFlags []*shared.FeatureFlag
			for _, f := range m.flags {
				if f.ProjectID == p.ID {
					projectFlags = append(projectFlags, f)
				}
			}
			m.flagsView = views.NewFlagsView(m.user, p.Name, projectFlags, nil)
			m.flagsView = m.flagsView.SetFlagsViewSize(m.height-7, m.width)
			m.activeView = viewFlags
		}
		return m, nil
	}

	var cmd tea.Cmd
	switch m.activeView {
	case viewFlags:
		m.flagsView, cmd = m.flagsView.Update(msg)
		if op := m.flagsView.TakeOp(); op != nil {
			return m.handleFlagOp(op)
		}
	case viewUsers:
		m.usersView, cmd = m.usersView.Update(msg, m.users)
		if op := m.usersView.TakeOp(); op != nil {
			return m.handleUserOp(op)
		}
	case viewProjects:
		m.projectsView, cmd = m.projectsView.Update(msg)
	}

	return m, cmd
}

func nextView(current appView, forward bool) appView {
	order := []appView{viewFlags, viewUsers, viewProjects}
	idx := 0
	for i, v := range order {
		if v == current {
			idx = i
			break
		}
	}
	if forward {
		idx = (idx + 1) % len(order)
	} else {
		idx = (idx - 1 + len(order)) % len(order)
	}
	return order[idx]
}

// Operation Handlers
func (m *appModel) handleFlagOp(op *views.FlagOp) (tea.Model, tea.Cmd) {
	switch op.Type {
	case "toggle":
		return m, m.apiToggleFlag(op.FlagID)
	case "rollout":
		return m, m.apiUpdateRollout(op.FlagID, op.Rollout)
	case "kill":
		return m, m.apiKillFlag(op.FlagID)
	case "create":
		return m, m.apiCreateFlag(op.Name, op.Description)
	case "update":
		return m, m.apiUpdateFlag(op.FlagID, op.Name, op.Description)
	case "delete":
		return m, m.apiDeleteFlag(op.FlagID)
	case "audit":
		return m, m.apiFetchAuditLog()
	}
	return m, nil
}

func (m *appModel) handleUserOp(op *views.UserOp) (tea.Model, tea.Cmd) {
	switch op.Type {
	case "create":
		return m, m.apiCreateUser(op.NewUser)
	case "role":
		return m, m.apiUpdateRole(op.UserID, op.Role)
	case "password":
		return m, m.apiUpdatePassword(op.UserID, op.Password)
	case "delete":
		return m, m.apiDeleteUser(op.UserID)
	case "presence":
		return m, m.apiTogglePresence(op.UserID)
	case "switch":
		return m, m.apiSwitchUser(op.UserName, op.Password)
	}
	return m, nil
}

// API Commands
func (m *appModel) apiToggleFlag(flagID string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.apiClient.ToggleFlag(flagID)
		return apiOpDoneMsg{err: err}
	}
}

func (m *appModel) apiUpdateRollout(flagID string, rollout int) tea.Cmd {
	return func() tea.Msg {
		_, err := m.apiClient.UpdateRollout(flagID, rollout)
		return apiOpDoneMsg{err: err}
	}
}

func (m *appModel) apiKillFlag(flagID string) tea.Cmd {
	return func() tea.Msg {
		err := m.apiClient.KillFlag(flagID)
		return apiOpDoneMsg{err: err}
	}
}

func (m *appModel) apiCreateFlag(name, description string) tea.Cmd {
	return func() tea.Msg {
		flag := &shared.FeatureFlag{
			Name:        name,
			Description: description,
			Status:      shared.StatusOff,
			Rollout:     0,
		}
		_, err := m.apiClient.CreateFlag(flag)
		return apiOpDoneMsg{err: err}
	}
}

func (m *appModel) apiUpdateFlag(flagID, name, description string) tea.Cmd {
	return func() tea.Msg {
		flag := &shared.FeatureFlag{
			ID:          flagID,
			Name:        name,
			Description: description,
		}
		_, err := m.apiClient.UpdateFlag(flag)
		return apiOpDoneMsg{err: err}
	}
}

func (m *appModel) apiDeleteFlag(flagID string) tea.Cmd {
	return func() tea.Msg {
		err := m.apiClient.DeleteFlag(flagID)
		return apiOpDoneMsg{err: err}
	}
}

func (m *appModel) apiFetchAuditLog() tea.Cmd {
	return func() tea.Msg {
		entries, err := m.apiClient.GetAuditLog()
		if err != nil {
			return refreshErrMsg{err: err}
		}
		return refreshDataMsg{auditLog: entries}
	}
}

func (m *appModel) apiCreateUser(user *shared.User) tea.Cmd {
	return func() tea.Msg {
		_, err := m.apiClient.CreateUser(user)
		return apiOpDoneMsg{err: err}
	}
}

func (m *appModel) apiUpdateRole(userID string, role shared.Role) tea.Cmd {
	return func() tea.Msg {
		err := m.apiClient.UpdateUserRole(userID, role)
		return apiOpDoneMsg{err: err}
	}
}

func (m *appModel) apiUpdatePassword(userID, password string) tea.Cmd {
	return func() tea.Msg {
		err := m.apiClient.UpdatePassword(userID, password)
		return apiOpDoneMsg{err: err}
	}
}

func (m *appModel) apiDeleteUser(userID string) tea.Cmd {
	return func() tea.Msg {
		err := m.apiClient.DeleteUser(userID)
		return apiOpDoneMsg{err: err}
	}
}

func (m *appModel) apiTogglePresence(userID string) tea.Cmd {
	return func() tea.Msg {
		err := m.apiClient.TogglePresence(userID)
		return apiOpDoneMsg{err: err}
	}
}

func (m *appModel) apiSwitchUser(username, password string) tea.Cmd {
	return func() tea.Msg {
		resp, err := m.apiClient.Login(username, password)
		if err != nil {
			return refreshErrMsg{err: fmt.Errorf("switch user failed: %w", err)}
		}
		m.apiClient.SetToken(resp.Token)

		projects, _ := m.apiClient.ListProjects()
		users, _ := m.apiClient.ListUsers()
		flags, _ := m.apiClient.ListFlags()

		return loginSuccessMsg{
			client:   m.apiClient,
			token:    resp.Token,
			user:     resp.User,
			projects: projects,
			users:    users,
			flags:    flags,
		}
	}
}

func (m *appModel) refreshAllData() tea.Cmd {
	return func() tea.Msg {
		projects, _ := m.apiClient.ListProjects()
		users, _ := m.apiClient.ListUsers()
		flags, _ := m.apiClient.ListFlags()
		audit, _ := m.apiClient.GetAuditLog()
		return refreshDataMsg{
			projects: projects,
			users:    users,
			flags:    flags,
			auditLog: audit,
		}
	}
}

// View Initialization
func (m *appModel) initViews() {
	var userProjectName string
	for _, p := range m.projects {
		if p.Name == m.user.Project {
			userProjectName = p.Name
			break
		}
	}
	if userProjectName == "" && len(m.projects) > 0 {
		userProjectName = m.projects[0].Name
	}

	var projectFlags []*shared.FeatureFlag
	for _, f := range m.flags {
		if !f.Killed {
			projectFlags = append(projectFlags, f)
		}
	}

	m.flagsView = views.NewFlagsView(m.user, userProjectName, projectFlags, nil)
	m.usersView = views.NewUsersView(m.users, m.user.Name, m.user.Project, m.user.Role == shared.RoleAdmin)
	m.projectsView = views.NewProjectsView(m.projects, m.users)
}

func (m *appModel) resizeViews() {
	const overheadLines = 7
	availableHeight := m.height - overheadLines

	margin := m.width / 20
	contentWidth := m.width - (margin * 3 / 2)

	m.flagsView = m.flagsView.SetFlagsViewSize(availableHeight, contentWidth)
	m.usersView = m.usersView.SetUsersViewSize(availableHeight, contentWidth)
	m.projectsView = m.projectsView.SetProjectsViewSize(availableHeight, contentWidth)
}

// View Rendering
func (m *appModel) View() string {
	switch m.state {
	case stateLogin:
		return m.renderLogin()
	case stateMain:
		return m.renderMain()
	default:
		return "Loading..."
	}
}

func (m *appModel) renderLogin() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		MarginBottom(1).
		Render("\tControl Feature Flags TUI\t")
	b.WriteString(title)
	b.WriteString("\n\n")

	inputStyle := lipgloss.NewStyle().
		Width(50).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1)
	focusStyle := lipgloss.NewStyle().
		Width(50).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("212")).
		Padding(0, 1)
	labelStyle := lipgloss.NewStyle().
		Width(16).
		Align(lipgloss.Right).
		Foreground(lipgloss.Color("230"))

	row := func(label, field string, focused bool) string {
		style := inputStyle
		if focused {
			style = focusStyle
		}
		box := style.Render(field)
		return lipgloss.JoinHorizontal(lipgloss.Center, labelStyle.Render(label)+" ", box)
	}

	b.WriteString(row("API URL:", m.urlInput.View(), m.loginFocus == 0))
	b.WriteString("\n\n")

	b.WriteString(row("Username:", m.userInput.View(), m.loginFocus == 1))
	b.WriteString("\n\n")

	b.WriteString(row("Password:", m.passInput.View(), m.loginFocus == 2))
	b.WriteString("\n\n")

	connectStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("42")).
		Foreground(lipgloss.Color("0")).
		Padding(0, 2).
		Bold(true).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("42"))
	b.WriteString(connectStyle.Render("[ Enter ] Connect"))
	b.WriteString("\n\n")

	if m.loginErr != "" {
		errStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("203")).
			Bold(true).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("203")).
			Padding(0, 1)
		b.WriteString(errStyle.Render("Error: " + m.loginErr))
		b.WriteString("\n")
	}

	help := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		"[tab] switch field  [enter] connect  [q] quit",
	)
	b.WriteString(help)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m *appModel) renderMain() string {
	var b strings.Builder

	b.WriteString(m.renderTabs())
	b.WriteString("\n\n")

	switch m.activeView {
	case viewFlags:
		b.WriteString(m.flagsView.View(m.session))
	case viewUsers:
		b.WriteString(m.usersView.View(m.session, m.users))
	case viewProjects:
		b.WriteString(m.projectsView.View(m.session))
	}

	return b.String()
}

func (m *appModel) renderTabs() string {
	tabs := []struct {
		id    appView
		label string
	}{
		{viewFlags, "1: Flags"},
		{viewUsers, "2: Users"},
		{viewProjects, "3: Projects"},
	}
	var rendered []string
	for _, t := range tabs {
		if t.id == m.activeView {
			rendered = append(rendered, styles.TabActiveStyle.Render(t.label))
		} else {
			rendered = append(rendered, styles.TabInactiveStyle.Render(t.label))
		}
	}
	return strings.Join(rendered, " ")
}

// Graceful Shutdown
func (m *appModel) Shutdown() {
	if m.apiClient != nil {
		m.apiClient.DisconnectWebSocket()
	}
}
