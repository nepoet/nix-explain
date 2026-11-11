package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nepoet/nix-explain/internal/kb"
)

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
	code := Run(nil, strings.NewReader(sample), &out, &errOut, testRules(), "test")
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
	code := Run([]string{"--json"}, strings.NewReader(sample), &out, &errOut, testRules(), "test")
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
	code := Run(nil, strings.NewReader("everything is fine"), &out, &errOut, testRules(), "test")
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
		strings.NewReader(""), &out, &errOut, testRules(), "test")
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
	if code := Run([]string{"list"}, strings.NewReader(""), &out, &errOut, testRules(), "test"); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "eval-infinite-recursion") {
		t.Fatalf("rule id missing:\n%s", out.String())
	}
}

func TestListFilter(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"list", "recursion"}, strings.NewReader(""), &out, &errOut, testRules(), "test"); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "eval-infinite-recursion") {
		t.Fatalf("filtered rule missing:\n%s", out.String())
	}
}

func TestDashReadsStdin(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"-"}, strings.NewReader(sample), &out, &errOut, testRules(), "test"); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Infinite recursion") {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}

func TestEmptyInputShowsUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run(nil, strings.NewReader(""), &out, &errOut, testRules(), "test"); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "Usage:") {
		t.Fatalf("usage not printed:\n%s", errOut.String())
	}
}
