package output

import (
	"fmt"
	"io"
	"time"
)

// NewLogger creates a new standard output interface
// that prefixes a timestamp to each line of output
func NewLogger(level Level, format string, utc bool, w io.Writer) *Interface {
	if format == "" {
		format = time.RFC3339Nano
	}
	writeln := func(line string) {
		now := time.Now()
		if utc {
			now = now.UTC()
		}
		fmt.Fprintf(w, "[%s] %s\n", now.Format(format), line)
	}
	return NewStdInterface(level, &StdOptions{
		ObjectFormat: StdObjectFormatJSON,
	}, NewLineWriter(writeln), NewLineWriter(writeln))
}
