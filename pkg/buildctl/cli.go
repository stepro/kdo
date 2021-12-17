package buildctl

import (
	"os/exec"

	"github.com/stepro/kdo/pkg/command"
	"github.com/stepro/kdo/pkg/output"
)

// Options represents global options for the buildctl CLI
type Options struct {
	Debug bool
}

// CLI represents the buildctl CLI
type CLI interface {
	// EachLine runs a buildctl command that sends
	// its lines of standard error to a callback
	EachErrLine(args []string, fn func(line string)) error
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
	if d.opt.Debug {
		globalOptions = append(globalOptions, "--debug")
	}
	cmd.Args = append(cmd.Args, append(globalOptions, arg...)...)

	return cmd
}

func (d *cli) EachErrLine(args []string, fn func(line string)) error {
	cmd := d.command(args...)
	cmd.Stderr = output.NewLineWriter(fn)

	return command.Run(cmd, d.out, d.verb)
}

// NewCLI creates a new buildctl CLI object
func NewCLI(path string, options *Options, out *output.Interface, verb output.Level) CLI {
	return &cli{
		path: path,
		opt:  options,
		out:  out,
		verb: verb,
	}
}
