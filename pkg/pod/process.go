package pod

import (
	"strconv"

	"github.com/stepro/kdo/pkg/kubectl"
)

// Process represents a process in a pod
type Process struct {
	k         kubectl.CLI
	Pod       string
	Container string
	exitCode  *int
}

// Exited indicates if the process has exited
func (p *Process) Exited() bool {
	return p.exitCode != nil
}

// ExitCode gets the exit code of the process
func (p *Process) ExitCode() (int, error) {
	if p.exitCode == nil {
		value, err := p.k.String("get", "pod", p.Pod, "--output", `go-template={{range .status.containerStatuses}}{{if eq .name "`+p.Container+`"}}{{if .state.terminated}}{{.state.terminated.exitCode}}{{end}}{{end}}{{end}}`)
		if err != nil || value == "" {
			return 0, err
		} else if code, err := strconv.Atoi(value); err != nil {
			return 0, err
		} else {
			p.exitCode = &code
		}
	}
	return *p.exitCode, nil
}
