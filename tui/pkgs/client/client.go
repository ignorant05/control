package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ignorant05/control/shared"
)

// APIClient wraps HTTP and WebSocket connections to the Control API.
type APIClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
	wsConn     *websocket.Conn

	OnFlagChange func(msg shared.WSMessage)
	OnUserChange func(msg shared.WSMessage)
	OnAudit      func(msg shared.WSMessage)
}

func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *APIClient) SetToken(token string) { c.token = token }

// Authentication
func (c *APIClient) Login(username, password string) (*shared.LoginResponse, error) {
	reqBody, _ := json.Marshal(shared.LoginRequest{Username: username, Password: password})
	resp, err := c.httpClient.Post(c.baseURL+"/api/v1/login", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("login failed: %s", resp.Status)
	}
	var result shared.LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	c.token = result.Token
	return &result, nil
}

func (c *APIClient) RegisterApp(appName string) (*shared.AppRegistrationResponse, error) {
	reqBody, _ := json.Marshal(shared.AppRegistrationRequest{AppName: appName})
	resp, err := c.httpClient.Post(c.baseURL+"/api/v1/register-app", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("registration failed: %s", resp.Status)
	}
	var result shared.AppRegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Projects
func (c *APIClient) ListProjects() ([]*shared.Project, error) {
	resp, err := c.doRequest("GET", "/api/v1/projects", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Projects []*shared.Project `json:"projects"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Projects, nil
}

// Feature Flags

func (c *APIClient) ListFlags() ([]*shared.FeatureFlag, error) {
	resp, err := c.doRequest("GET", "/api/v1/flags", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Flags []*shared.FeatureFlag `json:"flags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Flags, nil
}

func (c *APIClient) CreateFlag(flag *shared.FeatureFlag) (*shared.FeatureFlag, error) {
	resp, err := c.doRequest("POST", "/api/v1/flag/create", flag)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result shared.FeatureFlag
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *APIClient) UpdateFlag(flag *shared.FeatureFlag) (*shared.FeatureFlag, error) {
	resp, err := c.doRequest("PUT", "/api/v1/flag/update", flag)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result shared.FeatureFlag
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *APIClient) ToggleFlag(flagID string) (*shared.FeatureFlag, error) {
	resp, err := c.doRequest("POST", "/api/v1/flag/toggle?id="+flagID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result shared.FeatureFlag
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *APIClient) UpdateRollout(flagID string, rollout int) (*shared.FeatureFlag, error) {
	reqBody, _ := json.Marshal(map[string]int{"rollout": rollout})
	resp, err := c.doRequest("POST", "/api/v1/flag/rollout?id="+flagID, reqBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result shared.FeatureFlag
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *APIClient) KillFlag(flagID string) error {
	_, err := c.doRequest("DELETE", "/api/v1/flag/kill?id="+flagID, nil)
	return err
}

func (c *APIClient) DeleteFlag(flagID string) error {
	_, err := c.doRequest("DELETE", "/api/v1/flag/delete?id="+flagID, nil)
	return err
}

// Users
func (c *APIClient) ListUsers() ([]*shared.User, error) {
	resp, err := c.doRequest("GET", "/api/v1/users", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Users []*shared.User `json:"users"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	for _, u := range result.Users {
		u.Password = ""
	}
	return result.Users, nil
}

func (c *APIClient) CreateUser(user *shared.User) (*shared.User, error) {
	resp, err := c.doRequest("POST", "/api/v1/user/create", user)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result shared.User
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	result.Password = ""
	return &result, nil
}

func (c *APIClient) UpdateUserRole(userID string, role shared.Role) error {
	reqBody, _ := json.Marshal(map[string]shared.Role{"role": role})
	_, err := c.doRequest("PATCH", "/api/v1/user/role?id="+userID, reqBody)
	return err
}

func (c *APIClient) UpdatePassword(userID, password string) error {
	reqBody, _ := json.Marshal(map[string]string{"password": password})
	_, err := c.doRequest("PATCH", "/api/v1/user/password?id="+userID, reqBody)
	return err
}

func (c *APIClient) DeleteUser(userID string) error {
	_, err := c.doRequest("DELETE", "/api/v1/user/delete?id="+userID, nil)
	return err
}

func (c *APIClient) TogglePresence(userID string) error {
	_, err := c.doRequest("POST", "/api/v1/user/presence?id="+userID, nil)
	return err
}

// Audit Log
func (c *APIClient) GetAuditLog() ([]*shared.AuditEntry, error) {
	resp, err := c.doRequest("GET", "/api/v1/audit", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		AuditLog []*shared.AuditEntry `json:"audit_log"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.AuditLog, nil
}

// WebSocket
func (c *APIClient) ConnectWebSocket() error {
	if c.token == "" {
		return fmt.Errorf("no token set, login first")
	}
	wsScheme := "ws"
	if len(c.baseURL) > 5 && c.baseURL[:5] == "https" {
		wsScheme = "wss"
	}
	u, _ := url.Parse(c.baseURL)
	wsURL := fmt.Sprintf("%s://%s/ws?token=%s", wsScheme, u.Host, c.token)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}
	c.wsConn = conn
	go c.readPump()
	return nil
}

func (c *APIClient) DisconnectWebSocket() error {
	if c.wsConn != nil {
		return c.wsConn.Close()
	}
	return nil
}

func (c *APIClient) readPump() {
	defer c.wsConn.Close()
	for {
		_, data, err := c.wsConn.ReadMessage()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[WS] read error: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "[WS] raw message: %s\n", string(data))
		var msg shared.WSMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			fmt.Fprintf(os.Stderr, "[WS] unmarshal error: %v\n", err)
			continue
		}
		fmt.Fprintf(os.Stderr, "[WS] parsed type=%s project=%s\n", msg.Type, msg.ProjectID)
		switch msg.Type {
		case "flag_change":
			fmt.Fprintf(os.Stderr, "[WS] calling OnFlagChange\n")
			if c.OnFlagChange != nil {
				c.OnFlagChange(msg)
			}
		case "user_change":
			fmt.Fprintf(os.Stderr, "[WS] calling OnUserChange\n")
			if c.OnUserChange != nil {
				c.OnUserChange(msg)
			}
		case "audit":
			fmt.Fprintf(os.Stderr, "[WS] calling OnAudit\n")
			if c.OnAudit != nil {
				c.OnAudit(msg)
			}
		}
	}
}

// Helpers
func (c *APIClient) doRequest(method, path string, body interface{}) (*http.Response, error) {
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

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	return resp, nil
}
