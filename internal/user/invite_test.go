package user

import (
	"regexp"
	"testing"
)

func TestGenerateInviteCode_Format(t *testing.T) {
	code := GenerateInviteCode()

	if len(code) != 8 {
		t.Errorf("expected 8-character code, got %d characters: %q", len(code), code)
	}

	hexPattern := regexp.MustCompile(`^[0-9a-f]{8}$`)
	if !hexPattern.MatchString(code) {
		t.Errorf("expected lowercase hex string, got %q", code)
	}
}

func TestGenerateInviteCode_Uniqueness(t *testing.T) {
	seen := make(map[string]struct{})
	for i := range 100 {
		code := GenerateInviteCode()
		if _, exists := seen[code]; exists {
			t.Fatalf("duplicate code %q on iteration %d", code, i)
		}
		seen[code] = struct{}{}
	}
}
