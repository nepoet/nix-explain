// Package state stores the most recent error output per terminal, so that a
// bare `nix-explain` can explain the error the user just saw without any
// copying or piping.
package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// MaxAge is how old a captured error may be before it is ignored.
const MaxAge = 30 * time.Minute

// dir returns the directory for capture files. It honours SUDO_UID so that a
// system-wide shim running as root writes into the invoking user's runtime
// directory.
func dir() string {
	if d := os.Getenv("NIX_EXPLAIN_STATE_DIR"); d != "" {
		return d
	}

	// A shim running under sudo must write where the invoking user will look:
	// their runtime dir, or a per-uid temp dir as a fallback.
	if s := os.Getenv("SUDO_UID"); s != "" {
		if uid, err := strconv.Atoi(s); err == nil {
			runtime := fmt.Sprintf("/run/user/%d", uid)
			if fi, err := os.Stat(runtime); err == nil && fi.IsDir() {
				return filepath.Join(runtime, "nix-explain")
			}
			return filepath.Join(os.TempDir(), fmt.Sprintf("nix-explain-%d", uid))
		}
	}

	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		return filepath.Join(xdg, "nix-explain")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("nix-explain-%d", os.Getuid()))
}

// ttyKey identifies the terminal a capture belongs to. Different tabs and
// windows must not overwrite each other.
func ttyKey() string {
	for _, fd := range []string{"/proc/self/fd/2", "/proc/self/fd/0"} {
		target, err := os.Readlink(fd)
		if err != nil || !strings.HasPrefix(target, "/dev/") {
			continue
		}
		return strings.NewReplacer("/", "-", "\\", "-").Replace(
			strings.TrimPrefix(target, "/dev/"),
		)
	}
	return "last"
}

// Path is the capture file for the current terminal.
func Path() string {
	return filepath.Join(dir(), ttyKey()+".log")
}

// Save writes text atomically with 0600 permissions.
func Save(text string) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	d := dir()
	if err := os.MkdirAll(d, 0o700); err != nil {
		return err
	}
	path := Path()
	tmp, err := os.CreateTemp(d, ".capture-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(text); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	// When a root shim runs via sudo, hand the file to the invoking user.
	if uid, err := strconv.Atoi(os.Getenv("SUDO_UID")); err == nil {
		gid, _ := strconv.Atoi(os.Getenv("SUDO_GID"))
		_ = os.Chown(tmp.Name(), uid, gid)
	}
	return os.Rename(tmp.Name(), path)
}

// Load returns the capture for the current terminal if it is fresh.
func Load() (string, error) {
	path := Path()
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if time.Since(fi.ModTime()) > MaxAge {
		return "", errors.New("last error is stale")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(data)) == "" {
		return "", errors.New("last error is empty")
	}
	return string(data), nil
}
