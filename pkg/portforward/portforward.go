package portforward

import (
	"strings"

	"github.com/stepro/kdo/pkg/kubectl"
)

func start(k kubectl.CLI, namespace string, pod string, ports []string, readline func(line string) bool) (func(), error) {
	var args []string
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}
	args = append(args, "port-forward", pod)
	args = append(args, ports...)

	active := make(chan bool)
	ended := make(chan error)
	stop := k.StartLines(args, func(line string) {
		if readline != nil && readline(line) {
			readline = nil
			active <- true
		}
	}, ended)

	select {
	case err := <-ended:
		return nil, err
	case <-active:
		return stop, nil
	}
}

const forwardingPrefix = "Forwarding from 127.0.0.1:"

// StartOne starts forwarding a random local port to a port in a pod
func StartOne(k kubectl.CLI, namespace string, pod string, port string) (string, func(), error) {
	var localPort string
	stop, err := start(k, namespace, pod, []string{":" + port}, func(line string) bool {
		if !strings.HasPrefix(line, forwardingPrefix) {
			return false
		}
		localPort = strings.Split(line[len(forwardingPrefix):], " -> ")[0]
		return true
	})
	if err != nil {
		return "", nil, err
	}

	return localPort, stop, nil
}

// Start starts forwarding a set of local ports to ports a pod
func Start(k kubectl.CLI, pod string, ports []string) (func(), error) {
	var forwarded int
	stop, err := start(k, "", pod, ports, func(line string) bool {
		if !strings.HasPrefix(line, forwardingPrefix) {
			return false
		}
		forwarded++
		return forwarded == len(ports)
	})
	if err != nil {
		return nil, err
	}

	return stop, nil
}
