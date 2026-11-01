package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kthesword/gh-trending-repos/models"
)

var testSince = time.Date(2026, 10, 25, 7, 2, 43, 0, time.UTC)

// newTestClient starts an httptest server running h and returns a Client
// pointed at it.
func newTestClient(t *testing.T, h http.HandlerFunc, opts ...Option) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewClient(append([]Option{WithBaseURL(srv.URL), WithHTTPClient(srv.Client())}, opts...)...)
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Errorf("encoding response: %v", err)
	}
}

func TestBuildTrendingQuery(t *testing.T) {
	local := time.Date(2026, 10, 25, 10, 2, 43, 0, time.FixedZone("EAT", 3*60*60))
	want := "created:>=2026-10-25T07:02:43Z is:public"
	if got := BuildTrendingQuery(local); got != want {
		t.Errorf("BuildTrendingQuery() = %q, want %q", got, want)
	}
}

func TestSearchTrendingSendsCorrectRequestAndSorts(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/repositories" {
			t.Errorf("path = %q, want /search/repositories", r.URL.Path)
		}
		checkHeader(t, r, "User-Agent", DefaultUserAgent)
		checkHeader(t, r, "Accept", mediaType)
		checkHeader(t, r, "X-GitHub-Api-Version", apiVersion)
		checkHeader(t, r, "Authorization", "")

		q := r.URL.Query()
		checkParam(t, q.Get("q"), "q", "created:>=2026-10-25T07:02:43Z is:public")
		checkParam(t, q.Get("sort"), "sort", "stars")
		checkParam(t, q.Get("order"), "order", "desc")
		checkParam(t, q.Get("per_page"), "per_page", "5")
		checkParam(t, q.Get("page"), "page", "1")

		writeJSON(t, w, http.StatusOK, models.SearchResponse{
			TotalCount: 3,
			Items: []models.Repository{
				{FullName: "a/low", StargazersCount: 5},
				{FullName: "b/high", StargazersCount: 500},
				{FullName: "c/mid", StargazersCount: 50},
			},
		})
	})

	res, err := client.SearchTrending(context.Background(), testSince, 5)
	if err != nil {
		t.Fatalf("SearchTrending() error = %v", err)
	}
	if res.TotalCount != 3 {
		t.Errorf("TotalCount = %d, want 3", res.TotalCount)
	}
	want := []string{"b/high", "c/mid", "a/low"}
	if len(res.Repositories) != len(want) {
		t.Fatalf("got %d repositories, want %d", len(res.Repositories), len(want))
	}
	for i, name := range want {
		if res.Repositories[i].FullName != name {
			t.Errorf("Repositories[%d] = %q, want %q", i, res.Repositories[i].FullName, name)
		}
	}
}

func TestSearchTrendingSendsToken(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		checkHeader(t, r, "Authorization", "Bearer s3cret")
		writeJSON(t, w, http.StatusOK, models.SearchResponse{})
	}, WithToken("  s3cret  "))

	if _, err := client.SearchTrending(context.Background(), testSince, 1); err != nil {
		t.Fatalf("SearchTrending() error = %v", err)
	}
}

func TestSearchTrendingPaginates(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		checkParam(t, r.URL.Query().Get("per_page"), "per_page", "100")

		items := make([]models.Repository, 100)
		for i := range items {
			n := (page-1)*100 + i
			items[i] = models.Repository{FullName: fmt.Sprintf("o/r%04d", n), StargazersCount: 10_000 - n}
		}
		writeJSON(t, w, http.StatusOK, models.SearchResponse{TotalCount: 5000, Items: items})
	})

	res, err := client.SearchTrending(context.Background(), testSince, 150)
	if err != nil {
		t.Fatalf("SearchTrending() error = %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("made %d requests, want 2", got)
	}
	if len(res.Repositories) != 150 {
		t.Fatalf("got %d repositories, want 150", len(res.Repositories))
	}
	if first := res.Repositories[0].FullName; first != "o/r0000" {
		t.Errorf("first repository = %q, want o/r0000", first)
	}
}

func TestSearchTrendingStopsWhenResultsExhausted(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		writeJSON(t, w, http.StatusOK, models.SearchResponse{
			TotalCount: 2,
			Items:      []models.Repository{{FullName: "a/a"}, {FullName: "b/b"}},
		})
	})

	res, err := client.SearchTrending(context.Background(), testSince, 500)
	if err != nil {
		t.Fatalf("SearchTrending() error = %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("made %d requests, want 1", got)
	}
	if len(res.Repositories) != 2 {
		t.Errorf("got %d repositories, want 2", len(res.Repositories))
	}
}

func TestSearchTrendingRejectsInvalidLimit(t *testing.T) {
	client := NewClient(WithBaseURL("http://127.0.0.1:0")) // never contacted
	if _, err := client.SearchTrending(context.Background(), testSince, 0); err == nil {
		t.Fatal("expected an error for limit 0")
	}
}

func TestRateLimitPrimary403(t *testing.T) {
	reset := time.Now().Add(5 * time.Minute).Unix()
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "10")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))
		writeJSON(t, w, http.StatusForbidden, models.APIError{Message: "API rate limit exceeded for 203.0.113.7."})
	})

	_, err := client.SearchTrending(context.Background(), testSince, 10)

	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("error = %v (%T), want *RateLimitError", err, err)
	}
	if rl.StatusCode != http.StatusForbidden || rl.Limit != 10 || rl.Remaining != 0 {
		t.Errorf("unexpected fields: %+v", rl)
	}
	if rl.ResetAt.Unix() != reset {
		t.Errorf("ResetAt = %v, want unix %d", rl.ResetAt, reset)
	}
	if rl.Wait(time.Now()) <= 0 {
		t.Error("Wait() should be positive before the reset time")
	}
}

func TestRateLimit429RetryAfter(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		writeJSON(t, w, http.StatusTooManyRequests, models.APIError{Message: "slow down"})
	})

	_, err := client.SearchTrending(context.Background(), testSince, 10)

	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("error = %v (%T), want *RateLimitError", err, err)
	}
	if got := rl.Wait(time.Now()); got != 30*time.Second {
		t.Errorf("Wait() = %v, want 30s", got)
	}
}

func TestRateLimitSecondaryByMessage(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusForbidden, models.APIError{Message: "You have exceeded a secondary rate limit."})
	})

	_, err := client.SearchTrending(context.Background(), testSince, 10)

	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("error = %v (%T), want *RateLimitError", err, err)
	}
}

func TestForbiddenWithoutRateLimitIsStatusError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "7")
		writeJSON(t, w, http.StatusForbidden, models.APIError{Message: "Resource not accessible"})
	})

	_, err := client.SearchTrending(context.Background(), testSince, 10)

	var se *StatusError
	if !errors.As(err, &se) {
		t.Fatalf("error = %v (%T), want *StatusError", err, err)
	}
	if se.StatusCode != http.StatusForbidden {
		t.Errorf("StatusCode = %d, want 403", se.StatusCode)
	}
}

func TestValidationErrorIncludesDetails(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusUnprocessableEntity, models.APIError{
			Message: "Validation Failed",
			Errors:  []models.ErrorDetail{{Message: "The search query is invalid"}},
		})
	})

	_, err := client.SearchTrending(context.Background(), testSince, 10)

	var se *StatusError
	if !errors.As(err, &se) {
		t.Fatalf("error = %v (%T), want *StatusError", err, err)
	}
	if want := "Validation Failed: The search query is invalid"; se.Message != want {
		t.Errorf("Message = %q, want %q", se.Message, want)
	}
}

func TestServerErrorWithNonJSONBody(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "<html>Bad Gateway</html>", http.StatusBadGateway)
	})

	_, err := client.SearchTrending(context.Background(), testSince, 10)

	var se *StatusError
	if !errors.As(err, &se) {
		t.Fatalf("error = %v (%T), want *StatusError", err, err)
	}
	if se.StatusCode != http.StatusBadGateway || se.Message != "" {
		t.Errorf("unexpected fields: %+v", se)
	}
}

func TestMalformedJSONResponse(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("definitely not json"))
	})

	_, err := client.SearchTrending(context.Background(), testSince, 10)
	if err == nil {
		t.Fatal("expected a decoding error")
	}
	var se *StatusError
	if errors.As(err, &se) {
		t.Errorf("decoding failure should not be a StatusError: %v", err)
	}
}

func TestNetworkErrorWhenServerUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	baseURL := srv.URL
	srv.Close() // nothing is listening any more

	client := NewClient(WithBaseURL(baseURL))
	_, err := client.SearchTrending(context.Background(), testSince, 10)

	var ne *NetworkError
	if !errors.As(err, &ne) {
		t.Fatalf("error = %v (%T), want *NetworkError", err, err)
	}
	if ne.Timeout() {
		t.Error("connection refused should not be reported as a timeout")
	}
}

func TestNetworkErrorTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // hang until the client gives up
	}))
	t.Cleanup(srv.Close)

	client := NewClient(WithBaseURL(srv.URL), WithHTTPClient(&http.Client{Timeout: 50 * time.Millisecond}))
	_, err := client.SearchTrending(context.Background(), testSince, 10)

	var ne *NetworkError
	if !errors.As(err, &ne) {
		t.Fatalf("error = %v (%T), want *NetworkError", err, err)
	}
	if !ne.Timeout() {
		t.Errorf("Timeout() = false, want true (err: %v)", err)
	}
}

func TestContextCancellation(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, models.SearchResponse{})
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.SearchTrending(ctx, testSince, 10)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func checkHeader(t *testing.T, r *http.Request, key, want string) {
	t.Helper()
	if got := r.Header.Get(key); got != want {
		t.Errorf("header %s = %q, want %q", key, got, want)
	}
}

func checkParam(t *testing.T, got, name, want string) {
	t.Helper()
	if got != want {
		t.Errorf("query param %s = %q, want %q", name, got, want)
	}
}
