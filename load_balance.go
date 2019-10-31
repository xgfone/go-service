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
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

var errInit = errors.New("init")

// Predefine some errors.
var (
	ErrNoAvailableEndpoint = errors.New("no available endpoints")
)

type endpoints []Endpoint

func (es endpoints) Len() int      { return len(es) }
func (es endpoints) Swap(i, j int) { es[i], es[j] = es[j], es[i] }
func (es endpoints) Less(i, j int) bool {
	if es[i] == nil {
		return false
	} else if es[j] == nil {
		return true
	}
	return es[i].String() < es[j].String()
}

// LoadBalancer implements the LoadBalance function.
//
// A LoadBalancer instance is a group of endpoints that can handle the same
// request, which will forward the request to any endpoint to handle.
type LoadBalancer struct {
	lock sync.RWMutex

	session      SessionManager
	selector     Selector
	endpoints    []Endpoint
	failHandler  FailHandler
	failInterval time.Duration
}

// NewLoadBalancer returns a new LoadBalancer.
func NewLoadBalancer() *LoadBalancer {
	return &LoadBalancer{
		session:      NewMemorySessionManager(),
		selector:     RoundRobinSelector(),
		endpoints:    make([]Endpoint, 0, 64),
		failHandler:  FailFast(),
		failInterval: time.Millisecond * 100,
	}
}

// Len returns the number of the endpoints.
func (lb *LoadBalancer) Len() int {
	lb.lock.RLock()
	_len := len(lb.endpoints)
	lb.lock.RUnlock()
	return _len
}

// Endpoints returns all the endpoints.
func (lb *LoadBalancer) Endpoints() []Endpoint {
	lb.lock.RLock()
	endpoints := make([]Endpoint, len(lb.endpoints))
	copy(endpoints, lb.endpoints)
	lb.lock.RUnlock()
	return endpoints
}

// AddEndpoint is equal to lb.AddEndpoints(endpoint).
func (lb *LoadBalancer) AddEndpoint(endpoint Endpoint) {
	lb.AddEndpoints(endpoint)
}

// AddEndpoints adds the new endpoints.
func (lb *LoadBalancer) AddEndpoints(endpoint ...Endpoint) {
	var isSort bool
	eps := endpoint
	deps := []Endpoint{}

	lb.lock.Lock()

LOOP:
	for _, ep1 := range eps {
		addr := ep1.String()
		for i, ep2 := range lb.endpoints {
			if ep2.String() == addr {
				lb.endpoints[i] = ep1
				deps = append(deps, ep2)
				continue LOOP
			}
		}
		isSort = true
		lb.endpoints = append(lb.endpoints, ep1)
	}

	if isSort {
		sort.Sort(endpoints(lb.endpoints))
	}

	lb.lock.Unlock()

	for _, ep := range endpoint {
		if eps, ok := ep.(EndpointStatus); ok {
			eps.Activate(context.Background())
		}
	}
	for _, ep := range deps {
		if eps, ok := ep.(EndpointStatus); ok {
			eps.Deactivate(context.Background())
		}
	}
}

// DelEndpoint is equal to lb.DelEndpoints(endpoint).
func (lb *LoadBalancer) DelEndpoint(endpoint Endpoint) {
	lb.DelEndpoints(endpoint)
}

// DelEndpoints deletes some endpoints.
func (lb *LoadBalancer) DelEndpoints(endpoints ...Endpoint) {
	eps := make([]Endpoint, 0, len(endpoints))

	var num int
	lb.lock.Lock()
	for _, endpoint := range endpoints {
		ep, ok := lb.delEndpointByString(endpoint.String())
		if ok {
			num++
			eps = append(eps, ep)
		}
	}
	lb.updateEndpoints(num)
	lb.lock.Unlock()

	for _, ep := range eps {
		if eps, ok := ep.(EndpointStatus); ok {
			eps.Deactivate(context.Background())
		}
	}
}

// DelEndpointByString is equal to lb.DelEndpointsByString(endpoint).
func (lb *LoadBalancer) DelEndpointByString(endpoint string) {
	lb.DelEndpointsByString(endpoint)
}

// DelEndpointsByString deletes some endpoints by the endpoint addresses.
func (lb *LoadBalancer) DelEndpointsByString(endpoints ...string) {
	eps := make([]Endpoint, 0, len(endpoints))

	var num int
	lb.lock.Lock()
	for _, endpoint := range endpoints {
		ep, ok := lb.delEndpointByString(endpoint)
		if ok {
			num++
			eps = append(eps, ep)
		}
	}
	lb.updateEndpoints(num)
	lb.lock.Unlock()

	for _, ep := range eps {
		if eps, ok := ep.(EndpointStatus); ok {
			eps.Deactivate(context.Background())
		}
	}
}

func (lb *LoadBalancer) updateEndpoints(num int) {
	if num > 0 {
		sort.Sort(endpoints(lb.endpoints))
		lb.endpoints = lb.endpoints[:len(lb.endpoints)-num]

		// We will recycle the overmuch unused memory.
		if _len := len(lb.endpoints); cap(lb.endpoints)-_len > 256 {
			endpoints := make([]Endpoint, _len, _len+8)
			copy(endpoints, lb.endpoints)
			lb.endpoints = endpoints
		}
	}
}

func (lb *LoadBalancer) delEndpointByString(endpoint string) (Endpoint, bool) {
	for i, ep := range lb.endpoints {
		if ep.String() == endpoint {
			lb.endpoints[i] = nil
			return ep, true
		}
	}
	return nil, false
}

// SetSessionManager resets the session manager to sm.
//
// If sm is nil, it will disable the session manager.
func (lb *LoadBalancer) SetSessionManager(sm SessionManager) *LoadBalancer {
	lb.lock.Lock()
	lb.session = sm
	lb.lock.Unlock()
	return lb
}

// SetSelector resets the selector to s.
func (lb *LoadBalancer) SetSelector(s Selector) *LoadBalancer {
	if s == nil {
		panic("LoadBalancer: the selector must not be nil")
	}

	lb.lock.Lock()
	lb.selector = s
	lb.lock.Unlock()
	return lb
}

// SetFailHandler resets the fail handler to h.
func (lb *LoadBalancer) SetFailHandler(h FailHandler) *LoadBalancer {
	if h == nil {
		panic("LoadBalancer: the FailHandler must not be nil")
	}

	lb.lock.Lock()
	lb.failHandler = h
	lb.lock.Unlock()
	return lb
}

// SetFailInterval resets the interval time to retry to forward the request.
func (lb *LoadBalancer) SetFailInterval(interval time.Duration) *LoadBalancer {
	lb.lock.Lock()
	lb.failInterval = interval
	lb.lock.Unlock()
	return lb
}

// DeleteSession deletes the session cache.
func (lb *LoadBalancer) DeleteSession(raddr string) {
	lb.deleteEndpointFromSession(raddr)
}

func (lb *LoadBalancer) getEndpoint(raddr string, index int) Endpoint {
	var endpoint Endpoint
	lb.lock.RLock()
	if _len := len(lb.endpoints); _len > 0 {
		if index == _len {
			index = 0
		} else if index > _len {
			index %= _len
		}
		endpoint = lb.endpoints[index]
		lb.setEndpointToSession(raddr, endpoint)
	}
	lb.lock.RUnlock()
	return endpoint
}

func (lb *LoadBalancer) getEndpointFromSession(addr string) (Endpoint, int) {
	if addr != "" && lb.session != nil {
		endpoint := lb.session.GetEndpoint(addr)
		if endpoint != nil {
			eaddr := endpoint.String()
			for i, ep := range lb.endpoints {
				if ep.String() == eaddr {
					return endpoint, i
				}
			}
			lb.session.DelEndpoint(addr)
		}
	}
	return nil, 0
}

func (lb *LoadBalancer) setEndpointToSession(addr string, endpoint Endpoint) {
	if addr != "" && lb.session != nil {
		lb.session.SetEndpoint(addr, endpoint)
	}
}

func (lb *LoadBalancer) deleteEndpointFromSession(addr string) {
	if addr == "" {
		return
	}

	lb.lock.RLock()
	session := lb.session
	lb.lock.RUnlock()
	if session != nil {
		session.DelEndpoint(addr)
	}
}

func (lb *LoadBalancer) endpointIsAlive(endpoint Endpoint, key string) Endpoint {
	if endpoint != nil {
		addr := endpoint.String()
		for _, ep := range lb.endpoints {
			if ep.String() == addr {
				return endpoint
			}
		}

		// Remove the dead endpoint from the session cache.
		if lb.session != nil {
			lb.session.DelEndpoint(key)
		}
	}

	return nil
}

func (lb *LoadBalancer) selectEndpoint(req Request, raddr string, updateSession bool) (
	total, index int, endpoint Endpoint, interval time.Duration, handler FailHandler) {
	lb.lock.RLock()
	if total = len(lb.endpoints); total == 0 {
		lb.lock.Unlock()
		return
	}

	if updateSession {
		endpoint, index = lb.getEndpointFromSession(raddr)
		endpoint = lb.endpointIsAlive(endpoint, raddr)
	}

	if endpoint == nil {
		index = lb.selector(req, lb.endpoints)
		endpoint = lb.endpoints[index]
		if updateSession {
			lb.setEndpointToSession(raddr, endpoint)
		}
	}

	interval = lb.failInterval
	handler = lb.failHandler
	lb.lock.RUnlock()

	return
}

// RoundTrip selects an endpoint, then call it. If failed, it will retry it
// by the fail handler.
//
// Notice: the retry number won't exceeds the number of the endpoints.
func (lb *LoadBalancer) RoundTrip(ctx context.Context, req Request) (resp Response, err error) {
	raddr := req.RemoteAddrString()
	if sreq, ok := req.(RequestSession); ok {
		raddr = sreq.SessionID()
	}

	_len, index, endpoint, interval, failHandler := lb.selectEndpoint(req, raddr, true)
	if endpoint == nil {
		return nil, ErrNoAvailableEndpoint
	}

	var retry int
	err = errInit
	for retry <= _len && err != nil && endpoint != nil {
		if resp, err = endpoint.RoundTrip(ctx, req); err != nil {
			if index = failHandler(index, retry); index < 0 {
				break
			}

			select {
			case <-ctx.Done():
				break
			default:
			}

			if interval > 0 {
				time.Sleep(interval)
			}

			select {
			case <-ctx.Done():
				break
			default:
			}

			endpoint = lb.getEndpoint(raddr, index)
			retry++
		}
	}

	if err != nil {
		lb.deleteEndpointFromSession(raddr)
	}

	return
}

// SelectEndpoint selects an active endpoint.
//
// Return nil if no any active endpoint.
func (lb *LoadBalancer) SelectEndpoint(req Request) Endpoint {
	_, _, endpoint, _, _ := lb.selectEndpoint(req, "", false)
	return endpoint
}
