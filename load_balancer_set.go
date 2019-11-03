// Copyright 2019 xgfone
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

package service

import (
	"sync"
	"time"
)

// LoadBalancerSet is a set of LoadBalancers.
type LoadBalancerSet struct {
	lock  sync.RWMutex
	lbset map[string]*LoadBalancer

	provider     Provider
	session      SessionManager
	failRetry    FailRetry
	failInterval time.Duration
}

// NewLoadBalancerSet returns a new LoadBalancerSet with the default provider,
// sessionManager, failHandler, and failInterval.
func NewLoadBalancerSet(provider Provider, session SessionManager,
	failRetry FailRetry, failInterval time.Duration) *LoadBalancerSet {
	return &LoadBalancerSet{
		lbset: make(map[string]*LoadBalancer, 8),

		provider:     provider,
		session:      session,
		failRetry:    failRetry,
		failInterval: failInterval,
	}
}

// GetAllNames returns the names of all the loadbalancers.
func (lbs *LoadBalancerSet) GetAllNames() []string {
	lbs.lock.RLock()
	names := make([]string, 0, len(lbs.lbset))
	for name := range lbs.lbset {
		names = append(names, name)
	}
	lbs.lock.RUnlock()
	return names
}

// GetLoadBalancer returns the LoadBalancer by the name.
//
// Return nil if the loadbalancer does not exist.
func (lbs *LoadBalancerSet) GetLoadBalancer(name string) *LoadBalancer {
	lbs.lock.RLock()
	lb := lbs.lbset[name]
	lbs.lock.RUnlock()
	return lb
}

// AddLoadBalancer adds the LoadBalancer with the name.
func (lbs *LoadBalancerSet) AddLoadBalancer(name string, lb *LoadBalancer) {
	lbs.lock.Lock()
	lbs.lbset[name] = lb
	lbs.lock.Unlock()
}

// DelLoadBalancer removes and returns the LoadBalancer by the name.
//
// Return nil if the loadbalancer does not exist.
func (lbs *LoadBalancerSet) DelLoadBalancer(name string) *LoadBalancer {
	lbs.lock.Lock()
	lb := lbs.lbset[name]
	delete(lbs.lbset, name)
	lbs.lock.Unlock()
	return lb
}

// GetOrNewLoadBalancer is the same as Get(name), but new a LoadBalancer
// and return it instead of returning nil.
func (lbs *LoadBalancerSet) GetOrNewLoadBalancer(name string) *LoadBalancer {
	lbs.lock.Lock()
	lb, ok := lbs.lbset[name]
	if !ok {
		lb = NewLoadBalancer(lbs.provider)
		lb.Session = lbs.session
		lb.FailRetry = lbs.failRetry
		lb.FailInterval = lbs.failInterval
		lbs.lbset[name] = lb
	}
	lbs.lock.Unlock()
	return lb
}

// StatusLoadBalancerSet is a set of StatusLoadBalancers.
type StatusLoadBalancerSet struct {
	lock  sync.RWMutex
	lbset map[string]*StatusLoadBalancer

	provider     Provider
	session      SessionManager
	failRetry    FailRetry
	failInterval time.Duration
}

// NewStatusLoadBalancerSet returns a new StatusLoadBalancerSet with the default
// provider, sessionManager, failHandler, and failInterval.
func NewStatusLoadBalancerSet(provider Provider, session SessionManager,
	failRetry FailRetry, failInterval time.Duration) *StatusLoadBalancerSet {
	return &StatusLoadBalancerSet{
		lbset: make(map[string]*StatusLoadBalancer, 8),

		provider:     provider,
		session:      session,
		failRetry:    failRetry,
		failInterval: failInterval,
	}
}

// GetAllNames returns the names of all the StatusLoadBalancers.
func (slbs *StatusLoadBalancerSet) GetAllNames() []string {
	slbs.lock.RLock()
	names := make([]string, 0, len(slbs.lbset))
	for name := range slbs.lbset {
		names = append(names, name)
	}
	slbs.lock.RUnlock()
	return names
}

// GetStatusLoadBalancer returns the StatusLoadBalancer by the name.
//
// Return nil if the StatusLoadBalancer does not exist.
func (slbs *StatusLoadBalancerSet) GetStatusLoadBalancer(name string) *StatusLoadBalancer {
	slbs.lock.RLock()
	slb := slbs.lbset[name]
	slbs.lock.RUnlock()
	return slb
}

// AddStatusLoadBalancer adds the StatusLoadBalancer with the name.
func (slbs *StatusLoadBalancerSet) AddStatusLoadBalancer(name string, slb *StatusLoadBalancer) {
	slbs.lock.Lock()
	slbs.lbset[name] = slb
	slbs.lock.Unlock()
}

// DelStatusLoadBalancer removes and returns the StatusLoadBalancer by the name.
//
// Return nil if the StatusLoadBalancer does not exist.
func (slbs *StatusLoadBalancerSet) DelStatusLoadBalancer(name string) *StatusLoadBalancer {
	slbs.lock.Lock()
	slb := slbs.lbset[name]
	delete(slbs.lbset, name)
	slbs.lock.Unlock()
	return slb
}

// GetOrNewStatusLoadBalancer is the same as Get(name), but new a StatusLoadBalancer
// and return it instead of returning nil.
func (slbs *StatusLoadBalancerSet) GetOrNewStatusLoadBalancer(name string) *StatusLoadBalancer {
	slbs.lock.Lock()
	slb, ok := slbs.lbset[name]
	if !ok {
		slb = NewStatusLoadBalancer(slbs.provider)
		slb.Session = slbs.session
		slb.FailRetry = slbs.failRetry
		slb.FailInterval = slbs.failInterval
		slbs.lbset[name] = slb
	}
	slbs.lock.Unlock()
	return slb
}
