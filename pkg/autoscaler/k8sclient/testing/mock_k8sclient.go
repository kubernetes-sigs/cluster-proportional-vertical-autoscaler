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
	"github.com/kubernetes-incubator/cluster-proportional-vertical-autoscaler/pkg/autoscaler/k8sclient"
	apiv1 "k8s.io/api/core/v1"
)

var _ = k8sclient.K8sClient(&MockK8sClient{})

// MockK8sClient implements K8sClientInterface
type MockK8sClient struct {
	NumOfNodes int
	NumOfCores int
}

// GetClusterSize mocks counting schedulable nodes and cores in the cluster
func (k *MockK8sClient) GetClusterSize() (*k8sclient.ClusterSize, error) {
	return &k8sclient.ClusterSize{k.NumOfNodes, k.NumOfCores}, nil
}

// UpdateResources mocks updating resources needs for containers in the target
func (k *MockK8sClient) UpdateResources(resources map[string]apiv1.ResourceRequirements) error {
	return nil
}
