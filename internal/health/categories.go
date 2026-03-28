package health

import (
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var SymptomCategories = map[string]string{
	"dau dau":   "dau_dau",
	"chong mat": "chong_mat",
	"met moi":   "met_moi",
	"kho tho":   "kho_tho",
	"buon non":  "buon_non",
	"dau nguc":  "dau_nguc",
	"sot":       "sot",
	"ho":        "ho",
	"dau bung":  "dau_bung",
	"mo mat":    "mo_mat",
}

var symptomPatternsByLength = sortedKeys(SymptomCategories)

var MetricTypes = map[string]string{
	"huyet ap":    "huyet_ap",
	"duong huyet": "duong_huyet",
	"nhiet do":    "nhiet_do",
	"can nang":    "can_nang",
	"nhip tim":    "nhip_tim",
	"spo2":        "spo2",
}

var metricPatternsByLength = sortedKeys(MetricTypes)

// sortedKeys returns map keys sorted longest-first so longer
// patterns match before short substrings (e.g. "chong mat"
// before "ho").
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})
	return keys
}

var MetricUnits = map[string]string{
	"huyet_ap":    "mmHg",
	"duong_huyet": "mg/dL",
	"nhiet_do":    "°C",
	"can_nang":    "kg",
	"nhip_tim":    "bpm",
	"spo2":        "%",
}

func normalizeVietnamese(s string) string {
	s = strings.ToLower(s)
	t := norm.NFD.String(s)
	var b strings.Builder
	for _, r := range t {
		if !unicode.Is(unicode.Mn, r) {
			if r == 'đ' {
				b.WriteRune('d')
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func MatchSymptomCategory(input string) (string, bool) {
	normalized := normalizeVietnamese(input)
	for _, pattern := range symptomPatternsByLength {
		if strings.Contains(normalized, pattern) {
			return SymptomCategories[pattern], true
		}
	}
	return "", false
}

func MatchMetricType(input string) (string, string, bool) {
	normalized := normalizeVietnamese(input)
	for _, pattern := range metricPatternsByLength {
		if strings.Contains(normalized, pattern) {
			metricType := MetricTypes[pattern]
			unit := MetricUnits[metricType]
			return metricType, unit, true
		}
	}
	return "", "", false
}
