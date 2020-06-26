package pod

import (
	"github.com/stepro/kdo/pkg/kubectl"
	"github.com/stepro/kdo/pkg/output"
)

// Delete deletes the pod associated with a hash, if any
func Delete(k kubectl.CLI, hash string, out *output.Interface) error {
	return pkgerror(out.Do("Deleting pod", func(op output.Operation) error {
		name := Name(hash)

		stop := track(k, name, op)
		defer stop()

		return k.Run("delete", "pod", name, "--ignore-not-found", "--wait=false")
	}))
}

// DeleteAll deletes all pods associated with hashes
func DeleteAll(k kubectl.CLI, allNamespaces bool, out *output.Interface) error {
	return pkgerror(out.Do("Deleting all kdo pods", func() error {
		args := []string{"delete", "pod", "-l", "kdo-pod=1"}
		if allNamespaces {
			args = append(args, "--all-namespaces")
		}
		return k.Run(args...)
	}))
}
