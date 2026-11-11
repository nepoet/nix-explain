//go:build !linux && !darwin

package istty

// Is reports whether fd refers to a terminal. On platforms without a small
// ioctl wrapper we conservatively say no, which disables interactive prompts
// but never blocks on input.
func Is(fd uintptr) bool { return false }
