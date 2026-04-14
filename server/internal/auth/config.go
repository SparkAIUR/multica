package auth

import (
	"os"
	"strings"
)

// ParseCSVSet splits a comma-separated env value into normalized unique values.
func ParseCSVSet(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func AllowedKeycloakGroupsFromEnv() []string {
	return ParseCSVSet(os.Getenv("KEYCLOAK_ALLOWED_GROUPS"))
}

// IsAnyGroupAllowed returns true when group filtering is disabled or at least one
// claim group matches an allowed group exactly.
func IsAnyGroupAllowed(claimGroups, allowedGroups []string) bool {
	if len(allowedGroups) == 0 {
		return true
	}
	if len(claimGroups) == 0 {
		return false
	}
	allowed := make(map[string]struct{}, len(allowedGroups))
	for _, group := range allowedGroups {
		allowed[group] = struct{}{}
	}
	for _, group := range claimGroups {
		if _, ok := allowed[group]; ok {
			return true
		}
	}
	return false
}
