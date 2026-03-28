package emergency

import (
	"sync"
	"testing"
	"time"

	"github.com/team-dang/aiseebackend/internal/dbgen"
)

func TestNormalizeVN(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Nguyễn Văn A", "nguyen van a"},
		{"Đặng Thị B", "dang thi b"},
		{"hello", "hello"},
		{"TRẦN", "tran"},
		{"đồng", "dong"},
		{"", ""},
	}

	for _, tt := range tests {
		got := normalizeVN(tt.input)
		if got != tt.want {
			t.Errorf(
				"normalizeVN(%q) = %q, want %q",
				tt.input, got, tt.want,
			)
		}
	}
}

func TestFilterContacts_ByName(t *testing.T) {
	members := []dbgen.GetFamilyMembersOrderedRow{
		{DisplayName: "Nguyễn Văn A", Relationship: "bố"},
		{DisplayName: "Trần Thị B", Relationship: "mẹ"},
		{DisplayName: "Lê Văn C", Relationship: "anh"},
	}

	got := FilterContacts(members, "Nguyễn", nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got))
	}
	if got[0].DisplayName != "Nguyễn Văn A" {
		t.Errorf(
			"expected Nguyễn Văn A, got %s",
			got[0].DisplayName,
		)
	}

	got = FilterContacts(members, "mẹ", nil)
	if len(got) != 1 {
		t.Fatalf(
			"expected 1 match by relationship, got %d",
			len(got),
		)
	}
	if got[0].DisplayName != "Trần Thị B" {
		t.Errorf(
			"expected Trần Thị B, got %s",
			got[0].DisplayName,
		)
	}

	got = FilterContacts(members, "me", nil)
	if len(got) != 1 {
		t.Fatalf(
			"expected 1 match with normalized query, got %d",
			len(got),
		)
	}
}

func TestFilterContacts_WithSkip(t *testing.T) {
	members := []dbgen.GetFamilyMembersOrderedRow{
		{DisplayName: "Nguyễn Văn A", Relationship: "bố"},
		{DisplayName: "Trần Thị B", Relationship: "mẹ"},
		{DisplayName: "Lê Văn C", Relationship: "anh"},
	}

	got := FilterContacts(
		members, "",
		[]string{"Nguyễn Văn A", "Trần Thị B"},
	)
	if len(got) != 1 {
		t.Fatalf("expected 1 remaining, got %d", len(got))
	}
	if got[0].DisplayName != "Lê Văn C" {
		t.Errorf(
			"expected Lê Văn C, got %s",
			got[0].DisplayName,
		)
	}
}

func TestFilterContacts_Empty(t *testing.T) {
	members := []dbgen.GetFamilyMembersOrderedRow{
		{DisplayName: "Nguyễn Văn A", Relationship: "bố"},
	}

	got := FilterContacts(
		members, "", []string{"Nguyễn Văn A"},
	)
	if len(got) != 0 {
		t.Fatalf("expected 0 results, got %d", len(got))
	}

	got = FilterContacts(members, "xyz", nil)
	if len(got) != 0 {
		t.Fatalf(
			"expected 0 results for no match, got %d",
			len(got),
		)
	}
}

func TestDeduplication(t *testing.T) {
	svc := &Service{}

	userID := "550e8400-e29b-41d4-a716-446655440000"
	svc.lastAlert.Store(userID, time.Now())

	if _, loaded := svc.lastAlert.Load(userID); !loaded {
		t.Fatal("expected lastAlert to be stored")
	}

	last, _ := svc.lastAlert.Load(userID)
	elapsed := time.Since(last.(time.Time))
	if elapsed >= 60*time.Second {
		t.Fatal("expected elapsed < 60s for dedup check")
	}

	oldTime := time.Now().Add(-2 * time.Minute)
	svc.lastAlert.Store(userID, oldTime)

	last, _ = svc.lastAlert.Load(userID)
	elapsed = time.Since(last.(time.Time))
	if elapsed < 60*time.Second {
		t.Fatal("expected elapsed >= 60s after old timestamp")
	}
}

func TestDeduplication_Concurrent(t *testing.T) {
	svc := &Service{}
	userID := "550e8400-e29b-41d4-a716-446655440000"

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			svc.lastAlert.Store(userID, time.Now())
			svc.lastAlert.Load(userID)
		}(i)
	}
	wg.Wait()

	if _, loaded := svc.lastAlert.Load(userID); !loaded {
		t.Fatal("expected lastAlert to exist after concurrent access")
	}
}
