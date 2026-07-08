package broadcast

import "github.com/ignorant05/control/api/cmd/model"

// Broadcaster is the interface for sending real-time updates to connected clients
// Implemented by the WebSocket hub (in server package).
type Broadcaster interface {
	BroadcastFlagChange(projectID string, flag *model.FeatureFlag, action model.AuditAction, user string)
	BroadcastUserChange(projectID string, user *model.User, action string)
	BroadcastAuditEntry(projectID string, entry *model.AuditEntry)
}
