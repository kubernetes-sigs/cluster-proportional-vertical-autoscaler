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
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"time"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/clock"

	"github.com/kubernetes-sigs/cluster-proportional-vertical-autoscaler/cmd/cpvpa/options"
	"github.com/kubernetes-sigs/cluster-proportional-vertical-autoscaler/pkg/autoscaler/k8sclient"

	"github.com/golang/glog"
)

// AutoScaler determines the number of replicas to run
type AutoScaler struct {
	k8sClient     k8sclient.K8sClient
	defaultConfig ScaleConfig
	configFile    string
	lastFileInfo  os.FileInfo
	currentConfig ScaleConfig
	lastReqs      map[string]apiv1.ResourceRequirements
	pollPeriod    time.Duration
	clock         clock.WithTicker
	stopCh        chan struct{}
	readyCh       chan<- struct{} // For testing.
}

// NewAutoScaler returns a new AutoScaler
func NewAutoScaler(c *options.AutoScalerConfig) (*AutoScaler, error) {
	newK8sClient, err := k8sclient.NewK8sClient(c.Namespace, c.Target, c.Kubeconfig, c.DryRun)
	if err != nil {
		return nil, err
	}
	cfg := ScaleConfig{}
	if c.DefaultConfig != "" {
		if err := json.Unmarshal([]byte(c.DefaultConfig), &cfg); err != nil {
			return nil, fmt.Errorf("invalid default config: %v", err)
		}
	}
	return &AutoScaler{
		k8sClient:     newK8sClient,
		defaultConfig: cfg,
		configFile:    c.ConfigFile,
		pollPeriod:    time.Second * time.Duration(c.PollPeriodSeconds),
		clock:         clock.RealClock{},
		stopCh:        make(chan struct{}),
		readyCh:       make(chan struct{}, 1),
	}, nil
}

// Run periodically counts the number of nodes and cores, estimates the expected
// number of replicas, compares them to the actual replicas, and
// updates the target resource with the expected replicas if necessary.
func (s *AutoScaler) Run() {
	ticker := s.clock.NewTicker(s.pollPeriod)
	s.readyCh <- struct{}{} // For testing.

	// Don't wait for ticker and execute pollAPIServer() for the first time.
	s.pollAPIServer()

	for {
		select {
		case <-ticker.C():
			s.pollAPIServer()
		case <-s.stopCh:
			return
		}
	}
}

func (s *AutoScaler) pollAPIServer() {
	// Query the apiserver for the cluster status --- number of nodes and cores
	clusterSize, err := s.k8sClient.GetClusterSize()
	if err != nil {
		glog.Errorf("Error getting cluster size: %v", err)
		return
	}
	glog.V(4).Infof("Nodes %5d", clusterSize.Nodes)
	glog.V(4).Infof("Cores %5d", clusterSize.Cores)

	fileBytes, err := s.readConfigFileIfChanged()
	if err != nil {
		glog.Errorf("Failed to read config file %q: %v", s.configFile, err)
		return
	}
	if s.currentConfig == nil || len(fileBytes) > 0 {
		cfg := s.defaultConfig.DeepCopy()
		if len(fileBytes) > 0 {
			if err := json.Unmarshal(fileBytes, &cfg); err != nil {
				glog.Errorf("Failed to unmarshal config file %q: %v", s.configFile, err)
				return
			}
		}
		s.currentConfig = cfg
		glog.V(0).Infof("setting config = %s", s.currentConfig)
	}

	newReqs := map[string]apiv1.ResourceRequirements{}
	for ctr, ctrcfg := range s.currentConfig {
		newReqs[ctr] = apiv1.ResourceRequirements{
			Requests: map[apiv1.ResourceName]resource.Quantity{},
			Limits:   map[apiv1.ResourceName]resource.Quantity{},
		}
		for res, cfg := range ctrcfg.Requests {
			want := calculate(cfg, clusterSize)
			r := resource.NewQuantity(0, guessFormat(res))
			r.SetMilli(want)
			newReqs[ctr].Requests[apiv1.ResourceName(res)] = *r
			glog.V(4).Infof("Calculated %s requests[%q] = %v", ctr, res, r)
		}
		for res, cfg := range ctrcfg.Limits {
			want := calculate(cfg, clusterSize)
			r := resource.NewQuantity(0, guessFormat(res))
			r.SetMilli(want)
			newReqs[ctr].Limits[apiv1.ResourceName(res)] = *r
			glog.V(4).Infof("Calculated %s limits[%q] = %v", ctr, res, r)
		}
	}
	if reflect.DeepEqual(s.lastReqs, newReqs) {
		return
	}

	glog.V(0).Infof("Updating resource for nodes: %d, cores: %d",
		clusterSize.Nodes, clusterSize.Cores)
	logRequirements(newReqs)
	// Update resource target with new resources.
	if err = s.k8sClient.UpdateResources(newReqs); err != nil {
		glog.Errorf("Update failure: %s", err)
	} else {
		s.lastReqs = newReqs
	}
}

func logRequirements(reqs map[string]apiv1.ResourceRequirements) {
	for ctr, req := range reqs {
		for res, r := range req.Requests {
			glog.V(0).Infof("Setting %s requests[%q] = %v", ctr, res, &r)
		}
		for res, r := range req.Limits {
			glog.V(0).Infof("Setting %s limits[%q] = %v", ctr, res, &r)
		}
	}
}

func (s *AutoScaler) readConfigFileIfChanged() ([]byte, error) {
	if s.configFile == "" {
		return nil, nil
	}

	fi, err := os.Stat(s.configFile)
	if err != nil {
		return nil, fmt.Errorf("can't stat file %s: %v", s.configFile, err)
	}
	if os.SameFile(fi, s.lastFileInfo) {
		return nil, nil
	}
	s.lastFileInfo = fi
	fb, err := os.ReadFile(s.configFile)
	if err != nil {
		return nil, fmt.Errorf("can't read file %s: %v", s.configFile, err)
	}
	return fb, nil
}

func calculate(cfg ResourceScaleConfig, cluster *k8sclient.ClusterSize) int64 {
	var base int64
	if cfg.Base != nil {
		base = asInt64(cfg.Base)
	}
	var max int64
	if cfg.Max != nil {
		max = asInt64(cfg.Max)
	}
	var step int64
	if cfg.Step != nil {
		step = asInt64(cfg.Step)
	}
	var cpi int
	if cfg.CoresPerStep != nil {
		cpi = *cfg.CoresPerStep
	}
	var npi int
	if cfg.NodesPerStep != nil {
		npi = *cfg.NodesPerStep
	}
	wantByCores := base + (step * int64(increments(cluster.Cores, cpi)))
	if max > 0 && wantByCores > max {
		wantByCores = max
	}
	wantByNodes := base + (step * int64(increments(cluster.Nodes, npi)))
	if max > 0 && wantByNodes > max {
		wantByNodes = max
	}
	want := wantByCores
	if wantByNodes > want {
		want = wantByNodes
	}
	return want
}

func asInt64(q *resource.Quantity) int64 {
	if q.Value() > (math.MaxInt64 / int64(1000)) {
		panic(fmt.Sprintf("can't convert quantity %s to int64 milli-units", q))
	}
	return q.MilliValue()
}

func increments(count int, per int) int {
	if per == 0 {
		return 0
	}
	if per == 1 {
		return count
	}
	return (count + (per - 1)) / per
}

func guessFormat(res string) resource.Format {
	switch res {
	case string(apiv1.ResourceMemory), string(apiv1.ResourceStorage):
		return resource.DecimalSI
	}
	return resource.BinarySI
}

// ScaleConfig maps container names to per-container configs.
type ScaleConfig map[string]ContainerScaleConfig

// ContainmerScaleConfig holds per-container per-resource configs.
type ContainerScaleConfig struct {
	Requests map[string]ResourceScaleConfig
	Limits   map[string]ResourceScaleConfig
}

// ResourceScaleConfig holds the coefficients for a single resource scaling
// function. The final result will be the base plus the larger of the by-cores
// scaling and the by-nodes scaling, bounded by the max value.
//
// Example:
//   Base = 10
//   Max = 100
//   Step = 2
//   CoresPerStep = 4
//   NodesPerStep = 2
//
//   The core and node counts are rounded up to the next whole step.
//
//   If we find 64 cores and 4 nodes we get scalars of:
//     by-cores: 10 + (2 * (round(64, 4)/4)) = 10 + 32 = 42
//     by-nodes: 10 + (2 * (round(4, 2)/2)) = 10 + 4 = 14
//   The larger is by-cores, and it is less than Max, so the final value is 42.
//
//   If we find 3 cores and 3 nodes we get scalars of:
//     by-cores: 10 + (2 * (round(3, 4)/4)) = 10 + 2 = 12
//     by-nodes: 10 + (2 * (round(3, 2)/2)) = 10 + 4 = 14
type ResourceScaleConfig struct {
	// The baseline quantity required.
	Base *resource.Quantity
	// The maximum allowed quantity.
	Max *resource.Quantity
	// The amount of additional resources to grow by.  If this is too
	// fine-grained, the resizing action will happen too frequently.
	Step *resource.Quantity
	// The number of cores required to trigger an increase.
	CoresPerStep *int
	// The number of nodes required to trigger an increase.
	NodesPerStep *int
}

func (sc ScaleConfig) String() string {
	var buf bytes.Buffer
	buf.WriteString("{ ")
	for k, v := range sc {
		buf.WriteString(fmt.Sprintf("[%s]: %s, ", k, v))
	}
	buf.WriteString("}")
	return buf.String()
}

func (csc ContainerScaleConfig) String() string {
	var buf bytes.Buffer
	buf.WriteString("{ requests: { ")
	for k, v := range csc.Requests {
		buf.WriteString(fmt.Sprintf("[%s]: %s, ", k, v))
	}
	buf.WriteString("}, limits: { ")
	for k, v := range csc.Limits {
		buf.WriteString(fmt.Sprintf("[%s]: %s", k, v))
	}
	buf.WriteString("} }")
	return buf.String()
}

func (rsc ResourceScaleConfig) String() string {
	var buf bytes.Buffer
	buf.WriteString("{ ")
	if rsc.Base != nil {
		buf.WriteString(fmt.Sprintf("base=%s ", rsc.Base.String()))
	}
	if rsc.Max != nil {
		buf.WriteString(fmt.Sprintf("max=%s ", rsc.Max.String()))
	}
	if rsc.Step != nil {
		buf.WriteString(fmt.Sprintf("incr=%s ", rsc.Step.String()))
	}
	if rsc.CoresPerStep != nil {
		buf.WriteString(fmt.Sprintf("cores_incr=%d ", *rsc.CoresPerStep))
	}
	if rsc.NodesPerStep != nil {
		buf.WriteString(fmt.Sprintf("nodes_incr=%d ", *rsc.NodesPerStep))
	}
	buf.WriteString("}")
	return buf.String()
}

func (sc ScaleConfig) DeepCopy() ScaleConfig {
	out := ScaleConfig{}
	for k, v := range sc {
		out[k] = v.DeepCopy()
	}
	return out
}

func (csc ContainerScaleConfig) DeepCopy() ContainerScaleConfig {
	out := ContainerScaleConfig{
		Requests: map[string]ResourceScaleConfig{},
		Limits:   map[string]ResourceScaleConfig{},
	}
	for k, v := range csc.Requests {
		out.Requests[k] = v.DeepCopy()
	}
	for k, v := range csc.Limits {
		out.Limits[k] = v.DeepCopy()
	}
	return out

}

func (rsc ResourceScaleConfig) DeepCopy() ResourceScaleConfig {
	out := ResourceScaleConfig{}

	if rsc.Base != nil {
		q := rsc.Base.DeepCopy()
		out.Base = &q
	}
	if rsc.Max != nil {
		q := rsc.Max.DeepCopy()
		out.Max = &q
	}
	if rsc.Step != nil {
		q := rsc.Step.DeepCopy()
		out.Step = &q
	}
	if rsc.CoresPerStep != nil {
		out.CoresPerStep = new(int)
		*out.CoresPerStep = *rsc.CoresPerStep
	}
	if rsc.NodesPerStep != nil {
		out.NodesPerStep = new(int)
		*out.NodesPerStep = *rsc.NodesPerStep
	}
	return out
}
