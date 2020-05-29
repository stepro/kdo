package output

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ghodss/yaml"
)

// StdObjectFormat represents a standard output interface object format
type StdObjectFormat int

// Standard output interface object formats
const (
	StdObjectFormatYAML = StdObjectFormat(0)
	StdObjectFormatJSON = StdObjectFormat(1)
)

// StdOptions represents standard output interface options
type StdOptions struct {
	AcceptPrompts bool
	ObjectFormat  StdObjectFormat
}

type stdHandler struct {
	level  Level
	opt    *StdOptions
	out    io.Writer
	err    io.Writer
	cout   *console
	cerr   *console
	clines int
}

func (h *stdHandler) writers(err bool) (w io.Writer, c *console, fd string) {
	w = h.out
	c = h.cout
	if h.out == h.err {
		fd = "1:"
	}

	if err {
		w = h.err
		c = h.cerr
		if h.out == h.err {
			fd = "2:"
		}
	}

	return
}

func (h *stdHandler) write(err bool, format string, v ...interface{}) {
	w, c, fd := h.writers(err)
	if w == nil {
		return
	}

	line := fmt.Sprintf("%s%s", fd, fmt.Sprintf(format, v...))

	fmt.Fprintln(w, line)

	if c != nil {
		width := c.Width()
		length := len(line)
		h.clines += length / width
		if length%width != 0 {
			h.clines++
		}
	}
}

func (h *stdHandler) Message(level Level, format string, v ...interface{}) {
	if level > LevelVerbose {
		_, file, line, ok := runtime.Caller(3)
		if !ok {
			file = "unknown"
			line = 0
		}
		file = filepath.Base(file)
		format = fmt.Sprintf("[%s:%d]: %s", file, line, format)
	}
	h.write(level > LevelVerbose, format, v...)
}

func (h *stdHandler) Warning(format string, v ...interface{}) {
	h.write(true, "Warning: "+format, v...)
}

func (h *stdHandler) Error(format string, v ...interface{}) {
	h.write(true, "Error: "+format, v...)
}

func (h *stdHandler) Prompt(format string, v ...interface{}) bool {
	w, c, fd := h.writers(false)
	if w == nil {
		return h.opt.AcceptPrompts
	}

	line := fmt.Sprintf("%s%s [y/N]: ", fd, fmt.Sprintf(format, v...))

	fmt.Fprint(w, line)

	var accept bool
	if !h.opt.AcceptPrompts {
		r := bufio.NewReader(os.Stdin)
		l, _ := r.ReadString('\n')
		if !strings.HasSuffix(l, "\n") {
			fmt.Fprintln(w)
		}
		l = strings.TrimSuffix(l, "\n")
		l = strings.TrimSuffix(l, "\r")
		line += l
		accept = l == "Y" || l == "y"
	} else {
		line += "Y"
		fmt.Fprintln(w, "Y")
		accept = true
	}

	if c != nil {
		width := c.Width()
		length := len(line)
		h.clines += length / width
		if length%width != 0 {
			h.clines++
		}
	}

	return accept
}

type stdOperation struct {
	h      *stdHandler
	w      io.Writer
	c      *console
	line   int
	action string
}

func (o *stdOperation) fit(width int, format string, v ...interface{}) string {
	s := fmt.Sprintf(format, v...)

	if len(s) > width-1 {
		s = s[:width-1]
	}

	return s
}

func (h *stdHandler) Start(format string, v ...interface{}) Operation {
	w, c, fd := h.writers(false)

	line := 0
	if c != nil {
		line = h.clines
	}

	o := &stdOperation{
		h:      h,
		w:      w,
		c:      c,
		line:   line,
		action: fd + fmt.Sprintf(format, v...),
	}

	action := fmt.Sprintf("%s...", o.action)
	if c != nil {
		action = o.fit(c.Width(), action)
	}

	if w != nil {
		fmt.Fprintln(w, action)
	}

	if c != nil {
		h.clines++
	}

	return o
}

const (
	ansiColorRed     = 31
	ansiColorGreen   = 32
	ansiColorMagenta = 35
)

func (o *stdOperation) writeStatus(color int, status string) {
	h := o.h
	w := o.w
	c := o.c

	if c != nil {
		c.SetRaw()
		defer c.Reset()
		width := c.Width()
		action := o.fit(width, "%s...", o.action)
		status := o.fit(width-len(action), status)
		fmt.Fprint(w, "\r")                         // move to start of line
		fmt.Fprintf(w, "\x1b[%dA", h.clines-o.line) // move up # rows
		fmt.Fprint(w, "\x1b[K")                     // erase line
		fmt.Fprint(w, action)                       // write action
		fmt.Fprintf(w, "\x1b[%dm", color)           // set foreground color
		fmt.Fprint(w, status)                       // write status
		fmt.Fprint(w, "\x1b[m")                     // reset foreground color
		fmt.Fprint(w, "\r")                         // move to start of line
		fmt.Fprintf(w, "\x1b[%dB", h.clines-o.line) // move down # rows
	} else if w != nil {
		fmt.Fprintf(w, "%s: %s\n", o.action, status)
	}
}

func (o *stdOperation) Progress(format string, v ...interface{}) {
	o.writeStatus(ansiColorMagenta, fmt.Sprintf(format, v...))
}

func (o *stdOperation) Aborted() {
	o.writeStatus(ansiColorRed, "aborted")
}

func (o *stdOperation) Failed() {
	o.writeStatus(ansiColorRed, "failed")
}

func (o *stdOperation) Done() {
	o.writeStatus(ansiColorGreen, "done")
}

func (h *stdHandler) NewWriter(label string, level Level, err bool) io.WriteCloser {
	var prefix string
	if label != "" {
		prefix = fmt.Sprintf("[%s] ", label)
	}

	return NewLineWriter(func(line string) {
		h.write(err, "%s%s", prefix, line)
	})
}

func (h *stdHandler) Object(level Level, o interface{}) {
	w, c, fd := h.writers(false)

	if w == nil {
		return
	}

	var data []byte
	var err error
	switch h.opt.ObjectFormat {
	case StdObjectFormatYAML:
		data, err = yaml.Marshal(o)
	case StdObjectFormatJSON:
		data, err = json.MarshalIndent(o, "", "    ")
	}
	if err != nil {
		return
	}

	if fd != "" {
		data = append([]byte(fd), data...)
		data = bytes.ReplaceAll(data, []byte{'\n'}, append([]byte{'\n'}, []byte(fd)...))
		if len(data) > len(fd) && data[len(data)-len(fd)-1] == '\n' {
			data = data[:len(data)-len(fd)]
		}
	}

	w.Write(data)

	if c != nil {
		width := c.Width()
		lines := bytes.Split(data, []byte{'\n'})
		for _, line := range lines {
			length := len(string(line))
			h.clines += length / width
			if length%width != 0 {
				h.clines++
			}
		}
	}

	if h.opt.ObjectFormat == StdObjectFormatJSON {
		fmt.Fprintln(w)
	}
}

func (h *stdHandler) Result(o interface{}) {
	h.Object(LevelNormal, o)
}

func (h *stdHandler) Close() error {
	return nil
}

// NewStdInterface creates a new output interface that
// formats output to standard output and error writers
func NewStdInterface(level Level, opt *StdOptions, out io.Writer, err io.Writer) *Interface {
	if opt == nil {
		opt = &StdOptions{}
	}

	var cout *console
	var cerr *console
	if level == LevelNormal {
		if cout = getConsole(out); cout != nil && cout.Width() == 0 {
			cout = nil
		}
		if cerr = getConsole(err); cerr != nil && cerr.Width() == 0 {
			cerr = nil
		}
	}

	return NewInterface(level, &stdHandler{
		level: level,
		opt:   opt,
		out:   out,
		err:   err,
		cout:  cout,
		cerr:  cerr,
	})
}
