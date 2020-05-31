# <img src="images/logo.png" width="24"> Kudo: sudo for Kubernetes
[![Build](https://img.shields.io/github/workflow/status/stepro/kudo/Kudo)](https://github.com/stepro/kudo/actions?query=workflow%3AKudo)
[![Feature Requests](https://img.shields.io/github/issues/stepro/kudo/feature-request.svg)](https://github.com/stepro/kudo/issues?q=is%3Aopen+is%3Aissue+label%3Afeature-request+sort%3Areactions-%2B1-desc)
[![Bugs](https://img.shields.io/github/issues/stepro/kudo/bug.svg)](https://github.com/stepro/kudo/issues?q=is%3Aopen+is%3Aissue+label%3Abug)

Kudo is a command line tool that executes commands using the context of a new or existing workload in a Kubernetes cluster. It is modeled after the `sudo` command available on most *nix systems, which executes commands using the context of another user.

Kudo is designed primarily for development scenarios where you want to observe how a single command runs inside an image (existing or built on the fly using a Dockerfile) that optionally inherits configuration settings from an existing pod specification.

Kudo can also be used for longer-running connected development sessions where local file updates are pushed into the running container, enabling rapid iteration on code while continuing to run as a properly configured container in the Kubernetes cluster.

## Prerequisites and Installation

Kudo requires the `kubectl` CLI to communicate with a Kubernetes cluster and the `docker` CLI to perform dynamic image builds, so first make sure you have these installed and available in your PATH. Then, download the latest [release](https://github.com/stepro/kudo/releases) for your platform and add the `kudo` binary to your PATH.

By default `kudo` utilizes the current `kubectl` context, so point it at the Kubernetes cluster of your choice and you're good to go!

## Examples

Run a command shell in an `alpine` container:

```
kudo -it alpine
```

Run a DNS lookup in an `alpine` container:

```
kudo -it alpine nslookup kubernetes.default.svc.cluster.local
```

Run a Node.js app in a container built from the current directory:

```
kudo . npm start
```

Run the default command in a container built from the current directory that inherits configuration from the first container defined by the pod template in the `todo-app` deployment spec:

```
kudo -c deployment/todo-app .
```

Run a command shell in a container built from the current directory that inherits existing configuration from the first container defined by the first pod selected by the `todo-app` service, and also push any changes in the current directory to the container's `/app` directory:

```
kudo -c service/todo-app -s .:/app -it . sh
```

Debug a Node.js app in a container built from the current directory that inherits existing configuration from the first container defined by the `todo-app-56db-xdhfx` pod, and forward TCP connections made to local ports `8080` and `9229` to container ports `80` and `9229` respectively:

```
kudo -c todo-app-56db-xdhfx -p 8080:80 -p 9229:9229 . node --inspect-brk=0.0.0.0:9229 server.js
```

Run the default command in a `kudo-samples/todo-app` container that inherits its configuration from the `web` container defined by the pod template in the `todo-app` deployment spec, and also overlay any existing pods produced by that same deployment:

```
kudo -c deployment/todo-app:web -R kudo-samples/todo-app
```

## Usage

Kudo is a single command CLI that can be called in a small number of unique ways:

```
kudo [flags] image [command] [args...]
kudo [flags] build-dir [command] [args...]
kudo --[un]install [-q, --quiet] [-v, --verbose] [--debug]
kudo --version | --help
```

When called with an `image` parameter, this represents an existing image to be run in the Kubernetes cluster. This is distinguished from the `build-dir` parameter, which always starts with `.` and identifies a local Docker build context to be dynamically built into a custom image to run in the Kubernetes cluster.

When the `command` parameter is set, this configures the `command` property in the container and removes the `args` property.

When called with the `--install` or `--uninstall` flag, all other flags with the exception of those listed above are ignored and no positional parameters are allowed.

## Flags

Kudo can be customized in a variety of ways through a set of command line flags.

### Kubernetes flags

These flags customize how the `kubectl` CLI is used.

Flag | Default | Description
---- | ------- | -----------
`--kubectl` | `kubectl` | path to the kubectl CLI
`--kubeconfig` | `<empty>` | path to the kubeconfig file to use
`--context` | `<empty>` | the kubeconfig context to use
`-n, --namespace` | `<empty>` | the kubernetes namespace to use
`--kubectl-v` | `0` | the kubectl log level verbosity

### Installation flags

These flags are used to manage the kudo server components. These components are installed into the `kube-system` namespace as a daemon set, so using these flags requires administrative access to the cluster.

Flag | Description
---- | -----------
`--install` | install server components and exit
`--uninstall` | uninstall server components and exit

Normally the server components are installed automatically as needed, but this is not possible if the user does not have permission to install into the `kube-system` namespace. In that case, an alternative administrative user can use the `--install` flag to manually configure the cluster for other users.

The `--uninstall` flag can be used to explicitly remove any leftover kudo pods across all namespaces in addition to the server components from a cluster.

### Scope flag

The scope flag (`--scope`) can be used to change how kudo pods are uniquely named. By default, the local machine's hostname is used.

The scope value is combined with the `image` or `build-dir` parameter and any value of the `--inherit` flag, then SHA-1 hashed to produce the final pod identifier.

### Build flags

These flags customize how the `docker` CLI is used when building images.

Flag | Default | Description
---- | ------- | -----------
`--docker` | `docker` | path to the docker CLI
`--docker-config` | `<empty>` | path to the docker CLI config files
`--docker-log-level` | `<empty>` | the docker CLI logging level
`-f, --build-file` | `<build-dir>/Dockerfile` | dockerfile to build
`--build-arg` | `[]` | build-time variables in the form `name=value`
`--build-target` | `<empty>` | dockerfile target to build

When the `docker` CLI is invoked, it does not use the default configured Docker daemon. Instead, it uses the kudo server components to directly access the Docker daemon running on a node in the Kubernetes cluster. Therefore, it is theoretically not a requirement that the local machine is actually running Docker, although in most cases (e.g. Docker Desktop) this will be the case. It **is**, however, a requirement that the node on which the kudo pod is scheduled is using Docker for its container runtime and the Docker daemon socket at `/var/run/docker.sock` on the host can be volume mounted into the pod.

### Configuration flags

These flags customize the pod and container that runs the command.

Flag | Default | Description
---- | ------- | -----------
`-c, --inherit` | `<none>` | inherit an existing configuration
`--label` | `[]` | set pod labels in the form `name=value`
`--annotate` | `[]` | set pod annotations in the form `name=value`
`--no-lifecycle` | `false` | do not inherit lifecycle configuration
`--no-probes` | `false` | do not inherit probes configuration
`-e, --env` | `[]` | set container environment variables in the form `name=value`
`-R, --replace` | `false` | overlay inherited configuration's workload

The `-c, --inherit` flag inherits an existing configuration from a container specification identified in the form `[kind/]name[:container]`, where `kind` is a Kubernetes workload kind (`cronjob`, `daemonset`, `deployment`, `job`, `pod`, `replicaset`, `replicationcontroller` or `statefulset`) or `service` (default is `pod`). If the `kind` is not `pod`, the pod spec is based on the template in the outer workload spec, except in the case of `service`, when it is based on the workload that originally generated the first pod selected by the service. If `container` is not specified, the first container in the pod spec is selected. Init containers are not supported.

Note that even when inheriting an existing configuration, pod labels and annotations are *not* inherited to prevent the cluster from misunderstanding the role of the pod (for instance, automatically being added as an instance behind a service). The `--label` and `--annotate` flags can be used to re-add any labels and annotations that must be included for the pod to function properly.

When inheriting an existing configuration, there are cases when the existing pod lifecycle and probe configuration are not implemented, would cause problems, or are entirely irrelevant for the scenario. The `--no-lifecyle` and `--no-probes` flags can be used to ensure these properties are not inherited.

The `-e, --env` flags set environment variables, and in the case of an inherited configuration, override any inherited environment variables.

The `-R, --replace` flag only applies when the inherited configuration is from the `deployment`, `replicaset`, `replicationcontroller` and `statefulset` workload kinds, or from the `service` kind. For workloads, this flag scales the workload instance to zero for the duration of the command. For services, this flag changes the pod selector to select the kudo pod for the duration of the command.

### Session flags

These flags customize behavior that applies for the duration of the kudo process.

Flag | Default | Description
---- | ------- | -----------
`-s, --sync` | `[]` | push local file changes to the container in the form `localdir:remotedir`
`-p, --forward` | `[]` | forward local ports to container ports in the form `local:remote`
`-l, --listen` | `[]` | forward container ports to local ports in the form `remote:local`

The `-s, --sync` flag is only valid when using the `build-dir` parameter. It enables synchronization of changes to files in the local build context into an appropriate location in the container. For example, if the `build-dir` is `.`, then `--sync .:/app` maps the entire build context to the `/app` directory in the container. More complex usage can map individual directories, as in `./src:/app/src`.

The `-p, --forward` flag enables the local machine to access specific container ports, for example, `--forward 8080:80` will forward local port `8080` to container port `80`.

The `-l, --listen` flag enables code running in the container to access specific localhost ports that are forwarded back to the local machine. This can be used to replace external dependencies, such as data stores, used by the code running in the container, with an alternate endpoint on the local machine. For instance:

```
# Start a local Mongo database that can be accessed at localhost:27017
docker run -p 27017:27017 -d mongo:4

# Build and run a web server image in Kubernetes, forwarding local port
# 8080 to container port 80, and when the web server code connects to a
# Mongo database using the MONGO_CONNECTION_STRING environment variable,
# proxy the connection back to local port 27017.
kudo -p 8080:80 -e MONGO_CONNECTION_STRING=localhost:27017 -l 27017:27017 .
```

The `-s, --sync`, `-p, --forward` and `-l, --listen` flags cannot be combined with the `-d, --detach` flag.

### Command flags

These flags customize how the command is run.

Flag | Default | Description
---- | ------- | -----------
`-x, --exec` | `false` | execute command in an existing pod
`-i, --stdin` | `false` | connect standard input to the container
`-t, --tty` | `false` | allocate a pseudo-TTY in the container

When using the `-x, --exec` flag, build, configuration and session flags are ignored with the exception of the `-c, --inherit` flag which is used to help identify the target container. Additionally, this flag cannot be combined with the `-d, --detach` or `--delete` flags.

### Detached pod flags

These flags relate to running a pod in the background.

Flag | Default | Description
---- | ------- | -----------
`-d, --detach` | `false` | run pod in the background
`--delete` | `false` | delete a previously detached pod

These flags cannot be combined.

### Output flags

These flags customize how kudo outputs information.

Flag | Default | Description
---- | ------- | -----------
`-q, --quiet` | `false` | output no information
`-v, --verbose` | `false` | output more information
`--debug` | `false` | output debug information

If multiple of these flags are specified, the `-q, --quiet` takes highest precedence, followed by the `--debug` and `-v, --verbose` flags in that order.

### Other flags

Flag | Default | Description
---- | ------- | -----------
`--version` | `false` | show version information
`--help` | `false` | show help information

## License

Kudo is licensed under the [MIT](LICENSE) license.
