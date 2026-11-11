package normalize

import "testing"

func TestTextStripsANSI(t *testing.T) {
	in := "\x1b[31merror:\x1b[0m bad \x1b[1;33mthing\x1b[0m"
	if got, want := Text(in), "error: bad thing"; got != want {
		t.Fatalf("Text() = %q, want %q", got, want)
	}
}

func TestTextNormalisesLineEndings(t *testing.T) {
	if got, want := Text("a\r\nb\rc"), "a\nb\nc"; got != want {
		t.Fatalf("Text() = %q, want %q", got, want)
	}
}

func TestForMatchCollapsesWhitespace(t *testing.T) {
	in := "ERROR:\n\n   Infinite\t recursion   encountered"
	if got, want := ForMatch(in), "error: infinite recursion encountered"; got != want {
		t.Fatalf("ForMatch() = %q, want %q", got, want)
	}
}
