package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/kthesword/github-trending-repos/api"
)

// Process exit codes.
const (
	exitOK          = 0
	exitFailure     = 1   // runtime failure (network, API, rendering)
	exitUsage       = 2   // invalid flags or arguments
	exitInterrupted = 130 // cancelled by SIGINT (Ctrl+C), per shell convention
)

// usageError marks errors caused by invalid user input, as opposed to
// runtime failures. They receive a "--help" hint and exit code 2.
type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

func usageErrorf(format string, a ...any) error {
	return &usageError{err: fmt.Errorf(format, a...)}
}

// noPositionalArgs rejects stray arguments (a common typo is omitting the
// "--duration" flag name, e.g. `trending-repos month`).
func noPositionalArgs(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return usageErrorf("unexpected argument(s): %s (did you mean --duration %s?)",
			strings.Join(args, " "), args[0])
	}
	return nil
}

// friendlyError is a user-facing rendering of an error.
type friendlyError struct {
	message  string
	hints    []string
	exitCode int
}

// explain translates any error into an actionable, human-readable message.
func explain(err error, now time.Time) friendlyError {
	var (
		usageErr  *usageError
		rateErr   *api.RateLimitError
		netErr    *api.NetworkError
		statusErr *api.StatusError
	)

	switch {
	case errors.As(err, &usageErr):
		return friendlyError{
			message:  usageErr.Error(),
			hints:    []string{fmt.Sprintf("Run '%s --help' for usage.", appName)},
			exitCode: exitUsage,
		}

	case errors.Is(err, context.Canceled):
		return friendlyError{message: "Operation cancelled.", exitCode: exitInterrupted}

	case errors.As(err, &rateErr):
		fe := friendlyError{
			message:  fmt.Sprintf("GitHub API rate limit exceeded (HTTP %d).", rateErr.StatusCode),
			exitCode: exitFailure,
		}
		if wait := rateErr.Wait(now); wait > 0 {
			fe.hints = append(fe.hints, fmt.Sprintf("Try again in %s (at %s).",
				wait.Round(time.Second), now.Add(wait).Local().Format("15:04:05")))
		}
		fe.hints = append(fe.hints, fmt.Sprintf(
			"Unauthenticated clients get 10 searches per minute; set %s to a personal access token for a higher limit.",
			envToken))
		return fe

	case errors.As(err, &netErr):
		if netErr.Timeout() {
			return friendlyError{
				message:  "The request to GitHub timed out.",
				hints:    []string{"Check your internet connection, or allow more time with --timeout (e.g. --timeout 30s)."},
				exitCode: exitFailure,
			}
		}
		// url.Error includes the full request URL, which is noise for users.
		cause := netErr.Err
		var urlErr *url.Error
		if errors.As(cause, &urlErr) {
			cause = urlErr.Err
		}
		return friendlyError{
			message:  fmt.Sprintf("Could not reach GitHub: %v", cause),
			hints:    []string{"Check your internet connection, DNS, and proxy settings (HTTPS_PROXY)."},
			exitCode: exitFailure,
		}

	case errors.As(err, &statusErr):
		return explainStatus(statusErr)

	default:
		return friendlyError{message: err.Error(), exitCode: exitFailure}
	}
}

// explainStatus handles non-rate-limit HTTP errors.
func explainStatus(e *api.StatusError) friendlyError {
	fe := friendlyError{exitCode: exitFailure}

	switch code := e.StatusCode; {
	case code == http.StatusUnauthorized:
		fe.message = "GitHub rejected the provided credentials (HTTP 401)."
		fe.hints = []string{fmt.Sprintf("Check that %s holds a valid, unexpired token, or unset it to search anonymously.", envToken)}
	case code == http.StatusForbidden:
		fe.message = "GitHub denied access to the API (HTTP 403)."
	case code == http.StatusUnprocessableEntity:
		fe.message = "GitHub rejected the search query (HTTP 422)."
	case code >= 500:
		fe.message = fmt.Sprintf("GitHub is having trouble right now (HTTP %d).", code)
		fe.hints = []string{"Please try again in a few moments. Service status: https://www.githubstatus.com"}
	default:
		fe.message = fmt.Sprintf("Unexpected response from GitHub (HTTP %d %s).", code, http.StatusText(code))
	}

	if e.Message != "" {
		fe.hints = append([]string{"GitHub says: " + e.Message}, fe.hints...)
	}
	return fe
}

// printError writes a friendly error report to w and returns the exit code.
func printError(w io.Writer, err error) int {
	fe := explain(err, time.Now())

	label := color.New(color.FgHiRed, color.Bold).Sprint("Error:")
	fmt.Fprintf(w, "%s %s\n", label, fe.message)

	hint := color.New(color.FgHiBlack)
	for _, h := range fe.hints {
		fmt.Fprintf(w, "  %s %s\n", hint.Sprint("→"), h)
	}
	return fe.exitCode
}
