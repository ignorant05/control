package service

import (
	"context"
	"fmt"

	"github.com/ignorant05/control/api/cmd/model"
	"github.com/ignorant05/control/api/internal/hub"
	"github.com/ignorant05/control/api/internal/store"
)

type FlagService struct {
	store store.Store
	redis *store.RedisStore
	hub   *hub.Hub
}

func NewFlagService(s store.Store, r *store.RedisStore, h *hub.Hub) *FlagService {
	return &FlagService{store: s, redis: r, hub: h}
}

// CreateFlag creates a new feature flag
func (s *FlagService) CreateFlag(ctx context.Context, projectID string, flag *model.FeatureFlag, userName string) error {
	flag.ProjectID = projectID
	flag.Status = model.StatusOff
	flag.Rollout = 0
	flag.Killed = false

	if err := s.store.CreateFlag(ctx, flag); err != nil {
		return fmt.Errorf("create flag: %w", err)
	}

	s.store.LogAudit(ctx, &model.AuditEntry{
		User:      userName,
		Action:    model.AuditCreate,
		Flag:      flag.Name,
		Detail:    fmt.Sprintf("Created flag %s", flag.Name),
		ProjectID: projectID,
	})

	if s.redis != nil {
		s.redis.InvalidateFlag(ctx, projectID, flag.ID)
	}
	if s.hub != nil {
		s.hub.BroadcastFlagChange(projectID, flag, model.AuditCreate, userName)
	}

	return nil
}

// GetFlag retrieves a single flag
func (s *FlagService) GetFlag(ctx context.Context, flagID string) (*model.FeatureFlag, error) {
	return s.store.GetFlag(ctx, flagID)
}

// ListFlags returns all non-killed flags for a project
func (s *FlagService) ListFlags(ctx context.Context, projectID string) ([]*model.FeatureFlag, error) {
	// Try cache first
	if s.redis != nil {
		var cached []*model.FeatureFlag
		if err := s.redis.GetJSON(ctx, fmt.Sprintf("project:%s:flags", projectID), &cached); err == nil {
			return cached, nil
		}
	}

	flags, err := s.store.ListFlags(ctx, projectID)
	if err != nil {
		return nil, err
	}

	if s.redis != nil {
		s.redis.SetJSON(ctx, fmt.Sprintf("project:%s:flags", projectID), flags, 0)
	}

	return flags, nil
}

// UpdateFlag updates a flag's properties
func (s *FlagService) UpdateFlag(ctx context.Context, flag *model.FeatureFlag, userName string) error {
	oldFlag, err := s.store.GetFlag(ctx, flag.ID)
	if err != nil {
		return fmt.Errorf("get flag: %w", err)
	}

	if err := s.store.UpdateFlag(ctx, flag); err != nil {
		return fmt.Errorf("update flag: %w", err)
	}

	action := model.AuditUpdate
	detail := fmt.Sprintf("Updated flag %s", flag.Name)
	if oldFlag.Status != flag.Status {
		action = model.AuditToggle
		detail = fmt.Sprintf("%s -> %s", oldFlag.Status, flag.Status)
	} else if oldFlag.Rollout != flag.Rollout {
		action = model.AuditRollout
		detail = fmt.Sprintf("%d%% -> %d%%", oldFlag.Rollout, flag.Rollout)
	}

	s.store.LogAudit(ctx, &model.AuditEntry{
		User:      userName,
		Action:    action,
		Flag:      flag.Name,
		Detail:    detail,
		ProjectID: flag.ProjectID,
	})

	if s.redis != nil {
		s.redis.InvalidateFlag(ctx, flag.ProjectID, flag.ID)
	}
	if s.hub != nil {
		s.hub.BroadcastFlagChange(flag.ProjectID, flag, action, userName)
	}

	return nil
}

// ToggleFlag flips a flag's status
func (s *FlagService) ToggleFlag(ctx context.Context, flagID, userName string) (*model.FeatureFlag, error) {
	flag, err := s.store.GetFlag(ctx, flagID)
	if err != nil {
		return nil, err
	}

	prevStatus := flag.Status
	if flag.Status == model.StatusOn {
		flag.Status = model.StatusOff
	} else {
		flag.Status = model.StatusOn
	}

	if err := s.store.UpdateFlag(ctx, flag); err != nil {
		return nil, err
	}

	s.store.LogAudit(ctx, &model.AuditEntry{
		User:      userName,
		Action:    model.AuditToggle,
		Flag:      flag.Name,
		Detail:    fmt.Sprintf("%s -> %s", prevStatus, flag.Status),
		ProjectID: flag.ProjectID,
	})

	if s.redis != nil {
		s.redis.InvalidateFlag(ctx, flag.ProjectID, flag.ID)
	}
	if s.hub != nil {
		s.hub.BroadcastFlagChange(flag.ProjectID, flag, model.AuditToggle, userName)
	}

	return flag, nil
}

// UpdateRollout changes a flag's rollout percentage
func (s *FlagService) UpdateRollout(ctx context.Context, flagID string, rollout int, userName string) (*model.FeatureFlag, error) {
	if rollout < 0 {
		rollout = 0
	}
	if rollout > 100 {
		rollout = 100
	}

	flag, err := s.store.GetFlag(ctx, flagID)
	if err != nil {
		return nil, err
	}

	prevRollout := flag.Rollout
	flag.Rollout = rollout

	if err := s.store.UpdateFlag(ctx, flag); err != nil {
		return nil, err
	}

	s.store.LogAudit(ctx, &model.AuditEntry{
		User:      userName,
		Action:    model.AuditRollout,
		Flag:      flag.Name,
		Detail:    fmt.Sprintf("%d%% -> %d%%", prevRollout, rollout),
		ProjectID: flag.ProjectID,
	})

	if s.redis != nil {
		s.redis.InvalidateFlag(ctx, flag.ProjectID, flag.ID)
	}
	if s.hub != nil {
		s.hub.BroadcastFlagChange(flag.ProjectID, flag, model.AuditRollout, userName)
	}

	return flag, nil
}

// KillFlag permanently disables a flag
func (s *FlagService) KillFlag(ctx context.Context, flagID, userName string) error {
	flag, err := s.store.GetFlag(ctx, flagID)
	if err != nil {
		return err
	}

	flag.Killed = true
	flag.Status = model.StatusOff

	if err := s.store.UpdateFlag(ctx, flag); err != nil {
		return err
	}

	s.store.LogAudit(ctx, &model.AuditEntry{
		User:      userName,
		Action:    model.AuditKill,
		Flag:      flag.Name,
		Detail:    "Flag killed permanently",
		ProjectID: flag.ProjectID,
	})

	if s.redis != nil {
		s.redis.InvalidateFlag(ctx, flag.ProjectID, flag.ID)
	}
	if s.hub != nil {
		s.hub.BroadcastFlagChange(flag.ProjectID, flag, model.AuditKill, userName)
	}

	return nil
}

// DeleteFlag removes a flag from the database
func (s *FlagService) DeleteFlag(ctx context.Context, flagID, userName string) error {
	flag, err := s.store.GetFlag(ctx, flagID)
	if err != nil {
		return err
	}

	if err := s.store.DeleteFlag(ctx, flagID); err != nil {
		return err
	}

	s.store.LogAudit(ctx, &model.AuditEntry{
		User:      userName,
		Action:    model.AuditDelete,
		Flag:      flag.Name,
		Detail:    "Flag deleted",
		ProjectID: flag.ProjectID,
	})

	if s.redis != nil {
		s.redis.InvalidateFlag(ctx, flag.ProjectID, flagID)
	}
	if s.hub != nil {
		s.hub.BroadcastFlagChange(flag.ProjectID, &model.FeatureFlag{ID: flagID, Name: flag.Name}, model.AuditDelete, userName)
	}

	return nil
}
