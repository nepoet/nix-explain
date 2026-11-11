package cli

import (
	"strings"
	"sync"
	"testing"
)

func TestTailBufferKeepsTheEnd(t *testing.T) {
	b := newTailBuffer(10)
	if _, err := b.Write([]byte("0123456789abcdef")); err != nil {
		t.Fatal(err)
	}
	if got, want := b.String(), "6789abcdef"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestTailBufferKeepsLastChunk(t *testing.T) {
	b := newTailBuffer(4)
	b.Write([]byte("aaaa"))
	b.Write([]byte("bbbb"))
	if got, want := b.String(), "bbbb"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestTailBufferIsSafeForConcurrentWriters(t *testing.T) {
	b := newTailBuffer(1024)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				b.Write([]byte(strings.Repeat("x", 32)))
			}
		}()
	}
	wg.Wait()
	if got := len(b.String()); got > 1024 {
		t.Fatalf("buffer grew to %d bytes, want <= 1024", got)
	}
}
