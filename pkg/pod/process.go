package pod

import (
	"strconv"
	"time"

	"github.com/stepro/kdo/pkg/kubectl"
)

// Process represents the main process in a container
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

// ExitCode waits for the process to complete and gets its exit code
func (p *Process) ExitCode() (int, error) {
	if p.exitCode == nil {
		var value string
		var err error
		for {
			value, err = p.k.String("get", "pod", p.Pod, "--output", `go-template={{range .status.containerStatuses}}{{if eq .name "`+p.Container+`"}}{{if .state.terminated}}{{.state.terminated.exitCode}}{{end}}{{end}}{{end}}`)
			if err != nil {
				return 0, err
			} else if value == "" {
				time.Sleep(1 * time.Second)
				continue
			}
			break
		}
		code, err := strconv.Atoi(value)
		if err != nil {
			return 0, err
		}
		p.exitCode = &code
	}
	return *p.exitCode, nil
}
