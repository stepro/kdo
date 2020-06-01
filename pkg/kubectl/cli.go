package kubectl

import (
	"io"
	"os/exec"
	"strconv"
	"strings"

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
type CLI struct {
	path string
	opt  *Options
	out  *output.Interface
	verb output.Level
}

// NewCLI creates a new kubectl CLI object
func NewCLI(path string, options *Options, out *output.Interface, verb output.Level) *CLI {
	return &CLI{
		path: path,
		opt:  options,
		out:  out,
		verb: verb,
	}
}

func (k *CLI) command(arg ...string) *exec.Cmd {
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

// Run runs a kubectl command
func (k *CLI) Run(arg ...string) error {
	return command.Run(k.command(arg...), k.out, k.verb)
}

// Input runs a kubectl command with input
func (k *CLI) Input(input io.Reader, arg ...string) error {
	cmd := k.command(arg...)
	cmd.Stdin = input
	return command.Run(cmd, k.out, k.verb)
}

// String runs a kubectl command that outputs a string
func (k *CLI) String(arg ...string) (string, error) {
	return command.String(k.command(arg...), k.out, k.verb)
}

// Lines runs a kubectl command that outputs multiple lines
func (k *CLI) Lines(arg ...string) ([]string, error) {
	return command.Lines(k.command(arg...), k.out, k.verb)
}

// StartLines starts a long-running kubectl command that
// sends its lines of standard output to a callback function
func (k *CLI) StartLines(args []string, fn func(line string), end chan error) func() {
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

// Apply runs a kubectl apply command
func (k *CLI) Apply(manifest string) error {
	return k.Input(strings.NewReader(manifest), "apply", "-f", "-")
}

// Exec "replaces" the current process with a kubectl command
func (k *CLI) Exec(arg ...string) error {
	return command.Exec(k.command(arg...), k.out, k.verb)
}
