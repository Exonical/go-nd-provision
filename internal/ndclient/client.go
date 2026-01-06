package ndclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/banglin/go-nd/internal/config"
	"github.com/banglin/go-nd/internal/ndclient/lanfabric"
)

// APIError represents an HTTP error from the Nexus Dashboard API
// Use errors.As(err, &APIError{}) to extract status code and body in handlers
type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Body       []byte
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s %s failed with status %d", e.Method, e.Path, e.StatusCode)
}

// BodyString returns the error body as a string, truncated to limit if specified
func (e *APIError) BodyString(limit int) string {
	if limit <= 0 || len(e.Body) <= limit {
		return string(e.Body)
	}
	return string(e.Body[:limit]) + "…"
}

// IsNotFound returns true if the error is a 404 Not Found
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == 404
}

// IsConflict returns true if the error is a 409 Conflict (resource already exists)
func (e *APIError) IsConflict() bool {
	return e.StatusCode == 409
}

// IsBadRequest returns true if the error is a 400 Bad Request
func (e *APIError) IsBadRequest() bool {
	return e.StatusCode == 400
}

// IsNotFoundError checks if an error is an APIError with 404 status
func IsNotFoundError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsNotFound()
	}
	return false
}

// IsConflictError checks if an error is an APIError with 409 status
func IsConflictError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsConflict()
	}
	return false
}

// newAPIError creates an APIError from an HTTP response
func newAPIError(method, path string, resp *http.Response) error {
	b, _ := io.ReadAll(resp.Body)
	return &APIError{
		Method:     method,
		Path:       path,
		StatusCode: resp.StatusCode,
		Body:       b,
	}
}

// decodeJSON safely decodes JSON from a response, handling empty bodies
func decodeJSON(resp *http.Response, out any) error {
	if out == nil {
		return nil
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return nil
	}
	return json.Unmarshal(b, out)
}

type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
	apiKey     string // API key for X-Nd-Apikey header
	username   string // Username for X-Nd-Username header (required with API key)
	endpoints  Endpoints

	// Service instances (lazy initialized)
	lanFabricService *lanfabric.Service
}

// buildURL safely joins baseURL and path, handling trailing/leading slashes
func (c *Client) buildURL(path string) string {
	base := strings.TrimRight(c.baseURL, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

func NewClient(cfg *config.NexusDashboardConfig) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.Insecure, //nolint:gosec // User-configurable for self-signed certs
		},
	}

	client := &Client{
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Transport: transport,
			Jar:       jar,
			Timeout:   120 * time.Second, // ConfigDeploy can take a long time
		},
		endpoints: DefaultEndpoints(),
	}

	// API key takes priority over username/password
	// API key auth uses X-Nd-Apikey and X-Nd-Username headers
	if cfg.APIKey != "" {
		client.apiKey = cfg.APIKey
		client.username = cfg.Username
		return client, nil
	}

	// Fall back to username/password authentication
	if cfg.Username != "" && cfg.Password != "" {
		if err := client.authenticate(cfg.Username, cfg.Password); err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
	}

	return client, nil
}

// WithEndpoints allows overriding the default API endpoints
func (c *Client) WithEndpoints(e Endpoints) *Client {
	c.endpoints = e
	return c
}

type loginRequest struct {
	UserName string `json:"userName"`
	UserPass string `json:"userPasswd"`
	Domain   string `json:"domain"`
}

type loginResponse struct {
	Token string `json:"token"`
}

func (c *Client) authenticate(username, password string) error {
	loginData := loginRequest{
		UserName: username,
		UserPass: password,
		Domain:   "DefaultAuth",
	}

	body, err := json.Marshal(loginData)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.buildURL("/login"), bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return newAPIError("POST", "/login", resp)
	}

	var loginResp loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return err
	}

	c.token = loginResp.Token
	return nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.buildURL(path), reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	// API key auth uses X-Nd-Apikey and X-Nd-Username headers
	if c.apiKey != "" {
		req.Header.Set("X-Nd-Apikey", c.apiKey)
		req.Header.Set("X-Nd-Username", c.username)
	} else if c.token != "" {
		// Token-based auth (from username/password login)
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	return c.httpClient.Do(req)
}

func (c *Client) Get(ctx context.Context, path string, result interface{}) error {
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return newAPIError("GET", path, resp)
	}

	return decodeJSON(resp, result)
}

func (c *Client) Post(ctx context.Context, path string, body, result interface{}) error {
	resp, err := c.doRequest(ctx, "POST", path, body)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		return decodeJSON(resp, result)
	default:
		return newAPIError("POST", path, resp)
	}
}

func (c *Client) Put(ctx context.Context, path string, body, result interface{}) error {
	resp, err := c.doRequest(ctx, "PUT", path, body)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return newAPIError("PUT", path, resp)
	}

	return decodeJSON(resp, result)
}

func (c *Client) Delete(ctx context.Context, path string) error {
	resp, err := c.doRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent, http.StatusAccepted:
		return nil
	default:
		return newAPIError("DELETE", path, resp)
	}
}

// LANFabric returns the LAN fabric service for fabric/switch/port operations
func (c *Client) LANFabric() *lanfabric.Service {
	if c.lanFabricService == nil {
		c.lanFabricService = lanfabric.NewService(c)
	}
	return c.lanFabricService
}

// NDFCLanFabricPath implements lanfabric.ClientInterface
func (c *Client) NDFCLanFabricPath(parts ...string) (string, error) {
	return c.ndfcLanFabricPath(parts...)
}

// NDLanFabricPath implements lanfabric.ClientInterface
func (c *Client) NDLanFabricPath(parts ...string) (string, error) {
	return c.ndLanFabricPath(parts...)
}
