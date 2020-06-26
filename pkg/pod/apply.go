package pod

import (
	"bytes"
	"strconv"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/stepro/kdo/pkg/kubectl"
	"github.com/stepro/kdo/pkg/output"
	"github.com/stepro/kdo/pkg/replacer"
)

// Config represents configuration settings for a pod
type Config struct {
	InheritKind        string
	InheritName        string
	InheritLabels      bool
	InheritAnnotations bool
	Labels             map[string]*string
	Annotations        map[string]*string
	Container          string
	Image              string
	Env                map[string]*string
	NoLifecycle        bool
	NoProbes           bool
	Replace            bool
	Stdin              bool
	TTY                bool
	Command            []string
	Detach             bool
}

// Apply creates or replaces a pod associated with a hash
func Apply(k kubectl.CLI, hash string, config *Config, build func(pod string) error, out *output.Interface) (*Process, error) {
	var p *Process

	err := out.Do("Creating pod", func(op output.Operation) error {
		name := Name(hash)

		stop := track(k, name, op)
		defer stop()

		err := k.Run("delete", "pod", "--selector", "kdo-hash="+hash)
		if err != nil {
			return err
		}

		op.Progress("waiting for replacer")
		if err = replacer.Wait(k, hash); err != nil {
			return err
		}

		var manifest object
		var replicas int
		op.Progress("determining configuration")
		if manifest, replicas, err = baseline(k, config.InheritKind, config.InheritName); err != nil {
			return err
		}

		var selector string
		if config.InheritKind == "service" && config.Replace {
			op.Progress("determining pod selector")
			nameValues, err := k.Lines("get", "service", config.InheritName, "-o", "go-template={{range $k, $v := .spec.selector}}{{$k}}={{$v}}\n{{end}}")
			if err != nil {
				return err
			}
			selector = strings.Join(nameValues, ",")
		}

		var container string
		if config.InheritKind == "" {
			container = "kdo"
		} else {
			for _, c := range manifest.obj("spec").arr("containers") {
				container = c.(map[string]interface{})["name"].(string)
				break
			}
		}

		op.Progress("generating manifest")
		manifest.with("metadata", func(metadata object) {
			metadata["name"] = name
			sourceLabels := metadata.obj("labels")
			if !config.InheritLabels {
				delete(metadata, "labels")
			}
			metadata.with("labels", func(labels object) {
				for k, v := range config.Labels {
					if v == nil {
						if sourceLabels != nil && sourceLabels[k] != nil {
							labels[k] = sourceLabels[k]
						}
					} else if *v != "" {
						labels[k] = *v
					} else {
						delete(labels, k)
					}
				}
				labels["kdo-pod"] = "1"
				labels["kdo-hash"] = hash
			})
			sourceAnnotations := metadata.obj("annotations")
			if !config.InheritAnnotations {
				delete(metadata, "annotations")
			}
			metadata.with("annotations", func(annotations object) {
				for k, v := range config.Annotations {
					if v == nil {
						if sourceAnnotations != nil && sourceAnnotations[k] != nil {
							annotations[k] = sourceAnnotations[k]
						}
					} else if *v != "" {
						annotations[k] = *v
					} else {
						delete(annotations, k)
					}
				}
			})
		}).with("spec", func(spec object) {
			if build != nil {
				spec.appendobj("volumes", map[string]interface{}{
					"name": "kdo-docker-socket",
					"hostPath": map[string]interface{}{
						"path": "/var/run/docker.sock",
					},
				}).appendobj("initContainers", map[string]interface{}{
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
						`while [ -z "$(docker images ` + config.Image + ` --format '{{.Repository}}')" ]; do sleep 1; done`,
					},
				})
			}
			spec.withelem("containers", container, func(container object) {
				container["image"] = config.Image
				if build != nil {
					container["imagePullPolicy"] = "Never"
				}
				for k, v := range config.Env {
					container.withelem("env", k, func(e object) {
						if v != nil {
							delete(e, "valueFrom")
							e["value"] = v
						}
					})
				}
				if config.NoLifecycle {
					delete(container, "lifecycle")
				}
				if config.NoProbes || config.Stdin {
					delete(container, "livenessProbe")
					delete(container, "readinessProbe")
					delete(container, "startupProbe")
				}
				container["stdin"] = config.Stdin
				container["stdinOnce"] = config.Stdin
				container["tty"] = config.TTY
				if len(config.Command) > 0 {
					container["command"] = config.Command
					delete(container, "args")
				}
			})
			if !config.Detach {
				spec["restartPolicy"] = "Never"
			}
		})

		op.Progress("applying manifest")
		data, err := yaml.Marshal(manifest)
		if err != nil {
			return err
		} else if err = k.Input(bytes.NewReader(data), "apply", "-f", "-"); err != nil {
			return err
		}
		defer func() {
			if err != nil {
				Delete(k, hash, out)
			}
		}()

		if config.Replace {
			if err = replacer.Apply(k, config.InheritKind, config.InheritName, replicas, selector, hash, out); err != nil {
				return err
			}
		}

		if build != nil {
			if err = build(name); err != nil {
				return err
			}
		}

		p = &Process{
			k:         k,
			Pod:       name,
			Container: container,
		}

		for {
			ready, err := k.String("get", "pod", name, "--output", `go-template={{range .status.conditions}}{{if eq .type "Ready"}}{{.status}}{{end}}{{end}} {{range .status.containerStatuses}}{{if eq .name "`+container+`"}}{{if .state.terminated}}{{.state.terminated.exitCode}}{{end}}{{end}}{{end}}`)
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
	})
	if err != nil {
		return nil, pkgerror(err)
	}

	return p, nil
}
