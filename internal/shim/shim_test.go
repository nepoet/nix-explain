package shim

import (
	"strings"
	"testing"
)

func TestIsShim(t *testing.T) {
	for _, name := range []string{"nix", "nixos-rebuild", "home-manager", "/usr/bin/nix-shell"} {
		if !IsShim(name) {
			t.Errorf("IsShim(%q) = false", name)
		}
	}
	for _, name := range []string{"nix-explain", "bash", "/home/user/bin/nix-custom"} {
		if IsShim(name) {
			t.Errorf("IsShim(%q) = true", name)
		}
	}
}

func TestTailBufferKeepsEnd(t *testing.T) {
	b := newTailBuffer(8)
	b.Write([]byte("0123456789abcdef"))
	if got, want := b.String(), "89abcdef"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLooksLikeNixError(t *testing.T) {
	if !looksLikeNixError("error: infinite recursion encountered") {
		t.Fatal("error line not detected")
	}
	if looksLikeNixError(strings.Repeat("ok ", 100)) {
		t.Fatal("false positive")
	}
}
