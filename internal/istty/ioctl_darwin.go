//go:build darwin

package istty

import "syscall"

const ioctlReadTermios = syscall.TIOCGETA
