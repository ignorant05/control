package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ignorant05/control/shared"
	"github.com/ignorant05/control/tui/styles"
	"github.com/ignorant05/control/tui/types"
)

// allRoles represents all available roles
var allRoles = []shared.Role{shared.RoleAdmin, shared.RoleEditor, shared.RoleViewer}

// roleKeyMap maps keys to role indices for quick selection
var roleKeyMap = map[string]int{
	"1": 0, // Admin
	"2": 1, // Editor
	"3": 2, // Viewer
}

// rbacMode current rbac mode code
type rbacMode int

// represents all rbac operations
const (
	rbacBrowse rbacMode = iota
	rbacAddName
	rbacAddPassword
	rbacConfirmPassword
	rbacEditRole
	rbacConfirmDelete
	rbacChangePassword
	rbacConfirmNewPassword
	rbacLoginPassword
	rbacConfirmAdminSelfDemotion
)

// allowed password length interval
const minPasswordLen = 8
const maxPasswordLen = 12

// UserOp represents a pending user operation for the controller to execute via API
type UserOp struct {
	// "create", "role", "password", "delete", "presence", "switch"
	Type string

	UserID   string
	UserName string
	Role     shared.Role
	Password string
	NewUser  *shared.User
}

// UsersView represents users view components
type UsersView struct {
	currentName    string
	currentProject string
	isAdmin        bool

	tbl   table.Model
	mode  rbacMode
	input textinput.Model

	pendingName     string
	pendingPassword string

	pendingRoleIdx int
	revealed       map[int]bool

	pwTargetRow        int
	pendingNewPassword string

	loginTargetRow int

	statusMsg string
	statusErr bool

	pendingOp *UserOp
}

// NewUsersView creates the users dashboard view
func NewUsersView(users []*shared.User, currentName string, currentProject string, isAdmin bool) UsersView {
	columns := []table.Column{
		{Title: "User", Width: 18},
		{Title: "Role", Width: 8},
		{Title: "Project", Width: 12},
		{Title: "Privileges", Width: 20},
		{Title: "Session", Width: 8},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	ti := textinput.New()
	ti.CharLimit = 48

	v := UsersView{
		currentName:    currentName,
		currentProject: currentProject,
		isAdmin:        isAdmin,
		tbl:            t,
		input:          ti,
		revealed:       map[int]bool{},
	}
	v.refreshRows(users)
	return v
}

// refreshRows refreshes view on update
func (v *UsersView) refreshRows(users []*shared.User) {
	rows := make([]table.Row, 0, len(users))
	for _, u := range users {
		session := "offline"
		if u.LoggedIn {
			session = "online"
		}
		rows = append(rows, table.Row{
			u.Name,
			string(u.Role),
			u.Project,
			shared.PrivilegesString(u.Role.Privileges()),
			session,
		})
	}
	v.tbl.SetRows(rows)
}

// SetUsers sets all currents users in the user view
func (v UsersView) SetUsers(users []*shared.User) UsersView {
	v.refreshRows(users)
	return v
}

// TakeOp returns and clears the pending operation
func (v *UsersView) TakeOp() *UserOp {
	op := v.pendingOp
	v.pendingOp = nil
	return op
}

// SetUsersViewSize resizes users view
func (v UsersView) SetUsersViewSize(h, w int) UsersView {
	const nonTable = 2
	const tableChromeLines = 3
	rows := max((h - nonTable - tableChromeLines), 3)
	v.tbl.SetHeight(rows)

	avail := max((w - 8), 50)

	userW := avail * 20 / 100
	roleW := avail * 10 / 100
	projectW := avail * 15 / 100
	sessionW := avail * 10 / 100
	privW := avail - (avail * 5 / 10)

	v.tbl.SetColumns([]table.Column{
		{Title: "User", Width: userW},
		{Title: "Role", Width: roleW},
		{Title: "Project", Width: projectW},
		{Title: "Privileges", Width: privW},
		{Title: "Session", Width: sessionW},
	})
	return v
}

// deny denies access for lack of permission
func (v *UsersView) deny(msg string) {
	if msg == "" {
		msg = "Permission denied: requires Admin role within your own project"
	}
	v.statusMsg = msg
	v.statusErr = true
}

// currentRow returns current row based on cursor position
func (v *UsersView) currentRow() int {
	return v.tbl.Cursor()
}

// isOwnRow returns if the selected row accessible to user on not
func (v *UsersView) isOwnRow(row int, users []*shared.User) bool {
	return row >= 0 && row < len(users) && users[row].Name == v.currentName
}

// canManageRow reports whether the logged-in user has admin authority over the given row
func (v *UsersView) canManageRow(row int, users []*shared.User) bool {
	if row < 0 || row >= len(users) {
		return false
	}
	return v.isAdmin && users[row].Project == v.currentProject
}

// Update handles updates to the view
func (v UsersView) Update(msg tea.Msg, users []*shared.User) (UsersView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch v.mode {

		case rbacAddName:
			switch msg.String() {
			case "esc":
				v.mode = rbacBrowse
				v.input.Blur()
				v.input.SetValue("")
			case "enter":
				name := strings.TrimSpace(v.input.Value())
				if name == "" {
					v.statusMsg = "Name can't be empty"
					v.statusErr = true
					return v, nil
				}
				for _, u := range users {
					if strings.EqualFold(u.Name, name) {
						v.statusMsg = fmt.Sprintf("A user named %q already exists", name)
						v.statusErr = true
						return v, nil
					}
				}
				v.pendingName = name
				v.input.SetValue("")
				v.input.Placeholder = fmt.Sprintf("password (min %d characters/ max %d characters)...", minPasswordLen, maxPasswordLen)
				v.input.EchoMode = textinput.EchoPassword
				v.input.EchoCharacter = '*'
				v.mode = rbacAddPassword
				v.statusMsg = fmt.Sprintf("New user will be added to your project (%s) as Viewer", v.currentProject)
				v.statusErr = false
			default:
				var cmd tea.Cmd
				v.input, cmd = v.input.Update(msg)
				return v, cmd
			}
			return v, nil

		case rbacAddPassword:
			switch msg.String() {
			case "esc":
				v.mode = rbacBrowse
				v.input.Blur()
				v.input.SetValue("")
				v.input.EchoMode = textinput.EchoNormal
				v.pendingName = ""
			case "enter":
				password := v.input.Value()
				if len(password) < minPasswordLen || len(password) > maxPasswordLen {
					v.statusMsg = fmt.Sprintf("Password must be at least %d characters & %d characters maximum", minPasswordLen, maxPasswordLen)
					v.statusErr = true
					return v, nil
				}
				v.pendingPassword = password
				v.input.SetValue("")
				v.input.Placeholder = "confirm password..."
				v.mode = rbacConfirmPassword
				v.statusMsg = "Re-enter the password to confirm"
				v.statusErr = false
			default:
				var cmd tea.Cmd
				v.input, cmd = v.input.Update(msg)
				return v, cmd
			}
			return v, nil

		case rbacConfirmPassword:
			switch msg.String() {
			case "esc":
				v.mode = rbacBrowse
				v.input.Blur()
				v.input.SetValue("")
				v.input.EchoMode = textinput.EchoNormal
				v.pendingName = ""
				v.pendingPassword = ""
			case "enter":
				confirm := v.input.Value()
				if confirm != v.pendingPassword {
					v.statusMsg = "Passwords don't match, please try again"
					v.statusErr = true
					v.input.SetValue("")
					return v, nil
				}
				v.pendingOp = &UserOp{
					Type: "create",
					NewUser: &shared.User{
						Name:     v.pendingName,
						Role:     shared.RoleViewer,
						Project:  v.currentProject,
						Password: v.pendingPassword,
					},
				}
				v.statusMsg = fmt.Sprintf("Adding %q to %s as Viewer...", v.pendingName, v.currentProject)
				v.statusErr = false
				v.mode = rbacBrowse
				v.input.Blur()
				v.input.SetValue("")
				v.input.EchoMode = textinput.EchoNormal
				v.pendingName = ""
				v.pendingPassword = ""
			default:
				var cmd tea.Cmd
				v.input, cmd = v.input.Update(msg)
				return v, cmd
			}
			return v, nil

		case rbacEditRole:
			switch msg.String() {
			case "esc":
				v.mode = rbacBrowse
			case "1", "2", "3":
				idx, ok := roleKeyMap[msg.String()]
				if !ok {
					return v, nil
				}
				row := v.currentRow()
				if row < 0 || row >= len(users) {
					v.mode = rbacBrowse
					return v, nil
				}
				newRole := allRoles[idx]

				if v.isAdmin && newRole == shared.RoleAdmin && users[row].Name != v.currentName {
					v.mode = rbacConfirmAdminSelfDemotion
					v.statusMsg = "Critical privilege warning"
					v.statusErr = true
					return v, nil
				}

				v.pendingOp = &UserOp{
					Type:     "role",
					UserID:   users[row].ID,
					UserName: users[row].Name,
					Role:     newRole,
				}
				v.statusMsg = fmt.Sprintf("Updating %s to %s...", users[row].Name, newRole)
				v.statusErr = false
				v.mode = rbacBrowse
			}
			return v, nil

		case rbacConfirmAdminSelfDemotion:
			switch msg.String() {
			case "y", "Y":
				row := v.currentRow()
				if row >= 0 && row < len(users) {
					targetUser := users[row]

					// FIXED: Only send the promotion operation.
					// The API's UpdateUserRole already prevents self-demotion
					// without another admin. The warning tells the user this is
					// irreversible from their current session.
					v.pendingOp = &UserOp{
						Type:     "role",
						UserID:   targetUser.ID,
						UserName: targetUser.Name,
						Role:     shared.RoleAdmin,
					}

					v.statusMsg = fmt.Sprintf("Promoting %s to Admin...", targetUser.Name)
					v.statusErr = false
				}
				v.mode = rbacBrowse

			case "n", "N", "esc":
				v.statusMsg = "Promotion canceled. Kept your Admin privilege."
				v.statusErr = false
				v.mode = rbacBrowse
			}
			return v, nil

		case rbacChangePassword:
			switch msg.String() {
			case "esc":
				v.mode = rbacBrowse
				v.input.Blur()
				v.input.SetValue("")
				v.input.EchoMode = textinput.EchoNormal
			case "enter":
				password := v.input.Value()
				if len(password) < minPasswordLen {
					v.statusMsg = fmt.Sprintf("Password must be at least %d characters", minPasswordLen)
					v.statusErr = true
					return v, nil
				}
				v.pendingNewPassword = password
				v.input.SetValue("")
				v.input.Placeholder = "confirm new password..."
				v.mode = rbacConfirmNewPassword
				v.statusMsg = "Re-enter the new password to confirm"
				v.statusErr = false
			default:
				var cmd tea.Cmd
				v.input, cmd = v.input.Update(msg)
				return v, cmd
			}
			return v, nil

		case rbacConfirmNewPassword:
			switch msg.String() {
			case "esc":
				v.mode = rbacBrowse
				v.input.Blur()
				v.input.SetValue("")
				v.input.EchoMode = textinput.EchoNormal
				v.pendingNewPassword = ""
			case "enter":
				confirm := v.input.Value()
				if confirm != v.pendingNewPassword {
					v.statusMsg = "Passwords don't match, please try again"
					v.statusErr = true
					v.input.SetValue("")
					return v, nil
				}
				if v.pwTargetRow >= 0 && v.pwTargetRow < len(users) {
					v.pendingOp = &UserOp{
						Type:     "password",
						UserID:   users[v.pwTargetRow].ID,
						UserName: users[v.pwTargetRow].Name,
						Password: v.pendingNewPassword,
					}
					v.statusMsg = fmt.Sprintf("Updating password for %s...", users[v.pwTargetRow].Name)
					v.statusErr = false
				}
				v.mode = rbacBrowse
				v.input.Blur()
				v.input.SetValue("")
				v.input.EchoMode = textinput.EchoNormal
				v.pendingNewPassword = ""
			default:
				var cmd tea.Cmd
				v.input, cmd = v.input.Update(msg)
				return v, cmd
			}
			return v, nil

		case rbacLoginPassword:
			switch msg.String() {
			case "esc":
				v.mode = rbacBrowse
				v.input.Blur()
				v.input.SetValue("")
				v.input.EchoMode = textinput.EchoNormal
				v.loginTargetRow = -1
			case "enter":
				attempt := v.input.Value()
				if v.loginTargetRow < 0 || v.loginTargetRow >= len(users) {
					v.mode = rbacBrowse
					return v, nil
				}
				target := users[v.loginTargetRow]
				v.pendingOp = &UserOp{
					Type:     "switch",
					UserName: target.Name,
					Password: attempt,
				}
				v.statusMsg = fmt.Sprintf("Switching to %s...", target.Name)
				v.statusErr = false
				v.mode = rbacBrowse
				v.input.Blur()
				v.input.SetValue("")
				v.input.EchoMode = textinput.EchoNormal
				v.loginTargetRow = -1
			default:
				var cmd tea.Cmd
				v.input, cmd = v.input.Update(msg)
				return v, cmd
			}
			return v, nil

		case rbacConfirmDelete:
			switch msg.String() {
			case "y", "Y":
				row := v.currentRow()
				if row >= 0 && row < len(users) {
					if users[row].Name == v.currentName {
						v.statusMsg = "Can't delete the account you're currently logged in as"
						v.statusErr = true
						v.mode = rbacBrowse
						return v, nil
					}
					v.pendingOp = &UserOp{
						Type:     "delete",
						UserID:   users[row].ID,
						UserName: users[row].Name,
					}
					v.statusMsg = fmt.Sprintf("Deleting %q...", users[row].Name)
					v.statusErr = false
				}
				v.mode = rbacBrowse
			case "n", "N", "esc":
				v.mode = rbacBrowse
				v.statusMsg = "Delete cancelled"
				v.statusErr = false
			}
			return v, nil
		}

		row := v.currentRow()
		switch msg.String() {
		case "n":
			if !v.isAdmin {
				v.deny("Permission denied: only an Admin can create users")
				return v, nil
			}
			v.mode = rbacAddName
			v.input.Placeholder = "new user's name..."
			v.input.EchoMode = textinput.EchoNormal
			v.input.Focus()
			v.statusMsg = ""
			return v, nil

		case "e":
			if !v.canManageRow(row, users) {
				v.deny("Permission denied: you can only edit roles within your own project")
				return v, nil
			}
			for i, r := range allRoles {
				if r == users[row].Role {
					v.pendingRoleIdx = i
				}
			}
			v.mode = rbacEditRole
			v.statusMsg = "[1] Admin  [2] Editor  [3] Viewer  |  press number to select, esc to cancel"
			v.statusErr = false
			return v, nil

		case "c":
			if row < 0 || row >= len(users) {
				return v, nil
			}
			if !v.isOwnRow(row, users) && !v.canManageRow(row, users) {
				v.deny("Permission denied: you can only change your own password")
				return v, nil
			}
			v.mode = rbacChangePassword
			v.pwTargetRow = row
			v.input.Placeholder = "new password..."
			v.input.EchoMode = textinput.EchoPassword
			v.input.EchoCharacter = '*'
			v.input.SetValue("")
			v.input.Focus()
			v.statusMsg = fmt.Sprintf("Setting new password for %s", users[row].Name)
			v.statusErr = false
			return v, nil

		case "s":
			if row < 0 || row >= len(users) {
				return v, nil
			}
			if users[row].Name == v.currentName {
				v.statusMsg = "Already logged in as this user"
				v.statusErr = false
				return v, nil
			}
			v.loginTargetRow = row
			v.mode = rbacLoginPassword
			v.input.Placeholder = fmt.Sprintf("password for %s...", users[row].Name)
			v.input.EchoMode = textinput.EchoPassword
			v.input.EchoCharacter = '*'
			v.input.SetValue("")
			v.input.Focus()
			v.statusMsg = fmt.Sprintf("Enter %s's password to switch into their session", users[row].Name)
			v.statusErr = false
			return v, nil

		case "t":
			if !v.canManageRow(row, users) {
				v.deny("Permission denied: you can only manage presence within your own project")
				return v, nil
			}
			if users[row].Name == v.currentName {
				v.statusMsg = "Use 's' to switch sessions instead of toggling your own session off"
				v.statusErr = true
				return v, nil
			}
			v.pendingOp = &UserOp{
				Type:     "presence",
				UserID:   users[row].ID,
				UserName: users[row].Name,
			}
			v.statusMsg = fmt.Sprintf("Toggling presence for %s...", users[row].Name)
			v.statusErr = false
			return v, nil

		case "d":
			if !v.canManageRow(row, users) {
				v.deny("Permission denied: you can only delete users within your own project")
				return v, nil
			}
			sameProjectCount := 0
			for _, u := range users {
				if u.Project == v.currentProject {
					sameProjectCount++
				}
			}
			if sameProjectCount <= 1 {
				v.statusMsg = "Can't delete the last remaining user in your project"
				v.statusErr = true
				return v, nil
			}
			v.mode = rbacConfirmDelete
			v.statusMsg = fmt.Sprintf("Delete %q? [y] confirm  [n/esc] cancel", users[row].Name)
			v.statusErr = true
			return v, nil
		}
	}

	var cmd tea.Cmd
	v.tbl, cmd = v.tbl.Update(msg)
	return v, cmd
}

// View renders the header, and the current view in which the user is on
func (v UsersView) View(u types.SessionUser, users []*shared.User) string {
	var b strings.Builder

	header := fmt.Sprintf("LoggedIn User: %s [Role: %s] [Privileges: %s]  | Project: %s",
		u.Name, u.Role, u.Role.Privileges(), u.Project)
	b.WriteString(styles.HeaderStyle.Render(header))
	b.WriteString("\n\n")

	row := v.currentRow()

	switch v.mode {
	case rbacAddName:
		b.WriteString(styles.SearchBoxStyle.Render("New user, name: " + v.input.View()))
		b.WriteString("\n\n")
		b.WriteString(styles.BorderStyle.Render(v.tbl.View()))
	case rbacAddPassword:
		b.WriteString(styles.SearchBoxStyle.Render(fmt.Sprintf("New user %q @ %s with password: %s", v.pendingName, v.currentProject, v.input.View())))
		b.WriteString("\n\n")
		b.WriteString(styles.BorderStyle.Render(v.tbl.View()))
	case rbacConfirmPassword:
		b.WriteString(styles.SearchBoxStyle.Render("Confirm password: " + v.input.View()))
		b.WriteString("\n\n")
		b.WriteString(styles.BorderStyle.Render(v.tbl.View()))
	case rbacChangePassword:
		b.WriteString(styles.SearchBoxStyle.Render("New password: " + v.input.View()))
		b.WriteString("\n\n")
		b.WriteString(styles.BorderStyle.Render(v.tbl.View()))
	case rbacConfirmNewPassword:
		b.WriteString(styles.SearchBoxStyle.Render("Confirm new password: " + v.input.View()))
		b.WriteString("\n\n")
		b.WriteString(styles.BorderStyle.Render(v.tbl.View()))
	case rbacLoginPassword:
		b.WriteString(styles.SearchBoxStyle.Render(v.input.View()))
		b.WriteString("\n\n")
		b.WriteString(styles.BorderStyle.Render(v.tbl.View()))
	default:
		b.WriteString(styles.BorderStyle.Render(v.tbl.View()))
	}
	b.WriteString("\n")

	switch {
	case v.statusMsg == "":
		b.WriteString(" ")
	case v.statusErr:
		b.WriteString(styles.DeniedStyle.Render(v.statusMsg))
	default:
		b.WriteString(styles.OkStyle.Render(v.statusMsg))
	}
	b.WriteString("\n")

	help := "[n] new  [e] role  [c] change pw  [s] login as...  [t] toggle session  [d] delete [1/2] views  [q] quit"
	if v.mode == rbacConfirmAdminSelfDemotion {
		help = "\t[y] confirm action   [n/esc] cancel"
	} else if v.mode == rbacEditRole {
		help = "[1] Admin  [2] Editor  [3] Viewer  |  esc to cancel"
	} else if !v.isAdmin {
		help = "(non-admin, most actions limited to your own account)  " + help
	}
	b.WriteString(styles.HelpStyle.Render(help))

	baseView := b.String()

	if v.mode == rbacConfirmAdminSelfDemotion && row >= 0 && row < len(users) {
		promptText := fmt.Sprintf(
			"\tCRITICAL PRIVILEGE WARNING\t\n\n"+
				"Are you sure you want to grant %s Admin privileges?\n"+
				"Proceeding will immediately demote your active account\n"+
				"(%s) to Editor. This change is irreversible from\n"+
				"your current session.\n\n"+
				"[y] Confirm & Demote Self\t[n / esc] Cancel",
			users[row].Name, v.currentName,
		)

		modalStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF0000")).
			Padding(1, 3).
			Background(lipgloss.Color("#1a1a1a")).
			Align(lipgloss.Center)

		modalContent := modalStyle.Render(promptText)

		totalWidth := lipgloss.Width(baseView)
		totalHeight := lipgloss.Height(baseView)

		return lipgloss.Place(
			totalWidth,
			totalHeight,
			lipgloss.Center,
			lipgloss.Center,
			modalContent,
			lipgloss.WithWhitespaceChars(" "),
		)
	}

	return baseView
}

// InInputMode returns true if the view is currently in an input mode
func (v UsersView) InInputMode() bool {
	return v.mode != rbacBrowse && v.mode != rbacEditRole
}

// SetStatus sets a status message from the controller
func (v UsersView) SetStatus(msg string, isErr bool) UsersView {
	v.statusMsg = msg
	v.statusErr = isErr
	return v
}
