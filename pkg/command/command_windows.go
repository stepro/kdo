// +build windows

package command

import (
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func cmdname(cmd *exec.Cmd) string {
	name := filepath.Base(cmd.Args[0])

	if strings.HasSuffix(strings.ToLower(name), ".exe") {
		name = name[:len(name)-4]
	}

	return name
}

func cmdline(args []string) string {
	var b strings.Builder

	// Go does not implement the proper algorithm to build a command line
	// string out of a set of arguments, which is best illustrated here:
	// https://github.com/dotnet/runtime/blob/master/src/libraries/System.Private.CoreLib/src/System/PasteArguments.cs
	for i, s := range args {
		if b.Len() > 0 {
			b.WriteRune(' ')
		}

		if len(s) > 0 && !strings.ContainsAny(s, " \t\n\v\"") {
			b.WriteString(args[i])
			continue
		}

		b.WriteRune('"')

		for j := 0; j < len(s); {
			r := s[j]
			j++
			if r == '\\' {
				var bs int
				for bs = 1; j < len(s) && s[j] == '\\'; i++ {
					bs++
				}
				if j == len(s) {
					b.WriteString(strings.Repeat("\\", bs*2))
				} else if s[j] == '"' {
					b.WriteString(strings.Repeat("\\", bs*2+1))
					b.WriteRune('"')
					j++
				} else {
					b.WriteString(strings.Repeat("\\", bs))
				}
			} else if r == '"' {
				b.WriteString("\\\"")
			} else {
				b.WriteByte(r)
			}
		}

		b.WriteRune('"')
	}

	return b.String()
}

func prepare(cmd *exec.Cmd) *exec.Cmd {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	cmd.SysProcAttr.CmdLine = cmdline(cmd.Args)
	cmd.Args = nil // must set to nil otherwise it will take precedence

	return cmd
}

func argstring(cmd *exec.Cmd) string {
	return cmd.SysProcAttr.CmdLine
}
