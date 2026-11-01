package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kthesword/github-trending-repos/api"
	"github.com/kthesword/github-trending-repos/models"
)

// execute runs a fresh root command with args and returns its output.
func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

func TestInvalidInputIsUsageError(t *testing.T) {
	// Point at an unreachable URL: none of these cases may hit the network.
	t.Setenv(envAPIURL, "http://127.0.0.1:0")

	tests := map[string][]string{
		"bad duration":      {"--duration", "decade"},
		"zero limit":        {"--limit", "0"},
		"negative limit":    {"--limit", "-5"},
		"limit too large":   {"--limit", "1001"},
		"non-numeric limit": {"--limit", "ten"},
		"zero timeout":      {"--timeout", "0s"},
		"unknown flag":      {"--stars"},
		"positional arg":    {"month"},
	}

	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := execute(t, args...)
			var ue *usageError
			if !errors.As(err, &ue) {
				t.Fatalf("error = %v (%T), want *usageError", err, err)
			}
			if code := explain(err, time.Now()).exitCode; code != exitUsage {
				t.Errorf("exit code = %d, want %d", code, exitUsage)
			}
		})
	}
}

func TestEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if !strings.HasPrefix(q.Get("q"), "created:>=") {
			t.Errorf("q = %q, want created:>= qualifier", q.Get("q"))
		}
		if got := q.Get("per_page"); got != "2" {
			t.Errorf("per_page = %q, want 2", got)
		}
		if ua := r.Header.Get("User-Agent"); !strings.HasPrefix(ua, appName+"/") {
			t.Errorf("User-Agent = %q, want %s/<version>", ua, appName)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(models.SearchResponse{
			TotalCount: 2,
			Items: []models.Repository{
				{FullName: "acme/second", StargazersCount: 10, Language: "Go"},
				{FullName: "acme/first", StargazersCount: 900, Description: "The best"},
			},
		})
	}))
	t.Cleanup(srv.Close)

	t.Setenv(envAPIURL, srv.URL)
	t.Setenv(envToken, "")

	out, err := execute(t, "--duration", "month", "--limit", "2", "--no-color")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}

	first, second := strings.Index(out, "acme/first"), strings.Index(out, "acme/second")
	if first < 0 || second < 0 || first > second {
		t.Errorf("repositories missing or not sorted by stars:\n%s", out)
	}
	if !strings.Contains(out, "past month") {
		t.Errorf("output does not mention the period:\n%s", out)
	}
}

func TestVersionFlag(t *testing.T) {
	out, err := execute(t, "--version")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if !strings.Contains(out, "version") {
		t.Errorf("unexpected version output: %q", out)
	}
}

func TestExplain(t *testing.T) {
	now := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		err         error
		wantCode    int
		wantMessage string
		wantHint    string
	}{
		{
			name:        "rate limit",
			err:         &api.RateLimitError{StatusCode: 403, ResetAt: now.Add(2 * time.Minute)},
			wantCode:    exitFailure,
			wantMessage: "rate limit",
			wantHint:    envToken,
		},
		{
			name:        "cancelled",
			err:         context.Canceled,
			wantCode:    exitInterrupted,
			wantMessage: "cancelled",
		},
		{
			name:        "timeout",
			err:         &api.NetworkError{Err: timeoutError{}},
			wantCode:    exitFailure,
			wantMessage: "timed out",
			wantHint:    "--timeout",
		},
		{
			name:        "network",
			err:         &api.NetworkError{Err: errors.New("no such host")},
			wantCode:    exitFailure,
			wantMessage: "no such host",
		},
		{
			name:        "server error",
			err:         &api.StatusError{StatusCode: 503},
			wantCode:    exitFailure,
			wantMessage: "HTTP 503",
			wantHint:    "githubstatus.com",
		},
		{
			name:        "unauthorized",
			err:         &api.StatusError{StatusCode: 401, Message: "Bad credentials"},
			wantCode:    exitFailure,
			wantMessage: "credentials",
			wantHint:    "Bad credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fe := explain(tt.err, now)
			if fe.exitCode != tt.wantCode {
				t.Errorf("exitCode = %d, want %d", fe.exitCode, tt.wantCode)
			}
			if !strings.Contains(fe.message, tt.wantMessage) {
				t.Errorf("message = %q, want it to contain %q", fe.message, tt.wantMessage)
			}
			if tt.wantHint != "" && !strings.Contains(strings.Join(fe.hints, "\n"), tt.wantHint) {
				t.Errorf("hints = %q, want one containing %q", fe.hints, tt.wantHint)
			}
		})
	}
}

// timeoutError is a net.Error that reports a timeout.
type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }
