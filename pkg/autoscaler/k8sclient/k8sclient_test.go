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
	"net/http"
	"net/http/httptest"
	"testing"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var obj interface{}
		stable := metav1.APIResourceList{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Namespaced: true, Kind: "Deployment"},
				{Name: "daemonsets", Namespaced: true, Kind: "DaemonSet"},
				{Name: "replicasets", Namespaced: true, Kind: "ReplicaSet"},
			},
		}
		switch req.URL.Path {
		case "/api":
			obj = &metav1.APIVersions{
				Versions: []string{
					"v1",
				},
			}
		case "/api/v1":
			obj = &stable
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		output, err := json.Marshal(obj)
		if err != nil {
			t.Fatalf("unexpected encoding error: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(output)
	}))
	defer server.Close()

	for _, tc := range testCases {
		_, _, err := discoverAPI(
			clientset.NewForConfigOrDie(&restclient.Config{
				Host: server.URL,
				ContentConfig: restclient.ContentConfig{
					GroupVersion: &schema.GroupVersion{Group: tc.kind, Version: "v1"}}}),
			tc.kind)

		if err != nil && !tc.expError {
			t.Errorf("Expect no error, got error for kind: %q: %v", tc.kind, err)
			continue
		} else if err == nil && tc.expError {
			t.Errorf("Expect error, got no error for kind: %q", tc.kind)
			continue
		}
	}
}

func TestUpdateResources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var obj interface{}
		stable := metav1.APIResourceList{
			GroupVersion: "extensions/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Namespaced: true, Kind: "Deployment"},
				{Name: "daemonsets", Namespaced: true, Kind: "DaemonSet"},
				{Name: "replicasets", Namespaced: true, Kind: "ReplicaSet"},
			},
		}
		switch req.URL.Path {
		case "/api":
			obj = &metav1.APIVersions{
				Versions: []string{
					"extensions/v1beta1",
				},
			}
		case "/apis/extensions/v1beta1":
			obj = &stable
		case "/apis/extensions/v1beta1/namespaces/default/daemonsets/thing":
		case "/apis/extensions/v1beta1/namespaces/default/replicasets/thing":
		case "/apis/extensions/v1beta1/namespaces/default/deployments/thing":
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		output, err := json.Marshal(obj)
		if err != nil {
			t.Fatalf("unexpected encoding error: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(output)
	}))
	defer server.Close()

	testCases := []struct {
		target   string
		kind     string
		res      int
		expError bool
	}{
		{
			"deployment/thing",
			"deployment",
			10,
			false,
		},
		{

			"daemonset/thing",
			"daemonSet",
			20,
			false,
		},
		{
			"replicaset/thing",
			"replicaSet",
			30,
			false,
		},
	}

	for _, tc := range testCases {
		client := clientset.NewForConfigOrDie(&restclient.Config{
			Host: server.URL,
			ContentConfig: restclient.ContentConfig{
				GroupVersion: &schema.GroupVersion{Group: tc.kind, Version: "extensions/v1beta1"}}})

		target, err := makeTarget(client, tc.target, "default")
		if err != nil {
			t.Fatalf("error making target %q: %v", tc.target, err)
		}
		k8scli := &k8sClient{
			clientset: client,
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
			t.Errorf("failed to update resources for target %q: %v", tc.target, err)
		}
	}
}
