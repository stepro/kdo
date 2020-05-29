package output

import (
	"io"
)

// Handler represents an output interface handler
type Handler interface {
	Message(level Level, format string, v ...interface{})
	Warning(format string, v ...interface{})
	Error(format string, v ...interface{})
	Prompt(format string, v ...interface{}) bool
	Start(format string, v ...interface{}) Operation
	NewWriter(label string, level Level, err bool) io.WriteCloser
	Object(level Level, o interface{})
	Result(o interface{})
	io.Closer
}
