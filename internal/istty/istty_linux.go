// Package istty reports whether a file descriptor is a real terminal.
//
// os.ModeCharDevice is not enough: /dev/null is a character device too, and
// treating it as a terminal would make nix-explain wait for a paste that can
// never arrive.
package istty

import (
	"syscall"
	"unsafe"
)

// Is reports whether fd refers to a terminal.
func Is(fd uintptr) bool {
	var t syscall.Termios
	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL, fd, ioctlReadTermios,
		uintptr(unsafe.Pointer(&t)), 0, 0, 0,
	)
	return errno == 0
}
