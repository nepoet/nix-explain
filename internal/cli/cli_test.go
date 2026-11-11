package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/nepoet/nix-explain/internal/kb"
)

func TestMain(m *testing.M) {
	// Keep the default-language tests deterministic on a localised system.
	os.Setenv("LANG", "en_US.UTF-8")
	os.Unsetenv("LC_ALL")
	os.Unsetenv("LC_MESSAGES")
	os.Unsetenv("NIX_EXPLAIN_LANG")
	os.Exit(m.Run())
}

func testRules() []kb.Rule {
	return []kb.Rule{{
		ID:       "eval-infinite-recursion",
		Title:    "Infinite recursion while evaluating",
		Category: "eval",
		Match:    []string{"infinite recursion encountered"},
		Cause:    "Nix tried to compute a value in terms of itself.",
		Fix:      "Look at the innermost frame and break the cycle.",
	}}
}

const sample = `error: infinite recursion encountered
       at /home/user/lab/recursion.nix:2:7:
            1| let
            2|   a = b;
             |       ^
`

func TestExplainMatchesAndReportsLocation(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run(nil, strings.NewReader(sample), &out, &errOut, testRules(), nil, "test")
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, errOut.String())
	}
	for _, want := range []string{"Infinite recursion", "recursion.nix:2:7", "CAUSE", "FIX"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output does not contain %q:\n%s", want, out.String())
		}
	}
}

func TestExplainJSON(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"--json"}, strings.NewReader(sample), &out, &errOut, testRules(), nil, "test")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	var got struct {
		Matched  bool     `json:"matched"`
		Rule     *kb.Rule `json:"rule"`
		Location *struct {
			File string `json:"file"`
			Line int    `json:"line"`
		} `json:"location"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if !got.Matched || got.Rule == nil || got.Rule.ID != "eval-infinite-recursion" {
		t.Fatalf("unexpected result: %+v", got)
	}
	if got.Location == nil || got.Location.File != "/home/user/lab/recursion.nix" || got.Location.Line != 2 {
		t.Fatalf("unexpected location: %+v", got.Location)
	}
}

func TestExplainNoMatch(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run(nil, strings.NewReader("everything is fine"), &out, &errOut, testRules(), nil, "test")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(out.String(), "No known explanation") {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}

func TestRunPreservesExitCodeAndExplains(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"run", "--color", "never", "--", "sh", "-c",
		`echo "error: infinite recursion encountered"; echo "at /home/user/a.nix:1:2:" >&2; exit 3`},
		strings.NewReader(""), &out, &errOut, testRules(), nil, "test")
	if code != 3 {
		t.Fatalf("exit = %d, want 3", code)
	}
	if !strings.Contains(out.String(), "infinite recursion encountered") {
		t.Errorf("child stdout was not passed through:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "Infinite recursion") {
		t.Errorf("explanation not on stderr:\n%s", errOut.String())
	}
}

func TestList(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"list"}, strings.NewReader(""), &out, &errOut, testRules(), nil, "test"); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "eval-infinite-recursion") {
		t.Fatalf("rule id missing:\n%s", out.String())
	}
}

func TestListFilter(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"list", "recursion"}, strings.NewReader(""), &out, &errOut, testRules(), nil, "test"); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "eval-infinite-recursion") {
		t.Fatalf("filtered rule missing:\n%s", out.String())
	}
}

func TestDashReadsStdin(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"-"}, strings.NewReader(sample), &out, &errOut, testRules(), nil, "test"); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Infinite recursion") {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}

func TestEmptyInputShowsUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run(nil, strings.NewReader(""), &out, &errOut, testRules(), nil, "test"); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "Usage:") {
		t.Fatalf("usage not printed:\n%s", errOut.String())
	}
}

func ruTranslations() map[string]kb.Translation {
	return map[string]kb.Translation{
		"eval-infinite-recursion": {
			ID:    "eval-infinite-recursion",
			Title: "Бесконечная рекурсия при вычислении",
			Cause: "Nix вычисляет значение через само себя.",
			Fix:   "Разорвите цикл в определении.",
		},
	}
}

func TestRussianOutput(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"--lang", "ru"}, strings.NewReader(sample), &out, &errOut, testRules(), ruTranslations(), "test")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{"Бесконечная рекурсия", "ПРИЧИНА", "ИСПРАВЛЕНИЕ", "ТРАССИРОВКА"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output does not contain %q:\n%s", want, out.String())
		}
	}
}

func TestLanguageFromEnvironment(t *testing.T) {
	t.Setenv("NIX_EXPLAIN_LANG", "ru")
	var out, errOut bytes.Buffer
	if code := Run(nil, strings.NewReader(sample), &out, &errOut, testRules(), ruTranslations(), "test"); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "ПРИЧИНА") {
		t.Fatalf("environment language was not used:\n%s", out.String())
	}
}

func TestJSONReportsLanguage(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"--json", "--lang", "ru"}, strings.NewReader(sample), &out, &errOut, testRules(), ruTranslations(), "test"); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	var got struct {
		Lang string   `json:"lang"`
		Rule *kb.Rule `json:"rule"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Lang != "ru" || got.Rule == nil || got.Rule.Title != "Бесконечная рекурсия при вычислении" {
		t.Fatalf("unexpected JSON: %s", out.String())
	}
}

func TestRussianList(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"list", "--lang", "ru"}, strings.NewReader(""), &out, &errOut, testRules(), ruTranslations(), "test"); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "ВЫЧИСЛЕНИЕ") {
		t.Fatalf("category was not translated:\n%s", out.String())
	}
}
