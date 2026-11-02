package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/kthesword/github-trending-repos/models"
)

func TestBuildTrendingQueryWithLanguage(t *testing.T) {
	tests := []struct {
		name     string
		language string
		want     string
	}{
		{"none", "", "created:>=2026-10-25T07:02:43Z is:public"},
		{"simple", "Go", `created:>=2026-10-25T07:02:43Z is:public language:"Go"`},
		{"symbols", "C++", `created:>=2026-10-25T07:02:43Z is:public language:"C++"`},
		{"spaces", "Jupyter Notebook", `created:>=2026-10-25T07:02:43Z is:public language:"Jupyter Notebook"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BuildTrendingQuery(testSince, WithLanguage(tt.language)); got != tt.want {
				t.Errorf("BuildTrendingQuery() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSearchTrendingSendsLanguageQualifier(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		want := `created:>=2026-10-25T07:02:43Z is:public language:"C++"`
		checkParam(t, r.URL.Query().Get("q"), "q", want)
		writeJSON(t, w, http.StatusOK, models.SearchResponse{})
	})

	if _, err := client.SearchTrending(context.Background(), testSince, 5, WithLanguage("C++")); err != nil {
		t.Fatalf("SearchTrending() error = %v", err)
	}
}
