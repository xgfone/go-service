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
type endpoint2lbsWrapper struct {
	Endpoint      Endpoint
	LoadBalancers map[groupT]*loadBalancerWrapper
}
type loadBalancerWrapper struct {
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
	eps     map[endpointT]*endpoint2lbsWrapper
}

// NewLoadBalancerGroup returns a new LoadBalancerGroup.
func NewLoadBalancerGroup() *LoadBalancerGroup {
	lbg := LoadBalancerGroup{
		lbs: make(map[groupT]*loadBalancerWrapper, 16),
		eps: make(map[endpointT]*endpoint2lbsWrapper, 32),
	}
	lbg.updater = UpdaterFunc(lbg.updateEndpoint)
	return &lbg
}

func (lbg *LoadBalancerGroup) newLoadBalancer(group string) (lb *LoadBalancer) {
	if lbg.NewLoadBalancer != nil {
		lb = lbg.newLoadBalancer(group)
	} else {
		lb = NewLoadBalancer(nil)
	}

	if lb.Name == "" {
		lb.Name = group
	}
	return
}

func (lbg *LoadBalancerGroup) updateEndpoint(add bool, endpoint Endpoint) {
	ep := endpoint.String()

	lbg.lock.RLock()
	defer lbg.lock.RUnlock()

	if lbws, ok := lbg.eps[ep]; ok && len(lbws.LoadBalancers) > 0 {
		if add {
			for _, lbw := range lbws.LoadBalancers {
				lbw.LoadBalancer.EndpointManager().AddEndpoint(endpoint)
			}
		} else {
			for _, lbw := range lbws.LoadBalancers {
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

// DelGroup deletes the LoadBalancer group and returns the unused endpoints,
// which should be removed from the centralized health checker.
func (lbg *LoadBalancerGroup) DelGroup(group string) (endpoints Endpoints) {
	endpoints = Endpoints{}
	lbg.lock.Lock()
	if lbw, ok := lbg.lbs[group]; ok {
		delete(lbg.lbs, group)
		for ep, endpoint := range lbw.Endpoints {
			// lbw.LoadBalancer.EndpointManager().DelEndpointByString(ep)
			if lbws, ok := lbg.eps[ep]; ok {
				delete(lbws.LoadBalancers, group)
				if len(lbws.LoadBalancers) == 0 {
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

func (lbg *LoadBalancerGroup) getOrNewGroup(group string) *loadBalancerWrapper {
	lbw, ok := lbg.lbs[group]
	if !ok {
		lb := lbg.newLoadBalancer(group)
		eps := make(map[string]Endpoint, 8)
		lbw = &loadBalancerWrapper{Endpoints: eps, LoadBalancer: lb}
		lbg.lbs[group] = lbw
	}
	return lbw
}

// AddEndpoint associates the endpoint with the group.
//
// If the group does not exist, it will be created automatically.
func (lbg *LoadBalancerGroup) AddEndpoint(group string, endpoint Endpoint) {
	ep := endpoint.String()

	lbg.lock.Lock()
	lbw := lbg.getOrNewGroup(group)
	lbw.Endpoints[ep] = endpoint
	if lbws, ok := lbg.eps[ep]; !ok {
		lbg.eps[ep] = &endpoint2lbsWrapper{
			Endpoint:      endpoint,
			LoadBalancers: map[string]*loadBalancerWrapper{group: lbw},
		}
	} else if _lbw, ok := lbws.LoadBalancers[group]; !ok {
		lbws.LoadBalancers[group] = lbw
	} else if _lbw != lbw {
		lbg.lock.Unlock()
		panic("LoadBalancerGroup: invalid reference relationship")
	}
	lbg.lock.Unlock()

	if lbg.OnEndpoint != nil {
		lbg.OnEndpoint.AddEndpoint(endpoint)
	}
}

func (lbg *LoadBalancerGroup) delEndpoint(endpoint string) bool {
	var ep Endpoint
	lbg.lock.Lock()
	if lbws, ok := lbg.eps[endpoint]; ok {
		ep = lbws.Endpoint
		delete(lbg.eps, endpoint)
		for _, lbw := range lbws.LoadBalancers {
			delete(lbw.Endpoints, endpoint)
			if len(lbw.Endpoints) == 0 {
				delete(lbg.lbs, lbw.LoadBalancer.Name)
			} else {
				lbw.LoadBalancer.EndpointManager().DelEndpointByString(endpoint)
			}
		}
	}
	lbg.lock.Unlock()

	if ep != nil && lbg.OnEndpoint != nil {
		lbg.OnEndpoint.DelEndpoint(ep)
		return true
	}
	return false
}

// DelEndpoint is equal to DelEndpointByString(endpoint.String()).
func (lbg *LoadBalancerGroup) DelEndpoint(endpoint Endpoint) bool {
	return lbg.delEndpoint(endpoint.String())
}

// DelEndpointByString deletes the endpoints from all the LoadBalancer groups
// and reports whether it is deleted really, that's, there is no LoadBalancer
// group to refer to it.
//
// Notice: If a LoadBalancer group has no any endpoints, it will be deleted.
func (lbg *LoadBalancerGroup) DelEndpointByString(endpoint string) bool {
	return lbg.delEndpoint(endpoint)
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

// GetAllEndpoints returns all the endpoints of all the groups.
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
		epms[lbw.LoadBalancer.Name] = eps
	}
	lbg.lock.RUnlock()
	return
}
