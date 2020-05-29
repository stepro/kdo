// +build !windows

package output

import (
	"io"
	"os"

	"golang.org/x/sys/unix"
)

type console struct {
	fd      int
	termios *unix.Termios
}

func (c *console) Width() int {
	uws, err := unix.IoctlGetWinsize(c.fd, unix.TIOCGWINSZ)
	if err != nil {
		return 0
	}

	return int(uws.Col)
}

func (c *console) SetRaw() {
	t := *c.termios
	t.Iflag &^= (unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON)
	t.Oflag &^= unix.OPOST
	t.Lflag &^= (unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN)
	t.Cflag &^= (unix.CSIZE | unix.PARENB)
	t.Cflag &^= unix.CS8
	t.Cc[unix.VMIN] = 1
	t.Cc[unix.VTIME] = 0

	unix.IoctlSetTermios(c.fd, ioctlTcSet, &t)
}

func (c *console) Reset() {
	unix.IoctlSetTermios(c.fd, ioctlTcSet, c.termios)
}

func getConsole(w io.Writer) *console {
	f, ok := w.(*os.File)
	if !ok {
		return nil
	}

	fd := int(f.Fd())
	termios, err := unix.IoctlGetTermios(fd, ioctlTcGet)
	if err != nil {
		return nil
	}

	return &console{
		fd:      fd,
		termios: termios,
	}
}
