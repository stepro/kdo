package docker

import (
	"os/exec"

	"github.com/stepro/kdo/pkg/command"
	"github.com/stepro/kdo/pkg/output"
)

// Options represents global options for the docker CLI
type Options struct {
	Config   string
	LogLevel string
}

// CLI represents the docker CLI
type CLI interface {
	// EachLine runs a docker command that sends
	// its lines of standard output to a callback
	EachLine(args []string, fn func(line string)) error
}

type cli struct {
	path string
	opt  *Options
	out  *output.Interface
	verb output.Level
}

func (d *cli) command(arg ...string) *exec.Cmd {
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

func (d *cli) EachLine(args []string, fn func(line string)) error {
	return command.EachLine(d.command(args...), d.out, d.verb, fn)
}

// NewCLI creates a new docker CLI object
func NewCLI(path string, options *Options, out *output.Interface, verb output.Level) CLI {
	return &cli{
		path: path,
		opt:  options,
		out:  out,
		verb: verb,
	}
}
