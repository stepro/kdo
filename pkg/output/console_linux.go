// +build linux

package output

import (
	"golang.org/x/sys/unix"
)

const (
	ioctlTcGet = unix.TCGETS
	ioctlTcSet = unix.TCSETS
)
