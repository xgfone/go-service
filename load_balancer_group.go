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
)

type groupT = string
type endpointT = string
type loadBalancerWrapper struct {
	Group        string
	Endpoints    map[string]Endpoint
	LoadBalancer *LoadBalancer
}

// LoadBalancerGroup is used to group the LoadBalancer.
type LoadBalancerGroup struct {
	// NewLoadBalancer is used to new the LoadBalancer by AddGroup.
	//
	// It returns NewLoadBalancer(nil) by default.
	NewLoadBalancer func(group string) *LoadBalancer

	// OnEndpoint is used to notice someone, such as Updater, that an endpoint
	// is added or deleted.
	//
	// If nil, it does nothing.
	OnEndpoint Updater

	updater Updater
	lock    sync.RWMutex
	lbs     map[groupT]*loadBalancerWrapper
	eps     map[endpointT]map[groupT]*loadBalancerWrapper
}

// NewLoadBalancerGroup returns a new LoadBalancerGroup.
func NewLoadBalancerGroup() *LoadBalancerGroup {
	lbg := LoadBalancerGroup{
		lbs: make(map[groupT]*loadBalancerWrapper, 16),
		eps: make(map[endpointT]map[groupT]*loadBalancerWrapper, 32),
	}
	lbg.updater = UpdaterFunc(lbg.updateEndpoint)
	lbg.NewLoadBalancer = func(string) *LoadBalancer { return NewLoadBalancer(nil) }
	return &lbg
}

func (lbg *LoadBalancerGroup) updateEndpoint(add bool, endpoint Endpoint) {
	ep := endpoint.String()

	lbg.lock.RLock()
	defer lbg.lock.RUnlock()

	if lbws, ok := lbg.eps[ep]; ok && len(lbws) > 0 {
		if add {
			for _, lbw := range lbws {
				lbw.LoadBalancer.EndpointManager().AddEndpoint(endpoint)
			}
		} else {
			for _, lbw := range lbws {
				lbw.LoadBalancer.EndpointManager().DelEndpointByString(ep)
			}
		}
	}
}

// Updater returns an Updater to update the endpoint.
//
// Notice: the Provider of LoadBalancer must have implemented the interface
// ProviderEndpointManager.
func (lbg *LoadBalancerGroup) Updater() Updater { return lbg.updater }

// AddGroup adds the LoadBalancer group, and the group will associate
// LoadBalancer and Endpoint together.
//
// Notice: the group has existed, it does nothing and returns false.
// Or, create a new LoadBalancer with the group and return true..
func (lbg *LoadBalancerGroup) AddGroup(group string) bool {
	lbg.lock.Lock()
	_, ok := lbg.lbs[group]
	if !ok {
		lb := lbg.NewLoadBalancer(group)
		eps := make(map[string]Endpoint, 8)
		lbg.lbs[group] = &loadBalancerWrapper{Group: group, Endpoints: eps, LoadBalancer: lb}
	}
	lbg.lock.Unlock()
	return !ok
}

// DelGroup deletes the LoadBalancer group and returns the unused endpoints,
// which should be removed from the centralized health checker.
func (lbg *LoadBalancerGroup) DelGroup(group string) (endpoints Endpoints) {
	endpoints = Endpoints{}
	lbg.lock.Lock()
	if lbw, ok := lbg.lbs[group]; ok {
		delete(lbg.lbs, group)
		for ep, endpoint := range lbw.Endpoints {
			// lbw.LoadBalancer.EndpointManager().DelEndpointByString(ep)
			if eps, ok := lbg.eps[ep]; ok {
				delete(eps, group)
				if len(eps) == 0 {
					delete(lbg.eps, ep)
					endpoints = append(endpoints, endpoint)
				}
			}
		}
	}
	lbg.lock.Unlock()

	if lbg.OnEndpoint != nil && len(endpoints) > 0 {
		for _, ep := range endpoints {
			lbg.OnEndpoint.DelEndpoint(ep)
		}
	}
	return
}

// AddEndpoint associates the endpoint with the group and reports whether it is
// successful.
func (lbg *LoadBalancerGroup) AddEndpoint(group string, endpoint Endpoint) bool {
	ep := endpoint.String()

	lbg.lock.Lock()
	lbw, ok := lbg.lbs[group]
	if ok {
		lbw.Endpoints[ep] = endpoint
		if lbws, ok := lbg.eps[ep]; ok {
			if _lbw, ok := lbws[group]; ok {
				if _lbw != lbw {
					lbg.lock.Unlock()
					panic("LoadBalancerGroup: invalid reference relationship")
				}
			} else {
				lbws[group] = lbw
			}
		} else {
			lbg.eps[ep] = map[string]*loadBalancerWrapper{group: lbw}
		}
	}
	lbg.lock.Unlock()

	if ok && lbg.OnEndpoint != nil {
		lbg.OnEndpoint.AddEndpoint(endpoint)
	}
	return ok
}

// DelEndpoint unassociates the endpoints from the group and reports whether it is
// successful.
func (lbg *LoadBalancerGroup) DelEndpoint(endpoint Endpoint) bool {
	ep := endpoint.String()
	lbg.lock.Lock()
	lbws, ok := lbg.eps[ep]
	if ok {
		delete(lbg.eps, ep)
		for _, lbw := range lbws {
			delete(lbw.Endpoints, ep)
			lbw.LoadBalancer.EndpointManager().DelEndpointByString(ep)
		}
	}
	lbg.lock.Unlock()

	if ok && lbg.OnEndpoint != nil {
		lbg.OnEndpoint.DelEndpoint(endpoint)
	}
	return ok
}

// DelEndpointByString deletes the endpoints from all LoadBalancers.
//
// Notice: If you can, use DelEndpoint instead.
func (lbg *LoadBalancerGroup) DelEndpointByString(endpoint string) bool {
	return lbg.DelEndpoint(NewNoopEndpoint(endpoint))
}

// GetLoadBalancer returns the LoadBalancer by the group name.
//
// Return nil if the group does not exist.
func (lbg *LoadBalancerGroup) GetLoadBalancer(group string) *LoadBalancer {
	lbg.lock.RLock()
	lbw, ok := lbg.lbs[group]
	lbg.lock.RUnlock()

	if ok {
		return lbw.LoadBalancer
	}
	return nil
}

// GetAllGroups returns the names of all the groups.
func (lbg *LoadBalancerGroup) GetAllGroups() []string {
	lbg.lock.RLock()
	groups := make([]string, 0, len(lbg.lbs))
	for group := range lbg.lbs {
		groups = append(groups, group)
	}
	lbg.lock.RUnlock()
	return groups
}

// GetEndpoints returns all the endpoints of the given group.
//
// Notice: for obtaining the real endpoint, you should call the method Unwrap()
// of the returned endpoints.
func (lbg *LoadBalancerGroup) GetEndpoints(group string) (eps Endpoints) {
	lbg.lock.RLock()
	if lbw, ok := lbg.lbs[group]; ok {
		eps = make(Endpoints, 0, len(lbw.Endpoints))
		for _, ep := range lbw.Endpoints {
			healthy := lbw.LoadBalancer.IsActive(ep)
			eps = append(eps, statusEnpoind{Endpoint: ep, healthy: healthy})
		}
	}
	lbg.lock.RUnlock()
	return
}

// GetAllEndpoints returns all the endpoints of all the group.
//
// Notice: for obtaining the real endpoint, you should call the method Unwrap()
// of the returned endpoints.
func (lbg *LoadBalancerGroup) GetAllEndpoints() (epms map[string]Endpoints) {
	lbg.lock.RLock()
	epms = make(map[string]Endpoints)
	for _, lbw := range lbg.lbs {
		eps := make(Endpoints, 0, len(lbw.Endpoints))
		for _, ep := range lbw.Endpoints {
			healthy := lbw.LoadBalancer.IsActive(ep)
			eps = append(eps, statusEnpoind{Endpoint: ep, healthy: healthy})
		}
		epms[lbw.Group] = eps
	}

	lbg.lock.RUnlock()
	return
}
