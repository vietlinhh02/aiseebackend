package user

import (
	"os"
	"strings"
	"testing"
)

func TestNormalizeVietnamese(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Con Gái", "con gai"},
		{"Nguyễn Văn A", "nguyen van a"},
		{"mẹ", "me"},
		{"Chồng", "chong"},
		{"Lan", "lan"},
	}

	for _, tt := range tests {
		got := normalizeVietnamese(tt.input)
		if got != tt.want {
			t.Errorf("normalizeVietnamese(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMatchContact(t *testing.T) {
	tests := []struct {
		query        string
		displayName  string
		relationship string
		want         bool
	}{
		{"con gái", "Lan", "con gái", true},
		{"Lan", "Lan", "con gái", true},
		{"lan", "Lan", "con gái", true},
		{"con trai", "Lan", "con gái", false},
		{"mẹ", "Nguyễn Thị B", "mẹ", true},
		{"Nguyen", "Nguyễn Thị B", "mẹ", true},
	}

	for _, tt := range tests {
		got := matchContact(tt.query, tt.displayName, tt.relationship)
		if got != tt.want {
			t.Errorf("matchContact(%q, %q, %q) = %v, want %v",
				tt.query, tt.displayName, tt.relationship, got, tt.want)
		}
	}
}

func TestNormalizeInviteCode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{" abCD12 ", "abcd12"},
		{"\nFF00aa\t", "ff00aa"},
		{"", ""},
	}

	for _, tt := range tests {
		got := normalizeInviteCode(tt.input)
		if got != tt.want {
			t.Errorf("normalizeInviteCode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNextFamilyInviteRole(t *testing.T) {
	isPrimary, role := nextFamilyInviteRole(false)
	if !isPrimary || role != "billing_admin" {
		t.Fatalf("first family invite role = (%v, %q), want (true, billing_admin)", isPrimary, role)
	}

	isPrimary, role = nextFamilyInviteRole(true)
	if isPrimary || role != "family" {
		t.Fatalf("next family invite role = (%v, %q), want (false, family)", isPrimary, role)
	}
}

func TestJoinFamilyLocksCareGroupBeforeCapacityCheck(t *testing.T) {
	src, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read user service: %v", err)
	}
	body := string(src)
	if !strings.Contains(body, "lockCareGroupForCapacityTx") {
		t.Fatal("JoinFamily should lock the care group before checking member capacity")
	}
	if !strings.Contains(body, "FOR UPDATE") {
		t.Fatal("care group capacity lock should use SELECT ... FOR UPDATE")
	}
}
