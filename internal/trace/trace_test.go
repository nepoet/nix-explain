package trace

import (
	"strings"
	"testing"
)

const recursionSample = `error:
       … while evaluating the file '/home/user/lab/recursion.nix':

       error: infinite recursion encountered
       at /home/user/lab/recursion.nix:2:7:
            1| let
            2|   a = b;
             |       ^
            3|   b = a;
`

func TestParseFindsUserFrame(t *testing.T) {
	tr := Parse(recursionSample)
	if len(tr.Frames) != 1 {
		t.Fatalf("got %d frames, want 1: %+v", len(tr.Frames), tr.Frames)
	}
	loc, ok := tr.Location()
	if !ok {
		t.Fatal("no location found")
	}
	if loc.File != "/home/user/lab/recursion.nix" || loc.Line != 2 || loc.Col != 7 {
		t.Fatalf("unexpected location: %+v", loc)
	}
	if tr.Hidden() != 0 {
		t.Fatalf("hidden = %d, want 0", tr.Hidden())
	}
}

func TestParseHidesNixpkgsFrames(t *testing.T) {
	sample := `error:
       … while evaluating the option ` + "`services.foo.enable'" + `:

       … while calling the 'seq' builtin
         at /nix/store/abc123-source/lib/modules.nix:402:18:

       … while evaluating the attribute 'value'
         at /home/user/config.nix:12:5:

       error: The option ` + "`foo'" + ` does not exist.
`
	tr := Parse(sample)
	if got, want := len(tr.Frames), 2; got != want {
		t.Fatalf("frames = %d, want %d: %+v", got, want, tr.Frames)
	}
	if got, want := tr.Hidden(), 1; got != want {
		t.Fatalf("hidden = %d, want %d", got, want)
	}
	loc, ok := tr.Location()
	if !ok || loc.File != "/home/user/config.nix" || loc.Line != 12 {
		t.Fatalf("unexpected location: %+v (ok=%v)", loc, ok)
	}
}

func TestParseIgnoresStringFramesForLocation(t *testing.T) {
	sample := `error: undefined variable 'foo'
       at «string»:1:1:
            1| foo + 1
             | ^
`
	tr := Parse(sample)
	if len(tr.Frames) != 1 {
		t.Fatalf("frames = %d, want 1", len(tr.Frames))
	}
	if _, ok := tr.Location(); ok {
		t.Fatal("«string» frame must not be reported as a file location")
	}
}

func TestParseStripsANSI(t *testing.T) {
	tr := Parse("\x1b[31merror:\x1b[0m\n       at /home/user/a.nix:3:4:\n")
	if len(tr.Frames) != 1 {
		t.Fatalf("frames = %d, want 1", len(tr.Frames))
	}
	loc, _ := tr.Location()
	if !strings.HasSuffix(loc.File, "a.nix") {
		t.Fatalf("unexpected file %q", loc.File)
	}
}
