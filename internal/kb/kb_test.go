package kb_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nepoet/nix-explain/internal/kb"
)

// loadRules loads the real knowledge base from the repository root. Tests run
// with the package directory as the working directory.
func loadRules(t *testing.T) []kb.Rule {
	t.Helper()
	rules, err := kb.Load(os.DirFS("../.."), "rules")
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("no rules loaded")
	}
	return rules
}

func TestRulesAreValid(t *testing.T) {
	rules := loadRules(t) // Load validates every rule and duplicate ids.
	if len(rules) < 30 {
		t.Fatalf("only %d rules, expected at least 30", len(rules))
	}
}

func TestGoldenCases(t *testing.T) {
	rules := loadRules(t)
	dir := "../../testdata/cases"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read cases: %v", err)
	}

	checked := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".txt")
		wantFile := filepath.Join(dir, name+".want")
		wantBytes, err := os.ReadFile(wantFile)
		if err != nil {
			t.Fatalf("%s: missing .want file: %v", name, err)
		}
		want := strings.TrimSpace(string(wantBytes))
		text, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		hits := kb.Find(rules, string(text))
		if len(hits) == 0 {
			t.Errorf("%s: no rule matched (want %s)", name, want)
			continue
		}
		if got := hits[0].Rule.ID; got != want {
			t.Errorf("%s: best match = %s, want %s", name, got, want)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no golden cases were checked")
	}
	t.Logf("checked %d golden cases", checked)
}

func TestNoMatchOnHealthyOutput(t *testing.T) {
	rules := loadRules(t)
	for _, text := range []string{
		"Здравствуй, мир!",
		"Hello, world!",
		"these 28 paths will be fetched (10.3 MiB download, 42.6 MiB unpacked)",
		"copying path '/nix/store/abc-hello-2.12.3' from 'https://cache.nixos.org'",
	} {
		if hits := kb.Find(rules, text); len(hits) != 0 {
			t.Errorf("unexpected match for %q: %s", text, hits[0].Rule.ID)
		}
	}
}

func TestFindIsCaseAndWhitespaceInsensitive(t *testing.T) {
	rules := loadRules(t)
	hits := kb.Find(rules, "ERROR:   Infinite\n  RECURSION   encountered")
	if len(hits) == 0 || hits[0].Rule.ID != "eval-infinite-recursion" {
		t.Fatalf("got %+v, want eval-infinite-recursion", hits)
	}
}

func TestRussianTranslationsComplete(t *testing.T) {
	rules := loadRules(t)
	translations, err := kb.LoadTranslations(os.DirFS("../.."), "i18n", "ru")
	if err != nil {
		t.Fatalf("load translations: %v", err)
	}
	if len(translations) == 0 {
		t.Fatal("no Russian translations found")
	}

	known := make(map[string]bool, len(rules))
	for _, r := range rules {
		known[r.ID] = true
		tr, ok := translations[r.ID]
		if !ok {
			t.Errorf("%s: missing Russian translation", r.ID)
			continue
		}
		if tr.Title == "" || tr.Cause == "" || tr.Fix == "" {
			t.Errorf("%s: incomplete Russian translation", r.ID)
		}
	}
	for id := range translations {
		if !known[id] {
			t.Errorf("%s: translation for unknown rule", id)
		}
	}
}

func TestTranslateKeepsEnglishForMissingRules(t *testing.T) {
	rules := loadRules(t)
	out := kb.Translate(rules, map[string]kb.Translation{
		rules[0].ID: {ID: rules[0].ID, Title: "Заголовок", Cause: "причина", Fix: "исправление"},
	})
	if out[0].Title != "Заголовок" {
		t.Fatalf("translation not applied: %q", out[0].Title)
	}
	if out[1].Title != rules[1].Title {
		t.Fatalf("untranslated rule changed: %q", out[1].Title)
	}
	if rules[0].Title == "Заголовок" {
		t.Fatal("Translate modified the input slice")
	}
}
