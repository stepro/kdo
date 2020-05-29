package output

import (
	"io"
	"sync"
)

// MultiWriter represents a writer that duplicates its writes to
// a set of writers that can be dynamically attached and detached
type MultiWriter interface {
	Attach(w io.Writer)
	Detach(w io.Writer)
	io.Writer
}

type multiWriter struct {
	mu      sync.Mutex
	writers map[io.Writer]bool
}

func (mw *multiWriter) Attach(w io.Writer) {
	mw.mu.Lock()
	defer mw.mu.Unlock()

	mw.writers[w] = true
}

func (mw *multiWriter) Detach(w io.Writer) {
	mw.mu.Lock()
	defer mw.mu.Unlock()

	delete(mw.writers, w)
}

func (mw *multiWriter) Write(p []byte) (n int, err error) {
	mw.mu.Lock()
	defer mw.mu.Unlock()

	for w := range mw.writers {
		w.Write(p)
	}

	return len(p), nil
}

// NewMultiWriter creates a new multi writer
// with an initial set of attached writers
func NewMultiWriter(w ...io.Writer) MultiWriter {
	mw := multiWriter{
		writers: map[io.Writer]bool{},
	}

	for _, wr := range w {
		mw.writers[wr] = true
	}

	return &mw
}
