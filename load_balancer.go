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
	"time"
)

// Predefine some errors.
var (
	ErrNoAvailableEndpoint = errors.New("no available endpoints")
)

// LoadBalancer implements the LoadBalance function.
//
// A LoadBalancer instance is a group of endpoints that can handle the same
// request, which will forward the request to any endpoint to handle.
type LoadBalancer struct {
	Provider

	// Name is the name of LoadBalancer, which may be a set of the arbitrary
	// characters, such as an address.
	Name string

	// Session is used to manage the session stick, which is based on memory.
	// But it can be set to nil to disable it.
	Session SessionManager

	// FailRetry is used to retry when failing, which is FailOver(0) by default.
	FailRetry FailRetry

	// RetryDelay is used to get the interval time between the retries,
	// which is NewFixedDelay(10ms) by default.
	RetryDelay RetryDelay
}

// NewLoadBalancer returns a new LoadBalancer.
//
// If provider is nil, it will use NewGeneralProvider(RoundRobinSelector())
// by default.
func NewLoadBalancer(provider Provider) *LoadBalancer {
	if provider == nil {
		provider = NewGeneralProvider(RoundRobinSelector())
	}
	return &LoadBalancer{
		Provider:   provider,
		Session:    NewMemorySessionManager(),
		FailRetry:  FailOver(0),
		RetryDelay: NewFixedRetryDelay(time.Millisecond * 10),
	}
}

// ProviderSelector returns the ProviderSelector if the provider has implemented
// the interface ProviderSelector. Or returns nil instead.
func (lb *LoadBalancer) ProviderSelector() ProviderSelector {
	ps, _ := lb.Provider.(ProviderSelector)
	return ps
}

// EndpointGate returns the ProviderEndpointGate if the provider has implemented
// the interface ProviderEndpointGate. Or returns nil instead.
func (lb *LoadBalancer) EndpointGate() ProviderEndpointGate {
	eg, _ := lb.Provider.(ProviderEndpointGate)
	return eg
}

// EndpointManager asserts the provider to ProviderEndpointManager
// if the provider has implemented the interface ProviderEndpointManager.
// Or return nil instead.
func (lb *LoadBalancer) EndpointManager() ProviderEndpointManager {
	em, _ := lb.Provider.(ProviderEndpointManager)
	return em
}

// DeleteSession deletes the session cache.
func (lb *LoadBalancer) DeleteSession(raddr string) {
	lb.deleteEndpointFromSession(raddr)
}

func (lb *LoadBalancer) getEndpointFromSession(addr string) (ep Endpoint) {
	if addr != "" && lb.Session != nil {
		ep = lb.Session.GetEndpoint(addr)
	}
	return
}

func (lb *LoadBalancer) setEndpointToSession(addr string, endpoint Endpoint) {
	if addr != "" && lb.Session != nil {
		lb.Session.SetEndpoint(addr, endpoint)
	}
}

func (lb *LoadBalancer) deleteEndpointFromSession(addr string) {
	if addr != "" && lb.Session != nil {
		lb.Session.DelEndpoint(addr)
	}
}

func (lb *LoadBalancer) selectEndpoint(req Request, raddr string) (
	total, index int, endpoint Endpoint) {
	if total = lb.Provider.Len(); total == 0 {
		return
	}

	endpoint = lb.getEndpointFromSession(raddr)
	if endpoint != nil && !lb.Provider.IsActive(endpoint) {
		lb.deleteEndpointFromSession(raddr)
		endpoint = nil
	}

	if endpoint == nil {
		index, endpoint = lb.Provider.Select(req)
		lb.setEndpointToSession(raddr, endpoint)
	}

	return
}

func (lb *LoadBalancer) getEndpointByIndex(addr string, index int) (int, Endpoint) {
	index, endpoint := lb.Provider.SelectByIndex(index)
	if endpoint != nil {
		lb.setEndpointToSession(addr, endpoint)
	}
	return index, endpoint
}

// RoundTrip selects an endpoint, then call it. If failed, it will retry it
// by the fail handler.
func (lb *LoadBalancer) RoundTrip(ctx context.Context, req Request) (resp Response, err error) {
	raddr := req.RemoteAddrString()
	if sreq, ok := req.(RequestSession); ok {
		raddr = sreq.SessionID()
	}

	total, index, endpoint := lb.selectEndpoint(req, raddr)
	if endpoint == nil {
		return nil, ErrNoAvailableEndpoint
	}

	var retry int
	var interval time.Duration
	for endpoint != nil {
		lb.Hit(endpoint)
		resp, err = endpoint.RoundTrip(ctx, req)
		lb.Finish(endpoint)
		if err == nil {
			return
		}

		if lb.FailRetry == nil {
			break
		} else if index = lb.FailRetry.Next(total, index, retry); index < 0 {
			break
		}

		select {
		case <-ctx.Done():
			break
		default:
		}

		retry++
		if lb.RetryDelay != nil {
			if interval = lb.RetryDelay(retry, interval); interval > 0 {
				time.Sleep(interval)
				select {
				case <-ctx.Done():
					break
				default:
				}
			}
		}

		index, endpoint = lb.getEndpointByIndex(raddr, index)
	}

	lb.deleteEndpointFromSession(raddr)
	return
}
