package kubectl

import (
	"io"
	"os/exec"
	"strconv"

	"github.com/stepro/kdo/pkg/command"
	"github.com/stepro/kdo/pkg/output"
)

// Options represents global options for the kubectl CLI
type Options struct {
	Kubeconfig string
	Context    string
	Namespace  string
	Verbosity  int
}

// CLI represents the kubectl CLI
type CLI interface {
	// Run runs a kubectl command
	Run(arg ...string) error
	// Input runs a kubectl command with input
	Input(input io.Reader, arg ...string) error
	// String runs a kubectl command that outputs a string
	String(arg ...string) (string, error)
	// Lines runs a kubectl command that outputs multiple lines
	Lines(arg ...string) ([]string, error)
	// StartLines starts a kubectl command that
	// sends its lines of output to a callback
	StartLines(args []string, fn func(line string), end chan error) func()
	// Exec simulates replacing the current process with a kubectl command
	Exec(arg ...string) error
}

type cli struct {
	path string
	opt  *Options
	out  *output.Interface
	verb output.Level
}

func (k *cli) command(arg ...string) *exec.Cmd {
	cmd := exec.Command(k.path)

	var globalOptions []string
	if k.opt.Kubeconfig != "" {
		globalOptions = append(globalOptions, "--kubeconfig", k.opt.Kubeconfig)
	}
	if k.opt.Context != "" {
		globalOptions = append(globalOptions, "--context", k.opt.Context)
	}
	if k.opt.Namespace != "" {
		globalOptions = append(globalOptions, "--namespace", k.opt.Namespace)
	}
	if k.opt.Verbosity != 0 {
		globalOptions = append(globalOptions, "-v", strconv.Itoa(k.opt.Verbosity))
	}
	cmd.Args = append(cmd.Args, append(globalOptions, arg...)...)

	return cmd
}

func (k *cli) Run(arg ...string) error {
	return command.Run(k.command(arg...), k.out, k.verb)
}

func (k *cli) Input(input io.Reader, arg ...string) error {
	cmd := k.command(arg...)
	cmd.Stdin = input
	return command.Run(cmd, k.out, k.verb)
}

func (k *cli) String(arg ...string) (string, error) {
	return command.String(k.command(arg...), k.out, k.verb)
}

func (k *cli) Lines(arg ...string) ([]string, error) {
	return command.Lines(k.command(arg...), k.out, k.verb)
}

func (k *cli) StartLines(args []string, fn func(line string), end chan error) func() {
	cmd := k.command(args...)

	go func() {
		err := command.EachLine(cmd, k.out, k.verb, fn)
		if end != nil {
			end <- err
		}
	}()

	return func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}
}

func (k *cli) Exec(arg ...string) error {
	return command.Exec(k.command(arg...), k.out, k.verb)
}

// NewCLI creates a new kubectl CLI object
func NewCLI(path string, options *Options, out *output.Interface, verb output.Level) CLI {
	return &cli{
		path: path,
		opt:  options,
		out:  out,
		verb: verb,
	}
}
