package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const successCode = "0"

type Client struct {
	baseURL    string
	username   string
	token      string
	httpClient *http.Client
}

type APIError struct {
	StatusCode int
	Code       string
	Message    string
	RequestID  string
}

func (e *APIError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "request failed"
	}
	if strings.TrimSpace(e.Code) == "" {
		return fmt.Sprintf("request failed: %s (http %d)", message, e.StatusCode)
	}
	return fmt.Sprintf("request failed: %s (code=%s http=%d)", message, e.Code, e.StatusCode)
}

type apiEnvelope struct {
	Code      string          `json:"code"`
	Message   string          `json:"message"`
	Data      json.RawMessage `json:"data"`
	RequestID string          `json:"request_id"`
}

type HealthStatus struct {
	Name      string    `json:"name"`
	Env       string    `json:"env"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
}

type LoginResult struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	UserInfo User   `json:"userInfo"`
}

type User struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Email    string `json:"email,omitempty"`
	Ext      string `json:"ext,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
	Status   string `json:"status,omitempty"`
}

type Library struct {
	ID      uint64 `json:"id"`
	UserID  uint64 `json:"userId"`
	Name    string `json:"name"`
	Starred bool   `json:"starred"`
}

type ScrollLibrariesResult struct {
	Items   []Library `json:"items"`
	HasMore bool      `json:"hasMore"`
}

type Node struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	ParentID    uint64 `json:"parentId"`
	LibraryID   uint64 `json:"libraryId"`
	Ext         string `json:"ext,omitempty"`
	MIMEType    string `json:"mimeType,omitempty"`
	FileSize    int64  `json:"fileSize,omitempty"`
	StorageKey  string `json:"storageKey,omitempty"`
	BuiltInType string `json:"builtInType,omitempty"`
	ArchiveMode int    `json:"archiveMode,omitempty"`
	ViewMeta    string `json:"viewMeta,omitempty"`
}

type SearchNodesRequest struct {
	LibraryID    uint64   `json:"libraryId"`
	Keyword      string   `json:"keyword,omitempty"`
	TagIDs       []uint64 `json:"tagIds,omitempty"`
	TagMatchMode string   `json:"tagMatchMode,omitempty"`
	Limit        int      `json:"limit,omitempty"`
}

func NewClient(baseURL, username, token string) *Client {
	return &Client{
		baseURL:  normalizeBaseURL(baseURL),
		username: strings.TrimSpace(username),
		token:    strings.TrimSpace(token),
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (c *Client) Health(ctx context.Context) (HealthStatus, error) {
	var out HealthStatus
	err := c.doJSON(ctx, http.MethodGet, "/healthz", nil, nil, false, &out)
	return out, err
}

func (c *Client) Login(ctx context.Context, username, password string) (LoginResult, error) {
	payload := map[string]string{
		"username": strings.TrimSpace(username),
		"password": strings.TrimSpace(password),
	}

	var out LoginResult
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/auth/login", nil, payload, false, &out)
	return out, err
}

func (c *Client) AuthStatus(ctx context.Context) (bool, error) {
	query := url.Values{}
	query.Set("username", c.username)
	query.Set("token", c.token)

	var out bool
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/auth/status", query, nil, true, &out)
	return out, err
}

func (c *Client) Logout(ctx context.Context) error {
	query := url.Values{}
	query.Set("username", c.username)
	query.Set("token", c.token)
	return c.doJSON(ctx, http.MethodDelete, "/api/v1/auth/logout", query, nil, true, nil)
}

func (c *Client) WhoAmI(ctx context.Context) (User, error) {
	var out User
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/user/me", nil, nil, true, &out)
	return out, err
}

func (c *Client) ScrollLibraries(ctx context.Context, lastID uint64, size int) (ScrollLibrariesResult, error) {
	query := url.Values{}
	if lastID > 0 {
		query.Set("lastId", strconv.FormatUint(lastID, 10))
	}
	if size > 0 {
		query.Set("size", strconv.Itoa(size))
	}

	var out ScrollLibrariesResult
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/libraries/scroll", query, nil, true, &out)
	return out, err
}

func (c *Client) ListChildren(ctx context.Context, nodeID, libraryID uint64) ([]Node, error) {
	query := url.Values{}
	query.Set("libraryId", strconv.FormatUint(libraryID, 10))

	var out []Node
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/v1/nodes/%d/children", nodeID), query, nil, true, &out)
	return out, err
}

func (c *Client) SearchNodes(ctx context.Context, req SearchNodesRequest) ([]Node, error) {
	var out []Node
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/nodes/search", nil, req, true, &out)
	return out, err
}

func (c *Client) doJSON(
	ctx context.Context,
	method string,
	apiPath string,
	query url.Values,
	body any,
	needAuth bool,
	out any,
) error {
	endpoint := c.baseURL + apiPath
	if len(query) > 0 {
		endpoint = endpoint + "?" + query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if needAuth {
		if c.username == "" || c.token == "" {
			return fmt.Errorf("missing login session, run `of auth login` first")
		}
		req.Header.Set("username", c.username)
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request api: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var envelope apiEnvelope
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		if resp.StatusCode >= http.StatusBadRequest {
			return &APIError{
				StatusCode: resp.StatusCode,
				Message:    strings.TrimSpace(string(respBody)),
			}
		}
		if out != nil {
			if err := json.Unmarshal(respBody, out); err != nil {
				return fmt.Errorf("decode response body: %w", err)
			}
		}
		return nil
	}

	if resp.StatusCode >= http.StatusBadRequest || envelope.Code != successCode {
		return &APIError{
			StatusCode: resp.StatusCode,
			Code:       envelope.Code,
			Message:    envelope.Message,
			RequestID:  envelope.RequestID,
		}
	}

	if out == nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decode response data: %w", err)
	}
	return nil
}
