package pod

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/stepro/kdo/pkg/kubectl"
	"github.com/stepro/kdo/pkg/output"
	"github.com/stepro/kdo/pkg/server"
)

// Name gets the name of the pod associated with a hash
func Name(hash string) string {
	return "kdo-" + hash
}

// Settings represents settings for a pod
type Settings struct {
	Inherit     string
	Labels      []string
	Annotations []string
	NoLifecycle bool
	NoProbes    bool
	Image       string
	Env         []string
	Replace     bool
	Listen      bool
	Stdin       bool
	TTY         bool
	Command     []string
}

func parseInherit(inherit string) (kind, name, container string, err error) {
	kindName := strings.SplitN(inherit, "/", 2)
	if len(kindName) == 1 {
		kind = "pod"
		name = kindName[0]
	} else {
		kind = kindName[0]
		kind = strings.ToLower(kind)
		switch kind {
		default:
			err = fmt.Errorf(`Unknown kind "%s"`, kindName[0])
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

func baseline(k *kubectl.CLI, kind, name, container string) (object, int, string, error) {
	var manifest object
	manifest = map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
	}

	if kind == "" {
		return manifest, 0, "kdo", nil
	}

	if kind == "service" {
		pods, err := k.Lines("get", "endpoints", name, "-o", `go-template={{range .subsets}}{{range .addresses}}{{if .targetRef}}{{if eq .targetRef.kind "Pod"}}{{.targetRef.name}}`+"\n"+`{{end}}{{end}}{{end}}{{end}}`)
		if err != nil {
			return nil, 0, "", err
		} else if len(pods) == 0 {
			return nil, 0, "", fmt.Errorf(`Unable to determine pod from service "%s"`, name)
		}
		kind = "pod"
		name = pods[0]
	}

	var source object
	if s, err := k.String("get", kind, name, "-o", "json"); err != nil {
		return nil, 0, "", err
	} else if err = json.Unmarshal([]byte(s), &source); err != nil {
		return nil, 0, "", err
	}

	var replicas int
	switch kind {
	case "deployment", "replicaset", "replicationcontroller", "statefulset":
		replicas = source.obj("spec").num("replicas")
	}

	if kind == "cronjob" {
		source = source.obj("spec").obj("jobTemplate").obj("spec").obj("template")
	} else if kind != "pod" {
		source = source.obj("spec").obj("template")
	}

	manifest.with("spec", func(spec object) {
		spec.set(source.obj("spec"),
			"activeDeadlineSeconds",
			"affinity",
			"automountServiceAccountToken",
			"containers",
			"dnsConfig",
			"dnsPolicy",
			"enableServiceLinks",
			// "ephemeralContainers",
			"hostAliases",
			"hostIPC",
			"hostNetwork",
			"hostPID",
			// "hostname",
			"imagePullSecrets",
			"initContainers",
			"nodeName",
			"nodeSelector",
			// "overhead",
			// "preemptionPolicy",
			// "priority",
			// "priorityClassName",
			"readinessGates",
			// "restartPolicy",
			"runtimeClassName",
			"schedulerName",
			"securityContext",
			"serviceAccountName",
			"shareProcessNamespace",
			// "subdomain",
			"terminationGracePeriodSeconds",
			"tolerations",
			"topologySpreadConstraints",
			"volumes")
	})

	if container == "" {
		for _, c := range source.obj("spec").arr("containers") {
			container = c.(map[string]interface{})["name"].(string)
		}
	}

	return manifest, replicas, container, nil
}

// Process represents a process in a pod
type Process struct {
	k         *kubectl.CLI
	Pod       string
	Container string
	exitCode  *int
	out       *output.Interface
}

// Exited indicates if the process has exited
func (p *Process) Exited() bool {
	return p.exitCode != nil
}

// ExitCode gets the exit code of the process
func (p *Process) ExitCode() (int, error) {
	if p.exitCode == nil {
		value, err := p.k.String("get", "pod", p.Pod, "--output", `go-template={{range .status.containerStatuses}}{{if eq .name "`+p.Container+`"}}{{if .state.terminated}}{{.state.terminated.exitCode}}{{end}}{{end}}{{end}}`)
		if err != nil || value == "" {
			return 0, err
		} else if code, err := strconv.Atoi(value); err != nil {
			return 0, err
		} else {
			p.exitCode = &code
		}
	}
	return *p.exitCode, nil
}

// Apply creates or updates a pod associated with a hash
func Apply(k *kubectl.CLI, hash string, build func(dockerPod string, op output.Operation) error, settings *Settings, out *output.Interface) (*Process, error) {
	p := Process{
		k:   k,
		Pod: Name(hash),
		out: out,
	}

	if err := out.Do("Creating pod", func(op output.Operation) error {
		stop := track(k, p.Pod, op)
		defer stop()

		err := k.Run("delete", "pod", p.Pod, "--ignore-not-found")
		if err != nil {
			return err
		}

		var kind string
		var name string
		var container string
		if settings.Inherit != "" {
			if kind, name, container, err = parseInherit(settings.Inherit); err != nil {
				return err
			}
		}

		var selector string
		if settings.Replace && kind == "service" {
			nameValues, err := k.Lines("get", "service", name, "-o", "go-template={{range $k, $v := .spec.selector}}{{$k}}={{$v}}\n{{end}}")
			if err != nil {
				return err
			}
			selector = strings.Join(nameValues, ",")
		}

		var manifest object
		var replicas int
		if manifest, replicas, p.Container, err = baseline(k, kind, name, container); err != nil {
			return err
		}
		manifest.with("metadata", func(metadata object) {
			metadata["name"] = p.Pod
			metadata.with("labels", func(labels object) {
				for _, label := range settings.Labels {
					nameValue := strings.SplitN(label, "=", 2)
					if len(nameValue) == 1 {
						nameValue = []string{nameValue[0], ""}
					}
					labels[nameValue[0]] = nameValue[1]
				}
				labels["kdo-pod"] = "1"
				labels["kdo-hash"] = hash
			}).with("annotations", func(annotations object) {
				for _, annotation := range settings.Annotations {
					nameValue := strings.SplitN(annotation, "=", 2)
					if len(nameValue) == 1 {
						nameValue = []string{nameValue[0], ""}
					}
					annotations[nameValue[0]] = nameValue[1]
				}
			})
		}).with("spec", func(spec object) {
			if build != nil {
				spec.append("initContainers", map[string]interface{}{
					"name":  "kdo-await-image-build",
					"image": "docker:19.03",
					"volumeMounts": []map[string]interface{}{
						{
							"name":      "kdo-docker-socket",
							"mountPath": "/var/run/docker.sock",
						},
					},
					"command": []string{
						"/bin/sh",
						"-c",
						`while [ -z "$(docker images ` + settings.Image + ` --format '{{.Repository}}')" ]; do sleep 1; done`,
					},
				}).append("volumes", map[string]interface{}{
					"name": "kdo-docker-socket",
					"hostPath": map[string]interface{}{
						"path": "/var/run/docker.sock",
					},
				})
			}
			spec.withelem("containers", p.Container, func(container object) {
				container["image"] = settings.Image
				if build != nil {
					container["imagePullPolicy"] = "Never"
				}
				for _, envVar := range settings.Env {
					nameValue := strings.SplitN(envVar, "=", 2)
					container.withelem("env", nameValue[0], func(e object) {
						if len(nameValue) > 1 {
							e["value"] = nameValue[1]
						}
						delete(e, "valueFrom")
					})
				}
				container["stdin"] = settings.Stdin
				container["stdinOnce"] = settings.Stdin
				container["tty"] = settings.TTY
				if len(settings.Command) > 0 {
					container["command"] = settings.Command
					delete(container, "args")
				}
				if settings.NoLifecycle {
					delete(container, "lifecycle")
				}
				if settings.NoProbes || settings.Stdin {
					delete(container, "livenessProbe")
					delete(container, "readinessProbe")
					delete(container, "startupProbe")
				}
			})
			if settings.Replace {
				replacer := map[string]interface{}{
					"name":  "kdo-replacer",
					"image": "bitnami/kubectl",
					"securityContext": map[string]interface{}{
						"runAsUser": 0,
					},
				}
				switch kind {
				default:
					out.Warning("replacement is not relevant for kind %s", kind)
				case "deployment", "replicaset", "replicationcontroller", "statefulset":
					replacer["command"] = []string{"/bin/sh", "-c", fmt.Sprintf(
						"trap 'exec kubectl scale --replicas=%d %s/%s' TERM && "+
							"kubectl scale --replicas=0 %s/%s && "+
							"while true; do sleep 1; done",
						replicas, kind, name, kind, name)}
				case "service":
					replacer["command"] = []string{"/bin/sh", "-c", fmt.Sprintf(
						"trap 'exec kubectl set selector service %s \"%s\"' TERM && "+
							"kubectl set selector service %s kdo-hash=%s && "+
							"while true; do sleep 1; done",
						name, selector, name, hash)}
				}
				spec.append("containers", replacer)
			}
			spec["restartPolicy"] = "Never"
		})

		op.Progress("applying manifest")
		data, err := yaml.Marshal(manifest)
		if err != nil {
			return err
		} else if err = k.Input(bytes.NewReader(data), "apply", "-f", "-"); err != nil {
			return err
		}

		if build != nil {
			if err = out.Do("Building image", func(op output.Operation) error {
				op.Progress("determining build pod")
				var node string
				for {
					node, err = k.String("get", "pod", p.Pod, "--output", `go-template={{.spec.nodeName}}`)
					if err != nil {
						return err
					} else if node != "" {
						break
					}
					time.Sleep(1 * time.Second)
				}
				nodePods, err := server.NodePods(k, out)
				if err != nil {
					return err
				}
				if nodePods[node] == "" {
					return fmt.Errorf("Cannot build on node %s", node)
				}
				return build(nodePods[node], op)
			}); err != nil {
				return err
			}
		}

		for {
			ready, err := k.String("get", "pod", p.Pod, "--output", `go-template={{range .status.conditions}}{{if eq .type "Ready"}}{{.status}}{{end}}{{end}} {{range .status.containerStatuses}}{{if eq .name "`+p.Container+`"}}{{if .state.terminated}}{{.state.terminated.exitCode}}{{end}}{{end}}{{end}}`)
			if err != nil {
				return err
			}
			if strings.HasPrefix(ready, "True ") {
				break
			} else {
				tokens := strings.Split(ready, " ")
				if tokens[1] != "" {
					exitCode, err := strconv.Atoi(tokens[1])
					if err != nil {
						return err
					}
					p.exitCode = &exitCode
					break
				}
			}
			time.Sleep(1 * time.Second)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return &p, nil
}

// Delete deletes the pod associated with a hash, if any
func Delete(k *kubectl.CLI, hash string, out *output.Interface) error {
	return out.Do("Deleting pod", func(op output.Operation) error {
		name := Name(hash)

		stop := track(k, name, op)
		defer stop()

		return k.Run("delete", "pod", name, "--ignore-not-found", "--wait=false")
	})
}

// DeleteAll deletes all pods associated with hashes
func DeleteAll(k *kubectl.CLI, out *output.Interface) error {
	// TODO: across all namespaces
	return out.Do("Deleting pods", func() error {
		return k.Run("delete", "pod", "-l", "kdo-pod=1")
	})
}
