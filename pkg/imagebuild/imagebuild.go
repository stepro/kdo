package imagebuild

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/stepro/kdo/pkg/buildctl"
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
func Build(k kubectl.CLI, pod string, bc buildctl.CLI, d docker.CLI, options *Options, image string, context string, out *output.Interface) error {
	return pkgerror(out.Do("Building image", func(op output.Operation) error {
		op.Progress("determining build node")
		var node string
		for {
			var err error
			node, err = k.String("get", "pod", pod, "--output", `go-template={{.spec.nodeName}}`)
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

		if bc != nil {
			op.Progress("connecting to buildkit daemon")
		} else {
			op.Progress("connecting to docker daemon")
		}
		builderPort, stop, err := portforward.StartOne(k, "kube-system", nodePods[node], "2375")
		if err != nil {
			return err
		}
		defer stop()

		var buildArgs []string
		if bc != nil {
			buildArgs = []string{
				"--addr", "tcp://localhost:" + builderPort,
				"build",
				"--frontend", "dockerfile.v0",
				"--local", "context=" + context,
				"--local", "dockerfile=" + context,
			}
			if options.File == "" {
				buildArgs = append(buildArgs, "--local", "dockerfile="+context)
			} else {
				dir, file := filepath.Split(options.File)
				if dir == "" {
					buildArgs = append(buildArgs, "--local", "dockerfile="+context)
				} else {
					buildArgs = append(buildArgs, "--local", "dockerfile="+dir)
				}
				buildArgs = append(buildArgs, "--opt", "filename="+file)
			}
			for _, arg := range options.Args {
				buildArgs = append(buildArgs, "--opt", "build-arg:"+arg)
			}
			if options.Target != "" {
				buildArgs = append(buildArgs, "--opt", "target="+options.Target)
			}
			buildArgs = append(buildArgs, "--output", "type=image,name="+image+",unpack=true")
		} else /* if d != nil */ {
			buildArgs = []string{
				"--host", "localhost:" + builderPort,
				"build",
			}
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
		}

		op.Progress("running")
		if bc != nil {
			re := regexp.MustCompile(`^#[0-9]+\s(\[.*)$`)
			return bc.EachErrLine(buildArgs, func(line string) {
				if out.Level < output.LevelVerbose {
					if matches := re.FindStringSubmatch(line); len(matches) > 0 {
						op.Progress(matches[1])
					}
				} else {
					out.Verbose("[buildctl] %s", line)
				}
			})
		} else {
			return d.EachLine(buildArgs, func(line string) {
				if out.Level < output.LevelVerbose && (strings.HasPrefix(line, "Sending build context ") || strings.HasPrefix(line, "Step ")) {
					op.Progress("s" + line[1:])
				} else {
					out.Verbose("[docker] %s", line)
				}
			})
		}
	}))
}
