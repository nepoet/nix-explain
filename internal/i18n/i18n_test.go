package i18n

import "testing"

func TestResolve(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"en", "en"},
		{"ru", "ru"},
		{"RU", "ru"},
		{"auto", ""},
		{"", ""},
		{"de", "en"},
	} {
		if tc.in == "auto" || tc.in == "" {
			t.Setenv("LANG", "")
			t.Setenv("LC_ALL", "")
			t.Setenv("LC_MESSAGES", "")
			t.Setenv("NIX_EXPLAIN_LANG", "")
			if got := Resolve(tc.in); got != "en" {
				t.Errorf("Resolve(%q) = %q, want en", tc.in, got)
			}
			continue
		}
		if got := Resolve(tc.in); got != tc.want {
			t.Errorf("Resolve(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolveFromEnvironment(t *testing.T) {
	t.Setenv("LANG", "ru_RU.UTF-8")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("NIX_EXPLAIN_LANG", "")
	if got := Resolve("auto"); got != "ru" {
		t.Fatalf("Resolve(auto) = %q, want ru", got)
	}

	t.Setenv("NIX_EXPLAIN_LANG", "en")
	if got := Resolve("auto"); got != "en" {
		t.Fatalf("NIX_EXPLAIN_LANG was ignored: got %q", got)
	}
}

func TestGetFallsBackToEnglish(t *testing.T) {
	for _, lang := range []string{"en", "de", ""} {
		if got := Get(lang); got.Lang != "en" {
			t.Errorf("Get(%q).Lang = %q, want en", lang, got.Lang)
		}
	}
	if Get("ru").Cause != "ПРИЧИНА" {
		t.Errorf("Russian strings are not wired up")
	}
}
