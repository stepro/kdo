// +build darwin

package output

import (
	"golang.org/x/sys/unix"
)

const (
	ioctlTcGet = unix.TIOCGETA
	ioctlTcSet = unix.TIOCSETA
)
