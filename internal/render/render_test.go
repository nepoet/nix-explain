package render

import (
	"strings"
	"testing"
	"unicode/utf8"
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
