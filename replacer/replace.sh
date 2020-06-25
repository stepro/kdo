#!/bin/sh

set -ex

kubectl="kubectl -n $NAMESPACE"

if [ -n "$($kubectl get pod $POD)" ]; then
  if [ "$KIND" != "service" ]; then
    $kubectl scale --replicas=0 $KIND/$NAME
    $kubectl wait --for=delete pod/$POD --timeout=-1s
    $kubectl scale --current-replicas=0 --replicas=$REPLICAS $KIND/$NAME
  else
    $kubectl wait --for condition=Ready pod/$POD
    $kubectl set selector service $NAME kdo-hash=$HASH
    $kubectl get pod $POD -o jsonpath='{.metadata.deletionTimestamp}' -w | read -n1 -s
    $kubectl set selector service $NAME "$SELECTOR"
  fi
fi

$kubectl delete job,clusterrolebinding,serviceaccount -l kdo-replacer=$HASH
