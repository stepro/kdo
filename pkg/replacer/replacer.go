package replacer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stepro/kdo/pkg/kubectl"
	"github.com/stepro/kdo/pkg/output"
)

func pkgerror(err error) error {
	if err != nil {
		err = fmt.Errorf("replacer: %v", err)
	}
	return err
}

const manifest = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kdo-replacer
  labels:
    component: kdo-rbac
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: kdo-replacer
  labels:
    component: kdo-rbac
rules:
- apiGroups: [""]
  resources: [pods]
  verbs: [get, list, watch]
- apiGroups: [""]
  resources: [replicationcontrollers,services]
  verbs: [get, patch, update]
- apiGroups: [apps]
  resources: [deployments,daemonsets,replicasets,statefulsets]
  verbs: [get, patch, update]
- apiGroups: [batch]
  resources: [cronjobs, jobs]
  verbs: [get, patch, update]
- apiGroups: [extensions]
  resources: [deployments,daemonsets,replicasets]
  verbs: [get, patch, update]
- apiGroups: [batch]
  resources: [jobs]
  verbs: [delete]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: kdo-replacer
  labels:
    component: kdo-rbac
subjects:
- kind: ServiceAccount
  name: kdo-replacer
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: kdo-replacer
---
apiVersion: batch/v1
kind: Job
metadata:
  name: kdo-replacer-{hash}
  labels:
    kdo-job: "1"
spec:
  template:
    spec:
      serviceAccountName: kdo-replacer
      containers:
      - name: replacer
        image: bitnami/kubectl
        env:
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: KIND
          value: {kind}
        - name: NAME
          value: {name}
        - name: REPLICAS
          value: "{replicas}"
        - name: SELECTOR
          value: "{selector}"
        - name: HASH
          value: {hash}
        command: [/bin/bash, -c, {script}]
      restartPolicy: OnFailure
      terminationGracePeriodSeconds: 0
`

const workloadScript = `set -ex
kubectl="kubectl -n $NAMESPACE"
if [ -n "$($kubectl get pod kdo-$HASH)" ]; then
  $kubectl scale --replicas=0 $KIND/$NAME
  $kubectl wait --for=delete pod/kdo-$HASH --timeout=-1s
  $kubectl scale --current-replicas=0 --replicas=$REPLICAS $KIND/$NAME
fi
$kubectl delete job kdo-replacer-$HASH --wait=false
`

const serviceScript = `set -ex
kubectl="kubectl -n $NAMESPACE"
if [ -n "$($kubectl get pod kdo-$HASH)" ]; then
  $kubectl wait --for condition=Ready pod/kdo-$HASH
  $kubectl set selector service $NAME kdo-hash=$HASH
  $kubectl get pod kdo-$HASH -o jsonpath='{.metadata.deletionTimestamp}' -w | read -n1 -s
  $kubectl set selector service $NAME "$SELECTOR"
fi
$kubectl delete job kdo-replacer-$HASH --wait=false
`

// Apply creates or updates a replacer for a pod
func Apply(k kubectl.CLI, kind, name string, replicas int, selector string, hash string, out *output.Interface) error {
	return pkgerror(out.Do("Replacing %s", kind, func(op output.Operation) error {
		mf := strings.ReplaceAll(manifest, "{kind}", kind)
		mf = strings.ReplaceAll(mf, "{name}", name)
		mf = strings.ReplaceAll(mf, "{replicas}", fmt.Sprintf("%d", replicas))
		mf = strings.ReplaceAll(mf, "{selector}", selector)
		mf = strings.ReplaceAll(mf, "{hash}", hash)

		var script string
		if kind != "service" {
			script = workloadScript
		} else {
			script = serviceScript
		}
		data, err := json.Marshal(script)
		if err != nil {
			return err
		}
		mf = strings.ReplaceAll(mf, "{script}", string(data))

		op.Progress("applying manifest")
		if err := k.Input(strings.NewReader(mf), "apply", "--filename", "-"); err != nil {
			return err
		}

		return nil
	}))
}

// Wait waits for a replacer associated with a hash to complete (be deleted)
func Wait(k kubectl.CLI, hash string) error {
	s, err := k.ErrorString("wait", "--for", "delete", "job/kdo-replacer-"+hash)
	if err != nil {
		if strings.HasPrefix(s, "Error from server (NotFound)") {
			return nil
		}
		return err
	}

	return nil
}

// WaitAll waits for all replacers to complete (be deleted)
func WaitAll(k kubectl.CLI, out *output.Interface) error {
	return pkgerror(out.Do("Waiting for all replacers", func() error {
		s, err := k.ErrorString("wait", "--for", "delete", "job", "--selector", "kdo-job=1", "--all-namespaces")
		if err != nil {
			if strings.HasPrefix(s, "error: no matching resources found") {
				return nil
			}
			return err
		}
		return nil
	}))
}

// Uninstall deletes all replacer components
func Uninstall(k kubectl.CLI, out *output.Interface) error {
	return pkgerror(out.Do("Uninstalling replacer components", func() error {
		return k.Run("delete", "rolebinding,role,serviceaccount", "--selector", "component=kdo-rbac", "--all-namespaces")
	}))
}
