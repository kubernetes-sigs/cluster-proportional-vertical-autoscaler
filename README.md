# cluster-proportional-vertical-autoscaler (BETA)

[![Build Status](https://travis-ci.org/kubernetes-sigs/cluster-proportional-vertical-autoscaler.svg)](https://travis-ci.org/kubernetes-sigs/cluster-proportional-vertical-autoscaler)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-sigs/cluster-proportional-vertical-autoscaler)](https://goreportcard.com/report/github.com/kubernetes-sigs/cluster-proportional-vertical-autoscaler)

## Overview

This container image watches over the number of nodes and cores of the cluster and resizes
the resource limits and requests for a DaemonSet, ReplicaSet, or Deployment. This functionality 
may be desirable for applications where resources such as cpu and memory for a particular job need 
to be autoscaled with the size of the cluster.

Usage of cluster-proportional-vertical-autoscaler:

```
      --alsologtostderr[=false]: log to standard error as well as files
      --config-file: The default configuration (in JSON format).
      --default-config: A config file (in JSON format), which overrides the --default-config.
      --kube-config="": Path to a kubeconfig. Only required if running out-of-cluster.
      --log-backtrace-at=:0: when logging hits line file:N, emit a stack trace
      --log-dir="": If non-empty, write log files in this directory
      --logtostderr[=false]: log to standard error instead of files
      --namespace="": The Namespace of the --target. Defaults to ${MY_NAMESPACE}.
      --poll-period-seconds=10: The period, in seconds, to poll cluster size and perform autoscaling.
      --stderrthreshold=2: logs at or above this threshold go to stderr
      --target="": Target to scale. In format: deployment/*, replicaset/* or daemonset/* (not case sensitive).
      --v=0: log level for V logs
      --version[=false]: Print the version and exit.
      --vmodule=: comma-separated list of pattern=N settings for file-filtered logging
```

## Examples

Please try out the examples in [the examples folder](examples/README.md).

## Implementation Details

The code in this module is a Kubernetes Golang API client that, using the default service account credentials
available to Golang clients running inside pods, it connects to the API server and polls for the number of nodes
and cores in the cluster.

The scaling parameters and data points are provided via a config file in JSON format to the autoscaler and it 
refreshes its parameters table every poll interval to be up to date with the latest desired scaling parameters.

### Calculation of resource requests and limits

The resource requests and limits are computed by using the number of cores and nodes as input as well as
the provided step values bounded by provided base and max values.

Example:

```
Base = 10
Max = 100
Step = 2
CoresPerStep = 4
NodesPerStep = 2

The core and node counts are rounded up to the next whole step.

If we find 64 cores and 4 nodes we get scalars of:
  by-cores: 10 + (2 * (round(64, 4)/4)) = 10 + 32 = 42
  by-nodes: 10 + (2 * (round(4, 2)/2)) = 10 + 4 = 14
  
The larger is by-cores, and it is less than Max, so the final value is 42.

If we find 3 cores and 3 nodes we get scalars of:
  by-cores: 10 + (2 * (round(3, 4)/4)) = 10 + 2 = 12
  by-nodes: 10 + (2 * (round(3, 2)/2)) = 10 + 4 = 14
```

## Config parameters

The configuration should be in JSON format and supports the following parameters:
  - **base** The baseline quantity required.
  - **max**  The maximum allowed quantity.
  - **step** The amount of additional resources to grow by.  If this is too fine-grained, the resizing action will happen too frequently.
  - **coresPerStep** The number of cores required to trigger an increase.
  - **nodesPerStep** The number of nodes required to trigger an increase.
      
Example:

```
"containerA": {
  "requests": {
    "cpu": {
      "base": "10m", "step":"1m", "coresPerStep":1
    },
    "memory": {
      "base": "8Mi", "step":"1Mi", "coresPerStep":1
    }
  }
"containerB": {
  "requests": {
    "cpu": {
      "base": "250m", "step":"100m", "coresPerStep":10
    },
  }
}
```

## Running the cluster-proportional-vertical-autoscaler
This repo includes an example yaml files in the "examples" directory that can be used as examples demonstrating 
how to use the vertical autoscaler.

For example, consider a Deployment that needs to scale its resources (cpu, memory, etc...) proportional to the number of
cores in a cluster.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: thing
  namespace: kube-system
  labels:
    k8s-app: thing
spec:
  replicas: 3
  selector:
    matchLabels:
      k8s-app: thing
  template:
    metadata:
      labels:
        k8s-app: thing
    spec:
      containers:
      - image: nginx
        name: thing
```

```bash
kubectl create -f thing.yaml
```


The below config will scale the above defined deployment's CPU resource by "100m" step size
for every 10 nodes that are added to the cluster.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: thing-autoscaler
  namespace: kube-system
  labels:
    k8s-app: thing-autoscaler
    kubernetes.io/cluster-service: "true"
    addonmanager.kubernetes.io/mode: Reconcile
spec:
  selector:
    matchLabels:
      k8s-app: thing-autoscaler
  template:
    metadata:
      labels:
        k8s-app: thing-autoscaler
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      containers:
      - name: autoscaler
        image: k8s.gcr.io/cpvpa-amd64:v0.8.1
        resources:
          requests:
            cpu: "20m"
            memory: "10Mi"
        command:
          - /cpvpa
          - --target=deployment/thing
          - --namespace=kube-system
          - --logtostderr=true
          - --poll-period-seconds=10
          - --default-config={"thing":{"requests":{"cpu":{"base":"250m","step":"100m","nodesPerStep":10}}}}
      tolerations:
      - key: "CriticalAddonsOnly"
        operator: "Exists"
      serviceAccountName: thing-autoscaler
```
