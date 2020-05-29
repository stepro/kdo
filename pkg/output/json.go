package output

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type jsonHandler struct {
	level Level
	yes   bool
	out   io.Writer
	ops   int
}

type jsonObject struct {
	Type      string      `json:"type"`
	Label     string      `json:"label,omitempty"`
	Operation int         `json:"operation,omitempty"`
	Content   interface{} `json:"content,omitempty"`
}

func (h *jsonHandler) write(typeName string, label string, op int, content interface{}) error {
	o := &jsonObject{
		Type:      typeName,
		Label:     label,
		Operation: op,
		Content:   content,
	}

	data, err := json.Marshal(o)
	if err != nil {
		return err
	}

	h.out.Write(data)
	h.out.Write([]byte("\n"))

	return nil
}

func (h *jsonHandler) Message(level Level, format string, v ...interface{}) {
	var typeName string
	switch level {
	case LevelNormal:
		typeName = "info"
	case LevelVerbose:
		typeName = "verbose"
	case LevelDebug:
		typeName = "debug"
	}

	h.write(typeName, "", 0, fmt.Sprintf(format, v...))
}

func (h *jsonHandler) Warning(format string, v ...interface{}) {
	h.write("warning", "", 0, fmt.Sprintf(format, v...))
}

func (h *jsonHandler) Error(format string, v ...interface{}) {
	h.write("error", "", 0, fmt.Sprintf(format, v...))
}

func (h *jsonHandler) Prompt(format string, v ...interface{}) bool {
	return h.yes
}

type jsonOperation struct {
	h  *jsonHandler
	id int
}

func (h *jsonHandler) Start(format string, v ...interface{}) Operation {
	h.ops++

	o := &jsonOperation{
		h:  h,
		id: h.ops,
	}

	h.write("start", "", o.id, fmt.Sprintf(format, v...))

	return o
}

func (o *jsonOperation) Progress(format string, v ...interface{}) {
	o.h.write("progress", "", o.id, fmt.Sprintf(format, v...))
}

func (o *jsonOperation) Aborted() {
	o.h.write("aborted", "", o.id, nil)
}

func (o *jsonOperation) Failed() {
	o.h.write("failed", "", o.id, nil)
}

func (o *jsonOperation) Done() {
	o.h.write("done", "", o.id, nil)
}

func (h *jsonHandler) NewWriter(label string, level Level, err bool) io.WriteCloser {
	var typeName string
	switch level {
	case LevelNormal:
		typeName = "stream"
	case LevelVerbose:
		typeName = "verboseStream"
	case LevelDebug:
		typeName = "debugStream"
	}
	if err {
		typeName += "Err"
	}

	return NewLineWriter(func(line string) {
		h.write(typeName, label, 0, line)
	})
}

func (h *jsonHandler) Object(level Level, o interface{}) {
	var typeName string
	switch level {
	case LevelNormal:
		typeName = "object"
	case LevelVerbose:
		typeName = "verboseObject"
	case LevelDebug:
		typeName = "debugObject"
	}

	h.write(typeName, "", 0, o)
}

func (h *jsonHandler) Result(o interface{}) {
	h.write("result", "", 0, o)
}

func (h *jsonHandler) Close() error {
	return nil
}

// NewJSONInterface creates a new output interface
// that marshals JSON objects as lines to a writer
func NewJSONInterface(level Level, yes bool, out io.Writer) *Interface {
	return NewInterface(level, &jsonHandler{
		level: level,
		yes:   yes,
		out:   out,
	})
}

// ReplayFromJSON reads the output from a JSON output
// interface and replays it on another output interface,
// optionally capturing the result in an output object
func ReplayFromJSON(r io.Reader, in *Interface, result interface{}) {
	ops := map[int]Operation{}
	streams := map[string]*Stream{}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		var o jsonObject
		if err := json.Unmarshal(scanner.Bytes(), &o); err != nil {
			continue
		}

		switch o.Type {
		case "info":
			in.Info("%s", o.Content)
		case "verbose":
			in.Verbose("%s", o.Content)
		case "debug":
			in.Debug("%s", o.Content)
		case "warning":
			in.Warning("%s", o.Content)
		case "error":
			in.Error("%s", o.Content)
		case "start":
			ops[o.Operation] = in.Start("%s", o.Content)
		case "progress":
			ops[o.Operation].Progress("%s", o.Content)
		case "aborted":
			ops[o.Operation].Aborted()
			delete(ops, o.Operation)
		case "failed":
			ops[o.Operation].Failed()
			delete(ops, o.Operation)
		case "done":
			ops[o.Operation].Done()
			delete(ops, o.Operation)
		}

		if strings.HasSuffix(o.Type, "Stream") || strings.HasSuffix(o.Type, "StreamErr") {
			key := fmt.Sprintf("%s\n%s", o.Label, o.Type)
			s, ok := streams[key]
			if !ok {
				level := LevelNormal
				err := false
				switch o.Type {
				case "verboseStream":
					level = LevelVerbose
				case "debugStream":
					level = LevelDebug
				case "streamErr":
					err = true
				case "verboseStreamErr":
					level = LevelVerbose
					err = true
				case "debugStreamErr":
					level = LevelDebug
					err = true
				}
				streams[key] = in.NewStream(o.Label, level, err)
				s = streams[key]
			}
			fmt.Fprintln(s, o.Content)
		}

		switch o.Type {
		case "object":
			in.Object(LevelNormal, o.Content)
		case "verboseObject":
			in.Object(LevelVerbose, o.Content)
		case "debugObject":
			in.Object(LevelDebug, o.Content)
		case "result":
			if result == nil {
				in.Result(o.Content)
			} else if data, err := json.Marshal(o.Content); err != nil {
				in.Result(o.Content)
			} else if err := json.Unmarshal(data, result); err != nil {
				in.Result(o.Content)
			}
		}
	}
}
