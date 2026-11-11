package cli

import "sync"

// tailBuffer keeps the last max bytes written to it. Wrapping a command that
// produces a very large log must not grow memory without bound, and Nix puts
// the useful part of a failure at the end of the output.
//
// It is safe for concurrent use: os/exec writes the child's stdout and stderr
// from separate goroutines when they are not the same writer.
type tailBuffer struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func newTailBuffer(max int) *tailBuffer {
	if max <= 0 {
		max = 1 << 20
	}
	return &tailBuffer{max: max}
}

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
