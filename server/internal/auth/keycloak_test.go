package auth

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestParseGroupsClaim(t *testing.T) {
	tests := []struct {
		name string
		raw  any
		want []string
	}{
		{name: "array any", raw: []any{"team-a", "team-b"}, want: []string{"team-a", "team-b"}},
		{name: "array strings trims", raw: []string{" team-a ", "team-b"}, want: []string{"team-a", "team-b"}},
		{name: "single string", raw: "team-a", want: []string{"team-a"}},
		{name: "empty values ignored", raw: []any{"", "  ", "team-a"}, want: []string{"team-a"}},
		{name: "unsupported type", raw: 42, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGroupsClaim(tt.raw)
			if len(got) != len(tt.want) {
				t.Fatalf("len(parseGroupsClaim(%v)) = %d, want %d", tt.raw, len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("parseGroupsClaim[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestAudContains(t *testing.T) {
	if !audContains(jwt.ClaimStrings{"a", "b"}, "b") {
		t.Fatal("audContains should return true when audience is present")
	}
	if audContains(jwt.ClaimStrings{"a", "b"}, "c") {
		t.Fatal("audContains should return false when audience is absent")
	}
}
