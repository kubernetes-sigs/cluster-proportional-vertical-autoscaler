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
	"testing"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDiscoverAPI(t *testing.T) {
	testCases := []struct {
		kind     string
		expError bool
	}{
		{
			"deployment",
			false,
		},
		{
			"daemonset",
			false,
		},
		{
			"replicaset",
			false,
		},
		{
			"replicationcontroller",
			true,
		},
	}
	c := fake.NewSimpleClientset()
	for _, tc := range testCases {
		_, _, err := discoverAPI(c, tc.kind)
		if err != nil && !tc.expError {
			t.Errorf("Expect no error, got error for kind: %q: %v", tc.kind, err)
			continue
		} else if err == nil && tc.expError {
			t.Errorf("Expect error, got no error for kind: %q", tc.kind)
			continue
		}
	}
}

func TestMakeTarget(t *testing.T) {
	testCases := []struct {
		target   string
		expKind  string
		expName  string
		expError bool
	}{
		{
			"deployment/thing",
			"Deployment",
			"thing",
			false,
		},
		{
			"daemonset/thing",
			"DaemonSet",
			"thing",
			false,
		},
		{
			"replicaset/thing",
			"ReplicaSet",
			"thing",
			false,
		},
		{
			"replicationcontroller/thing",
			"",
			"",
			true,
		},
		{
			"daemonset/thing1/thing2",
			"",
			"",
			true,
		},
	}
	c := fake.NewSimpleClientset()
	for _, tc := range testCases {
		target, err := makeTarget(c, tc.target, "test")
		if err != nil && !tc.expError {
			t.Errorf("expect no error, got error for target: %q: %v", tc.target, err)
			continue
		} else if err == nil && tc.expError {
			t.Errorf("expect error, got no error for target: %q", tc.target)
			continue
		} else if tc.expKind != target.kind {
			t.Errorf("expected kind: %q, got %q", tc.expKind, target.kind)
			continue
		} else if tc.expName != target.name {
			t.Errorf("expected name: %q, got %q", tc.expName, target.name)
			continue
		}
	}
}

func TestUpdateResources(t *testing.T) {
	testCases := []struct {
		target   string
		res      int
		expError bool
	}{
		{
			"deployment/thing",
			10,
			false,
		},
		{
			"daemonset/thing",
			20,
			false,
		},
		{
			"replicaset/thing",
			30,
			false,
		},
	}
	for _, tc := range testCases {
		c := fake.NewSimpleClientset()
		target, err := makeTarget(c, tc.target, "test")
		if err != nil {
			t.Errorf("failed to make target %q: %v", target, err)
		}

		k8scli := k8sClient{
			clientset: c,
			target:    target,
		}
		newReqs := map[string]apiv1.ResourceRequirements{}
		newReqs["thing"] = apiv1.ResourceRequirements{
			Requests: map[apiv1.ResourceName]resource.Quantity{},
			Limits:   map[apiv1.ResourceName]resource.Quantity{},
		}
		r := resource.NewQuantity(0, resource.BinarySI)
		r.SetMilli(10)
		newReqs["thing"].Requests[apiv1.ResourceName("cpu")] = *r
		if err := k8scli.UpdateResources(newReqs); err != nil {
			t.Errorf("failed to update resources for target %q: %v", target, err)
		}
	}
}
