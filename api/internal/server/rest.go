package server

import (
	"encoding/json"
	"net/http"

	"github.com/ignorant05/control/api/cmd/model"
	"github.com/ignorant05/control/api/internal/hub"
	"github.com/ignorant05/control/api/internal/service"
	"github.com/ignorant05/control/api/internal/store"
)

// Handler holds all service dependencies
type Handler struct {
	AuthService    *service.AuthService
	FlagService    *service.FlagService
	UserService    *service.UserService
	ProjectService *service.ProjectService
	Store          store.Store
	Hub            *hub.Hub
}

// Login handles user authentication
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	resp, err := h.AuthService.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// RegisterApp handles external application registration
func (h *Handler) RegisterApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req model.AppRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.AppName == "" {
		http.Error(w, `{"error": "app_name is required"}`, http.StatusBadRequest)
		return
	}

	resp, err := h.AuthService.RegisterApp(r.Context(), req.AppName)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// --- Project Endpoints ---

// ListProjects returns all projects
func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	projects, err := h.ProjectService.ListProjects(r.Context())
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"projects": projects})
}

// GetProject returns a single project with details
func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	projectID := r.URL.Query().Get("id")
	if projectID == "" {
		http.Error(w, `{"error": "id parameter required"}`, http.StatusBadRequest)
		return
	}

	project, err := h.ProjectService.GetProject(r.Context(), projectID)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(project)
}

// ListFlags returns flags for a project
func (h *Handler) ListFlags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	project, err := h.ProjectService.GetProjectByName(r.Context(), user.Project)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	flags, err := h.FlagService.ListFlags(r.Context(), project.ID)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"flags": flags})
}

// CreateFlag creates a new flag
func (h *Handler) CreateFlag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	if !user.Role.Has(model.PrivWrite) {
		http.Error(w, `{"error": "permission denied: requires Write privilege"}`, http.StatusForbidden)
		return
	}

	var flag model.FeatureFlag
	if err := json.NewDecoder(r.Body).Decode(&flag); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	if flag.Name == "" {
		http.Error(w, `{"error": "flag name is required"}`, http.StatusBadRequest)
		return
	}

	project, err := h.ProjectService.GetProjectByName(r.Context(), user.Project)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	if err := h.FlagService.CreateFlag(r.Context(), project.ID, &flag, user.Name); err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(flag)
}

// GetFlag returns a single flag
func (h *Handler) GetFlag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	flagID := r.URL.Query().Get("id")
	if flagID == "" {
		http.Error(w, `{"error": "id parameter required"}`, http.StatusBadRequest)
		return
	}

	flag, err := h.FlagService.GetFlag(r.Context(), flagID)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(flag)
}

// UpdateFlag updates a flag
func (h *Handler) UpdateFlag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	if !user.Role.Has(model.PrivWrite) {
		http.Error(w, `{"error": "permission denied: requires Write privilege"}`, http.StatusForbidden)
		return
	}

	var flag model.FeatureFlag
	if err := json.NewDecoder(r.Body).Decode(&flag); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := h.FlagService.UpdateFlag(r.Context(), &flag, user.Name); err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(flag)
}

// ToggleFlag flips a flag's status
func (h *Handler) ToggleFlag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	if !user.Role.Has(model.PrivWrite) {
		http.Error(w, `{"error": "permission denied: requires Write privilege"}`, http.StatusForbidden)
		return
	}

	flagID := r.URL.Query().Get("id")
	if flagID == "" {
		http.Error(w, `{"error": "id parameter required"}`, http.StatusBadRequest)
		return
	}

	flag, err := h.FlagService.ToggleFlag(r.Context(), flagID, user.Name)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(flag)
}

// UpdateRollout changes a flag's rollout percentage
func (h *Handler) UpdateRollout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	if !user.Role.Has(model.PrivWrite) {
		http.Error(w, `{"error": "permission denied: requires Write privilege"}`, http.StatusForbidden)
		return
	}

	flagID := r.URL.Query().Get("id")
	if flagID == "" {
		http.Error(w, `{"error": "id parameter required"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Rollout int `json:"rollout"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	flag, err := h.FlagService.UpdateRollout(r.Context(), flagID, req.Rollout, user.Name)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(flag)
}

// KillFlag permanently disables a flag
func (h *Handler) KillFlag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	if !user.Role.Has(model.PrivKill) {
		http.Error(w, `{"error": "permission denied: requires Kill privilege"}`, http.StatusForbidden)
		return
	}

	flagID := r.URL.Query().Get("id")
	if flagID == "" {
		http.Error(w, `{"error": "id parameter required"}`, http.StatusBadRequest)
		return
	}

	if err := h.FlagService.KillFlag(r.Context(), flagID, user.Name); err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteFlag removes a flag
func (h *Handler) DeleteFlag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	if !user.Role.Has(model.PrivKill) {
		http.Error(w, `{"error": "permission denied: requires Kill privilege"}`, http.StatusForbidden)
		return
	}

	flagID := r.URL.Query().Get("id")
	if flagID == "" {
		http.Error(w, `{"error": "id parameter required"}`, http.StatusBadRequest)
		return
	}

	if err := h.FlagService.DeleteFlag(r.Context(), flagID, user.Name); err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListUsers returns all users (admin sees their project, but endpoint returns all for TUI compatibility)
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	users, err := h.UserService.ListUsers(r.Context())
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	for _, u := range users {
		u.Password = ""
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"users": users})
}

// CreateUser creates a new user
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	adminUser := GetUserFromContext(r.Context())
	if adminUser == nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var newUser model.User
	if err := json.NewDecoder(r.Body).Decode(&newUser); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	if newUser.Name == "" || newUser.Password == "" {
		http.Error(w, `{"error": "name and password are required"}`, http.StatusBadRequest)
		return
	}

	if err := h.UserService.CreateUser(r.Context(), adminUser, &newUser); err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusForbidden)
		return
	}

	newUser.Password = ""
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newUser)
}

// UpdateUserRole changes a user's role
func (h *Handler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	adminUser := GetUserFromContext(r.Context())
	if adminUser == nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	userID := r.URL.Query().Get("id")
	if userID == "" {
		http.Error(w, `{"error": "id parameter required"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Role model.Role `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := h.UserService.UpdateUserRole(r.Context(), adminUser, userID, req.Role); err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusForbidden)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateUserPassword changes a user's password
func (h *Handler) UpdateUserPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	userID := r.URL.Query().Get("id")
	if userID == "" {
		http.Error(w, `{"error": "id parameter required"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	if len(req.Password) < 8 {
		http.Error(w, `{"error": "password must be at least 8 characters"}`, http.StatusBadRequest)
		return
	}

	if err := h.UserService.UpdateUserPassword(r.Context(), user, userID, req.Password); err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusForbidden)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteUser removes a user
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	adminUser := GetUserFromContext(r.Context())
	if adminUser == nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	userID := r.URL.Query().Get("id")
	if userID == "" {
		http.Error(w, `{"error": "id parameter required"}`, http.StatusBadRequest)
		return
	}

	if err := h.UserService.DeleteUser(r.Context(), adminUser, userID); err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusForbidden)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TogglePresence toggles a user's online status
func (h *Handler) TogglePresence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	adminUser := GetUserFromContext(r.Context())
	if adminUser == nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	userID := r.URL.Query().Get("id")
	if userID == "" {
		http.Error(w, `{"error": "id parameter required"}`, http.StatusBadRequest)
		return
	}

	if err := h.UserService.TogglePresence(r.Context(), adminUser, userID); err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusForbidden)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetAuditLog returns audit entries for a project
func (h *Handler) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	project, err := h.ProjectService.GetProjectByName(r.Context(), user.Project)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	entries, err := h.Store.GetAuditLog(r.Context(), project.ID, 100)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"audit_log": entries})
}

// HealthCheck returns service health status
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"websocket": h.Hub != nil,
	})
}
