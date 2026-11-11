package state

import (
	"os"
	"testing"
	"time"
)

func TestSaveLoad(t *testing.T) {
	t.Setenv("NIX_EXPLAIN_STATE_DIR", t.TempDir())

	if _, err := Load(); err == nil {
		t.Fatal("Load() on an empty dir should fail")
	}
	if err := Save("error: infinite recursion encountered\n"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != "error: infinite recursion encountered\n" {
		t.Fatalf("got %q", got)
	}

	fi, err := os.Stat(Path())
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("permissions = %o, want 600", fi.Mode().Perm())
	}
}

func TestSaveIgnoresEmpty(t *testing.T) {
	t.Setenv("NIX_EXPLAIN_STATE_DIR", t.TempDir())
	if err := Save("   \n"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(Path()); !os.IsNotExist(err) {
		t.Fatalf("empty capture should not be written, stat err = %v", err)
	}
}

func TestLoadIgnoresStale(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NIX_EXPLAIN_STATE_DIR", dir)

	if err := Save("error: old\n"); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * MaxAge)
	if err := os.Chtimes(Path(), old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("stale capture should be ignored")
	}
}
