package models

import "testing"

func TestParseLanguage(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "", want: ""},
		{in: "   ", want: ""},
		{in: "go", want: "go"},
		{in: "  Go  ", want: "Go"},
		{in: "c++", want: "c++"},
		{in: "C#", want: "C#"},
		{in: "Jupyter   Notebook", want: "Jupyter Notebook"},
		{in: `go" stars:>1`, wantErr: true},
		{in: "go\x1b[2J", wantErr: true},
		{in: string(make([]rune, 51)), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseLanguage(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseLanguage(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseLanguage(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
