package formatter

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/kthesword/github-trending-repos/models"
)

var testMeta = Meta{
	Period:     "past 7 days",
	Since:      time.Date(2026, 10, 25, 7, 0, 0, 0, time.UTC),
	TotalCount: 48213,
}

func render(t *testing.T, meta Meta, repos []models.Repository, opts ...Option) string {
	t.Helper()
	var buf bytes.Buffer
	if err := NewTable(&buf, append([]Option{WithColor(false)}, opts...)...).Render(meta, repos); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	return buf.String()
}

func TestRenderContainsAllFields(t *testing.T) {
	out := render(t, testMeta, []models.Repository{
		{FullName: "acme/rocket", Description: "Blazing fast things", StargazersCount: 12345, Language: "Go"},
		{FullName: "acme/no-lang", StargazersCount: 7},
	})

	for _, want := range []string{
		"REPOSITORY", "STARS", "LANGUAGE", "DESCRIPTION",
		"acme/rocket", "12,345", "Go", "Blazing fast things",
		"acme/no-lang", "past 7 days", "48,213",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("output contains ANSI escapes although color is disabled")
	}
}

func TestRenderAlignsColumns(t *testing.T) {
	out := render(t, testMeta, []models.Repository{
		{FullName: "a/short", StargazersCount: 5, Language: "Rust", Description: "x"},
		{FullName: "someone/a-much-longer-name", StargazersCount: 123456, Language: "TypeScript", Description: "y"},
	})

	lines := strings.Split(out, "\n")
	headerCol := -1
	var dataCols []int
	for _, line := range lines {
		switch {
		case strings.Contains(line, "LANGUAGE"):
			headerCol = strings.Index(line, "LANGUAGE")
		case strings.Contains(line, "a/short"):
			dataCols = append(dataCols, strings.Index(line, "Rust"))
		case strings.Contains(line, "a-much-longer-name"):
			dataCols = append(dataCols, strings.Index(line, "TypeScript"))
		}
	}
	if headerCol < 0 || len(dataCols) != 2 {
		t.Fatalf("could not locate columns in output:\n%s", out)
	}
	for _, c := range dataCols {
		if c != headerCol {
			t.Errorf("language column misaligned: header at %d, data at %d\n%s", headerCol, c, out)
		}
	}
}

func TestRenderWithColorKeepsAlignment(t *testing.T) {
	var buf bytes.Buffer
	repos := []models.Repository{
		{FullName: "a/b", StargazersCount: 1, Language: "Go"},
		{FullName: "longer/name", StargazersCount: 99999, Language: "C"},
	}
	if err := NewTable(&buf, WithColor(true)).Render(testMeta, repos); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "\x1b[") {
		t.Error("expected ANSI escapes when color is enabled")
	}
}

func TestRenderEmpty(t *testing.T) {
	out := render(t, testMeta, nil)
	if !strings.Contains(out, "No repositories found") {
		t.Errorf("unexpected output for empty result: %q", out)
	}
}

func TestRenderIncompleteNotice(t *testing.T) {
	meta := testMeta
	meta.Incomplete = true
	out := render(t, meta, []models.Repository{{FullName: "a/b"}})
	if !strings.Contains(out, "incomplete") {
		t.Errorf("expected incomplete-results notice:\n%s", out)
	}
}

func TestRenderSanitizesUntrustedText(t *testing.T) {
	out := render(t, testMeta, []models.Repository{{
		FullName:    "evil/repo",
		Description: "line one\nline two\tTAB \x1b[2Jclear",
	}})
	if strings.Contains(out, "\x1b") {
		t.Error("escape sequence from description leaked into output")
	}
	if strings.Contains(out, "\t") {
		t.Error("tab from description leaked into output")
	}
	if !strings.Contains(out, "line one line two TAB [2Jclear") {
		t.Errorf("description not collapsed onto one line:\n%s", out)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"héllo wörld again", 8, "héllo w…"},
		{"trailing space here", 10, "trailing…"},
		{"abc", 1, "…"},
		{"unlimited", 0, "unlimited"},
	}
	for _, tt := range tests {
		if got := truncate(tt.in, tt.max); got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
		}
	}
}

func TestFormatCount(t *testing.T) {
	tests := map[int]string{
		0:         "0",
		7:         "7",
		999:       "999",
		1000:      "1,000",
		12345:     "12,345",
		123456:    "123,456",
		1234567:   "1,234,567",
		-1234:     "-1,234",
		-999:      "-999",
		100000000: "100,000,000",
	}
	for in, want := range tests {
		if got := formatCount(in); got != want {
			t.Errorf("formatCount(%d) = %q, want %q", in, got, want)
		}
	}
}
