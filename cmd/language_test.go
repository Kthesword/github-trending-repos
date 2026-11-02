package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kthesword/github-trending-repos/models"
)

func TestInvalidLanguageIsUsageError(t *testing.T) {
	t.Setenv(envAPIURL, "http://127.0.0.1:0") // must not be contacted

	_, err := execute(t, "--language", `go" stars:>1`)

	var ue *usageError
	if !errors.As(err, &ue) {
		t.Fatalf("error = %v (%T), want *usageError", err, err)
	}
}

func TestLanguageFlagFiltersAndIsShown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if q := r.URL.Query().Get("q"); !strings.Contains(q, `language:"Rust"`) {
			t.Errorf("q = %q, want it to contain language:\"Rust\"", q)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(models.SearchResponse{
			TotalCount: 1,
			Items:      []models.Repository{{FullName: "acme/oxide", Language: "Rust", StargazersCount: 5}},
		})
	}))
	t.Cleanup(srv.Close)

	t.Setenv(envAPIURL, srv.URL)
	t.Setenv(envToken, "")

	out, err := execute(t, "-L", "Rust", "--no-color")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if !strings.Contains(out, "Rust · past 7 days") {
		t.Errorf("title does not show the language filter:\n%s", out)
	}
}
