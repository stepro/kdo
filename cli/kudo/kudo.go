package main

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/microsoft/kudo/pkg/docker"
	"github.com/microsoft/kudo/pkg/filesync"
	"github.com/microsoft/kudo/pkg/kubectl"
	"github.com/microsoft/kudo/pkg/output"
	"github.com/microsoft/kudo/pkg/pod"
	"github.com/microsoft/kudo/pkg/server"
	"github.com/spf13/cobra"
)

var cmd = &cobra.Command{
	Short:   "kudo is sudo for Kubernetes",
	Use:     usage,
	Version: "0.1.0",
	Example: examples,
	RunE:    run,
}

var usage = strings.TrimSpace(`
  kudo [flags] image [command] [args...]
  kudo [flags] build-dir [command] [args...]
  kudo --[un]install [-q, --quiet] [-v, --verbose] [--debug]
  kudo --version | --help
`)

var examples = strings.Trim(`
  # Run a command shell in an "alpine" container
  kudo -it alpine

  # Run a DNS lookup in an "alpine" container
  kudo alpine nslookup kubernetes.default.svc.cluster.local

  # Run a Node.js app in a container built from the current directory
  kudo . npm start

  # Run the default command in a container built from the current
  # directory that inherits configuration from the first container
  # defined by the pod template in the "todo-app" deployment spec
  kudo -c deployment/todo-app .

  # Run a command shell in a container built from the current directory
  # that inherits existing configuration from the first container defined
  # by the first pod selected by the "todo-app" service, and also push any
  # changes in the current directory to the container's "/app" directory
  kudo -c service/todo-app -s .:/app -it . sh

  # Debug a Node.js app in a container built from the current directory
  # that inherits existing configuration from the first container defined
  # by the todo-app-56db-xdhfx pod, and forward TCP connections made to
  # local ports 8080 and 9229 to container ports 80 and 9229 respectively
  kudo -c todo-app-56db-xdhfx -p 8080:80 -p 9229:9229 \
    . node --inspect-brk=0.0.0.0:9229 server.js

  # Run the default command in a "kudo-samples/todo-app" container
  # that inherits its configuration from the "web" container defined
  # by the pod template in the "todo-app" deployment spec, and also
  # overlay any existing pods produced by that same deployment
  kudo -c deployment/todo-app:web -R kudo-samples/todo-app
`, "\r\n")

func fatal(err error) {
	fmt.Fprintf(os.Stderr, fmt.Sprintf("Fatal error: %v", err))
	os.Exit(1)
}

var flags struct {
	kubectl struct {
		path string
		kubectl.Options
	}
	install   bool
	uninstall bool
	scope     string
	build     struct {
		docker struct {
			path string
			docker.Options
		}
		file   string
		args   []string
		target string
	}
	inherit     string
	labels      []string
	annotations []string
	noLifecycle bool
	noProbes    bool
	env         []string
	sync        []string
	ports       []string
	listen      []string
	replace     bool
	command     struct {
		stdin bool
		tty   bool
	}
	detach  bool
	delete  bool
	quiet   bool
	verbose bool
	debug   bool
}

var out *output.Interface

func init() {
	// Kubernetes options
	cmd.Flags().StringVar(&flags.kubectl.path,
		"kubectl", "kubectl", "path to the kubectl CLI")
	cmd.Flags().StringVar(&flags.kubectl.Kubeconfig,
		"kubeconfig", "", "path to the kubeconfig file to use")
	cmd.Flags().StringVar(&flags.kubectl.Context,
		"context", "", "the kubeconfig context to use")
	cmd.Flags().StringVarP(&flags.kubectl.Namespace,
		"namespace", "n", "", "the kubernetes namespace to use")
	cmd.Flags().IntVar(&flags.kubectl.Verbosity,
		"kubectl-v", 0, "the kubectl log level verbosity")

	// Installation options
	cmd.Flags().BoolVar(&flags.install,
		"install", false, "install server components and exit")
	cmd.Flags().BoolVar(&flags.uninstall,
		"uninstall", false, "uninstall server components and exit")

	// Scope options
	cmd.Flags().StringVar(&flags.scope,
		"scope", "", "scoping identifier for images and pods")

	// Build options
	cmd.Flags().StringVar(&flags.build.docker.path,
		"docker", "docker", "path to the docker CLI")
	cmd.Flags().StringVar(&flags.build.docker.Config,
		"docker-config", "", "path to the docker CLI config files")
	cmd.Flags().StringVar(&flags.build.docker.LogLevel,
		"docker-log-level", "", "the docker CLI logging level")
	cmd.Flags().StringVarP(&flags.build.file,
		"build-file", "f", "Dockerfile", "dockerfile to build")
	cmd.Flags().StringArrayVar(&flags.build.args,
		"build-arg", nil, "build-time variables")
	cmd.Flags().StringVar(&flags.build.target,
		"build-target", "", "dockerfile target to build")

	// Container options
	cmd.Flags().StringVarP(&flags.inherit,
		"inherit", "c", "", "inherit an existing configuration")
	cmd.Flags().StringArrayVar(&flags.labels,
		"label", nil, "set pod labels (never inherited)")
	cmd.Flags().StringArrayVar(&flags.annotations,
		"annotate", nil, "set pod annotations (never inherited)")
	cmd.Flags().BoolVar(&flags.noLifecycle,
		"no-lifecycle", false, "do not inherit lifecycle configuration")
	cmd.Flags().BoolVar(&flags.noProbes,
		"no-probes", false, "do not inherit probes configuration")
	cmd.Flags().StringArrayVarP(&flags.env,
		"env", "e", nil, "set container environment variables")
	cmd.Flags().StringArrayVarP(&flags.sync,
		"sync", "s", nil, "push local file changes to the container")
	cmd.Flags().StringArrayVarP(&flags.ports,
		"forward", "p", nil, "forward local ports to container ports")
	cmd.Flags().StringArrayVarP(&flags.listen,
		"listen", "l", nil, "forward container ports to local ports")

	// Replacement options
	cmd.Flags().BoolVarP(&flags.replace,
		"replace", "R", false, "overlay inherited configuration's workload")

	// Command options
	cmd.Flags().BoolVarP(&flags.command.stdin,
		"stdin", "i", false, "connect standard input to the container")
	cmd.Flags().BoolVarP(&flags.command.tty,
		"tty", "t", false, "allocate a pseudo-TTY in the container")

	// Detached pod options
	cmd.Flags().BoolVarP(&flags.detach,
		"detach", "d", false, "run pod in the background")
	cmd.Flags().BoolVar(&flags.delete,
		"delete", false, "delete a previously detached pod")

	// Output options
	cmd.Flags().BoolVarP(&flags.quiet,
		"quiet", "q", false, "output no information")
	cmd.Flags().BoolVarP(&flags.verbose,
		"verbose", "v", false, "output more information")
	cmd.Flags().BoolVar(&flags.debug,
		"debug", false, "output debug information")

	// Other options
	cmd.Flags().Bool(
		"version", false, "show version information")
	cmd.Flags().Bool(
		"help", false, "show help information")

	// Once a positional argument is processed, do
	// not process any additional arguments as flags
	cmd.Flags().SetInterspersed(false)

	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	cobra.OnInitialize(func() {
		if flags.scope == "" {
			hostname, err := os.Hostname()
			if err != nil {
				fatal(err)
			}
			flags.scope = hostname
		}

		var level output.Level
		if flags.quiet {
			level = output.LevelQuiet
		} else {
			if flags.verbose {
				level = output.LevelVerbose
			}
			if flags.debug {
				level = output.LevelDebug
			}
		}
		out = output.NewStdInterface(level, nil, os.Stdout, os.Stderr)
	})
}

func run(cmd *cobra.Command, args []string) error {
	var k = kubectl.NewCLI(
		flags.kubectl.path,
		&flags.kubectl.Options,
		out, output.LevelVerbose)

	if flags.install {
		if flags.uninstall || len(args) > 0 {
			cmd.Help()
			return nil
		}
		return server.Install(k, out)
	}

	if flags.uninstall {
		if flags.install || len(args) > 0 {
			cmd.Help()
			return nil
		}
		if err := pod.DeleteAll(k, out); err != nil {
			return err
		}
		return server.Uninstall(k, out)
	}

	if (len(flags.sync) > 0 || len(flags.ports) > 0 || len(flags.listen) > 0) && flags.detach {
		return errors.New("Cannot combine sync, forward or listen flags with detach flag")
	}
	if flags.detach && flags.delete {
		return errors.New("Cannot combine detach and delete flags")
	}

	var dirOrImage string
	if len(args) == 0 {
		cmd.Help()
		return nil
	}
	dirOrImage = args[0]
	args = args[1:]

	hash := fmt.Sprintf("%s\n%s\n%s", flags.scope, dirOrImage, flags.inherit)
	hash = fmt.Sprintf("%x", sha1.Sum([]byte(hash)))[:16]

	if flags.delete {
		return pod.Delete(k, hash, out)
	}

	var dir string
	var image string
	if strings.HasPrefix(dirOrImage, ".") {
		dir = dirOrImage
		image = fmt.Sprintf("kudo-%s:%d", hash, time.Now().UnixNano())
	} else {
		image = dirOrImage
	}

	var build func(dockerPod string, op output.Operation) error
	if dir != "" {
		build = func(dockerPod string, op output.Operation) error {
			op.Progress("connecting to docker daemon")
			var dockerPort string
			portForwarded := make(chan string)
			portForwardEnded := make(chan error)
			stop := k.StartLines([]string{"port-forward", "-n", "kube-system", dockerPod, ":2375"}, func(line string) {
				if dockerPort == "" && strings.HasPrefix(line, "Forwarding from 127.0.0.1:") {
					line = line[len("Forwarding from 127.0.0.1:"):]
					tokens := strings.Split(line, " -> ")
					portForwarded <- tokens[0]
				}
			}, portForwardEnded)
			select {
			case err := <-portForwardEnded:
				return err
			case dockerPort = <-portForwarded:
			}
			defer stop()

			d := docker.NewCLI(
				flags.build.docker.path,
				&flags.build.docker.Options,
				out, output.LevelVerbose)

			buildArgs := []string{"--host", "localhost:" + dockerPort, "build"}
			if flags.build.file != "" {
				buildArgs = append(buildArgs, "--file", flags.build.file)
			}
			for _, arg := range flags.build.args {
				buildArgs = append(buildArgs, "--build-arg", arg)
			}
			if flags.build.target != "" {
				buildArgs = append(buildArgs, "--target", flags.build.target)
			}
			buildArgs = append(buildArgs, "--tag", image, dir)

			op.Progress("running")
			return d.EachLine(buildArgs, func(line string) {
				if out.Level < output.LevelVerbose && (strings.HasPrefix(line, "Sending build context ") || strings.HasPrefix(line, "Step ")) {
					op.Progress("s" + line[1:])
				} else {
					out.Verbose("[docker] %s", line)
				}
			})
		}
	}

	p, err := pod.Apply(k, hash, build, &pod.Settings{
		Inherit: flags.inherit,
		Image:   image,
		Env:     flags.env,
		Listen:  len(flags.listen) > 0,
		Stdin:   flags.command.stdin,
		TTY:     flags.command.tty,
		Command: args,
	}, out)
	if err != nil {
		return err
	}

	if flags.detach {
		return nil
	}

	defer func() {
		pod.Delete(k, hash, out)
	}()

	if dir != "" && len(flags.sync) > 0 {
		// TODO
		if err = filesync.Watch(dir, func(added []string, updated []string, deleted []string) {
			for _, path := range added {
				out.Debug("added %s", path)
			}
			for _, path := range updated {
				out.Debug("updated %s", path)
			}
			for _, path := range deleted {
				out.Debug("deleted %s", path)
			}
		}); err != nil {
			return err
		}
	}

	if len(flags.ports) > 0 {
		op := out.Start("Forwarding ports")
		var hasForwardedPorts bool
		portsForwarded := make(chan bool)
		portForwardEnded := make(chan error)
		stop := k.StartLines(append([]string{"port-forward", p.Pod}, flags.ports...), func(line string) {
			if !hasForwardedPorts && strings.HasPrefix(line, "Forwarding from 127.0.0.1:") {
				hasForwardedPorts = true
				portsForwarded <- true
			}
		}, portForwardEnded)
		select {
		case err := <-portForwardEnded:
			op.Failed()
			return err
		case <-portsForwarded:
			op.Done()
		}
		defer stop()
	}

	if len(flags.listen) > 0 {
		// TODO
	}

	if flags.command.stdin && !p.Exited() {
		if err = k.Exec("logs", p.Pod, "--container", p.Container); err != nil {
			return err
		}
		attachArgs := []string{"attach", p.Pod, "--container", p.Container, "--stdin"}
		if flags.command.tty {
			attachArgs = append(attachArgs, "--tty")
		}
		return k.Exec(attachArgs...)
	}

	if err = k.Exec("logs", "--follow", p.Pod, "--container", p.Container); err != nil {
		return err
	} else if code, err := p.ExitCode(); err != nil {
		return err
	} else {
		out.Info("Pod container process exited with code %d", code)
	}

	return nil
}

func main() {
	err := cmd.Execute()
	if out != nil {
		out.Close()
	}
	if err != nil {
		exitCode := 1
		if exitErr, ok := err.(*exec.ExitError); !ok {
			fmt.Fprintf(os.Stderr, "Error: %s\n", strings.TrimRight(err.Error(), "\r\n"))
		} else {
			exitCode = exitErr.ExitCode()
		}
		os.Exit(exitCode)
	}
}
