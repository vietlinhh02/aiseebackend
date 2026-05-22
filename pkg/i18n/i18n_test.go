package i18n

import "testing"

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"":         LangVI,
		"vi":       LangVI,
		"vi-VN":    LangVI,
		"en":       LangEN,
		"en-US":    LangEN,
		"  EN-GB ": LangEN,
		"fr":       LangVI,
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseAcceptLanguage(t *testing.T) {
	cases := map[string]string{
		"":                           LangVI,
		"vi":                         LangVI,
		"en-US,en;q=0.9":             LangEN,
		"fr;q=0.8,en;q=0.7":          LangEN,
		"vi-VN,vi;q=0.9,en-US;q=0.8": LangVI,
		"de,fr;q=0.5":                LangVI,
	}
	for in, want := range cases {
		if got := ParseAcceptLanguage(in); got != want {
			t.Errorf("ParseAcceptLanguage(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestT_FallbacksToDefault(t *testing.T) {
	if got := T(LangEN, "notif.emergency.body"); got != "Urgent help needed!" {
		t.Errorf("T(en, notif.emergency.body) = %q", got)
	}
	if got := T(LangVI, "notif.emergency.body"); got != "Cần giúp đỡ khẩn cấp!" {
		t.Errorf("T(vi, notif.emergency.body) = %q", got)
	}
	if got := T("zh", "notif.emergency.body"); got != "Cần giúp đỡ khẩn cấp!" {
		t.Errorf("T(zh -> default vi) = %q", got)
	}
	if got := T(LangEN, "missing.key"); got != "missing.key" {
		t.Errorf("missing key should return key, got %q", got)
	}
}

func TestT_FormatsArgs(t *testing.T) {
	got := T(LangEN, "notif.medication.body", "Aspirin", "1 tablet")
	want := "Time to take Aspirin (1 tablet)"
	if got != want {
		t.Errorf("format args: got %q, want %q", got, want)
	}
}

func TestFromUserSettings(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{``, LangVI},
		{`{}`, LangVI},
		{`{"preferred_language":"en"}`, LangEN},
		{`{"preferred_language":"vi-VN"}`, LangVI},
		{`{"preferred_language":""}`, LangVI},
		{`not-json`, LangVI},
	}
	for _, c := range cases {
		got := FromUserSettings([]byte(c.in))
		if got != c.want {
			t.Errorf("FromUserSettings(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
