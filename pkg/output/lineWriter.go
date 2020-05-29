package output

import (
	"bytes"
	"errors"
	"io"
)

type lineWriter struct {
	buffer  bytes.Buffer
	writeln func(line string)
}

func (w *lineWriter) Write(p []byte) (n int, err error) {
	if w.writeln == nil {
		return 0, errors.New("output: write occurred to closed line writer")
	}

	for {
		if len(p) == 0 {
			return
		}

		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			nw, err := w.buffer.Write(p)
			n += nw
			return n, err
		}

		line := p[:i]

		if len(line) > 0 && line[i-1] == '\r' {
			line = p[:i-1]
		}

		if w.buffer.Len() > 0 {
			line = append(w.buffer.Bytes(), line...)
			w.buffer.Reset()
		}

		w.writeln(string(line))

		i++
		n += i
		p = p[i:]
	}
}

func (w *lineWriter) Close() error {
	if w.writeln == nil {
		return nil
	}

	if w.buffer.Len() > 0 {
		w.writeln(w.buffer.String())
	}

	w.buffer = bytes.Buffer{}
	w.writeln = nil

	return nil
}

// NewLineWriter creates a new line-segmented writer
func NewLineWriter(writeln func(line string)) io.WriteCloser {
	return &lineWriter{
		writeln: writeln,
	}
}
