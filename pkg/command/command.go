package command

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/stepro/kdo/pkg/output"
)

// Run runs a command
func Run(cmd *exec.Cmd, out *output.Interface, verb output.Level) error {
	var stderr *bytes.Buffer
	if out == nil || (out.Level < verb && cmd.Stderr == nil) {
		stderr = &bytes.Buffer{}
		cmd.Stderr = stderr
	}

	label := cmdname(cmd)

	if out != nil {
		if cmd.Stdout == nil {
			cmd.Stdout = out.NewStream(label, verb, false)
		}
		if cmd.Stderr == nil {
			cmd.Stderr = out.NewStream(label, verb, true)
		}
	}

	cmd = prepare(cmd)

	var args string
	if out != nil {
		args = argstring(cmd)
		out.Debug("running: %s", args)
	}

	if err := cmd.Run(); err != nil {
		if out != nil {
			out.Debug("failed: %s", args)
		}
		if stderr != nil {
			err = fmt.Errorf("%s: %s", label, string(stderr.Bytes()))
		} else {
			err = fmt.Errorf("%s: %v", label, err)
		}
		return err
	}

	if out != nil {
		out.Debug("completed: %s", args)
	}

	return nil
}

func buffer(cmd *exec.Cmd, out *output.Interface, verb output.Level) (*bytes.Buffer, error) {
	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout

	if err := Run(cmd, out, verb); err != nil {
		return nil, err
	}

	return stdout, nil
}

// String runs a command and returns its standard output as a string
func String(cmd *exec.Cmd, out *output.Interface, verb output.Level) (string, error) {
	stdout, err := buffer(cmd, out, verb)
	if err != nil {
		return "", err
	}

	return strings.ReplaceAll(stdout.String(), "\r\n", "\n"), nil
}

// Lines runs a command and returns its lines of standard output as strings
func Lines(cmd *exec.Cmd, out *output.Interface, verb output.Level) ([]string, error) {
	stdout, err := buffer(cmd, out, verb)
	if err != nil {
		return nil, err
	}

	var lines []string
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, nil
}

// EachLine runs a command that sends its lines of
// standard output as strings to a callback function
func EachLine(cmd *exec.Cmd, out *output.Interface, verb output.Level, fn func(line string)) error {
	cmd.Stdout = output.NewLineWriter(fn)

	return Run(cmd, out, verb)
}

// Exec "replaces" the current process with a command
func Exec(cmd *exec.Cmd, out *output.Interface, verb output.Level) error {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd = prepare(cmd)

	var args string
	if out != nil {
		args = argstring(cmd)
		out.Debug("running: %s", args)
	}

	// Ensure Ctrl+C waits for child processes
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	if err := cmd.Run(); err != nil {
		if out != nil {
			out.Debug("failed: %s", args)
		}
		return err
	}

	if out != nil {
		out.Debug("completed: %s", args)
	}

	return nil
}
