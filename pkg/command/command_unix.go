// +build !windows

package command

import (
	"os/exec"
	"path/filepath"
	"strings"
)

func cmdname(cmd *exec.Cmd) string {
	return filepath.Base(cmd.Args[0])
}

func prepare(cmd *exec.Cmd) *exec.Cmd {
	return cmd
}

func argstring(cmd *exec.Cmd) string {
	return strings.Join(cmd.Args, " ")
}
