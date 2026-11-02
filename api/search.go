package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/kthesword/github-trending-repos/models"
)

const (
	// MaxPerPage is the largest page size the Search API accepts.
	MaxPerPage = 100

	// MaxSearchResults is the hard cap GitHub places on the number of results
	// reachable through pagination for any single search query.
	MaxSearchResults = 1000
)

// SearchResult is the aggregated outcome of a (possibly paginated) search.
type SearchResult struct {
	// TotalCount is the number of repositories GitHub matched, which may be
	// far larger than the number of repositories returned.
	TotalCount int

	// Incomplete reports whether GitHub flagged any page as incomplete
	// (the search timed out server-side before finding every match).
	Incomplete bool

	// Repositories are sorted by star count, highest first.
	Repositories []models.Repository
}

// SearchOption customizes a trending search.
type SearchOption func(*searchParams)

type searchParams struct {
	language string
}

// WithLanguage restricts results to repositories whose primary language is
// language (e.g. "Go", "C++", "Jupyter Notebook"). GitHub matches language
// names case-insensitively. An empty value applies no filter. The value
// should already be validated with models.ParseLanguage.
func WithLanguage(language string) SearchOption {
	return func(p *searchParams) { p.language = language }
}

// BuildTrendingQuery returns the GitHub search query (the "q" parameter)
// matching public repositories created at or after since, narrowed by any
// options such as WithLanguage.
//
// A full UTC timestamp is used instead of a bare date so that short windows
// such as "day" are precise to the second.
func BuildTrendingQuery(since time.Time, opts ...SearchOption) string {
	var p searchParams
	for _, opt := range opts {
		opt(&p)
	}

	q := fmt.Sprintf("created:>=%s is:public", since.UTC().Format(time.RFC3339))
	if p.language != "" {
		// Quoted so names containing spaces ("Jupyter Notebook") stay one term.
		q += fmt.Sprintf(` language:"%s"`, p.language)
	}
	return q
}

// SearchTrending fetches up to limit public repositories created at or after
// since, ordered by star count (descending).
//
// When limit exceeds MaxPerPage the results are fetched across multiple
// pages. limit is clamped to MaxSearchResults, GitHub's own ceiling.
func (c *Client) SearchTrending(ctx context.Context, since time.Time, limit int, opts ...SearchOption) (*SearchResult, error) {
	if limit < 1 {
		return nil, fmt.Errorf("limit must be at least 1, got %d", limit)
	}
	limit = min(limit, MaxSearchResults)

	query := BuildTrendingQuery(since, opts...)
	perPage := min(limit, MaxPerPage)

	result := &SearchResult{Repositories: make([]models.Repository, 0, limit)}
	// Star counts can change between page requests, shifting items across
	// page boundaries; track what we've seen to avoid duplicates.
	seen := make(map[string]struct{}, limit)

	for page := 1; len(result.Repositories) < limit; page++ {
		resp, err := c.searchRepositories(ctx, query, page, perPage)
		if err != nil {
			return nil, err
		}

		result.TotalCount = resp.TotalCount
		result.Incomplete = result.Incomplete || resp.IncompleteResults

		for _, repo := range resp.Items {
			if _, dup := seen[repo.FullName]; dup {
				continue
			}
			seen[repo.FullName] = struct{}{}
			result.Repositories = append(result.Repositories, repo)
		}

		// Stop when the API has nothing more to give: a short page, or we've
		// reached the end of the reachable result set.
		reachable := min(resp.TotalCount, MaxSearchResults)
		if len(resp.Items) < perPage || page*perPage >= reachable {
			break
		}
	}

	if len(result.Repositories) > limit {
		result.Repositories = result.Repositories[:limit]
	}

	// The API already sorts by stars, but enforce the ordering locally so the
	// guarantee holds regardless of pagination drift or API quirks.
	models.SortByStars(result.Repositories)
	return result, nil
}

// searchRepositories requests a single page of search results.
func (c *Client) searchRepositories(ctx context.Context, query string, page, perPage int) (*models.SearchResponse, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("sort", "stars")
	params.Set("order", "desc")
	params.Set("per_page", strconv.Itoa(perPage))
	params.Set("page", strconv.Itoa(page))

	endpoint := c.baseURL + "/search/repositories?" + params.Encode()

	var out models.SearchResponse
	if err := c.getJSON(ctx, endpoint, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
