package pod

// Inheritance represents inheritance settings for a pod
type Inheritance struct {
	Kind        string
	Name        string
	Labels      bool
	Annotations bool
	Container   string
}

// Config represents configuration settings for a pod
type Config struct {
	Inherit     *Inheritance
	Labels      []string
	Annotations []string
	NoLifecycle bool
	NoProbes    bool
	Image       string
	Env         []string
	Replace     bool
	Listen      []string
	Stdin       bool
	TTY         bool
	Command     []string
}

// // Apply creates or updates a pod associated with a hash
// func Apply(k *kubectl.CLI, hash string, build func(dockerPod string, op output.Operation) error, settings *Settings, out *output.Interface) (*Process, error) {
// 	p := Process{
// 		k:   k,
// 		Pod: Name(hash),
// 		out: out,
// 	}

// 	if err := out.Do("Creating pod", func(op output.Operation) error {
// 		stop := track(k, p.Pod, op)
// 		defer stop()

// 		err := k.Run("delete", "job,pod", "--selector", "kdo-hash="+hash)
// 		if err != nil {
// 			return err
// 		}

// 		var kind string
// 		var name string
// 		var container string
// 		if settings.Inherit != "" {
// 			if kind, name, container, err = ParseInherit(settings.Inherit); err != nil {
// 				return err
// 			}
// 		}

// 		var selector string
// 		if settings.Replace && kind == "service" {
// 			op.Progress("identifying pod selector")
// 			nameValues, err := k.Lines("get", "service", name, "-o", "go-template={{range $k, $v := .spec.selector}}{{$k}}={{$v}}\n{{end}}")
// 			if err != nil {
// 				return err
// 			}
// 			selector = strings.Join(nameValues, ",")
// 		}

// 		var bl *baseline
// 		op.Progress("inheriting pod configuration")
// 		if bl, err = inherit(k, kind, name, settings.InheritLabels, settings.InheritAnnotations, container); err != nil {
// 			return err
// 		}

// 		op.Progress("generating manifest")
// 		bl.manifest.with("metadata", func(metadata object) {
// 			metadata["name"] = p.Pod
// 			metadata.with("labels", func(labels object) {
// 				var sourceLabels object
// 				if bl.source != nil {
// 					if sourceLabels = bl.source.obj("metadata"); sourceLabels != nil {
// 						sourceLabels = sourceLabels.obj("labels")
// 					}
// 				}
// 				for _, label := range settings.Labels {
// 					nameValue := strings.SplitN(label, "=", 2)
// 					if len(nameValue) == 1 {
// 						if sourceLabels != nil && sourceLabels[nameValue[0]] != nil {
// 							labels[nameValue[0]] = sourceLabels[nameValue[0]]
// 						}
// 					} else if nameValue[1] != "" {
// 						labels[nameValue[0]] = nameValue[1]
// 					} else {
// 						delete(labels, nameValue[0])
// 					}
// 				}
// 				labels["kdo-pod"] = "1"
// 				labels["kdo-hash"] = hash
// 			}).with("annotations", func(annotations object) {
// 				var sourceAnnotations object
// 				if bl.source != nil {
// 					if sourceAnnotations = bl.source.obj("metadata"); sourceAnnotations != nil {
// 						sourceAnnotations = sourceAnnotations.obj("annotations")
// 					}
// 				}
// 				for _, annotation := range settings.Annotations {
// 					nameValue := strings.SplitN(annotation, "=", 2)
// 					if len(nameValue) == 1 {
// 						if sourceAnnotations != nil && sourceAnnotations[nameValue[0]] != nil {
// 							annotations[nameValue[0]] = sourceAnnotations[nameValue[0]]
// 						}
// 					} else if nameValue[1] != "" {
// 						annotations[nameValue[0]] = nameValue[1]
// 					} else {
// 						delete(annotations, nameValue[0])
// 					}
// 				}
// 			})
// 		}).with("spec", func(spec object) {
// 			if build != nil {
// 				spec.appendobj("initContainers", map[string]interface{}{
// 					"name":  "kdo-await-image-build",
// 					"image": "docker:19.03",
// 					"volumeMounts": []map[string]interface{}{
// 						{
// 							"name":      "kdo-docker-socket",
// 							"mountPath": "/var/run/docker.sock",
// 						},
// 					},
// 					"command": []string{
// 						"/bin/sh",
// 						"-c",
// 						`while [ -z "$(docker images ` + settings.Image + ` --format '{{.Repository}}')" ]; do sleep 1; done`,
// 					},
// 				}).appendobj("volumes", map[string]interface{}{
// 					"name": "kdo-docker-socket",
// 					"hostPath": map[string]interface{}{
// 						"path": "/var/run/docker.sock",
// 					},
// 				})
// 			}
// 			spec.withelem("containers", p.Container, func(container object) {
// 				container["image"] = settings.Image
// 				if build != nil {
// 					container["imagePullPolicy"] = "Never"
// 				}
// 				for _, envVar := range settings.Env {
// 					nameValue := strings.SplitN(envVar, "=", 2)
// 					container.withelem("env", nameValue[0], func(e object) {
// 						if len(nameValue) > 1 {
// 							e["value"] = nameValue[1]
// 						}
// 						delete(e, "valueFrom")
// 					})
// 				}
// 				container["stdin"] = settings.Stdin
// 				container["stdinOnce"] = settings.Stdin
// 				container["tty"] = settings.TTY
// 				if len(settings.Command) > 0 {
// 					container["command"] = settings.Command
// 					delete(container, "args")
// 				}
// 				if settings.NoLifecycle {
// 					delete(container, "lifecycle")
// 				}
// 				if settings.NoProbes || settings.Stdin {
// 					delete(container, "livenessProbe")
// 					delete(container, "readinessProbe")
// 					delete(container, "startupProbe")
// 				}
// 			})
// 			spec["restartPolicy"] = "Never"
// 		})

// 		var svcAccount object
// 		var roleBinding object
// 		var replacer object
// 		if settings.Replace {
// 			op.Progress("generating replacer manifest")
// 			svcAccount = map[string]interface{}{
// 				"apiVersion": "v1",
// 				"kind":       "ServiceAccount",
// 				"metadata": map[string]interface{}{
// 					"name": "kdo-replacer",
// 				},
// 			}
// 			roleBinding = map[string]interface{}{
// 				"apiVersion": "rbac.authorization.k8s.io/v1",
// 				"kind":       "ClusterRoleBinding",
// 				"metadata": map[string]interface{}{
// 					"name": "kdo-replacer",
// 				},
// 				"subjects": []interface{}{
// 					map[string]interface{}{
// 						"kind":      "ServiceAccount",
// 						"name":      "kdo-replacer",
// 						"namespace": bl.manifest.obj("metadata").str("namespace"),
// 					},
// 				},
// 				"roleRef": map[string]interface{}{
// 					"kind":     "ClusterRole",
// 					"name":     "cluster-admin",
// 					"apiGroup": "rbac.authorization.k8s.io",
// 				},
// 			}
// 			env := []interface{}{
// 				map[string]interface{}{
// 					"name": "NAMESPACE",
// 					"valueFrom": map[string]interface{}{
// 						"fieldRef": map[string]interface{}{
// 							"fieldPath": "metadata.namespace",
// 						},
// 					},
// 				},
// 				map[string]interface{}{
// 					"name":  "KIND",
// 					"value": kind,
// 				},
// 				map[string]interface{}{
// 					"name":  "NAME",
// 					"value": name,
// 				},
// 				map[string]interface{}{
// 					"name":  "POD",
// 					"value": p.Pod,
// 				},
// 			}
// 			replacerContainer := map[string]interface{}{
// 				"name":  "replacer",
// 				"image": "bitnami/kubectl",
// 				"securityContext": map[string]interface{}{
// 					"runAsUser": 0,
// 				},
// 				"env": env,
// 			}
// 			replacer = map[string]interface{}{
// 				"apiVersion": "batch/v1",
// 				"kind":       "Job",
// 				"metadata": map[string]interface{}{
// 					"name": "kdo-replacer-" + hash,
// 				},
// 				"spec": map[string]interface{}{
// 					"template": map[string]interface{}{
// 						"serviceAccountName": "kdo-replacer-" + hash,
// 						"containers":         []interface{}{replacerContainer},
// 					},
// 				},
// 			}
// 			replacer = map[string]interface{}{
// 				"name":  "kdo-replacer",
// 				"image": "bitnami/kubectl",
// 			}
// 			switch kind {
// 			default:
// 				out.Warning("replacement is not relevant for kind %s", kind)
// 			case "deployment", "replicaset", "replicationcontroller", "statefulset":
// 				replacer["command"] = []string{"/bin/sh", "-c", fmt.Sprintf(
// 					"trap 'exec kubectl scale --replicas=%d %s/%s' TERM && "+
// 						"kubectl scale --replicas=0 %s/%s && "+
// 						"while true; do sleep 1; done",
// 					replicas, kind, name, kind, name)}
// 			case "service":
// 				replacer["command"] = []string{"/bin/sh", "-c", fmt.Sprintf(
// 					"trap 'exec kubectl set selector service %s \"%s\"' TERM && "+
// 						"kubectl set selector service %s kdo-hash=%s && "+
// 						"while true; do sleep 1; done",
// 					name, selector, name, hash)}
// 			}
// 			spec.appendobj("containers", replacer)
// 		}

// 		op.Progress("applying manifest")
// 		data, err := yaml.Marshal(manifest)
// 		if err != nil {
// 			return err
// 		} else if err = k.Input(bytes.NewReader(data), "apply", "-f", "-"); err != nil {
// 			return err
// 		}

// 		if build != nil {
// 			if err = out.Do("Building image", func(op output.Operation) error {
// 				op.Progress("determining build pod")
// 				var node string
// 				for {
// 					node, err = k.String("get", "pod", p.Pod, "--output", `go-template={{.spec.nodeName}}`)
// 					if err != nil {
// 						return err
// 					} else if node != "" {
// 						break
// 					}
// 					time.Sleep(1 * time.Second)
// 				}
// 				nodePods, err := server.NodePods(k, out)
// 				if err != nil {
// 					return err
// 				}
// 				if nodePods[node] == "" {
// 					return fmt.Errorf("Cannot build on node %s", node)
// 				}
// 				return build(nodePods[node], op)
// 			}); err != nil {
// 				return err
// 			}
// 		}

// 		for {
// 			ready, err := k.String("get", "pod", p.Pod, "--output", `go-template={{range .status.conditions}}{{if eq .type "Ready"}}{{.status}}{{end}}{{end}} {{range .status.containerStatuses}}{{if eq .name "`+p.Container+`"}}{{if .state.terminated}}{{.state.terminated.exitCode}}{{end}}{{end}}{{end}}`)
// 			if err != nil {
// 				return err
// 			}
// 			if strings.HasPrefix(ready, "True ") {
// 				break
// 			} else {
// 				tokens := strings.Split(ready, " ")
// 				if tokens[1] != "" {
// 					exitCode, err := strconv.Atoi(tokens[1])
// 					if err != nil {
// 						return err
// 					}
// 					p.exitCode = &exitCode
// 					break
// 				}
// 			}
// 			time.Sleep(1 * time.Second)
// 		}

// 		return nil
// 	}); err != nil {
// 		return nil, err
// 	}

// 	return &p, nil
// }
