// Copyright 2021 xgfone
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loadbalancer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// Predefine some errors.
var (
	ErrNoAvailableEndpoint = errors.New("no available endpoints")
)

var _ EndpointUpdater = &LoadBalancer{}
var _ LoadBalancerRoundTripper = &LoadBalancer{}

// LoadBalancer is a group of endpoints that can handle the same request,
// which will forward the request to any endpoint to handle it.
type LoadBalancer struct {
	Provider

	fr     FailRetry
	frLock sync.RWMutex
	hasfr  uint32
	name   string
}

// NewLoadBalancer returns a new LoadBalancer.
//
// provider is equal to NewProvider("", nil) by default.
func NewLoadBalancer(name string, provider Provider) *LoadBalancer {
	if provider == nil {
		provider = NewGeneralProvider(nil)
	}

	return &LoadBalancer{Provider: provider, name: name}
}

// Name returns the name of the loadbalancer.
func (lb *LoadBalancer) Name() string { return lb.name }

// String implements the interface fmt.Stringer.
func (lb *LoadBalancer) String() string {
	if name := lb.Name(); name != "" {
		return fmt.Sprintf("LoadBalancer(name=%s, provider=%s)", name, lb.Provider.String())
	}
	return fmt.Sprintf("LoadBalancer(provider=%s)", lb.Provider.String())
}

// AddEndpoint implements the interface EndpointManager.
func (lb *LoadBalancer) AddEndpoint(ep Endpoint) {
	lb.Provider.(EndpointManager).AddEndpoint(ep)
}

// DelEndpoint implements the interface EndpointManager.
func (lb *LoadBalancer) DelEndpoint(ep Endpoint) {
	lb.Provider.(EndpointManager).DelEndpoint(ep)
}

func (lb *LoadBalancer) GetFailRetry() (failretry FailRetry) {
	if atomic.LoadUint32(&lb.hasfr) == 1 {
		lb.frLock.RLock()
		failretry = lb.fr
		lb.frLock.RUnlock()
	}
	return
}

// SetFailRetry sets the fail retry.
//
// If failretry is equal to nil, unset it to disable the fail retry.
func (lb *LoadBalancer) SetFailRetry(failretry FailRetry) {
	lb.frLock.Lock()
	lb.fr = failretry
	if failretry == nil {
		atomic.StoreUint32(&lb.hasfr, 0)
	} else {
		atomic.StoreUint32(&lb.hasfr, 1)
	}
	lb.frLock.Unlock()
}

// RoundTrip selects an endpoint, then call it. If failed, it will retry it
// by the fail handler if it's set.
func (lb *LoadBalancer) RoundTrip(c context.Context, r Request) (resp interface{}, err error) {
	ep := lb.Provider.Select(r, true)
	if ep == nil {
		return nil, ErrNoAvailableEndpoint
	}

	if resp, err = ep.RoundTrip(c, r); err == nil {
		return resp, nil
	} else if failRetry := lb.GetFailRetry(); failRetry != nil {
		resp, err = failRetry.Retry(c, lb.Provider, r, ep, err)
	}

	return
}
