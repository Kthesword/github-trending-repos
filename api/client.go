// Package api provides a small GitHub REST API client focused on the
// repository search endpoint used to discover trending projects.
//
// The client is safe for concurrent use and has no dependencies beyond the
// standard library and the local models package.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the root of the public GitHub REST API.
	DefaultBaseURL = "https://api.github.com"

	// DefaultUserAgent identifies this client to GitHub. GitHub requires every
	// request to carry a User-Agent header and rejects requests without one.
	DefaultUserAgent = "gh-trending-repos-cli"

	// DefaultTimeout bounds each HTTP request when no custom client is given.
	DefaultTimeout = 15 * time.Second

	// apiVersion pins the REST API version so response shapes stay stable.
	apiVersion = "2022-11-28"

	// mediaType is GitHub's recommended Accept header value.
	mediaType = "application/vnd.github+json"

	// maxErrorBodyBytes caps how much of an error response body is read.
	maxErrorBodyBytes = 1 << 20 // 1 MiB
)

// Client talks to the GitHub REST API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
	token      string
}

// Option customizes a Client.
type Option func(*Client)

// NewClient builds a Client with sensible defaults, then applies opts.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: DefaultTimeout},
		baseURL:    DefaultBaseURL,
		userAgent:  DefaultUserAgent,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithHTTPClient replaces the underlying *http.Client (e.g. to change the
// timeout or transport). A nil client is ignored.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// WithBaseURL points the client at a different API root, such as a GitHub
// Enterprise Server instance or a test server. Empty values are ignored.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		if u := strings.TrimRight(strings.TrimSpace(baseURL), "/"); u != "" {
			c.baseURL = u
		}
	}
}

// WithUserAgent overrides the User-Agent header. Empty values are ignored so
// the client can never send a request without one.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		if ua = strings.TrimSpace(ua); ua != "" {
			c.userAgent = ua
		}
	}
}

// WithToken sets an optional personal access token. Authentication is not
// required for public data but raises GitHub's rate limits considerably.
func WithToken(token string) Option {
	return func(c *Client) {
		c.token = strings.TrimSpace(token)
	}
}

// getJSON performs a GET request against endpoint and decodes a successful
// (HTTP 200) JSON response into v. Non-200 responses are converted into
// *RateLimitError or *StatusError; transport failures into *NetworkError.
func (c *Client) getJSON(ctx context.Context, endpoint string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", mediaType)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Surface caller-initiated cancellation (e.g. Ctrl+C) as-is so it is
		// not mistaken for a connectivity problem.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return &NetworkError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return newResponseError(resp)
	}

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("decoding GitHub API response: %w", err)
	}
	return nil
}
