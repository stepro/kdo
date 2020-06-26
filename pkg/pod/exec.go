package pod

import (
	"github.com/stepro/kdo/pkg/kubectl"
	"github.com/stepro/kdo/pkg/portforward"
)

// Exec executes a command in an existing pod associated with a hash
func Exec(k kubectl.CLI, hash string, container string, prekill []string, forward []string, stdin bool, tty bool, command ...string) error {
	name := Name(hash)

	args := []string{"exec", name}

	if container != "" {
		args = append(args, "--container", container)
	}

	if len(prekill) > 0 {
		killArgs := append(args, "--", "pkill", "-9")
		killArgs = append(killArgs, prekill...)
		k.Run(killArgs...) // ignore errors
	}

	if len(forward) > 0 {
		stop, err := portforward.Start(k, name, forward)
		if err != nil {
			return err
		}
		defer stop()
	}
	if stdin {
		args = append(args, "--stdin")
	}
	if tty {
		args = append(args, "--tty")
	}

	args = append(args, "--")
	args = append(args, command...)

	return k.Exec(args...)
}
