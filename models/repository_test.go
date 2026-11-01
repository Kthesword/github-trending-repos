package models

import "testing"

func TestSortByStars(t *testing.T) {
	repos := []Repository{
		{FullName: "z/low", StargazersCount: 10},
		{FullName: "b/tie", StargazersCount: 500},
		{FullName: "a/high", StargazersCount: 9000},
		{FullName: "A/tie", StargazersCount: 500},
		{FullName: "m/mid", StargazersCount: 120},
	}

	SortByStars(repos)

	want := []string{"a/high", "A/tie", "b/tie", "m/mid", "z/low"}
	for i, name := range want {
		if repos[i].FullName != name {
			t.Fatalf("position %d = %q, want %q (full order: %v)", i, repos[i].FullName, name, names(repos))
		}
	}
}

func TestSortByStarsEmpty(t *testing.T) {
	SortByStars(nil) // must not panic
}

func TestAPIErrorSummary(t *testing.T) {
	tests := []struct {
		name string
		in   APIError
		want string
	}{
		{"empty", APIError{}, ""},
		{"message only", APIError{Message: "Not Found"}, "Not Found"},
		{"details only", APIError{Errors: []ErrorDetail{{Message: "bad q"}}}, "bad q"},
		{
			"message and details",
			APIError{Message: "Validation Failed", Errors: []ErrorDetail{{Message: "a"}, {Message: " "}, {Message: "b"}}},
			"Validation Failed: a; b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.Summary(); got != tt.want {
				t.Errorf("Summary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func names(repos []Repository) []string {
	out := make([]string, len(repos))
	for i, r := range repos {
		out[i] = r.FullName
	}
	return out
}
