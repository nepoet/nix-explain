// Package shim implements the transparent wrappers installed for nix,
// nixos-rebuild and friends. The shim runs the real binary, passes its output
// through unchanged and saves the error output so that `nix-explain` can
// explain it afterwards.
package shim

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nepoet/nix-explain/internal/i18n"
	"github.com/nepoet/nix-explain/internal/istty"
	"github.com/nepoet/nix-explain/internal/state"
)

// Names are the commands we install shims for.
var Names = []string{"nix", "nixos-rebuild", "home-manager", "nix-shell", "nix-build"}

// IsShim reports whether the program was invoked through one of our symlinks.
func IsShim(argv0 string) bool {
	base := filepath.Base(argv0)
	for _, n := range Names {
		if base == n {
			return true
		}
	}
	return false
}

// Run executes the real command, captures its stderr and returns its exit
// code. Set NIX_EXPLAIN=0 to bypass the shim entirely.
func Run(name string, args []string) int {
	real, err := resolveReal(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nix-explain: %v\n", err)
		return 127
	}
	if os.Getenv("NIX_EXPLAIN") == "0" {
		return runPlain(real, args)
	}

	cmd := exec.Command(real, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	capture := newTailBuffer(4 << 20)
	cmd.Stderr = io.MultiWriter(os.Stderr, capture)

	code := exitCode(cmd.Run())
	if code == 0 {
		return 0
	}

	text := capture.String()
	if looksLikeNixError(text) {
		_ = state.Save(text)
		if isTerminal(os.Stderr) && os.Getenv("NIX_EXPLAIN_HINT") != "0" {
			fmt.Fprint(os.Stderr, "\n"+i18n.Get(i18n.Resolve("")).Hint+"\n")
		}
	}
	return code
}

func runPlain(real string, args []string) int {
	cmd := exec.Command(real, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return exitCode(cmd.Run())
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if c := ee.ExitCode(); c >= 0 {
			return c
		}
		return 1
	}
	return 1
}

func looksLikeNixError(text string) bool {
	return strings.Contains(strings.ToLower(text), "error")
}

func isTerminal(f *os.File) bool {
	return istty.Is(f.Fd())
}

// resolveReal finds the real command in PATH, skipping the directory that
// contains our own binary (otherwise the shim would call itself).
func resolveReal(name string) (string, error) {
	env := "NIX_EXPLAIN_REAL_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	if v := os.Getenv(env); v != "" {
		return v, nil
	}

	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	self, _ = filepath.EvalSymlinks(self)
	selfDir := filepath.Dir(self)

	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			dir = "."
		}
		abs, err := filepath.Abs(dir)
		if err != nil || abs == selfDir {
			continue
		}
		candidate := filepath.Join(abs, name)
		fi, err := os.Stat(candidate)
		if err != nil || fi.IsDir() || fi.Mode()&0o111 == 0 {
			continue
		}
		if resolved, err := filepath.EvalSymlinks(candidate); err == nil && resolved == self {
			continue
		}
		return candidate, nil
	}
	return "", fmt.Errorf("%s not found in PATH (set NIX_EXPLAIN_REAL_%s)",
		name, strings.ToUpper(strings.ReplaceAll(name, "-", "_")))
}

// tailBuffer keeps the last max bytes; it is safe for concurrent writers.
type tailBuffer struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func newTailBuffer(max int) *tailBuffer { return &tailBuffer{max: max} }

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := len(p)
	if n >= t.max {
		t.buf = append(t.buf[:0], p[n-t.max:]...)
		return n, nil
	}
	if over := len(t.buf) + n - t.max; over > 0 {
		copy(t.buf, t.buf[over:])
		t.buf = t.buf[:len(t.buf)-over]
	}
	t.buf = append(t.buf, p...)
	return n, nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}
