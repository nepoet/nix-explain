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
