package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kthesword/github-trending-repos/models"
)

// RateLimitError is returned when GitHub refuses a request because a primary
// or secondary rate limit was exceeded (HTTP 403 or 429).
type RateLimitError struct {
	StatusCode int
	Message    string

	// Limit and Remaining mirror the X-RateLimit-* headers when present.
	Limit     int
	Remaining int

	// ResetAt is when the primary rate-limit window resets (zero if unknown).
	ResetAt time.Time

	// RetryAfter is the server-suggested back-off (zero if not provided).
	RetryAfter time.Duration
}

// Error implements the error interface.
func (e *RateLimitError) Error() string {
	msg := fmt.Sprintf("GitHub API rate limit exceeded (HTTP %d)", e.StatusCode)
	if wait := e.Wait(time.Now()); wait > 0 {
		msg += fmt.Sprintf("; retry in %s", wait.Round(time.Second))
	}
	return msg
}

// Wait reports how long the caller should wait, measured from now, before
// retrying. It returns 0 when GitHub gave no hint.
func (e *RateLimitError) Wait(now time.Time) time.Duration {
	if e.RetryAfter > 0 {
		return e.RetryAfter
	}
	if !e.ResetAt.IsZero() {
		if d := e.ResetAt.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}

// StatusError is returned for any other non-200 response.
type StatusError struct {
	StatusCode       int
	Status           string // e.g. "422 Unprocessable Entity"
	Message          string // message reported by GitHub, if any
	DocumentationURL string
}

// Error implements the error interface.
func (e *StatusError) Error() string {
	status := e.Status
	if status == "" {
		status = fmt.Sprintf("%d %s", e.StatusCode, http.StatusText(e.StatusCode))
	}
	if e.Message != "" {
		return fmt.Sprintf("GitHub API returned %s: %s", status, e.Message)
	}
	return "GitHub API returned " + status
}

// NetworkError wraps transport-level failures: DNS resolution, refused
// connections, TLS problems, timeouts, and so on.
type NetworkError struct {
	Err error
}

// Error implements the error interface.
func (e *NetworkError) Error() string { return "network error: " + e.Err.Error() }

// Unwrap exposes the underlying error to errors.Is / errors.As.
func (e *NetworkError) Unwrap() error { return e.Err }

// Timeout reports whether the failure was caused by a timeout.
func (e *NetworkError) Timeout() bool {
	var ne net.Error
	return errors.As(e.Err, &ne) && ne.Timeout()
}

// newResponseError converts a non-200 response into a typed error.
func newResponseError(resp *http.Response) error {
	// The body is best-effort: GitHub normally sends JSON, but proxies and
	// outages can produce HTML or nothing at all.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
	var payload models.APIError
	_ = json.Unmarshal(body, &payload)
	message := payload.Summary()

	if isRateLimited(resp, message) {
		return newRateLimitError(resp, message)
	}
	return &StatusError{
		StatusCode:       resp.StatusCode,
		Status:           resp.Status,
		Message:          message,
		DocumentationURL: payload.DocumentationURL,
	}
}

// isRateLimited applies GitHub's documented rules for detecting rate limits:
// HTTP 429 always, or HTTP 403 with an exhausted quota, a Retry-After header,
// or an explicit rate-limit message (used for secondary limits).
func isRateLimited(resp *http.Response, message string) bool {
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return true
	case http.StatusForbidden:
		if remaining, ok := headerInt(resp.Header, "X-RateLimit-Remaining"); ok && remaining == 0 {
			return true
		}
		if resp.Header.Get("Retry-After") != "" {
			return true
		}
		return strings.Contains(strings.ToLower(message), "rate limit")
	default:
		return false
	}
}

func newRateLimitError(resp *http.Response, message string) *RateLimitError {
	e := &RateLimitError{StatusCode: resp.StatusCode, Message: message}
	if v, ok := headerInt(resp.Header, "X-RateLimit-Limit"); ok {
		e.Limit = int(v)
	}
	if v, ok := headerInt(resp.Header, "X-RateLimit-Remaining"); ok {
		e.Remaining = int(v)
	}
	if v, ok := headerInt(resp.Header, "X-RateLimit-Reset"); ok && v > 0 {
		e.ResetAt = time.Unix(v, 0)
	}
	if v, ok := headerInt(resp.Header, "Retry-After"); ok && v > 0 {
		e.RetryAfter = time.Duration(v) * time.Second
	}
	return e
}

// headerInt parses an integer header, reporting whether it was present and
// well-formed.
func headerInt(h http.Header, key string) (int64, bool) {
	raw := strings.TrimSpace(h.Get(key))
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
