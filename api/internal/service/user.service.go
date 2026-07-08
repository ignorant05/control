package service

import (
	"context"
	"fmt"

	"github.com/ignorant05/control/api/cmd/model"
	"github.com/ignorant05/control/api/internal/hub"
	"github.com/ignorant05/control/api/internal/store"
)

type UserService struct {
	store store.Store
	redis *store.RedisStore
	hub   *hub.Hub
}

func NewUserService(s store.Store, r *store.RedisStore, h *hub.Hub) *UserService {
	return &UserService{store: s, redis: r, hub: h}
}

// CreateUser creates a new user (admin only, within their project)
func (s *UserService) CreateUser(ctx context.Context, adminUser *model.User, newUser *model.User) error {
	if adminUser.Role != model.RoleAdmin {
		return fmt.Errorf("permission denied: requires Admin role")
	}
	if newUser.Project != adminUser.Project {
		return fmt.Errorf("admin can only create users in their own project")
	}

	newUser.Role = model.RoleViewer
	newUser.Project = adminUser.Project

	hashedPw, err := HashPassword(newUser.Password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	newUser.Password = hashedPw

	if err := s.store.CreateUser(ctx, newUser); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	if s.redis != nil {
		s.redis.InvalidateProject(ctx, "")
	}
	if s.hub != nil {
		s.hub.BroadcastUserChange(newUser.Project, newUser, "created")
	}

	return nil
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(ctx context.Context, userID string) (*model.User, error) {
	return s.store.GetUser(ctx, userID)
}

// ListUsers returns all users
func (s *UserService) ListUsers(ctx context.Context) ([]*model.User, error) {
	return s.store.ListUsers(ctx)
}

// ListUsersByProject returns users in a specific project
func (s *UserService) ListUsersByProject(ctx context.Context, projectName string) ([]*model.User, error) {
	return s.store.ListUsersByProject(ctx, projectName)
}

// UpdateUserRole updates a user's role (admin only, within project)
func (s *UserService) UpdateUserRole(ctx context.Context, adminUser *model.User, targetUserID string, newRole model.Role) error {
	if adminUser.Role != model.RoleAdmin {
		return fmt.Errorf("permission denied: requires Admin role")
	}

	targetUser, err := s.store.GetUser(ctx, targetUserID)
	if err != nil {
		return fmt.Errorf("get target user: %w", err)
	}

	if targetUser.Project != adminUser.Project {
		return fmt.Errorf("admin can only manage users in their own project")
	}

	if targetUser.Name == adminUser.Name && newRole != model.RoleAdmin {
		users, err := s.store.ListUsersByProject(ctx, adminUser.Project)
		if err != nil {
			return err
		}
		hasOtherAdmin := false
		for _, u := range users {
			if u.Name != adminUser.Name && u.Role == model.RoleAdmin {
				hasOtherAdmin = true
				break
			}
		}
		if !hasOtherAdmin {
			return fmt.Errorf("cannot demote yourself: no other admin in project")
		}
	}

	targetUser.Role = newRole
	if err := s.store.UpdateUser(ctx, targetUser); err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	if s.redis != nil {
		s.redis.InvalidateProject(ctx, "")
	}
	if s.hub != nil {
		s.hub.BroadcastUserChange(targetUser.Project, targetUser, "role_updated")
	}

	return nil
}

// UpdateUserPassword updates a user's password
func (s *UserService) UpdateUserPassword(ctx context.Context, requestingUser *model.User, targetUserID, newPassword string) error {
	targetUser, err := s.store.GetUser(ctx, targetUserID)
	if err != nil {
		return fmt.Errorf("get target user: %w", err)
	}

	canChange := requestingUser.Name == targetUser.Name ||
		(requestingUser.Role == model.RoleAdmin && targetUser.Project == requestingUser.Project)

	if !canChange {
		return fmt.Errorf("permission denied: can only change your own password or reset within your project as admin")
	}

	hashedPw, err := HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	targetUser.Password = hashedPw

	if err := s.store.UpdateUser(ctx, targetUser); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	return nil
}

// DeleteUser removes a user (admin only, within project, cannot delete self)
func (s *UserService) DeleteUser(ctx context.Context, adminUser *model.User, targetUserID string) error {
	if adminUser.Role != model.RoleAdmin {
		return fmt.Errorf("permission denied: requires Admin role")
	}

	targetUser, err := s.store.GetUser(ctx, targetUserID)
	if err != nil {
		return fmt.Errorf("get target user: %w", err)
	}

	if targetUser.Project != adminUser.Project {
		return fmt.Errorf("admin can only delete users in their own project")
	}

	if targetUser.Name == adminUser.Name {
		return fmt.Errorf("cannot delete your own account")
	}

	users, err := s.store.ListUsersByProject(ctx, adminUser.Project)
	if err != nil {
		return err
	}
	if len(users) <= 1 {
		return fmt.Errorf("cannot delete the last user in the project")
	}

	if err := s.store.DeleteUser(ctx, targetUserID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	if s.redis != nil {
		s.redis.InvalidateProject(ctx, "")
	}
	if s.hub != nil {
		s.hub.BroadcastUserChange(targetUser.Project, targetUser, "deleted")
	}

	return nil
}

// TogglePresence toggles a user's logged_in status (admin only)
func (s *UserService) TogglePresence(ctx context.Context, adminUser *model.User, targetUserID string) error {
	if adminUser.Role != model.RoleAdmin {
		return fmt.Errorf("permission denied: requires Admin role")
	}

	targetUser, err := s.store.GetUser(ctx, targetUserID)
	if err != nil {
		return fmt.Errorf("get target user: %w", err)
	}

	if targetUser.Project != adminUser.Project {
		return fmt.Errorf("admin can only manage presence within their own project")
	}

	if targetUser.Name == adminUser.Name {
		return fmt.Errorf("use login endpoint to switch sessions instead")
	}

	targetUser.LoggedIn = !targetUser.LoggedIn
	if err := s.store.UpdateUser(ctx, targetUser); err != nil {
		return fmt.Errorf("update presence: %w", err)
	}

	if s.hub != nil {
		s.hub.BroadcastUserChange(targetUser.Project, targetUser, "presence_toggled")
	}

	return nil
}
