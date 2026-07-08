package types

import "github.com/ignorant05/control/shared"

// Single Source of Truth for the header information
type SessionUser struct {
	Name       string
	Role       shared.Role
	Project    string
	Privileges string
}

// UpdateSession updates the current driver session using a fresh copy of the target user (current User)
func (s *SessionUser) UpdateSession(u shared.User) {
	s.Name = u.Name
	s.Role = u.Role
	s.Project = u.Project
	s.Privileges = shared.PrivilegesString(u.Role.Privileges())
}
