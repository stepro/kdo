package server

import (
	"strconv"
	"strings"
	"time"

	"github.com/stepro/kdo/pkg/kubectl"
	"github.com/stepro/kdo/pkg/output"
)

var manifest = `
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: kube-system
  name: kdo-server
  labels:
    component: kdo-server
data:
  docker-daemon.sh: |-
    #!/bin/sh
    apk add --no-cache socat
    exec socat -d tcp4-listen:2375,fork UNIX-CONNECT:/var/run/docker.sock
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  namespace: kube-system
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
        beta.kubernetes.io/os: linux	
      volumes:
      - name: config
        configMap:
          name: kdo-server
          items:
          - key: docker-daemon.sh
            path: docker-daemon.sh
            mode: 0777
      - name: docker-socket
        hostPath:
          path: /var/run/docker.sock
      containers:
      - name: docker-daemon
        image: alpine:3
        volumeMounts:
        - name: config
          subPath: docker-daemon.sh
          mountPath: /docker-daemon.sh
        - name: docker-socket
          mountPath: /var/run/docker.sock
        command:
        - /docker-daemon.sh
        readinessProbe:
          tcpSocket:
            port: 2375
`

// Install installs server components
func Install(k *kubectl.CLI, out *output.Interface) error {
	return out.Do("Installing server components", func(op output.Operation) error {
		op.Progress("applying manifests")
		if err := k.Apply(manifest); err != nil {
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
				if current == desired {
					break
				}
				op.Progress("%d/%d instances are ready", current, desired)
			}
			time.Sleep(1 * time.Second)
		}

		return nil
	})
}

// NodePods first ensures server components are installed
// and then gets a map of nodes to server component pods
func NodePods(k *kubectl.CLI, out *output.Interface) (map[string]string, error) {
	var nodePods map[string]string

	pods, err := k.Lines("--namespace", "kube-system", "get", "pod", "--selector", "component=kdo-server",
		"--output", "go-template={{range .items}}{{.spec.nodeName}} {{.metadata.name}} {{range .status.containerStatuses}}{{.ready}}{{end}}\n{{end}}")
	if err != nil {
		return nil, err
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
func Uninstall(k *kubectl.CLI, out *output.Interface) error {
	return out.Do("Uninstalling server components", func() error {
		return k.Run("--namespace", "kube-system", "delete", "daemonset,configmap", "--selector", "component=kdo-server")
	})
}
