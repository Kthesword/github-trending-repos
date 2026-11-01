// Package models defines the data structures used to decode GitHub REST API
// payloads, together with small domain helpers (sorting, time ranges) that
// operate on them.
package models

import (
	"cmp"
	"slices"
	"strings"
	"time"
)

// SearchResponse mirrors the top-level JSON document returned by
// GET /search/repositories.
type SearchResponse struct {
	TotalCount        int          `json:"total_count"`
	IncompleteResults bool         `json:"incomplete_results"`
	Items             []Repository `json:"items"`
}

// Repository holds the subset of repository fields this CLI cares about.
// GitHub returns many more fields; unknown ones are ignored by encoding/json.
//
// Nullable JSON fields (description, language, pushed_at) decode to their
// zero values, which callers treat as "not available".
type Repository struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	FullName        string    `json:"full_name"`
	Description     string    `json:"description"`
	HTMLURL         string    `json:"html_url"`
	Language        string    `json:"language"`
	StargazersCount int       `json:"stargazers_count"`
	ForksCount      int       `json:"forks_count"`
	CreatedAt       time.Time `json:"created_at"`
	PushedAt        time.Time `json:"pushed_at"`
	Owner           Owner     `json:"owner"`
}

// Owner identifies the user or organization that owns a repository.
type Owner struct {
	Login   string `json:"login"`
	HTMLURL string `json:"html_url"`
}

// APIError is the error document GitHub returns alongside non-2xx responses.
type APIError struct {
	Message          string        `json:"message"`
	DocumentationURL string        `json:"documentation_url"`
	Errors           []ErrorDetail `json:"errors"`
}

// ErrorDetail describes an individual validation problem (typically sent
// with HTTP 422 Unprocessable Entity).
type ErrorDetail struct {
	Resource string `json:"resource"`
	Field    string `json:"field"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// Summary flattens the top-level message and any detailed validation
// messages into a single human-readable line.
func (e APIError) Summary() string {
	details := make([]string, 0, len(e.Errors))
	for _, d := range e.Errors {
		if m := strings.TrimSpace(d.Message); m != "" {
			details = append(details, m)
		}
	}

	msg := strings.TrimSpace(e.Message)
	switch {
	case len(details) == 0:
		return msg
	case msg == "":
		return strings.Join(details, "; ")
	default:
		return msg + ": " + strings.Join(details, "; ")
	}
}

// SortByStars orders repositories in place by star count, highest first.
// Ties are broken alphabetically (case-insensitive) by full name so that the
// output is deterministic across runs.
func SortByStars(repos []Repository) {
	slices.SortStableFunc(repos, func(a, b Repository) int {
		if c := cmp.Compare(b.StargazersCount, a.StargazersCount); c != 0 {
			return c
		}
		return strings.Compare(strings.ToLower(a.FullName), strings.ToLower(b.FullName))
	})
}
