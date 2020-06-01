package filesync

import (
	"strings"

	"github.com/stepro/kudo/pkg/kubectl"
	"github.com/stepro/kudo/pkg/output"
)

// Start starts synchronizing files
func Start(dir string, sync [][2]string, k *kubectl.CLI, pod string, container string, out *output.Interface) error {
	return start(dir, func(added []string, updated []string, deleted []string) {
		if len(deleted) > 0 {
			execArgs := []string{"exec", pod, "--container", container, "--", "rm", "-rf"}
			for _, path := range deleted {
				for _, rule := range sync {
					if rule[0] == "" || strings.HasPrefix(path, rule[0]+"/") {
						execArgs = append(execArgs, rule[1]+"/"+path)
					}
				}
			}
			if err := k.Run(execArgs...); err != nil {
				out.Debug("failed to synchronize deleted files: %v", err)
			} else {
				for _, path := range deleted {
					out.Debug("deleted %s", path)
				}
			}
		}
		if len(updated) > 0 {
			if err := k.Input(newTarchive(dir, sync, updated...), "exec", pod, "--container", container, "-i", "--", "tar", "-xof", "-", "-C", "/"); err != nil {
				out.Debug("failed to synchronize updated files: %v", err)
			} else {
				for _, path := range updated {
					out.Debug("updated %s", path)
				}
			}
		}
		if len(added) > 0 {
			if err := k.Input(newTarchive(dir, sync, added...), "exec", pod, "--container", container, "-i", "--", "tar", "-xof", "-", "-C", "/"); err != nil {
				out.Debug("failed to synchronize added files: %v", err)
			} else {
				for _, path := range added {
					out.Debug("added %s", path)
				}
			}
		}
	})
}
