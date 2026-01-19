package rancher

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/goldyfruit/elemental-node-mapper/internal/types"
)

const (
	defaultTimeout = 20 * time.Second
	defaultRetries = 3
)

type Client struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
}

type APIError struct {
	StatusCode int
	Err        error
}

func (e *APIError) Error() string {
	if e.StatusCode == http.StatusUnauthorized {
		return "rancher authentication failed"
	}
	if e.StatusCode == http.StatusForbidden {
		return "rancher authorization failed"
	}
	if e.StatusCode >= 400 {
		return fmt.Sprintf("rancher API error (status %d)", e.StatusCode)
	}
	return fmt.Sprintf("rancher API error: %v", e.Err)
}

func (e *APIError) Unwrap() error {
	return e.Err
}

func NewClient(rawURL, token string, insecureSkipVerify bool) (*Client, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("rancher URL is required")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid rancher URL: %w", err)
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify},
	}
	return &Client{
		baseURL: parsed,
		token:   token,
		httpClient: &http.Client{
			Timeout:   defaultTimeout,
			Transport: transport,
		},
	}, nil
}

func (c *Client) ListInventoryHosts(ctx context.Context) ([]types.InventoryHost, error) {
	var hosts []types.InventoryHost
	nextURL := c.withLimit(c.baseURL, 200)

	for nextURL != nil {
		page, err := c.fetchPage(ctx, nextURL)
		if err != nil {
			return nil, err
		}
		for _, raw := range page.Data {
			hosts = append(hosts, normalizeHost(raw))
		}
		nextURL = page.NextURL(c.baseURL)
	}

	return hosts, nil
}

type listResponse struct {
	Data       []map[string]any  `json:"data"`
	Links      map[string]string `json:"links"`
	Pagination struct {
		Next string `json:"next"`
	} `json:"pagination"`
}

func (r listResponse) NextURL(base *url.URL) *url.URL {
	if r.Pagination.Next != "" {
		return resolveURL(base, r.Pagination.Next)
	}
	if r.Links != nil {
		if next, ok := r.Links["next"]; ok && next != "" {
			return resolveURL(base, next)
		}
	}
	return nil
}

func (c *Client) fetchPage(ctx context.Context, target *url.URL) (listResponse, error) {
	var lastErr error
	for attempt := 0; attempt < defaultRetries; attempt++ {
		resp, err := c.doRequest(ctx, target)
		if err != nil {
			lastErr = err
			if shouldRetry(err) {
				time.Sleep(backoff(attempt))
				continue
			}
			return listResponse{}, err
		}
		return resp, nil
	}
	if lastErr == nil {
		lastErr = errors.New("rancher request failed")
	}
	return listResponse{}, lastErr
}

func (c *Client) doRequest(ctx context.Context, target *url.URL) (listResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return listResponse{}, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "elemental-node-map/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return listResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return listResponse{}, &APIError{StatusCode: resp.StatusCode, Err: fmt.Errorf("status %d", resp.StatusCode)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return listResponse{}, &APIError{StatusCode: resp.StatusCode, Err: fmt.Errorf("status %d", resp.StatusCode)}
	}

	var payload listResponse
	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return listResponse{}, &APIError{StatusCode: resp.StatusCode, Err: err}
	}
	return payload, nil
}

func (c *Client) doJSONRequest(ctx context.Context, method string, target *url.URL, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, target.String(), nil)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "elemental-node-map/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &APIError{StatusCode: resp.StatusCode, Err: fmt.Errorf("status %d", resp.StatusCode)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode, Err: fmt.Errorf("status %d", resp.StatusCode)}
	}

	if out == nil {
		return nil
	}
	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()
	if err := decoder.Decode(out); err != nil {
		return &APIError{StatusCode: resp.StatusCode, Err: err}
	}
	return nil
}

func (c *Client) withLimit(base *url.URL, limit int) *url.URL {
	clone := *base
	q := clone.Query()
	if q.Get("limit") == "" {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	clone.RawQuery = q.Encode()
	return &clone
}

func resolveURL(base *url.URL, next string) *url.URL {
	if next == "" {
		return nil
	}
	parsed, err := url.Parse(next)
	if err != nil {
		return nil
	}
	if parsed.IsAbs() {
		return parsed
	}
	if parsed.Path == "" && parsed.RawQuery != "" {
		clone := *base
		clone.RawQuery = parsed.RawQuery
		return &clone
	}
	clone := *base
	if strings.HasPrefix(next, "/") {
		clone.Path = next
		clone.RawQuery = parsed.RawQuery
		return &clone
	}
	clone.Path = strings.TrimSuffix(clone.Path, "/") + "/" + next
	clone.RawQuery = parsed.RawQuery
	return &clone
}

func shouldRetry(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= 500
	}
	return true
}

func backoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 250 * time.Millisecond
	}
	if attempt == 1 {
		return 600 * time.Millisecond
	}
	return 1200 * time.Millisecond
}
