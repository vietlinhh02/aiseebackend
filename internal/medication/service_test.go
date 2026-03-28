package medication

import (
	"testing"
	"time"
)

func TestNextOccurrence_FutureToday(t *testing.T) {
	now := time.Now().In(hcmTZ)
	future := now.Add(2 * time.Hour)
	timeStr := future.Format("15:04")

	got, err := nextOccurrence(timeStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Day() != now.Day() || got.Month() != now.Month() {
		t.Errorf(
			"expected today (%s), got %s",
			now.Format("2006-01-02"),
			got.Format("2006-01-02"),
		)
	}
	if got.Before(now) {
		t.Errorf("expected future time, got %s (now: %s)",
			got, now,
		)
	}
}

func TestNextOccurrence_PastToday(t *testing.T) {
	now := time.Now().In(hcmTZ)
	past := now.Add(-2 * time.Hour)
	timeStr := past.Format("15:04")

	got, err := nextOccurrence(timeStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tomorrow := now.AddDate(0, 0, 1)
	if got.Day() != tomorrow.Day() ||
		got.Month() != tomorrow.Month() {
		t.Errorf(
			"expected tomorrow (%s), got %s",
			tomorrow.Format("2006-01-02"),
			got.Format("2006-01-02"),
		)
	}
}

func TestNextOccurrence_InvalidFormat(t *testing.T) {
	cases := []string{
		"invalid",
		"25:00",
		"",
		"12-30",
		"abc:def",
	}

	for _, tc := range cases {
		_, err := nextOccurrence(tc)
		if err == nil {
			t.Errorf(
				"expected error for input %q, got nil", tc,
			)
		}
	}
}

func TestNormalizeVN(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{
			input: "Paracetamol",
			want:  "paracetamol",
		},
		{
			input: "Thuốc đau đầu",
			want:  "thuoc dau dau",
		},
		{
			input: "Đặng Văn Minh",
			want:  "dang van minh",
		},
		{
			input: "VIỆT NAM",
			want:  "viet nam",
		},
		{
			input: "Phở bò",
			want:  "pho bo",
		},
	}

	for _, tc := range cases {
		got := normalizeVN(tc.input)
		if got != tc.want {
			t.Errorf(
				"normalizeVN(%q) = %q, want %q",
				tc.input, got, tc.want,
			)
		}
	}
}
