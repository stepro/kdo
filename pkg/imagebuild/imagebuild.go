package imagebuild

import (
	"fmt"
	"strings"
	"time"

	"github.com/stepro/kdo/pkg/docker"
	"github.com/stepro/kdo/pkg/kubectl"
	"github.com/stepro/kdo/pkg/output"
	"github.com/stepro/kdo/pkg/portforward"
	"github.com/stepro/kdo/pkg/server"
)

func pkgerror(err error) error {
	if err != nil {
		err = fmt.Errorf("imagebuild: %v", err)
	}
	return err
}

// Options represents image build options
type Options struct {
	File   string
	Args   []string
	Target string
}

// Build builds an image on the node that is running a pod
func Build(k kubectl.CLI, pod string, d docker.CLI, options *Options, image string, context string, out *output.Interface) error {
	return pkgerror(out.Do("Building image", func(op output.Operation) error {
		op.Progress("determining build node")
		var node string
		for {
			node, err := k.String("get", "pod", pod, "--output", `go-template={{.spec.nodeName}}`)
			if err != nil {
				return err
			} else if node != "" {
				break
			}
			time.Sleep(1 * time.Second)
		}

		op.Progress("determining build pod")
		nodePods, err := server.NodePods(k, out)
		if err != nil {
			return err
		}
		if nodePods[node] == "" {
			return fmt.Errorf("cannot build on node %s", node)
		}

		op.Progress("connecting to docker daemon")
		dockerPort, stop, err := portforward.StartOne(k, "kube-system", nodePods[node], "2375")
		if err != nil {
			return err
		}
		defer stop()

		buildArgs := []string{"--host", "localhost:" + dockerPort, "build"}
		if options.File != "" {
			buildArgs = append(buildArgs, "--file", options.File)
		}
		for _, arg := range options.Args {
			buildArgs = append(buildArgs, "--build-arg", arg)
		}
		if options.Target != "" {
			buildArgs = append(buildArgs, "--target", options.Target)
		}
		buildArgs = append(buildArgs, "--tag", image, context)

		op.Progress("running")
		return d.EachLine(buildArgs, func(line string) {
			if out.Level < output.LevelVerbose && (strings.HasPrefix(line, "Sending build context ") || strings.HasPrefix(line, "Step ")) {
				op.Progress("s" + line[1:])
			} else {
				out.Verbose("[docker] %s", line)
			}
		})
	}))
}
