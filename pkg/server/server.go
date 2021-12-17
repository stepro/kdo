package server

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/stepro/kdo/pkg/kubectl"
	"github.com/stepro/kdo/pkg/output"
)

func pkgerror(err error) error {
	if err != nil {
		err = fmt.Errorf("server: %v", err)
	}
	return err
}

const manifest = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: kdo-server
  labels:
    component: kdo-server
data:
  buildkitd.toml: |-
    [worker.containerd]
      namespace = "k8s.io"
  entrypoint.sh: |-
    #!/bin/sh
    if [ -e "/run/containerd/containerd.sock" ]; then
      exec buildkitd --addr tcp://0.0.0.0:2375 \
        --root /var/lib/kdo/buildkit \
        --oci-worker false --containerd-worker true
    elif [ -e "/run/docker.sock" ]; then
      apk add --no-cache socat
      exec socat -d tcp4-listen:2375,fork UNIX-CONNECT:/run/docker.sock
    fi
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kdo-server
  labels:
    component: kdo-server
spec:
  selector:
    matchLabels:
      component: kdo-server
  template:
    metadata:
      labels:
        component: kdo-server
    spec:
      nodeSelector:
        kubernetes.io/os: linux
      volumes:
      - name: host-run-containerd
        hostPath:
          type: DirectoryOrCreate
          path: /run/containerd
      - name: host-run-docker-sock
        hostPath:
          # type: SocketOrCreate
          path: /run/docker.sock
      - name: host-tmp
        hostPath:
          type: Directory
          path: /tmp
      - name: host-var-lib-buildkit
        hostPath:
          type: DirectoryOrCreate
          path: /var/lib/kdo/buildkit
      - name: host-var-lib-containerd
        hostPath:
          type: DirectoryOrCreate
          path: /var/lib/containerd
      - name: host-var-log
        hostPath:
          type: Directory
          path: /var/log
      - name: config
        configMap:
          name: kdo-server
          items:
          - key: buildkitd.toml
            path: buildkitd.toml
            mode: 0644
          - key: entrypoint.sh
            path: entrypoint.sh
            mode: 0777
      containers:
      - name: kdo-server
        image: moby/buildkit
        volumeMounts:
        - name: host-run-containerd
          mountPath: /run/containerd
          mountPropagation: Bidirectional
        - name: host-run-docker-sock
          mountPath: /run/docker.sock
        - name: host-tmp
          mountPath: /tmp
          mountPropagation: Bidirectional
        - name: host-var-lib-buildkit
          mountPath: /var/lib/kdo/buildkit
          mountPropagation: Bidirectional
        - name: host-var-lib-containerd
          mountPath: /var/lib/containerd
          mountPropagation: Bidirectional
        - name: host-var-log
          mountPath: /var/log
          mountPropagation: Bidirectional
        - name: config
          subPath: buildkitd.toml
          mountPath: /etc/buildkit/buildkitd.toml
        - name: config
          subPath: entrypoint.sh
          mountPath: /entrypoint.sh
        securityContext:
          privileged: true
        command:
        - /entrypoint.sh
        readinessProbe:
          tcpSocket:
            port: 2375
`

// Install installs server components
func Install(k kubectl.CLI, out *output.Interface) error {
	return pkgerror(out.Do("Installing server components", func(op output.Operation) error {
		op.Progress("applying manifest")
		if err := k.Input(strings.NewReader(manifest), "--namespace", "kube-system", "apply", "--filename", "-"); err != nil {
			return err
		}

		op.Progress("checking readiness")
		for {
			readiness, err := k.String("--namespace", "kube-system", "get", "daemonset", "kdo-server",
				"--output", "go-template={{.status.numberReady}} {{.status.desiredNumberScheduled}}")
			if err != nil {
				return err
			}
			if readiness != "" {
				tokens := strings.Split(readiness, " ")
				current, err := strconv.Atoi(tokens[0])
				if err != nil {
					return err
				}
				desired, err := strconv.Atoi(tokens[1])
				if err != nil {
					return err
				}
				op.Progress("%d/%d instances are ready", current, desired)
				if current == desired {
					break
				}
			}
			time.Sleep(1 * time.Second)
		}

		return nil
	}))
}

// NodePods first ensures server components are installed
// and then gets a map of nodes to server component pods
func NodePods(k kubectl.CLI, out *output.Interface) (map[string]string, error) {
	var nodePods map[string]string

	pods, err := k.Lines("--namespace", "kube-system", "get", "pod", "--selector", "component=kdo-server",
		"--output", "go-template={{range .items}}{{.spec.nodeName}} {{.metadata.name}} {{range .status.containerStatuses}}{{.ready}}{{end}}\n{{end}}")
	if err != nil {
		return nil, pkgerror(err)
	}

	if len(pods) > 0 {
		nodePods = map[string]string{}
		for _, pod := range pods {
			nodePodReady := strings.Split(pod, " ")
			if nodePodReady[2] == "true" {
				nodePods[nodePodReady[0]] = nodePodReady[1]
			}
		}
	}

	if nodePods == nil || len(nodePods) < len(pods) {
		if err = Install(k, out); err != nil {
			return nil, err
		}
		return NodePods(k, out)
	}

	return nodePods, nil
}

// Uninstall uninstalls server components
func Uninstall(k kubectl.CLI, out *output.Interface) error {
	return pkgerror(out.Do("Uninstalling server components", func() error {
		return k.Run("--namespace", "kube-system", "delete", "daemonset,configmap", "--selector", "component=kdo-server")
	}))
}
