// Copyright 2020 xgfone
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
	"sync"
	"time"
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

	// OnEndpoint is used to notice someone that an endpoint is added or deleted.
	//
	// If nil, it does nothing.
	OnEndpoint Updater

	updater Updater
	lock    sync.RWMutex
	lbs     map[groupT]*loadBalancerWrapper
	eps     map[endpointT]*endpoint2lbsWrapper

	hc         *HealthCheck
	hcTimeout  time.Duration
	hcInterval time.Duration
	hcRetryNum int
}

// NewLoadBalancerGroup returns a new LoadBalancerGroup.
func NewLoadBalancerGroup() *LoadBalancerGroup {
	lbg := LoadBalancerGroup{
		lbs: make(map[groupT]*loadBalancerWrapper, 16),
		eps: make(map[endpointT]*endpoint2lbsWrapper, 32),
	}
	lbg.updater = UpdaterFunc("LoadBalancerGroup", lbg.updateEndpoint)
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
				lbw.LoadBalancer.EndpointManager().DelEndpoint(lbws.Endpoint)
			}
		}
	}
}

func (lbg *LoadBalancerGroup) noticeAddEndpoint(lbw *loadBalancerWrapper,
	hc *HealthCheck, ep Endpoint, interval, timeout time.Duration, retryNum int) {
	if lbg.OnEndpoint != nil {
		lbg.OnEndpoint.AddEndpoint(ep)
	}

	if hc != nil {
		hc.AddEndpointWithDuration(ep, interval, timeout, retryNum)
		if hc.IsHealthy(ep.String()) && !lbw.LoadBalancer.IsActive(ep) {
			lbw.LoadBalancer.EndpointManager().AddEndpoint(ep)
		}
	}
}

func (lbg *LoadBalancerGroup) noticeDelEndpoint(hc *HealthCheck, endpoint Endpoint) {
	if lbg.OnEndpoint != nil {
		lbg.OnEndpoint.DelEndpoint(endpoint)
	}

	if hc != nil {
		hc.DelEndpoint(endpoint)
	}
}

// SetHealthCheck sets the HealthCheck to hc. If hc is nil, it will unset it.
//
// If not nil, it will use it to maintain the statuses of all the endpoints.
func (lbg *LoadBalancerGroup) SetHealthCheck(hc *HealthCheck, interval, timeout time.Duration, retryNum int) {
	lbg.lock.Lock()
	defer lbg.lock.Unlock()

	if lbg.hc != nil {
		lbg.hc.UnsubscribeByUpdater(lbg.updater)
	}

	hc.Subscribe("", lbg.updater)
	lbg.hcRetryNum = retryNum
	lbg.hcInterval = interval
	lbg.hcTimeout = timeout
	lbg.hc = hc
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
			// lbw.LoadBalancer.EndpointManager().DelEndpoint(endpoint)
			if lbws, ok := lbg.eps[ep]; ok {
				delete(lbws.LoadBalancers, group)
				if len(lbws.LoadBalancers) == 0 {
					delete(lbg.eps, ep)
					endpoints = append(endpoints, endpoint)
				}
			}
		}
	}
	hc := lbg.hc
	lbg.lock.Unlock()

	for _, ep := range endpoints {
		lbg.noticeDelEndpoint(hc, ep)
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
	hc := lbg.hc
	timeout := lbg.hcTimeout
	interval := lbg.hcInterval
	retryNum := lbg.hcRetryNum
	lbg.lock.Unlock()

	lbg.noticeAddEndpoint(lbw, hc, endpoint, interval, timeout, retryNum)
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
				lbw.LoadBalancer.EndpointManager().DelEndpoint(ep)
			}
		}
	}
	hc := lbg.hc
	lbg.lock.Unlock()

	if ep != nil {
		lbg.noticeDelEndpoint(hc, ep)
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

// DelEndpointFromGroup is the same as DelEndpointByStringFromGroup.
func (lbg *LoadBalancerGroup) DelEndpointFromGroup(group string, endpoint Endpoint) {
	lbg.DelEndpointByStringFromGroup(group, endpoint.String())
}

// DelEndpointByStringFromGroup is used to delete the endpoint only from the group.
func (lbg *LoadBalancerGroup) DelEndpointByStringFromGroup(group, endpoint string) {
	var ep Endpoint
	lbg.lock.Lock()
	if lbw, ok := lbg.lbs[group]; ok {
		if _ep, ok := lbw.Endpoints[endpoint]; ok {
			if len(lbw.Endpoints) == 1 {
				delete(lbg.lbs, group)
			} else {
				delete(lbw.Endpoints, endpoint)
				lbw.LoadBalancer.EndpointManager().DelEndpoint(_ep)
			}
		}

		if lbws, ok := lbg.eps[endpoint]; ok {
			if _, ok := lbws.LoadBalancers[group]; ok {
				if len(lbws.LoadBalancers) == 1 {
					delete(lbg.eps, endpoint)
					ep = lbws.Endpoint
				} else {
					delete(lbws.LoadBalancers, group)
				}
			}
		}
	}

	hc := lbg.hc
	lbg.lock.Unlock()

	if ep != nil {
		lbg.noticeDelEndpoint(hc, ep)
	}
}

// GetRoundTripper is the same as GetLoadBalancer, but returns the RoundTripper.
func (lbg *LoadBalancerGroup) GetRoundTripper(group string) RoundTripper {
	if lb := lbg.GetLoadBalancer(group); lb != nil {
		return lb
	}
	return nil
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
			eps = append(eps, statusEndpoint{Endpoint: ep, IsActive: lbw.LoadBalancer.IsActive})
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
			eps = append(eps, statusEndpoint{Endpoint: ep, IsActive: lbw.LoadBalancer.IsActive})
		}
		epms[lbw.LoadBalancer.Name] = eps
	}
	lbg.lock.RUnlock()
	return
}

type statusEndpoint struct {
	Endpoint
	IsActive func(Endpoint) bool
}

func (se statusEndpoint) Unwrap() Endpoint               { return se.Endpoint }
func (se statusEndpoint) IsHealthy(context.Context) bool { return se.IsActive(se.Endpoint) }
