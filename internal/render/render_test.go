package render

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/nepoet/nix-explain/internal/kb"
	"github.com/nepoet/nix-explain/internal/trace"
)

func TestWrapCountsRunesNotBytes(t *testing.T) {
	body := strings.TrimSpace(strings.Repeat("привет мир ", 20))
	for _, line := range wrap(body, 40) {
		if n := utf8.RuneCountInString(line); n > 40 {
			t.Errorf("line has %d runes, want <= 40: %q", n, line)
		}
	}
}

func TestWrapJoinsSoftWrappedLines(t *testing.T) {
	lines := wrap("first line that\ncontinues here.\nnext sentence.", 80)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), lines)
	}
	if lines[0] != "first line that continues here." {
		t.Fatalf("unexpected first line: %q", lines[0])
	}
}

func TestWrapKeepsListItemsSeparate(t *testing.T) {
	lines := wrap("intro:\n  - one\n  - two", 80)
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3: %q", len(lines), lines)
	}
}

func TestExplainRespectsWidth(t *testing.T) {
	var buf bytes.Buffer
	rule := kb.Rule{
		ID:       "test-width",
		Title:    "A title",
		Category: "eval",
		Cause:    strings.TrimSpace(strings.Repeat("слово слово ", 20)),
		Fix:      "fix",
	}
	Explain(&buf, kb.Hit{Rule: rule}, trace.Trace{}, Options{Width: 40})
	for _, line := range strings.Split(buf.String(), "\n") {
		if n := utf8.RuneCountInString(line); n > 42 {
			t.Errorf("line has %d runes, want <= 42: %q", n, line)
		}
	}
}

func TestListNarrowLayoutFitsWidth(t *testing.T) {
	var buf bytes.Buffer
	List(&buf, []kb.Rule{
		{ID: "eval-infinite-recursion", Title: "Infinite recursion while evaluating", Category: "eval"},
	}, Options{Width: 40})
	for _, line := range strings.Split(buf.String(), "\n") {
		if n := utf8.RuneCountInString(line); n > 40 {
			t.Errorf("line has %d runes, want <= 40: %q", n, line)
		}
	}
}
