package docker

import (
	"os/exec"

	"github.com/microsoft/kudo/pkg/command"
	"github.com/microsoft/kudo/pkg/output"
)

// Options represents global options for the docker CLI
type Options struct {
	Config   string
	LogLevel string
}

// CLI represents the docker CLI
type CLI struct {
	path string
	opt  *Options
	out  *output.Interface
	verb output.Level
}

// NewCLI creates a new docker CLI object
func NewCLI(path string, options *Options, out *output.Interface, verb output.Level) *CLI {
	return &CLI{
		path: path,
		opt:  options,
		out:  out,
		verb: verb,
	}
}

func (d *CLI) command(arg ...string) *exec.Cmd {
	cmd := exec.Command(d.path)

	var globalOptions []string
	if d.opt.Config != "" {
		globalOptions = append(globalOptions, "--config", d.opt.Config)
	}
	if d.opt.LogLevel != "" {
		globalOptions = append(globalOptions, "--log-level", d.opt.LogLevel)
	}
	cmd.Args = append(cmd.Args, append(globalOptions, arg...)...)

	return cmd
}

// EachLine runs a docker command and streams its lines
// of standard output as strings to a callback function
func (d *CLI) EachLine(args []string, fn func(line string)) error {
	return command.EachLine(d.command(args...), d.out, d.verb, fn)
}
