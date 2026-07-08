package model

import (
	"time"
)

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

// FlagStatus represents the on/off state of a feature flag
type FlagStatus string

const (
	StatusOn  FlagStatus = "ON"
	StatusOff FlagStatus = "OFF"
)

type FeatureFlag struct {
	ID            string     `json:"id" db:"id"`
	Name          string     `json:"name" db:"name"`
	Status        FlagStatus `json:"status" db:"status"`
	Rollout       int        `json:"rollout" db:"rollout"`
	Description   string     `json:"description" db:"description"`
	Killed        bool       `json:"killed" db:"killed"`
	TargetedUsers []string   `json:"targeted_users,omitempty" db:"targeted_users"`
	LastModified  time.Time  `json:"last_modified" db:"last_modified"`
	ProjectID     string     `json:"project_id" db:"project_id"`
}

func (f *FeatureFlag) IsStale() bool {
	if f.Rollout != 100 || f.LastModified.IsZero() || f.Killed {
		return false
	}
	return time.Since(f.LastModified) > 14*24*time.Hour
}

// User represents a system user
type User struct {
	ID       string `json:"id" db:"id"`
	Name     string `json:"name" db:"name"`
	Role     Role   `json:"role" db:"role"`
	Project  string `json:"project" db:"project"`
	LoggedIn bool   `json:"logged_in" db:"logged_in"`
	Password string `json:"password" db:"password"`
}

// Project represents a project/workspace
type Project struct {
	ID         string         `json:"id" db:"id"`
	Name       string         `json:"name" db:"name"`
	Flags      []*FeatureFlag `json:"flags,omitempty"`
	AdminNames []string       `json:"admin_names,omitempty"`
	CreatedAt  time.Time      `json:"created_at" db:"created_at"`
}

// AuditAction identifies what kind of change an AuditEntry records
type AuditAction string

const (
	AuditToggle  AuditAction = "TOGGLE"
	AuditRollout AuditAction = "ROLLOUT"
	AuditKill    AuditAction = "KILL"
	AuditCreate  AuditAction = "CREATE"
	AuditUpdate  AuditAction = "UPDATE"
	AuditDelete  AuditAction = "DELETE"
)

// AuditEntry records a single change made to a flag
type AuditEntry struct {
	ID        string      `json:"id" db:"id"`
	Time      time.Time   `json:"time" db:"created_at"`
	User      string      `json:"user" db:"user_name"`
	Action    AuditAction `json:"action" db:"action"`
	Flag      string      `json:"flag" db:"flag_name"`
	Detail    string      `json:"detail" db:"detail"`
	ProjectID string      `json:"project_id" db:"project_id"`
}

// WSMessage is the envelope for WebSocket broadcasts
type WSMessage struct {
	Type      string      `json:"type"`
	ProjectID string      `json:"project_id"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}

// AppRegistrationRequest used when external apps register
type AppRegistrationRequest struct {
	AppName string `json:"app_name"`
}

// AppRegistrationResponse returned after successful registration
type AppRegistrationResponse struct {
	ProjectID     string `json:"project_id"`
	ProjectName   string `json:"project_name"`
	AdminUsername string `json:"admin_username"`
	AdminPassword string `json:"admin_password"`
}

// LoginRequest for JWT token exchange
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse contains the JWT token
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      User      `json:"user"`
}
