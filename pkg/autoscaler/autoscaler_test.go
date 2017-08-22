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

package autoscaler

import (
	"encoding/json"
	"fmt"
	"log"
	"testing"
	"time"

	k8sclient "github.com/kubernetes-incubator/cluster-proportional-vertical-autoscaler/pkg/autoscaler/k8sclient/testing"
	"k8s.io/apimachinery/pkg/util/clock"
)

func TestRun(t *testing.T) {
	var asConfig = `
{
  "fake-agent": {
    "requests": {
      "cpu": {
        "base": "10m", "increment":"1m", "coresPerIncrement":1
      },
      "memory": {
        "base": "8Mi", "increment":"1Mi", "coresPerIncrement":1
      }
    }
  }
}
`
	mockK8s := k8sclient.MockK8sClient{
		NumOfNodes: 4,
		NumOfCores: 7,
	}

	fakeClock := clock.NewFakeClock(time.Now())
	fakePollPeriod := 5 * time.Second
	cfg := ScaleConfig{}
	if err := json.Unmarshal([]byte(asConfig), &cfg); err != nil {
		log.Fatalf("invalid default config: %v", err)
	}
	autoScaler := &AutoScaler{
		k8sClient:     &mockK8s,
		defaultConfig: cfg,
		configFile:    asConfig,
		pollPeriod:    fakePollPeriod,
		clock:         fakeClock,
		stopCh:        make(chan struct{}),
		readyCh:       make(chan<- struct{}, 1),
	}

	go autoScaler.Run()
	defer close(autoScaler.stopCh)
}

func TestCalculatePerCores(t *testing.T) {
	var coresPerIncrement = `
{
  "fake-agent": {
    "requests": {
      "cpu": {
        "base": "%dm", "increment":"%dm", "coresPerIncrement":%d
      }
    }
  }
}
`
	for _, tt := range []struct {
		name         string
		numNodes     int
		numCores     int
		expVal       int64
		base         int
		increment    int
		perIncrement int
	}{
		{
			"base 10, increment 1,  per increment 1",
			4,
			7,
			17,
			10,
			1,
			1,
		},
		{
			"base 10, increment 2, per increment 1",
			4,
			7,
			24,
			10,
			2,
			1,
		},
		{
			"base 10, increment 2, per increment 2",
			4,
			20,
			30,
			10,
			2,
			2,
		},
		{
			"base 10, increment 4, per increment 3",
			4,
			20,
			20,
			10,
			1,
			2,
		},
		{
			"base 10, increment 1, per increment 0",
			4,
			20,
			10,
			10,
			1,
			0,
		},
		{
			"base 10, increment 1, per increment -22",
			4,
			20,
			10,
			10,
			1,
			-2,
		},
	} {
		mockK8s := k8sclient.MockK8sClient{
			NumOfNodes: tt.numNodes,
			NumOfCores: tt.numCores,
		}
		conf := fmt.Sprintf(coresPerIncrement, tt.base, tt.increment, tt.perIncrement)
		cfg := ScaleConfig{}
		if err := json.Unmarshal([]byte(conf), &cfg); err != nil {
			t.Fatalf("invalid default config: %v", err)
		}

		sz, err := mockK8s.GetClusterSize()
		if err != nil {
			t.Errorf("failed to get cluster size")
		}
		val := calculate(cfg["fake-agent"].Requests["cpu"], sz)
		if val != tt.expVal {
			t.Errorf("expected %d got %d", tt.expVal, val)
		}
	}
}

func TestCalculatePerNodes(t *testing.T) {
	var nodesPerIncrement = `
{
  "fake-agent": {
    "requests": {
      "cpu": {
        "base": "%dm", "increment":"%dm", "nodesPerIncrement":%d
      }
    }
  }
}
`
	for _, tt := range []struct {
		name         string
		numNodes     int
		numCores     int
		expVal       int64
		base         int
		increment    int
		perIncrement int
	}{
		{
			"base 10, increment 1,  per increment 1",
			4,
			7,
			14,
			10,
			1,
			1,
		},
		{
			"base 10, increment 2, per increment 1",
			4,
			7,
			18,
			10,
			2,
			1,
		},
		{
			"base 10, increment 2, per increment 2",
			4,
			20,
			14,
			10,
			2,
			2,
		},
		{
			"base 10, increment 4, per increment 3",
			4,
			20,
			12,
			10,
			1,
			2,
		},
		{
			"base 10, increment 1, per increment 0",
			4,
			20,
			10,
			10,
			1,
			0,
		},
		{
			"base 10, increment 1, per increment -2",
			4,
			20,
			10,
			10,
			1,
			-2,
		},
	} {
		mockK8s := k8sclient.MockK8sClient{
			NumOfNodes: tt.numNodes,
			NumOfCores: tt.numCores,
		}
		conf := fmt.Sprintf(nodesPerIncrement, tt.base, tt.increment, tt.perIncrement)
		cfg := ScaleConfig{}
		if err := json.Unmarshal([]byte(conf), &cfg); err != nil {
			t.Fatalf("invalid default config: %v", err)
		}

		sz, err := mockK8s.GetClusterSize()
		if err != nil {
			t.Errorf("failed to get cluster size")
		}
		val := calculate(cfg["fake-agent"].Requests["cpu"], sz)
		if val != tt.expVal {
			t.Errorf("expected %d got %d", tt.expVal, val)
		}
	}
}
