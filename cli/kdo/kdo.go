package main

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/stepro/kdo/pkg/docker"
	"github.com/stepro/kdo/pkg/filesync"
	"github.com/stepro/kdo/pkg/imagebuild"
	"github.com/stepro/kdo/pkg/kubectl"
	"github.com/stepro/kdo/pkg/output"
	"github.com/stepro/kdo/pkg/pod"
	"github.com/stepro/kdo/pkg/portforward"
	"github.com/stepro/kdo/pkg/server"
)

var cmd = &cobra.Command{
	Short:   "Kdo: deployless development on Kubernetes",
	Use:     usage,
	Version: "0.6.0",
	Example: examples,
	RunE:    run,
}

var usage = strings.TrimSpace(`
  kdo [flags] image [command] [args...]
  kdo [flags] build-dir [command] [args...]
  kdo --[un]install [-q, --quiet] [-v, --verbose] [--debug]
  kdo --version | --help
`)

var examples = strings.Trim(`
  # Run a command shell in an "alpine" container
  kdo -it alpine

  # Run a DNS lookup in an "alpine" container
  kdo alpine nslookup kubernetes.default.svc.cluster.local

  # Run a Node.js app in a container built from the current directory
  kdo . npm start

  # Run the default command in a container built from the current
  # directory that inherits configuration from the first container
  # defined by the pod template in the "todo-app" deployment spec
  kdo -c deployment/todo-app .

  # Run a command shell in a container built from the current directory
  # that inherits existing configuration from the first container defined
  # by the first pod selected by the "todo-app" service, and also push any
  # changes in the current directory to the container's "/app" directory
  kdo -c service/todo-app -s .:/app -it . sh

  # Debug a Node.js app in a container built from the current directory
  # that inherits existing configuration from the first container defined
  # by the todo-app-56db-xdhfx pod, and forward TCP connections made to
  # local ports 8080 and 9229 to container ports 80 and 9229 respectively
  kdo -c todo-app-56db-xdhfx -p 8080:80 -p 9229:9229 \
    . node --inspect-brk=0.0.0.0:9229 server.js

  # Run the default command in a "kdo-samples/todo-app" container
  # that inherits its configuration from the "web" container defined
  # by the pod template in the "todo-app" deployment spec, and also
  # overlay any existing pods produced by that same deployment
  kdo -c deployment/todo-app:web -R kdo-samples/todo-app
`, "\r\n")

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
		imagebuild.Options
	}
	config struct {
		inherit            string
		inheritLabels      bool
		inheritAnnotations bool
		labels             []string
		annotations        []string
		env                []string
		noLifecycle        bool
		noProbes           bool
	}
	replace bool
	session struct {
		sync    []string
		forward []string
		listen  []string
	}
	command struct {
		exec    bool
		prekill []string
		stdin   bool
		tty     bool
	}
	detach    bool
	delete    bool
	deleteAll bool
	output    struct {
		quiet   bool
		verbose bool
		debug   bool
	}
}

var out *output.Interface

func fatal(err error) {
	fmt.Fprintf(os.Stderr, fmt.Sprintf("Fatal error: %v", err))
	os.Exit(1)
}

func init() {
	// Kubernetes flags
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

	// Installation flags
	cmd.Flags().BoolVar(&flags.install,
		"install", false, "install server components and exit")
	cmd.Flags().BoolVar(&flags.uninstall,
		"uninstall", false, "uninstall server components and exit")

	// Scope flag
	cmd.Flags().StringVar(&flags.scope,
		"scope", "", "scoping identifier for cluster resources")

	// Build flags
	cmd.Flags().StringVar(&flags.build.docker.path,
		"docker", "docker", "path to the docker CLI")
	cmd.Flags().StringVar(&flags.build.docker.Config,
		"docker-config", "", "path to the docker CLI config files")
	cmd.Flags().StringVar(&flags.build.docker.LogLevel,
		"docker-log-level", "", "the docker CLI logging level")
	cmd.Flags().StringVarP(&flags.build.File,
		"build-file", "f", "Dockerfile", "dockerfile to build")
	cmd.Flags().StringArrayVar(&flags.build.Args,
		"build-arg", nil, "build-time variables")
	cmd.Flags().StringVar(&flags.build.Target,
		"build-target", "", "dockerfile target to build")

	// Configuration flags
	cmd.Flags().StringVarP(&flags.config.inherit,
		"inherit", "c", "", "inherit an existing configuration")
	cmd.Flags().BoolVarP(&flags.config.inheritLabels,
		"inherit-labels", "L", false, "inherit pod labels")
	cmd.Flags().BoolVarP(&flags.config.inheritAnnotations,
		"inherit-annotations", "A", false, "inherit pod annotations")
	cmd.Flags().StringArrayVar(&flags.config.labels,
		"label", nil, "inherit, set or remove pod labels")
	cmd.Flags().StringArrayVar(&flags.config.annotations,
		"annotate", nil, "inherit, set or remove pod annotations")
	cmd.Flags().StringArrayVarP(&flags.config.env,
		"env", "e", nil, "set container environment variables")
	cmd.Flags().BoolVar(&flags.config.noLifecycle,
		"no-lifecycle", false, "do not inherit container lifecycle")
	cmd.Flags().BoolVar(&flags.config.noProbes,
		"no-probes", false, "do not inherit container probes")

	// Replace flag
	cmd.Flags().BoolVarP(&flags.replace,
		"replace", "R", false, "overlay inherited configuration's workload")

	// Session flags
	cmd.Flags().StringArrayVarP(&flags.session.sync,
		"sync", "s", nil, "push local file changes to the container")
	cmd.Flags().StringArrayVarP(&flags.session.forward,
		"forward", "p", nil, "forward local ports to container ports")
	cmd.Flags().StringArrayVarP(&flags.session.listen,
		"listen", "l", nil, "forward container ports to local ports")

	// Command flags
	cmd.Flags().BoolVarP(&flags.command.exec,
		"exec", "x", false, "execute command in an existing container")
	cmd.Flags().StringArrayVarP(&flags.command.prekill,
		"prekill", "k", nil, "kill existing processes prior to an exec")
	cmd.Flags().BoolVarP(&flags.command.stdin,
		"stdin", "i", false, "connect standard input to the command")
	cmd.Flags().BoolVarP(&flags.command.tty,
		"tty", "t", false, "allocate a pseudo-TTY for the command")

	// Detach flags
	cmd.Flags().BoolVarP(&flags.detach,
		"detach", "d", false, "run pod in the background")
	cmd.Flags().BoolVar(&flags.delete,
		"delete", false, "delete a previously detached pod")
	cmd.Flags().BoolVar(&flags.deleteAll,
		"delete-all", false, "delete all previously detached pods")

	// Output flags
	cmd.Flags().BoolVarP(&flags.output.quiet,
		"quiet", "q", false, "output no information")
	cmd.Flags().BoolVarP(&flags.output.verbose,
		"verbose", "v", false, "output more information")
	cmd.Flags().BoolVar(&flags.output.debug,
		"debug", false, "output debug information")

	// Other flags
	cmd.Flags().Bool(
		"version", false, "show version information")
	cmd.Flags().Bool(
		"help", false, "show help information")

	// Once a positional argument is processed, do
	// not process any additional arguments as flags
	cmd.Flags().SetInterspersed(false)

	cobra.OnInitialize(func() {
		if flags.scope == "" {
			hostname, err := os.Hostname()
			if err != nil {
				fatal(err)
			}
			flags.scope = hostname
		}

		var level output.Level
		if flags.output.quiet {
			level = output.LevelQuiet
		} else {
			if flags.output.verbose {
				level = output.LevelVerbose
			}
			if flags.output.debug {
				level = output.LevelDebug
			}
		}
		out = output.NewStdInterface(level, nil, os.Stdout, os.Stderr)
	})

	// Do not show usage if there is an error
	cmd.SilenceUsage = true

	// Do not show errors in the default manner
	cmd.SilenceErrors = true
}

func parseInherit(flag string) (kind, name, container string, err error) {
	kindName := strings.SplitN(flag, "/", 2)
	if len(kindName) == 1 {
		kind = "pod"
		name = kindName[0]
	} else {
		kind = kindName[0]
		kind = strings.ToLower(kind)
		switch kind {
		default:
			err = fmt.Errorf(`unknown kind "%s"`, kindName[0])
			return
		case "cj", "cronjob", "cronjobs":
			kind = "cronjob"
		case "ds", "daemonset", "daemonsets":
			kind = "daemonset"
		case "deploy", "deployment", "deployments":
			kind = "deployment"
		case "job", "jobs":
			kind = "job"
		case "po", "pod", "pods":
			kind = "pod"
		case "rs", "replicaset", "replicasets":
			kind = "replicaset"
		case "rc", "replicationcontroller", "replicationcontrollers":
			kind = "replicationcontroller"
		case "svc", "service", "services":
			kind = "service"
		case "sts", "statefulset", "statefulsets":
			kind = "statefulset"
		}
		name = kindName[1]
	}

	nameContainer := strings.SplitN(name, ":", 2)
	if len(nameContainer) == 2 {
		name = nameContainer[0]
		container = nameContainer[1]
	}

	return
}

func parseKeyValues(flags []string) map[string]*string {
	keyValues := map[string]*string{}

	for _, flag := range flags {
		keyValue := strings.SplitN(flag, "=", 2)
		if len(keyValue) == 1 {
			keyValues[keyValue[0]] = nil
		} else {
			keyValues[keyValue[0]] = &keyValue[1]
		}
	}

	return keyValues
}

func parseSync(flags []string) ([]filesync.Rule, error) {
	var rules []filesync.Rule

	for _, rule := range flags {
		localRemote := strings.SplitN(rule, ":", 2)
		if len(localRemote) == 1 {
			localRemote = []string{"", localRemote[0]}
		}
		if filepath.IsAbs(localRemote[0]) {
			return nil, fmt.Errorf(`invalid sync rule "%s": local path must be relative to the build context`, rule)
		} else if !path.IsAbs(localRemote[1]) {
			return nil, fmt.Errorf(`invalid sync rule "%s": remote path must be absolute`, rule)
		}
		if localRemote[0] == "." {
			localRemote[0] = ""
		}
		localRemote[0] = filepath.ToSlash(localRemote[0])
		localRemote[0] = strings.TrimSuffix(localRemote[0], "/")
		localRemote[1] = strings.TrimSuffix(localRemote[1], "/")
		rules = append(rules, filesync.Rule{
			LocalPath:  localRemote[0],
			RemotePath: localRemote[1],
		})
	}

	return rules, nil
}

var exitCode int

func run(cmd *cobra.Command, args []string) error {
	var k = kubectl.NewCLI(
		flags.kubectl.path,
		&flags.kubectl.Options,
		out, output.LevelVerbose)

	if flags.install {
		if flags.uninstall {
			return errors.New("cannot specify --uninstall flag with --install flag")
		}
		if len(args) > 0 {
			return errors.New("cannot specify command or arguments with --install flag")
		}
		return server.Install(k, out)
	}

	if flags.uninstall {
		if len(args) > 0 {
			return errors.New("cannot specify command or arguments with --uninstall flag")
		}
		if err := pod.DeleteAll(k, true, out); err != nil {
			return err
		}
		return server.Uninstall(k, out)
	}

	if flags.config.inherit == "" && flags.replace {
		return errors.New("cannot specify -R,--replace flag without -c,--inherit flag")
	}
	if len(flags.session.sync) > 0 && (len(args) == 0 || !strings.HasPrefix(args[0], ".")) {
		return errors.New("cannot specify -s,--sync flag without build-dir argument")
	}
	if len(flags.session.sync) > 0 || len(flags.session.forward) > 0 || len(flags.session.listen) > 0 {
		if flags.detach {
			return errors.New("cannot combine -s,--sync, -p,--forward or -l,--listen flags with -d,--detach flag")
		}
	}
	if !flags.command.exec && len(flags.command.prekill) > 0 {
		return errors.New("cannot specify -k,--prekill flag without -x,--exec flag")
	}
	if flags.command.exec && (len(flags.session.sync) > 0 || len(flags.session.listen) > 0) {
		return errors.New("cannot combine -s,--sync or -l,--listen flags with -x,--exec flag")
	}
	if flags.command.exec && flags.detach {
		return errors.New("cannot combine -x,--exec and -d,--detach flags")
	}
	if flags.command.exec && (flags.delete || flags.deleteAll) {
		return errors.New("cannot combine -x,--exec and --delete[-all] flags")
	}
	if flags.detach && (flags.delete || flags.deleteAll) {
		return errors.New("cannot combine -d,--detach and --delete[-all] flags")
	}
	if flags.delete && len(args) > 1 {
		return errors.New("cannot specify command or arguments with --delete flag")
	}
	if flags.deleteAll && len(args) > 0 {
		return errors.New("cannot specify any arguments with --delete-all flag")
	}

	if flags.deleteAll {
		return pod.DeleteAll(k, false, out)
	}

	if len(args) == 0 {
		cmd.Help()
		return nil
	}

	var image string
	var buildDir string
	var hash string
	var err error
	if !strings.HasPrefix(args[0], ".") {
		image = args[0]
		hash = image
	} else {
		if buildDir, err = filepath.Abs(args[0]); err != nil {
			return err
		}
		hash = buildDir
	}
	hash = fmt.Sprintf("%s\n%s\n%s", flags.scope, hash, flags.config.inherit)
	hash = fmt.Sprintf("%x", sha1.Sum([]byte(hash)))[:16]
	if buildDir != "" {
		image = fmt.Sprintf("kdo-%s:%d", hash, time.Now().UnixNano())
	}
	command := args[1:]

	if flags.delete {
		return pod.Delete(k, hash, out)
	}

	var inheritKind string
	var inheritName string
	var container string
	if flags.config.inherit != "" {
		if inheritKind, inheritName, container, err = parseInherit(flags.config.inherit); err != nil {
			return err
		}
	}

	if flags.command.exec {
		return pod.Exec(k, hash, container, flags.command.prekill, flags.session.forward, flags.command.stdin, flags.command.tty, command...)
	}

	var build func(pod string) error
	if buildDir != "" {
		build = func(pod string) error {
			d := docker.NewCLI(
				flags.build.docker.path,
				&flags.build.docker.Options,
				out, output.LevelVerbose)
			return imagebuild.Build(k, pod, d, &flags.build.Options, image, buildDir, out)
		}
	}

	// var selector string
	// if config.InheritKind == "service" && config.Replace {
	// 	op.Progress("determining pod selector")
	// 	nameValues, err := k.Lines("get", "service", config.InheritName, "-o", "go-template={{range $k, $v := .spec.selector}}{{$k}}={{$v}}\n{{end}}")
	// 	if err != nil {
	// 		return err
	// 	}
	// 	selector = strings.Join(nameValues, ",")
	// }

	syncRules, err := parseSync(flags.session.sync)
	if err != nil {
		return err
	}

	p, err := pod.Apply(k, hash, &pod.Config{
		InheritKind:        inheritKind,
		InheritName:        inheritName,
		InheritLabels:      flags.config.inheritLabels,
		InheritAnnotations: flags.config.inheritAnnotations,
		Labels:             parseKeyValues(flags.config.labels),
		Annotations:        parseKeyValues(flags.config.annotations),
		Container:          container,
		Image:              image,
		Env:                parseKeyValues(flags.config.env),
		NoLifecycle:        flags.config.noLifecycle,
		NoProbes:           flags.config.noProbes,
		Replace:            flags.replace,
		Stdin:              flags.command.stdin,
		TTY:                flags.command.tty,
		Command:            command,
		Detach:             flags.detach,
	}, build, out)
	if err != nil {
		return err
	}

	if flags.detach {
		return nil
	}

	defer pod.Delete(k, hash, out)

	if len(syncRules) > 0 {
		if err = filesync.Start(buildDir, syncRules, k, p.Pod, p.Container, out); err != nil {
			return err
		}
	}

	if len(flags.session.forward) > 0 {
		op := out.Start("Forwarding ports")
		stop, err := portforward.Start(k, p.Pod, flags.session.forward)
		if err != nil {
			op.Failed()
			return err
		}
		op.Done()
		defer stop()
	}

	if len(flags.session.listen) > 0 {
		// TODO
	}

	var cmdArgs []string
	if flags.command.stdin && !p.Exited() {
		if err = k.Exec("logs", p.Pod, "--container", p.Container); err != nil {
			return err
		}
		cmdArgs = []string{"attach", p.Pod, "--container", p.Container, "--stdin"}
		if flags.command.tty {
			cmdArgs = append(cmdArgs, "--tty")
		}
	} else {
		cmdArgs = []string{"logs", "--follow", p.Pod, "--container", p.Container}
	}

	if err = k.Exec(cmdArgs...); err != nil {
		return err
	} else if exitCode, err = p.ExitCode(); err != nil {
		return err
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
	} else if exitCode != 0 {
		os.Exit(exitCode)
	}
}
