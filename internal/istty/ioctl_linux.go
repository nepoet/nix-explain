//go:build linux

package istty

import "syscall"

const ioctlReadTermios = syscall.TCGETS
