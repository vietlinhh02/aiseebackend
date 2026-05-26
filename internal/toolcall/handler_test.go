package toolcall

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	"github.com/team-dang/aiseebackend/pkg/pgutil"
)

func TestToolCallRequest_UnmarshalJSON(t *testing.T) {
	// Basic sanity test that the types compile
	req := ToolCallRequest{
		Tool:   "make_voice_call",
		UserID: "test-user",
		Params: map[string]any{"contact_name": "Lan"},
	}
	if req.Tool != "make_voice_call" {
		t.Error("unexpected tool name")
	}
}

func TestBuildFamilyContactsResponse(t *testing.T) {
	t.Parallel()

	rows := []dbgen.GetFamilyMembersOrderedRow{
		{
			ID:               pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
			Email:            "family@example.com",
			DisplayName:      "Linh Viet",
			Role:             "family",
			Relationship:     "Gia dinh",
			IsPrimaryContact: true,
		},
	}

	got := buildFamilyContactsResponse(rows)

	if got["status"] != "ok" {
		t.Fatalf("expected status ok, got %#v", got["status"])
	}
	if got["has_family_link"] != true {
		t.Fatalf("expected has_family_link true, got %#v", got["has_family_link"])
	}
	if got["count"] != 1 {
		t.Fatalf("expected count 1, got %#v", got["count"])
	}

	contacts, ok := got["contacts"].([]map[string]any)
	if !ok || len(contacts) != 1 {
		t.Fatalf("expected one contact, got %#v", got["contacts"])
	}
	if contacts[0]["display_name"] != "Linh Viet" {
		t.Fatalf("expected display name Linh Viet, got %#v", contacts[0]["display_name"])
	}
	if contacts[0]["relationship"] != "Gia dinh" {
		t.Fatalf("expected relationship Gia dinh, got %#v", contacts[0]["relationship"])
	}
}

func TestIsGenericSupportRequest(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"":                 true,
		"nguoi ho tro":     true,
		"người hỗ trợ":     true,
		"người giúp":       true,
		"tình nguyện viên": true,
		"tinh nguyen vien": true,
		"support":          true,
		"Me":               false,
	}

	for input, want := range cases {
		if got := isGenericSupportRequest(input); got != want {
			t.Fatalf("isGenericSupportRequest(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestFamilyResolveError(t *testing.T) {
	t.Parallel()

	multiple := familyResolveError("me", errors.New("multiple contacts match 'me', please be more specific"))
	if multiple["status"] != "multiple_contacts" {
		t.Fatalf("expected multiple_contacts, got %#v", multiple["status"])
	}

	notFound := familyResolveError("me", errors.New("contact not found: me"))
	if notFound["status"] != "contact_not_found" {
		t.Fatalf("expected contact_not_found, got %#v", notFound["status"])
	}
}

func TestBuildFamilyFallbackContactsExcludesCurrentAndLimitsThree(t *testing.T) {
	t.Parallel()

	rows := []dbgen.GetFamilyMembersOrderedRow{
		{
			ID:               pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
			Email:            "one@example.com",
			DisplayName:      "One",
			Role:             "family",
			Relationship:     "Me",
			IsPrimaryContact: true,
		},
		{
			ID:               pgtype.UUID{Bytes: [16]byte{2}, Valid: true},
			Email:            "two@example.com",
			DisplayName:      "Two",
			Role:             "family",
			Relationship:     "Chi",
			IsPrimaryContact: false,
		},
		{
			ID:               pgtype.UUID{Bytes: [16]byte{3}, Valid: true},
			Email:            "three@example.com",
			DisplayName:      "Three",
			Role:             "family",
			Relationship:     "Anh",
			IsPrimaryContact: false,
		},
		{
			ID:               pgtype.UUID{Bytes: [16]byte{4}, Valid: true},
			Email:            "four@example.com",
			DisplayName:      "Four",
			Role:             "family",
			Relationship:     "Em",
			IsPrimaryContact: false,
		},
	}

	got := buildFamilyFallbackContacts(rows, pgutil.PgUUIDToString(rows[0].ID))
	if len(got) != 3 {
		t.Fatalf("expected 3 fallback contacts, got %d", len(got))
	}
	if got[0]["display_name"] != "Two" {
		t.Fatalf("expected first fallback to be Two, got %#v", got[0]["display_name"])
	}
}
