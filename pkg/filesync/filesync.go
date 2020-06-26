package filesync

import (
	"fmt"
	"strings"

	"github.com/stepro/kdo/pkg/kubectl"
	"github.com/stepro/kdo/pkg/output"
)

func pkgerror(err error) error {
	if err != nil {
		err = fmt.Errorf("filesync: %v", err)
	}
	return err
}

// Rule represents a file synchronization rule
type Rule struct {
	LocalPath  string
	RemotePath string
}

// Start starts synchronizing files from a directory to a container in a pod
func Start(dir string, sync []Rule, k kubectl.CLI, pod string, container string, out *output.Interface) error {
	return pkgerror(start(dir, func(added []string, updated []string, deleted []string) {
		if len(deleted) > 0 {
			execArgs := []string{"exec", pod, "--container", container, "--", "rm", "-rf"}
			for _, path := range deleted {
				for _, rule := range sync {
					if rule.LocalPath == "" || strings.HasPrefix(path, rule.LocalPath+"/") {
						execArgs = append(execArgs, rule.RemotePath+"/"+path)
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
	}))
}
