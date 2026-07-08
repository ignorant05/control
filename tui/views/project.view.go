package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ignorant05/control/shared"
	"github.com/ignorant05/control/tui/styles"
	"github.com/ignorant05/control/tui/types"
)

// ProjectsView captures the current projects registered in the tool alongside the corresponding users and state
type ProjectsView struct {
	projects []*shared.Project
	users    []*shared.User
	tbl      table.Model
}

// NewProjectsView creates project view with necessary columns
func NewProjectsView(projects []*shared.Project, users []*shared.User) ProjectsView {
	columns := []table.Column{
		{Title: "Project", Width: 18},
		{Title: "Flags", Width: 8},
		{Title: "ON", Width: 6},
		{Title: "Stale", Width: 8},
		{Title: "Admins", Width: 24},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	pv := ProjectsView{projects: projects, users: users, tbl: t}
	pv.refresh()
	return pv
}

// refresh refreshes state on change
func (v *ProjectsView) refresh() {
	rows := make([]table.Row, 0, len(v.projects))
	for _, p := range v.projects {
		onCount, staleCount := 0, 0
		for _, f := range p.Flags {
			if f.Killed {
				continue
			}
			if f.Status == shared.StatusOn {
				onCount++
			}
			if f.IsStale() {
				staleCount++
			}
		}
		admins := adminNames(v.users, p.Name)
		rows = append(rows, table.Row{
			p.Name,
			fmt.Sprintf("%d", len(p.Flags)),
			fmt.Sprintf("%d", onCount),
			fmt.Sprintf("%d", staleCount),
			admins,
		})
	}
	v.tbl.SetRows(rows)
}

// adminNames retrieves the admin name for project
func adminNames(users []*shared.User, project string) string {
	var names []string
	for _, u := range users {
		if u.Project == project && u.Role == shared.RoleAdmin {
			names = append(names, u.Name)
		}
	}
	if len(names) == 0 {
		return "—"
	}
	return strings.Join(names, ", ")
}

// SetUsers lets the router refresh the user list when it changes
func (v *ProjectsView) SetUsers(users []*shared.User) {
	v.users = users
	v.refresh()
}

// SetProjects updates the project list
func (v *ProjectsView) SetProjects(projects []*shared.Project) {
	v.projects = projects
	v.refresh()
}

// SelectedProject returns the project currently highlighted in the table
func (v ProjectsView) SelectedProject() *shared.Project {
	row := v.tbl.Cursor()
	if row < 0 || row >= len(v.projects) {
		return nil
	}
	return v.projects[row]
}

// Update updates the view depending on text size
func (v ProjectsView) Update(msg tea.Msg) (ProjectsView, tea.Cmd) {
	var cmd tea.Cmd
	v.tbl, cmd = v.tbl.Update(msg)
	return v, cmd
}

// View renders the view
func (v ProjectsView) View(u types.SessionUser) string {
	var b strings.Builder

	header := fmt.Sprintf("LoggedIn User: %s [Role: %s] [Privileges: %s]  | Project: %s",
		u.Name, u.Role, u.Role.Privileges(), u.Project)
	b.WriteString(styles.HeaderStyle.Render(header))
	b.WriteString("\n\n")

	b.WriteString(styles.HeaderStyle.Render("Projects Overview"))
	b.WriteString("\n\n")
	b.WriteString(styles.BorderStyle.Render(v.tbl.View()))
	b.WriteString("\n\n")
	b.WriteString(styles.HelpStyle.Render("[enter] open in Flags view  [←/→] switch windows  [q] quit"))
	return b.String()
}

// SetProjectsViewSize resizes the view size
func (v ProjectsView) SetProjectsViewSize(h, w int) ProjectsView {
	tableHeight := h - 6
	tableHeight = max(tableHeight, 3)
	v.tbl.SetHeight(tableHeight)

	avail := w - 8
	avail = max(avail, 40)
	d := avail - (avail*1/4 + avail*1/8 + avail*1/8 + avail*1/4)

	cols := []table.Column{
		{Title: "Project", Width: avail * 1 / 4},
		{Title: "Flags", Width: avail * 1 / 8},
		{Title: "ON", Width: avail * 1 / 8},
		{Title: "Stale", Width: avail * 1 / 4},
		{Title: "Admins", Width: d},
	}
	v.tbl.SetColumns(cols)
	return v
}
