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

var errInit = errors.New("init")

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

	Session      SessionManager
	FailHandler  FailHandler
	FailInterval time.Duration
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
		Provider:     provider,
		Session:      NewMemorySessionManager(),
		FailHandler:  FailOver(0),
		FailInterval: time.Millisecond * 10,
	}
}

// EndpointManager asserts the provider to ProviderEndpointManager.
func (lb *LoadBalancer) EndpointManager() ProviderEndpointManager {
	return lb.Provider.(ProviderEndpointManager)
}

// EndpointEvent asserts the provider to ProviderEndpointEvent.
func (lb *LoadBalancer) EndpointEvent() ProviderEndpointEvent {
	return lb.Provider.(ProviderEndpointEvent)
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

func (lb *LoadBalancer) selectEndpoint(req Request, raddr string, updateSession bool) (
	total, index int, endpoint Endpoint) {
	if total = lb.Provider.Len(); total == 0 {
		return
	}

	if updateSession {
		endpoint = lb.getEndpointFromSession(raddr)
		if endpoint != nil && !lb.Provider.IsActive(endpoint) {
			lb.deleteEndpointFromSession(raddr)
			endpoint = nil
		}
	}

	if endpoint == nil {
		index, endpoint = lb.Provider.Select(req)
		if updateSession {
			lb.setEndpointToSession(raddr, endpoint)
		}
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
//
// Notice: the retry number won't exceeds the number of the endpoints.
func (lb *LoadBalancer) RoundTrip(ctx context.Context, req Request) (resp Response, err error) {
	raddr := req.RemoteAddrString()
	if sreq, ok := req.(RequestSession); ok {
		raddr = sreq.SessionID()
	}

	total, index, endpoint := lb.selectEndpoint(req, raddr, true)
	if endpoint == nil {
		return nil, ErrNoAvailableEndpoint
	}
	defer lb.Finish(endpoint)

	var retry int
	err = errInit
	for retry <= total && err != nil && endpoint != nil {
		if resp, err = endpoint.RoundTrip(ctx, req); err != nil {
			if lb.FailHandler == nil {
				break
			} else if index = lb.FailHandler(index, retry); index < 0 {
				break
			}

			select {
			case <-ctx.Done():
				break
			default:
			}

			if lb.FailInterval > 0 {
				time.Sleep(lb.FailInterval)

				select {
				case <-ctx.Done():
					break
				default:
				}
			}

			index, endpoint = lb.getEndpointByIndex(raddr, index)
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
	_, _, endpoint := lb.selectEndpoint(req, "", false)
	return endpoint
}
