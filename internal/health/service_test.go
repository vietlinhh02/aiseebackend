package health

import (
	"testing"
)

func TestMatchSymptomCategory(t *testing.T) {
	tests := []struct {
		input  string
		want   string
		wantOK bool
	}{
		{"đau đầu", "dau_dau", true},
		{"Đau Đầu", "dau_dau", true},
		{"dau dau", "dau_dau", true},
		{"chóng mặt quá", "chong_mat", true},
		{"unknown symptom", "", false},
		{"mờ mắt", "mo_mat", true},
		{"ho", "ho", true},
	}

	for _, tt := range tests {
		got, ok := MatchSymptomCategory(tt.input)
		if ok != tt.wantOK || got != tt.want {
			t.Errorf(
				"MatchSymptomCategory(%q) = (%q, %v), want (%q, %v)",
				tt.input, got, ok, tt.want, tt.wantOK,
			)
		}
	}
}

func TestMatchMetricType(t *testing.T) {
	tests := []struct {
		input    string
		wantType string
		wantUnit string
		wantOK   bool
	}{
		{"huyết áp", "huyet_ap", "mmHg", true},
		{"huyet ap", "huyet_ap", "mmHg", true},
		{"đường huyết", "duong_huyet", "mg/dL", true},
		{"nhiệt độ", "nhiet_do", "°C", true},
		{"cân nặng", "can_nang", "kg", true},
		{"nhịp tim", "nhip_tim", "bpm", true},
		{"SpO2", "spo2", "%", true},
		{"spo2", "spo2", "%", true},
		{"unknown", "", "", false},
	}

	for _, tt := range tests {
		gotType, gotUnit, ok := MatchMetricType(tt.input)
		if ok != tt.wantOK || gotType != tt.wantType || gotUnit != tt.wantUnit {
			t.Errorf(
				"MatchMetricType(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.input, gotType, gotUnit, ok,
				tt.wantType, tt.wantUnit, tt.wantOK,
			)
		}
	}
}

func TestNormalizeVietnamese(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Đau Đầu", "dau dau"},
		{"chóng mặt", "chong mat"},
		{"mệt mỏi", "met moi"},
		{"SpO2", "spo2"},
		{"already ascii", "already ascii"},
	}

	for _, tt := range tests {
		got := normalizeVietnamese(tt.input)
		if got != tt.want {
			t.Errorf(
				"normalizeVietnamese(%q) = %q, want %q",
				tt.input, got, tt.want,
			)
		}
	}
}
