// Package sftpgo provisions SFTP accounts through the SFTPGo REST API.
package sftpgo

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

const (
	apiKeyHeader     = "X-SFTPGO-API-KEY" //nolint:gosec // header name, not a credential
	jsonMediaType    = "application/json"
	requestTimeout   = 10 * time.Second
	maxUpstreamBytes = 1 << 20
	maxErrorDetail   = 500
)

// Client calls the SFTPGo admin API with an API key. The key is never logged.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient builds a client for a SFTPGo server root (without /api/v2).
func NewClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: requestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}

	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/api/v2",
		apiKey:     apiKey,
		httpClient: httpClient,
	}
}

// GetFolder returns the folder and false when SFTPGo responds 404.
func (c *Client) GetFolder(ctx context.Context, name string) (map[string]any, bool, error) {
	return c.get(ctx, "/folders/"+url.PathEscape(name))
}

// CreateFolder creates a virtual folder.
func (c *Client) CreateFolder(ctx context.Context, folder map[string]any) error {
	return c.write(ctx, http.MethodPost, "/folders", folder)
}

// GetUser returns the user and false when SFTPGo responds 404.
func (c *Client) GetUser(ctx context.Context, username string) (map[string]any, bool, error) {
	return c.get(ctx, "/users/"+url.PathEscape(username))
}

// CreateUser creates a user. The body may contain a password; this method does not log it.
func (c *Client) CreateUser(ctx context.Context, user map[string]any) error {
	return c.write(ctx, http.MethodPost, "/users", user)
}

// UpdateUser replaces a user. Callers must omit the password so the current one is kept.
func (c *Client) UpdateUser(ctx context.Context, username string, user map[string]any) error {
	return c.write(ctx, http.MethodPut, "/users/"+url.PathEscape(username), user)
}

func (c *Client) get(ctx context.Context, resource string) (map[string]any, bool, error) {
	status, body, err := c.roundTrip(ctx, http.MethodGet, resource, nil)
	if err != nil {
		return nil, false, err
	}

	if status == http.StatusNotFound {
		return nil, false, nil
	}

	if !successStatus(status) {
		return nil, false, c.upstream(http.MethodGet, resource, status, body)
	}

	doc, err := decodeObject(body)
	if err != nil {
		return nil, false, c.upstream(http.MethodGet, resource, status, []byte("invalid JSON"))
	}

	return doc, true, nil
}

func (c *Client) write(ctx context.Context, method, resource string, payload map[string]any) error {
	status, body, err := c.roundTrip(ctx, method, resource, payload)
	if err != nil {
		return err
	}

	if !successStatus(status) {
		return c.upstream(method, resource, status, body)
	}

	return nil
}

func (c *Client) roundTrip(ctx context.Context, method, resource string, payload any) (int, []byte, error) {
	var reader io.Reader

	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}

		reader = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+resource, reader)
	if err != nil {
		return 0, nil, c.transportFailure(method, resource, err)
	}

	request.Header.Set("Accept", jsonMediaType)
	request.Header.Set("Content-Type", jsonMediaType)
	request.Header.Set(apiKeyHeader, c.apiKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return 0, nil, c.transportFailure(method, resource, err)
	}

	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxUpstreamBytes))
	if err != nil {
		return 0, nil, c.transportFailure(method, resource, err)
	}

	return response.StatusCode, body, nil
}

func (c *Client) transportFailure(method, resource string, err error) error {
	return c.upstream(method, resource, 0, []byte(err.Error()))
}

func (c *Client) upstream(method, resource string, status int, body []byte) *model.SFTPGoError {
	detail := trimDetail(string(body))
	if c.apiKey != "" {
		detail = strings.ReplaceAll(detail, c.apiKey, "[redacted]")
	}

	message := "sftpgo " + method + " " + resource
	if status > 0 {
		message += ": HTTP " + strconv.Itoa(status)
	}

	if detail != "" {
		message += ": " + detail
	}

	return &model.SFTPGoError{Status: status, Message: message}
}

func successStatus(status int) bool {
	return status >= http.StatusOK && status < http.StatusMultipleChoices
}

func decodeObject(body []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return map[string]any{}, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()

	var doc map[string]any
	if err := decoder.Decode(&doc); err != nil {
		return nil, err
	}

	if doc == nil {
		return map[string]any{}, nil
	}

	return doc, nil
}

func trimDetail(body string) string {
	body = strings.TrimSpace(body)

	runes := []rune(body)
	if len(runes) <= maxErrorDetail {
		return body
	}

	return string(runes[:maxErrorDetail])
}
