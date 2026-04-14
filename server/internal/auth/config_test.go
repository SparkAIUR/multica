package auth

import "testing"

func TestParseCSVSet(t *testing.T) {
	got := ParseCSVSet(" team-a,team-b, team-a ,,team-c ")
	want := []string{"team-a", "team-b", "team-c"}
	if len(got) != len(want) {
		t.Fatalf("len(ParseCSVSet) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ParseCSVSet[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestIsAnyGroupAllowed(t *testing.T) {
	tests := []struct {
		name        string
		claimGroups []string
		allowed     []string
		want        bool
	}{
		{name: "empty allowed allows all", claimGroups: nil, allowed: nil, want: true},
		{name: "exact match", claimGroups: []string{"team-a"}, allowed: []string{"team-a", "team-b"}, want: true},
		{name: "no match", claimGroups: []string{"team-c"}, allowed: []string{"team-a", "team-b"}, want: false},
		{name: "missing claim groups with policy", claimGroups: nil, allowed: []string{"team-a"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAnyGroupAllowed(tt.claimGroups, tt.allowed)
			if got != tt.want {
				t.Fatalf("IsAnyGroupAllowed(%v, %v) = %v, want %v", tt.claimGroups, tt.allowed, got, tt.want)
			}
		})
	}
}
