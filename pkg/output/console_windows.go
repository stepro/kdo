// +build windows

package output

import (
	"io"
	"os"

	"golang.org/x/sys/windows"
)

type console struct {
	handle windows.Handle
	mode   uint32
}

func (c *console) Width() int {
	var info windows.ConsoleScreenBufferInfo
	err := windows.GetConsoleScreenBufferInfo(c.handle, &info)
	if err != nil {
		return 0
	}

	return int(info.Window.Right - info.Window.Left + 1)
}

func (c *console) SetRaw() {
	windows.SetConsoleMode(c.handle, c.mode|
		windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING|
		windows.DISABLE_NEWLINE_AUTO_RETURN)
}

func (c *console) Reset() {
	windows.SetConsoleMode(c.handle, c.mode)
}

func getConsole(w io.Writer) *console {
	f, ok := w.(*os.File)
	if !ok {
		return nil
	}

	h := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return nil
	}

	// Test if the file handle represents a console by attempting to enable
	// virtual terminal processing; if that fails, then there is no console
	if err := windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return nil
	}
	defer windows.SetConsoleMode(h, mode)

	return &console{
		handle: h,
		mode:   mode,
	}
}
