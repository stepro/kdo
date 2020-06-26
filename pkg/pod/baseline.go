package pod

import (
	"encoding/json"
	"fmt"

	"github.com/stepro/kdo/pkg/kubectl"
)

func baseline(k kubectl.CLI, kind, name string) (manifest object, replicas int, err error) {
	manifest = map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
	}

	if kind == "" {
		return
	}

	if kind == "service" {
		pods, err := k.Lines("get", "endpoints", name, "-o", `go-template={{range .subsets}}{{range .addresses}}{{if .targetRef}}{{if eq .targetRef.kind "Pod"}}{{.targetRef.name}}`+"\n"+`{{end}}{{end}}{{end}}{{end}}`)
		if err != nil {
			return nil, 0, err
		} else if len(pods) == 0 {
			return nil, 0, fmt.Errorf(`unable to determine pod from service "%s"`, name)
		}
		kind = "pod"
		name = pods[0]
	}

	var source object
	if s, err := k.String("get", kind, name, "-o", "json"); err != nil {
		return nil, 0, err
	} else if err = json.Unmarshal([]byte(s), &source); err != nil {
		return nil, 0, err
	}

	switch kind {
	case "deployment", "replicaset", "replicationcontroller", "statefulset":
		replicas = source.obj("spec").num("replicas")
	}

	if kind == "cronjob" {
		source = source.obj("spec").obj("jobTemplate").obj("spec").obj("template")
	} else if kind != "pod" {
		source = source.obj("spec").obj("template")
	}

	manifest.with("metadata", func(metadata object) {
		metadata.set(source.obj("metadata"),
			"labels",
			"annotations")
	}).with("spec", func(spec object) {
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

	return
}
