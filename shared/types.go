package shared

import (
	"strings"
	"time"
)

// Privilege represents a user capability
type Privilege string

const (
	PrivRead  Privilege = "Read"
	PrivWrite Privilege = "Write"
	PrivKill  Privilege = "Kill"
)

type Role string

const (
	RoleAdmin  Role = "Admin"
	RoleEditor Role = "Editor"
	RoleViewer Role = "Viewer"
)

var rolePrivileges = map[Role][]Privilege{
	RoleAdmin:  {PrivRead, PrivWrite, PrivKill},
	RoleEditor: {PrivRead, PrivWrite},
	RoleViewer: {PrivRead},
}

func (r Role) Privileges() []Privilege {
	return rolePrivileges[r]
}

func (r Role) Has(p Privilege) bool {
	for _, priv := range rolePrivileges[r] {
		if priv == p {
			return true
		}
	}
	return false
}

func PrivilegesString(privs []Privilege) string {
	strs := make([]string, len(privs))
	for i, p := range privs {
		strs[i] = string(p)
	}
	if len(strs) == 0 {
		return "None"
	}
	return strings.Join(strs, ", ")
}

func MaskPassword() string {
	return strings.Repeat("*", 12)
}

// FlagStatus represents the on/off state of a feature flag
type FlagStatus string

const (
	StatusOn  FlagStatus = "ON"
	StatusOff FlagStatus = "OFF"
)

// Core Models
type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Role     Role   `json:"role"`
	Project  string `json:"project"`
	LoggedIn bool   `json:"logged_in"`
	Password string `json:"password"`
}

type FeatureFlag struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Status        FlagStatus `json:"status"`
	Rollout       int        `json:"rollout"`
	Description   string     `json:"description"`
	Killed        bool       `json:"killed"`
	TargetedUsers []string   `json:"targeted_users,omitempty"`
	LastModified  time.Time  `json:"last_modified"`
	ProjectID     string     `json:"project_id"`
}

func (f *FeatureFlag) IsStale() bool {
	if f.Rollout != 100 || f.LastModified.IsZero() || f.Killed {
		return false
	}
	return time.Since(f.LastModified) > 14*24*time.Hour
}

func (f FeatureFlag) Matches(query string) bool {
	if query == "" {
		return true
	}
	q := strings.ToLower(query)
	return strings.Contains(strings.ToLower(f.Name), q) ||
		strings.Contains(strings.ToLower(f.Description), q) ||
		strings.Contains(strings.ToLower(string(f.Status)), q)
}

type Project struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Flags      []*FeatureFlag `json:"flags,omitempty"`
	AdminNames []string       `json:"admin_names,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

// Audit
type AuditAction string

const (
	AuditToggle  AuditAction = "TOGGLE"
	AuditRollout AuditAction = "ROLLOUT"
	AuditKill    AuditAction = "KILL"
	AuditCreate  AuditAction = "CREATE"
	AuditUpdate  AuditAction = "UPDATE"
	AuditDelete  AuditAction = "DELETE"
)

type AuditEntry struct {
	ID        string      `json:"id"`
	Time      time.Time   `json:"time"`
	User      string      `json:"user"`
	Action    AuditAction `json:"action"`
	Flag      string      `json:"flag"`
	Detail    string      `json:"detail"`
	ProjectID string      `json:"project_id"`
}

// WebSocket
type WSMessage struct {
	Type      string      `json:"type"`
	ProjectID string      `json:"project_id"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}

// API Request/Response
type AppRegistrationRequest struct {
	AppName string `json:"app_name"`
}

type AppRegistrationResponse struct {
	ProjectID     string `json:"project_id"`
	ProjectName   string `json:"project_name"`
	AdminUsername string `json:"admin_username"`
	AdminPassword string `json:"admin_password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      User      `json:"user"`
}
