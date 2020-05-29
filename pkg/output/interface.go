package output

import (
	"io"
	"sync"
)

// Level represents an output level
type Level int

// Output levels
const (
	LevelQuiet   = Level(-1)
	LevelNormal  = Level(0)
	LevelVerbose = Level(1)
	LevelDebug   = Level(2)
)

// Is indicates if the level matches a level
func (l Level) Is(level Level) bool {
	switch level {
	case LevelQuiet:
		return l < LevelNormal
	case LevelNormal:
		return l == LevelNormal
	default:
		return l >= level
	}
}

// Interface represents an output interface
type Interface struct {
	Level
	mu      sync.Mutex
	closers map[io.Closer]bool
	handler Handler
}

func (in *Interface) message(level Level, format string, v ...interface{}) {
	if in.Level < level {
		return
	}

	in.mu.Lock()
	defer in.mu.Unlock()

	if in.handler == nil {
		return
	}

	in.handler.Message(level, format, v...)
}

// Info outputs an informational message
func (in *Interface) Info(format string, v ...interface{}) {
	in.message(LevelNormal, format, v...)
}

// Verbose outputs a detailed message
func (in *Interface) Verbose(format string, v ...interface{}) {
	in.message(LevelVerbose, format, v...)
}

// Debug outputs a diagnostic message
func (in *Interface) Debug(format string, v ...interface{}) {
	in.message(LevelDebug, format, v...)
}

// Warning outputs a warning message
func (in *Interface) Warning(format string, v ...interface{}) {
	if in.Level.Is(LevelQuiet) {
		return
	}

	in.mu.Lock()
	defer in.mu.Unlock()

	if in.handler == nil {
		return
	}

	in.handler.Warning(format, v...)
}

// Error outputs an error message
func (in *Interface) Error(format string, v ...interface{}) {
	in.mu.Lock()
	defer in.mu.Unlock()

	if in.handler == nil {
		return
	}

	in.handler.Error(format, v...)
}

// Prompt asks an interactive user to answer a question, or for
// non-interactive sessions, returns a pre-configured response
func (in *Interface) Prompt(format string, v ...interface{}) bool {
	in.mu.Lock()
	defer in.mu.Unlock()

	if in.handler == nil {
		return false
	}

	return in.handler.Prompt(format, v...)
}

// Operation represents an output operation
type Operation interface {
	Progress(format string, v ...interface{})
	Aborted()
	Failed()
	Done()
}

type operation struct {
	in *Interface
	op Operation
}

// Start reports the start of an operation
func (in *Interface) Start(format string, v ...interface{}) Operation {
	if in.Level.Is(LevelQuiet) {
		return &operation{
			in: in,
		}
	}

	in.mu.Lock()
	defer in.mu.Unlock()

	if in.closers == nil {
		return &operation{
			in: in,
		}
	}

	o := &operation{
		in: in,
		op: in.handler.Start(format, v...),
	}

	in.closers[o] = true

	return o
}

func (o *operation) Progress(format string, v ...interface{}) {
	o.in.mu.Lock()
	defer o.in.mu.Unlock()

	if o.op == nil {
		return
	}

	o.op.Progress(format, v...)
}

func (o *operation) end() {
	o.op = nil

	if o.in.closers != nil {
		delete(o.in.closers, o)
	}
}

func (o *operation) Aborted() {
	o.in.mu.Lock()
	defer o.in.mu.Unlock()

	if o.op == nil {
		return
	}

	o.op.Aborted()

	o.end()
}

func (o *operation) Failed() {
	o.in.mu.Lock()
	defer o.in.mu.Unlock()

	if o.op == nil {
		return
	}

	o.op.Failed()

	o.end()
}

func (o *operation) Done() {
	o.in.mu.Lock()
	defer o.in.mu.Unlock()

	if o.op == nil {
		return
	}

	o.op.Done()

	o.end()
}

// Do performs an operation
func (in *Interface) Do(format string, v ...interface{}) error {
	op := in.Start(format, v[:len(v)-1]...)

	var err error
	defer func() {
		if err != nil {
			op.Failed()
		} else {
			op.Done()
		}
	}()

	if f, _ := v[len(v)-1].(func() error); f != nil {
		err = f()
	} else if f, _ := v[len(v)-1].(func(Operation) error); f != nil {
		err = f(op)
	}

	return err
}

func (o *operation) Close() error {
	o.Aborted()

	return nil
}

// Stream represents an output stream
type Stream struct {
	in    *Interface
	Label string
	Level
	Err    bool
	writer io.WriteCloser
}

// NewStream creates a new output stream
func (in *Interface) NewStream(label string, level Level, err bool) *Stream {
	if in.Level.Is(LevelQuiet) || level.Is(LevelQuiet) || in.Level < level {
		return &Stream{
			in:    in,
			Label: label,
			Level: level,
			Err:   err,
		}
	}

	in.mu.Lock()
	defer in.mu.Unlock()

	if in.closers == nil {
		return &Stream{
			in:    in,
			Label: label,
			Level: level,
			Err:   err,
		}
	}

	s := &Stream{
		in:     in,
		Label:  label,
		Level:  level,
		Err:    err,
		writer: in.handler.NewWriter(label, level, err),
	}

	in.closers[s] = true

	return s
}

// Write writes data to the output stream
func (s *Stream) Write(p []byte) (n int, err error) {
	s.in.mu.Lock()
	defer s.in.mu.Unlock()

	if s.writer == nil {
		return len(p), nil
	}

	return s.writer.Write(p)
}

// Close flushes and closes the output stream
func (s *Stream) Close() error {
	s.in.mu.Lock()
	defer s.in.mu.Unlock()

	if s.writer == nil {
		return nil
	}

	if err := s.writer.Close(); err != nil {
		return err
	}

	s.writer = nil

	return nil
}

// Object outputs an object
func (in *Interface) Object(level Level, o interface{}) {
	if in.Level.Is(LevelQuiet) || level.Is(LevelQuiet) || in.Level < level {
		return
	}

	in.mu.Lock()
	defer in.mu.Unlock()

	if in.handler == nil {
		return
	}

	in.handler.Object(level, o)
}

// Result returns a result object
func (in *Interface) Result(o interface{}) {
	if in.Level.Is(LevelQuiet) {
		return
	}

	in.mu.Lock()
	defer in.mu.Unlock()

	if in.handler == nil {
		return
	}

	in.handler.Result(o)
}

// Close closes the output interface
func (in *Interface) Close() error {
	in.mu.Lock()

	if in.handler == nil {
		in.mu.Unlock()
		return nil
	}

	var closers []io.Closer
	for closer := range in.closers {
		closers = append(closers, closer)
	}
	in.closers = nil

	in.mu.Unlock()

	for _, closer := range closers {
		closer.Close()
	}

	in.mu.Lock()
	defer in.mu.Unlock()

	if err := in.handler.Close(); err != nil {
		return err
	}

	in.handler = nil

	return nil
}

// NewInterface creates a new output interface
func NewInterface(level Level, handler Handler) *Interface {
	return &Interface{
		Level:   level,
		closers: map[io.Closer]bool{},
		handler: handler,
	}
}
