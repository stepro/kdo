package pod

import (
	"encoding/json"
	"fmt"

	"github.com/stepro/kdo/pkg/kubectl"
)

type baseline struct {
	manifest  object
	replicas  int
	container string
	source    object
}

func inherit(k kubectl.CLI, kind, name string, labels, annotations bool, container string) (*baseline, error) {
	var bl baseline

	bl.manifest = map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
	}

	if kind == "" {
		return &bl, nil
	}

	if kind == "service" {
		pods, err := k.Lines("get", "endpoints", name, "-o", `go-template={{range .subsets}}{{range .addresses}}{{if .targetRef}}{{if eq .targetRef.kind "Pod"}}{{.targetRef.name}}`+"\n"+`{{end}}{{end}}{{end}}{{end}}`)
		if err != nil {
			return nil, err
		} else if len(pods) == 0 {
			return nil, fmt.Errorf(`Unable to determine pod from service "%s"`, name)
		}
		kind = "pod"
		name = pods[0]
	}

	if s, err := k.String("get", kind, name, "-o", "json"); err != nil {
		return nil, err
	} else if err = json.Unmarshal([]byte(s), &bl.source); err != nil {
		return nil, err
	}

	switch kind {
	case "deployment", "replicaset", "replicationcontroller", "statefulset":
		bl.replicas = bl.source.obj("spec").num("replicas")
	}

	if kind == "cronjob" {
		bl.source = bl.source.obj("spec").obj("jobTemplate").obj("spec").obj("template")
	} else if kind != "pod" {
		bl.source = bl.source.obj("spec").obj("template")
	}

	bl.manifest.with("metadata", func(metadata object) {
		if labels {
			metadata.set(bl.source.obj("metadata"), "labels")
		}
		if annotations {
			metadata.set(bl.source.obj("metadata"), "annotations")
		}
	})

	bl.manifest.with("spec", func(spec object) {
		spec.set(bl.source.obj("spec"),
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
		for _, c := range bl.source.obj("spec").arr("containers") {
			container = c.(map[string]interface{})["name"].(string)
		}
	}

	return &bl, nil
}
