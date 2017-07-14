/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k8sclient

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/golang/glog"

	"k8s.io/client-go/1.4/kubernetes"
	"k8s.io/client-go/1.4/pkg/api"
	"k8s.io/client-go/1.4/pkg/api/resource"
	apiv1 "k8s.io/client-go/1.4/pkg/api/v1"
	"k8s.io/client-go/1.4/rest"
	"k8s.io/client-go/1.4/tools/clientcmd"
)

// K8sClient - Wraps all needed client functionalities for autoscaler
type K8sClient interface {
	// GetClusterSize counts schedulable nodes and cores in the cluster
	GetClusterSize() (*ClusterSize, error)
	// UpdateResources updates the resource needs for the containers in the target
	UpdateResources(resources map[string]apiv1.ResourceRequirements) error
}

// k8sClient - Wraps all Kubernetes API client functionality.
type k8sClient struct {
	target        *targetSpec
	clientset     *kubernetes.Clientset
	clusterStatus *ClusterSize
}

// NewK8sClient gives a k8sClient with the given dependencies.
func NewK8sClient(namespace, target, kubeconfig string) (K8sClient, error) {
	var config *rest.Config
	var err error
	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, err
	}
	// Use protobufs for communication with apiserver.
	config.ContentType = "application/vnd.kubernetes.protobuf"
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	tgt, err := makeTarget(clientset, target, namespace)
	if err != nil {
		return nil, err
	}

	return &k8sClient{
		clientset: clientset,
		target:    tgt,
	}, nil
}

func makeTarget(client *kubernetes.Clientset, target, namespace string) (*targetSpec, error) {
	splits := strings.Split(target, "/")
	if len(splits) != 2 {
		return &targetSpec{}, fmt.Errorf("target format error: %v", target)
	}
	kind := splits[0]
	name := splits[1]

	kind, apigroup, apiver, err := discoverAPI(client, kind)
	if err != nil {
		return nil, err
	}
	glog.V(4).Infof("discovered target %s = %s/%s.%s", target, apigroup, apiver, kind)
	return &targetSpec{kind, apigroup, apiver, name, namespace}, nil
}

func discoverAPI(client *kubernetes.Clientset, kindArg string) (kind string, apigroup string, apiver string, err error) {
	kind = ""
	plural := ""
	switch strings.ToLower(kindArg) {
	case "deployment":
		kind = "Deployment"
		plural = "deployments"
	default:
		return "", "", "", fmt.Errorf("unknown kind %q", kindArg)
	}
	resources, err := client.Discovery().ServerPreferredNamespacedResources()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to discover apigroup for kind %q: %v", kind, err)
	}
	apigroup = ""
	apiver = ""
	for _, res := range resources {
		if res.Resource == plural {
			// Avoid legacy "extensions" if we can.
			if apigroup == "" || apigroup == "extensions" {
				apigroup = res.Group
				apiver = res.Version
			}
		}
	}
	return kind, apigroup, apiver, nil
}

// targetSpec stores the scalable target resource.
type targetSpec struct {
	kind      string
	group     string
	version   string
	name      string
	namespace string
}

// ClusterSize defines the cluster status.
type ClusterSize struct {
	Nodes int
	Cores int
}

func (k *k8sClient) GetClusterSize() (clusterStatus *ClusterSize, err error) {
	opt := api.ListOptions{Watch: false}

	nodes, err := k.clientset.Nodes().List(opt)
	if err != nil || nodes == nil {
		return nil, err
	}
	clusterStatus = &ClusterSize{}
	clusterStatus.Nodes = len(nodes.Items)
	var tc resource.Quantity
	for _, node := range nodes.Items {
		tc.Add(node.Status.Capacity[apiv1.ResourceCPU])
	}

	tcInt64, tcOk := tc.AsInt64()
	if !tcOk {
		return nil, fmt.Errorf("unable to compute integer values of cores in the cluster")
	}
	clusterStatus.Cores = int(tcInt64)
	k.clusterStatus = clusterStatus
	return clusterStatus, nil
}

// TODO: this should switch on resource kind to handle ReplicaSets.
func (k *k8sClient) UpdateResources(resources map[string]apiv1.ResourceRequirements) error {
	ctrs := []interface{}{}
	for ctrName, res := range resources {
		ctrs = append(ctrs, map[string]interface{}{
			"name":      ctrName,
			"resources": res,
		})
	}
	patch := map[string]interface{}{
		"apiVersion": fmt.Sprintf("%s/%s", k.target.group, k.target.version),
		"kind":       k.target.kind,
		"metadata": map[string]interface{}{
			"name": k.target.name,
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": ctrs,
				},
			},
		},
	}
	jb, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("can't marshal patch to JSON: %v", err)
	}
	_, err = k.clientset.Deployments(k.target.namespace).Patch(k.target.name, api.StrategicMergePatchType, jb)
	if err != nil {
		return fmt.Errorf("patch failed: %v", err)
	}
	return nil
}
