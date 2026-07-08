package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ignorant05/control/shared"
	"github.com/ignorant05/control/tui/styles"
	"github.com/ignorant05/control/tui/types"
)

// rolloutEditState handles (increase/decrease) rollout percentage per flag as state
type rolloutEditState struct {
	active bool
	row    int
}

// sortKey functions as sorting flags key based on either; rollout/status/name
type sortKey int

const (
	sortByName sortKey = iota
	sortByStatus
	sortByRollout
)

// String handles sorting logic mapping
func (s sortKey) String() string {
	switch s {
	case sortByStatus:
		return "Status"
	case sortByRollout:
		return "Rollout"
	default:
		return "Name"
	}
}

// FlagOp represents a pending flag operation for the controller to execute via API
type FlagOp struct {
	Type        string // "toggle", "rollout", "kill", "audit", "create", "update", "delete"
	FlagID      string
	Flag        *shared.FeatureFlag
	Rollout     int
	Name        string
	Description string
}

// FlagsView captures current state of each entry; users, current user, search input state, flags ...etc
type FlagsView struct {
	user        shared.User
	projectName string
	flags       []*shared.FeatureFlag
	auditLog    []*shared.AuditEntry

	search        textinput.Model
	input         textinput.Model
	tbl           table.Model
	filtered      []*shared.FeatureFlag
	rollout       rolloutEditState
	statusMsg     string
	statusIsErr   bool
	confirmKill   *shared.FeatureFlag
	confirmDelete *shared.FeatureFlag
	sortKey       sortKey
	showAudit     bool

	addingFlag  bool
	editingFlag bool
	editTarget  *shared.FeatureFlag
	pendingName string
	pendingDesc string

	pendingOp *FlagOp
}

// NewFlagsView creates the backbone of the flag's screen
func NewFlagsView(user shared.User, projectName string, flags []*shared.FeatureFlag, auditLog []*shared.AuditEntry) FlagsView {
	ti := textinput.New()
	ti.Placeholder = "search name, description, status..."
	ti.Prompt = ""
	ti.CharLimit = 64

	input := textinput.New()
	input.CharLimit = 64

	columns := []table.Column{
		{Title: "Flag Name", Width: 20},
		{Title: "Status", Width: 8},
		{Title: "Rollout", Width: 9},
		{Title: "Project", Width: 14},
		{Title: "Stale", Width: 10},
		{Title: "Description", Width: 22},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	fv := FlagsView{
		user:        user,
		projectName: projectName,
		flags:       flags,
		auditLog:    auditLog,
		search:      ti,
		input:       input,
		tbl:         t,
	}
	fv.refresh()
	return fv
}

// IsFocused checks if user is currently in search mode
func IsFocused(v *FlagsView) bool {
	return v.search.Focused() || v.input.Focused()
}

// canWrite handles who can write to the current project based on active user and flag's properties
func (v *FlagsView) canWrite() bool {
	if v.user.Project != v.projectName {
		return false
	}
	return v.user.Role.Has(shared.PrivWrite)
}

// canKill handles who can kill/delete flags based on active user and flag's properties
func (v *FlagsView) canKill() bool {
	if v.user.Project != v.projectName {
		return false
	}
	return v.user.Role.Has(shared.PrivKill)
}

// refresh recomputes the filtered list from search query, applies the current sort, and rebuilds table rows.
func (v *FlagsView) refresh() {
	query := v.search.Value()
	v.filtered = v.filtered[:0]
	for _, f := range v.flags {
		if f.Killed {
			continue
		}
		if f.Matches(query) {
			v.filtered = append(v.filtered, f)
		}
	}

	switch v.sortKey {
	case sortByStatus:
		sort.SliceStable(v.filtered, func(i, j int) bool {
			return v.filtered[i].Status < v.filtered[j].Status
		})
	case sortByRollout:
		sort.SliceStable(v.filtered, func(i, j int) bool {
			return v.filtered[i].Rollout > v.filtered[j].Rollout
		})
	default:
		sort.SliceStable(v.filtered, func(i, j int) bool {
			return v.filtered[i].Name < v.filtered[j].Name
		})
	}

	rows := make([]table.Row, 0, len(v.filtered))
	for _, f := range v.filtered {
		stale := "—"
		if f.IsStale() {
			stale = "⚠ cleanup"
		}
		rows = append(rows, table.Row{
			f.Name,
			string(f.Status),
			fmt.Sprintf("%d%%", f.Rollout),
			v.projectName,
			stale,
			truncate(f.Description, 22),
		})
	}
	v.tbl.SetRows(rows)
}

// truncate obscures the rest of the string (description mostly) based on length and window length
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// selectedFlag holds current selected flags
func (v *FlagsView) selectedFlag() *shared.FeatureFlag {
	row := v.tbl.Cursor()
	if row < 0 || row >= len(v.filtered) {
		return nil
	}
	return v.filtered[row]
}

// TakeOp returns and clears the pending operation
func (v *FlagsView) TakeOp() *FlagOp {
	op := v.pendingOp
	v.pendingOp = nil
	return op
}

// SetFlags updates the flag list from external source (API refresh)
func (v FlagsView) SetFlags(flags []*shared.FeatureFlag) FlagsView {
	v.flags = flags
	v.refresh()
	return v
}

// SetAuditLog updates the audit log from external source
func (v FlagsView) SetAuditLog(auditLog []*shared.AuditEntry) FlagsView {
	v.auditLog = auditLog
	return v
}

// SetStatus sets a status message from the controller
func (v FlagsView) SetStatus(msg string, isErr bool) FlagsView {
	v.statusMsg = msg
	v.statusIsErr = isErr
	return v
}

// denyOrSay handles permission error — displays red message for denied, green for allowed
func (v *FlagsView) denyOrSay(msg string, allowed bool) {
	if allowed {
		v.statusMsg = msg
		v.statusIsErr = false
	} else {
		v.statusMsg = "Permission denied: requires Write privilege on this project"
		v.statusIsErr = true
	}
}

// Update update the flags view based on user operations
func (v FlagsView) Update(msg tea.Msg) (FlagsView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if v.showAudit {
			switch msg.String() {
			case "esc", "L", "l":
				v.showAudit = false
			}
			return v, nil
		}

		if v.confirmKill != nil {
			switch msg.String() {
			case "y", "Y":
				v.pendingOp = &FlagOp{
					Type:   "kill",
					FlagID: v.confirmKill.ID,
					Flag:   v.confirmKill,
				}
				v.statusMsg = fmt.Sprintf("Killing flag %q...", v.confirmKill.Name)
				v.statusIsErr = false
				v.confirmKill = nil
			case "n", "N", "esc":
				v.confirmKill = nil
				v.statusMsg = "Kill cancelled"
				v.statusIsErr = false
			}
			return v, nil
		}

		if v.confirmDelete != nil {
			switch msg.String() {
			case "y", "Y":
				v.pendingOp = &FlagOp{
					Type:   "delete",
					FlagID: v.confirmDelete.ID,
					Flag:   v.confirmDelete,
				}
				v.statusMsg = fmt.Sprintf("Deleting flag %q...", v.confirmDelete.Name)
				v.statusIsErr = false
				v.confirmDelete = nil
			case "n", "N", "esc":
				v.confirmDelete = nil
				v.statusMsg = "Delete cancelled"
				v.statusIsErr = false
			}
			return v, nil
		}

		if v.addingFlag {
			switch msg.String() {
			case "esc":
				v.addingFlag = false
				v.input.Blur()
				v.input.SetValue("")
				v.pendingName = ""
				v.pendingDesc = ""
			case "enter":
				if v.pendingName == "" {
					name := strings.TrimSpace(v.input.Value())
					if name == "" {
						v.statusMsg = "Flag name cannot be empty"
						v.statusIsErr = true
						return v, nil
					}
					v.pendingName = name
					v.input.SetValue("")
					v.input.Placeholder = "description (optional)..."
					v.statusMsg = "Enter flag description (or press Enter to skip)"
					v.statusIsErr = false
				} else if v.pendingDesc == "" {
					v.pendingDesc = strings.TrimSpace(v.input.Value())
					v.pendingOp = &FlagOp{
						Type:        "create",
						Name:        v.pendingName,
						Description: v.pendingDesc,
					}
					v.statusMsg = fmt.Sprintf("Creating flag %q...", v.pendingName)
					v.statusIsErr = false
					v.addingFlag = false
					v.input.Blur()
					v.input.SetValue("")
					v.pendingName = ""
					v.pendingDesc = ""
				}
			default:
				var cmd tea.Cmd
				v.input, cmd = v.input.Update(msg)
				return v, cmd
			}
			return v, nil
		}

		if v.editingFlag {
			switch msg.String() {
			case "esc":
				v.editingFlag = false
				v.input.Blur()
				v.input.SetValue("")
				v.editTarget = nil
				v.pendingName = ""
				v.pendingDesc = ""
			case "enter":
				if v.editTarget == nil {
					v.editingFlag = false
					v.input.Blur()
					return v, nil
				}
				if v.pendingName == "" {
					name := strings.TrimSpace(v.input.Value())
					if name == "" {
						v.statusMsg = "Flag name cannot be empty"
						v.statusIsErr = true
						return v, nil
					}
					v.pendingName = name
					v.input.SetValue(v.editTarget.Description)
					v.input.Placeholder = "new description..."
					v.statusMsg = "Enter new description (or press Enter to keep current)"
					v.statusIsErr = false
				} else {
					desc := strings.TrimSpace(v.input.Value())
					v.pendingOp = &FlagOp{
						Type:        "update",
						FlagID:      v.editTarget.ID,
						Name:        v.pendingName,
						Description: desc,
					}
					v.statusMsg = fmt.Sprintf("Updating flag %q...", v.editTarget.Name)
					v.statusIsErr = false
					v.editingFlag = false
					v.input.Blur()
					v.input.SetValue("")
					v.editTarget = nil
					v.pendingName = ""
					v.pendingDesc = ""
				}
			default:
				var cmd tea.Cmd
				v.input, cmd = v.input.Update(msg)
				return v, cmd
			}
			return v, nil
		}

		if v.rollout.active {
			f := v.selectedFlag()
			switch msg.String() {
			case "left", "-":
				if f != nil && f.Rollout > 0 {
					f.Rollout -= 5
					if f.Rollout < 0 {
						f.Rollout = 0
					}
				}
			case "right", "+":
				if f != nil && f.Rollout < 100 {
					f.Rollout += 5
					if f.Rollout > 100 {
						f.Rollout = 100
					}
				}
			case "enter":
				v.rollout.active = false
				if f != nil {
					v.pendingOp = &FlagOp{
						Type:    "rollout",
						FlagID:  f.ID,
						Flag:    f,
						Rollout: f.Rollout,
					}
					v.statusMsg = fmt.Sprintf("Updating rollout for %s to %d%%...", f.Name, f.Rollout)
					v.statusIsErr = false
				}
			case "esc":
				v.rollout.active = false
			}
			v.refresh()
			return v, nil
		}

		if v.search.Focused() {
			switch msg.String() {
			case "esc", "enter":
				v.search.Blur()
				return v, nil
			}
			var cmd tea.Cmd
			v.search, cmd = v.search.Update(msg)
			v.refresh()
			return v, cmd
		}

		switch msg.String() {
		case "/":
			v.search.Focus()
			return v, nil

		case " ":
			if !v.canWrite() {
				v.denyOrSay("", false)
				return v, nil
			}
			if f := v.selectedFlag(); f != nil {
				v.pendingOp = &FlagOp{
					Type:   "toggle",
					FlagID: f.ID,
					Flag:   f,
				}
				v.statusMsg = fmt.Sprintf("Toggling %q...", f.Name)
				v.statusIsErr = false
			}

		case "n":
			if !v.canWrite() {
				v.denyOrSay("", false)
				return v, nil
			}
			v.addingFlag = true
			v.input.Placeholder = "new flag name..."
			v.input.EchoMode = textinput.EchoNormal
			v.input.SetValue("")
			v.input.Focus()
			v.statusMsg = "Enter new flag name (esc to cancel)"
			v.statusIsErr = false
			return v, nil

		case "m":
			if !v.canWrite() {
				v.denyOrSay("", false)
				return v, nil
			}
			if f := v.selectedFlag(); f != nil {
				v.editingFlag = true
				v.editTarget = f
				v.input.Placeholder = "new name..."
				v.input.EchoMode = textinput.EchoNormal
				v.input.SetValue(f.Name)
				v.input.Focus()
				v.statusMsg = fmt.Sprintf("Editing %s — enter new name (esc to cancel)", f.Name)
				v.statusIsErr = false
			}
			return v, nil

		case "r":
			if !v.canWrite() {
				v.denyOrSay("", false)
				return v, nil
			}
			if f := v.selectedFlag(); f != nil {
				v.rollout.active = true
				v.statusMsg = "Editing rollout: ←/→ adjust, enter to commit, esc to cancel"
				v.statusIsErr = false
			}

		case "s":
			v.sortKey = (v.sortKey + 1) % 3
			v.statusMsg = fmt.Sprintf("Sorted by %s", v.sortKey)
			v.statusIsErr = false
			v.refresh()

		case "L":
			v.showAudit = true
			v.pendingOp = &FlagOp{Type: "audit"}
			v.statusMsg = "Loading audit log..."
			v.statusIsErr = false

		case "k":
			if !v.canKill() {
				v.statusMsg = "Permission denied: requires Kill privilege on this project"
				v.statusIsErr = true
				return v, nil
			}
			if f := v.selectedFlag(); f != nil {
				v.confirmKill = f
			}

		case "D":
			if !v.canKill() {
				v.statusMsg = "Permission denied: requires Kill privilege on this project"
				v.statusIsErr = true
				return v, nil
			}
			if f := v.selectedFlag(); f != nil {
				v.confirmDelete = f
			}
		}
	}

	var cmd tea.Cmd
	v.tbl, cmd = v.tbl.Update(msg)
	v.refresh()
	return v, cmd
}

// View renders the actual view (flags view in this case)
func (v FlagsView) View(u types.SessionUser) string {
	var b strings.Builder

	header := fmt.Sprintf("LoggedIn User: %s [Role: %s] [Privileges: %s]  | Project: %s",
		u.Name, u.Role, u.Role.Privileges(), u.Project)
	b.WriteString(styles.HeaderStyle.Render(header))
	b.WriteString("\n\n")

	if v.addingFlag {
		if v.pendingName == "" {
			b.WriteString(styles.SearchBoxStyle.Render("New flag name: " + v.input.View()))
		} else {
			b.WriteString(styles.SearchBoxStyle.Render(fmt.Sprintf("Description for %q: %s", v.pendingName, v.input.View())))
		}
		b.WriteString("\n\n")
		b.WriteString(styles.BorderStyle.Render(v.tbl.View()))
	} else if v.editingFlag {
		if v.pendingName == "" {
			b.WriteString(styles.SearchBoxStyle.Render("New name: " + v.input.View()))
		} else {
			b.WriteString(styles.SearchBoxStyle.Render(fmt.Sprintf("New description for %q: %s", v.pendingName, v.input.View())))
		}
		b.WriteString("\n\n")
		b.WriteString(styles.BorderStyle.Render(v.tbl.View()))
	} else {
		b.WriteString(styles.SearchBoxStyle.Render(v.search.View()))
		b.WriteString("\n\n")
		b.WriteString(styles.BorderStyle.Render(v.tbl.View()))
	}
	b.WriteString("\n")

	switch {
	case v.statusMsg == "":
		b.WriteString(" ")
	case v.statusIsErr:
		b.WriteString(styles.DeniedStyle.Render(v.statusMsg))
	default:
		b.WriteString(styles.OkStyle.Render(v.statusMsg))
	}
	b.WriteString("\n")

	help := fmt.Sprintf(
		"[/] search  [space] toggle  [n] new  [m] modify  [r] rollout  [s] sort(%s)  [L] log  [k] kill  [D] delete  [1/2] views  [q] quit",
		v.sortKey,
	)
	if v.confirmKill != nil {
		help = "[y] confirm kill   [n/esc] cancel"
	} else if v.confirmDelete != nil {
		help = "[y] confirm delete   [n/esc] cancel"
	} else if v.showAudit {
		help = "[esc/L] close log"
	}
	b.WriteString(styles.HelpStyle.Render(help))

	baseView := b.String()

	fullContentHeight := lipgloss.Height(baseView)
	fullContentWidth := lipgloss.Width(baseView)

	constrainedStyle := lipgloss.NewStyle().Height(fullContentHeight).MaxHeight(fullContentHeight)

	if v.confirmKill != nil {
		promptText := fmt.Sprintf(
			"\tCRITICAL DELETION WARNING\t\n\n"+
				"Kill flag %q permanently?\n"+
				"This disables it and removes it from all views.\n"+
				"This action cannot be undone.\n\n"+
				"[y] Confirm Kill\t[n / esc] Cancel",
			v.confirmKill.Name,
		)

		modalStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF0000")).
			Padding(1, 3).
			Background(lipgloss.Color("#1a1a1a")).
			Align(lipgloss.Center)

		modalContent := modalStyle.Render(promptText)

		baseView = lipgloss.Place(
			fullContentWidth,
			fullContentHeight,
			lipgloss.Center,
			lipgloss.Center,
			modalContent,
			lipgloss.WithWhitespaceChars(" "),
		)
	} else if v.confirmDelete != nil {
		promptText := fmt.Sprintf(
			"\tPERMANENT DELETE WARNING\t\n\n"+
				"Delete flag %q from the database?\n\n"+
				"This completely removes the flag and its audit history.\n"+
				"This action cannot be undone.\n\n"+
				"[y] Confirm Delete\t[n / esc] Cancel",
			v.confirmDelete.Name,
		)

		modalStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF0000")).
			Padding(1, 3).
			Background(lipgloss.Color("#1a1a1a")).
			Align(lipgloss.Center)

		modalContent := modalStyle.Render(promptText)

		baseView = lipgloss.Place(
			fullContentWidth,
			fullContentHeight,
			lipgloss.Center,
			lipgloss.Center,
			modalContent,
			lipgloss.WithWhitespaceChars(" "),
		)
	} else if v.showAudit {
		baseView = v.renderAuditOverlay(baseView, fullContentHeight)
	}

	return constrainedStyle.Render(baseView)
}

// renderAuditOverlay renders logs pop up window
func (v FlagsView) renderAuditOverlay(baseView string, h int) string {
	var lines []string
	entries := v.auditLog
	start := 0
	if len(entries) > 12 {
		start = len(entries) - 12
	}
	for _, e := range entries[start:] {
		lines = append(lines, fmt.Sprintf("%s  %-8s %-10s %-12s %s",
			e.Time.Format("01-02 15:04"), e.User, e.Action, e.Flag, e.Detail))
	}
	if len(lines) == 0 {
		lines = []string{"No changes recorded yet."}
	}

	content := fmt.Sprintf("Audit Log: %s \n%s\n\n[esc/L] close", v.projectName, strings.Join(lines, "\n"))

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5FAFFF")).
		Padding(1, 3).
		Background(lipgloss.Color("#1a1a1a"))

	modalContent := modalStyle.Render(content)

	return lipgloss.Place(
		lipgloss.Height(baseView),
		lipgloss.Width(baseView),
		lipgloss.Center,
		lipgloss.Center,
		modalContent,
		lipgloss.WithWhitespaceChars(" "),
	)
}

// SetFlagsViewSize resizes flags view
func (v FlagsView) SetFlagsViewSize(h, w int) FlagsView {
	tableHeight := h - 8
	tableHeight = max(tableHeight, 3)
	v.tbl.SetHeight(tableHeight)

	avail := w - 8
	avail = max(avail, 40)
	d := avail - (avail*2/10 + avail*1/10 + avail*1/10 + avail*2/10 + avail*1/10)

	cols := []table.Column{
		{Title: "Flag Name", Width: avail * 2 / 10},
		{Title: "Status", Width: avail * 1 / 10},
		{Title: "Rollout", Width: avail * 1 / 10},
		{Title: "Project", Width: avail * 2 / 10},
		{Title: "Stale", Width: avail * 1 / 10},
		{Title: "Description", Width: d},
	}

	v.tbl.SetColumns(cols)
	v.search.Width = w - 10
	v.input.Width = w - 10

	return v
}

// BlocksArrowNav blocks arrow navigation based on current state
func (v *FlagsView) BlocksArrowNav() bool {
	return v.rollout.active || v.confirmKill != nil || v.confirmDelete != nil || v.showAudit || v.search.Focused() || v.input.Focused()
}
