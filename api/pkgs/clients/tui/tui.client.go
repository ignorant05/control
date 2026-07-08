// Package tuiclient provides a reusable client for the Control API
// that can be integrated into the Bubble Tea TUI application.
package tuiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ignorant05/control/api/cmd/model"
)

// Client wraps HTTP and WebSocket connections to the API
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	wsConn     *websocket.Conn
	wsURL      string

	OnFlagChange func(msg model.WSMessage)
	OnUserChange func(msg model.WSMessage)
	OnAudit      func(msg model.WSMessage)
}

// NewClient creates a new API client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SetToken configures the JWT token for authenticated requests
func (c *Client) SetToken(token string) {
	c.token = token
}

// Login exchanges credentials for a JWT token
func (c *Client) Login(username, password string) (*model.LoginResponse, error) {
	reqBody, _ := json.Marshal(model.LoginRequest{
		Username: username,
		Password: password,
	})

	resp, err := c.httpClient.Post(
		c.baseURL+"/api/v1/login",
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("login failed: %s", resp.Status)
	}

	var result model.LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	c.token = result.Token
	return &result, nil
}

// RegisterApp creates a new project and admin account
func (c *Client) RegisterApp(appName string) (*model.AppRegistrationResponse, error) {
	reqBody, _ := json.Marshal(model.AppRegistrationRequest{AppName: appName})

	resp, err := c.httpClient.Post(
		c.baseURL+"/api/v1/register-app",
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("registration failed: %s", resp.Status)
	}

	var result model.AppRegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListProjects returns all projects
func (c *Client) ListProjects() ([]*model.Project, error) {
	resp, err := c.doRequest("GET", "/api/v1/projects", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Projects []*model.Project `json:"projects"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Projects, nil
}

// GetProject returns a single project
func (c *Client) GetProject(id string) (*model.Project, error) {
	resp, err := c.doRequest("GET", "/api/v1/project?id="+id, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var project model.Project
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, err
	}
	return &project, nil
}

// ListFlags returns flags for the current user's project
func (c *Client) ListFlags() ([]*model.FeatureFlag, error) {
	resp, err := c.doRequest("GET", "/api/v1/flags", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Flags []*model.FeatureFlag `json:"flags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Flags, nil
}

// CreateFlag creates a new feature flag
func (c *Client) CreateFlag(flag *model.FeatureFlag) (*model.FeatureFlag, error) {
	resp, err := c.doRequest("POST", "/api/v1/flag/create", flag)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result model.FeatureFlag
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ToggleFlag flips a flag's status
func (c *Client) ToggleFlag(flagID string) (*model.FeatureFlag, error) {
	resp, err := c.doRequest("POST", "/api/v1/flag/toggle?id="+flagID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result model.FeatureFlag
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateRollout changes a flag's rollout percentage
func (c *Client) UpdateRollout(flagID string, rollout int) (*model.FeatureFlag, error) {
	reqBody, _ := json.Marshal(map[string]int{"rollout": rollout})
	resp, err := c.doRequest("POST", "/api/v1/flag/rollout?id="+flagID, reqBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result model.FeatureFlag
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// KillFlag permanently disables a flag
func (c *Client) KillFlag(flagID string) error {
	resp, err := c.doRequest("DELETE", "/api/v1/flag/kill?id="+flagID, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// ListUsers returns all users
func (c *Client) ListUsers() ([]*model.User, error) {
	resp, err := c.doRequest("GET", "/api/v1/users", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Users []*model.User `json:"users"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Users, nil
}

// CreateUser creates a new user
func (c *Client) CreateUser(user *model.User) (*model.User, error) {
	resp, err := c.doRequest("POST", "/api/v1/user/create", user)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result model.User
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateUserRole changes a user's role
func (c *Client) UpdateUserRole(userID string, role model.Role) error {
	reqBody, _ := json.Marshal(map[string]model.Role{"role": role})
	resp, err := c.doRequest("PATCH", "/api/v1/user/role?id="+userID, reqBody)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// UpdatePassword changes a user's password
func (c *Client) UpdatePassword(userID, password string) error {
	reqBody, _ := json.Marshal(map[string]string{"password": password})
	resp, err := c.doRequest("PATCH", "/api/v1/user/password?id="+userID, reqBody)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// DeleteUser removes a user
func (c *Client) DeleteUser(userID string) error {
	resp, err := c.doRequest("DELETE", "/api/v1/user/delete?id="+userID, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// TogglePresence toggles a user's online status
func (c *Client) TogglePresence(userID string) error {
	resp, err := c.doRequest("POST", "/api/v1/user/presence?id="+userID, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// GetAuditLog returns audit entries
func (c *Client) GetAuditLog() ([]*model.AuditEntry, error) {
	resp, err := c.doRequest("GET", "/api/v1/audit", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		AuditLog []*model.AuditEntry `json:"audit_log"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.AuditLog, nil
}

// ConnectWebSocket establishes a WebSocket connection for real-time updates
func (c *Client) ConnectWebSocket() error {
	if c.token == "" {
		return fmt.Errorf("no token set, login first")
	}

	wsScheme := "ws"
	if c.baseURL[:5] == "https" {
		wsScheme = "wss"
	}

	u, _ := url.Parse(c.baseURL)
	wsURL := fmt.Sprintf("%s://%s/ws?token=%s", wsScheme, u.Host, c.token)
	c.wsURL = wsURL

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}
	c.wsConn = conn

	go c.readPump()
	return nil
}

// DisconnectWebSocket closes the WebSocket connection
func (c *Client) DisconnectWebSocket() error {
	if c.wsConn != nil {
		return c.wsConn.Close()
	}
	return nil
}

// readPump handles incoming WebSocket messages
func (c *Client) readPump() {
	defer c.wsConn.Close()

	for {
		_, data, err := c.wsConn.ReadMessage()
		if err != nil {
			return
		}

		var msg model.WSMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "flag_change":
			if c.OnFlagChange != nil {
				c.OnFlagChange(msg)
			}
		case "user_change":
			if c.OnUserChange != nil {
				c.OnUserChange(msg)
			}
		case "audit":
			if c.OnAudit != nil {
				c.OnAudit(msg)
			}
		}
	}
}

// Helpers

func (c *Client) doRequest(method, path string, body interface{}) (*http.Response, error) {
	var reqBody []byte
	var err error

	if body != nil {
		switch v := body.(type) {
		case []byte:
			reqBody = v
		default:
			reqBody, err = json.Marshal(body)
			if err != nil {
				return nil, err
			}
		}
	}

	req, err := http.NewRequest(method, c.baseURL+path, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	return c.httpClient.Do(req)
}
