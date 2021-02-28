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
	"errors"
	"fmt"
	"time"

	"github.com/xgfone/go-service/retry"
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

	// FailRetry is used to retry when failing, which is by default:
	//
	//     FailOver(0, retry.DefaultRetryNewer(time.Millisecond*10))
	//
	// However, you can set it to nil to disable it.
	FailRetry FailRetry
}

// NewLoadBalancer returns a new LoadBalancer.
//
// If provider is nil, it will use NewGeneralProvider(nil) as the default.
func NewLoadBalancer(provider Provider) *LoadBalancer {
	if provider == nil {
		provider = NewGeneralProvider(nil)
	}
	return &LoadBalancer{
		Provider:  provider,
		FailRetry: FailOver(0, retry.DefaultRetryNewer(time.Millisecond*10)),
	}
}

// String implements the interface fmt.Stringer.
func (lb *LoadBalancer) String() string {
	if lb.Name == "" {
		return "LoadBalancer"
	}
	return fmt.Sprintf("LoadBalancer(name=%s)", lb.Name)
}

// Updater returns the updater of the load balancer.
func (lb *LoadBalancer) Updater() Updater {
	return UpdaterFunc(lb.Name, lb.updateEndpoint)
}

func (lb *LoadBalancer) updateEndpoint(addOrDel bool, ep Endpoint) {
	if addOrDel {
		lb.EndpointManager().AddEndpoint(ep)
	} else {
		lb.EndpointManager().DelEndpoint(ep)
	}
}

// ProviderSelector returns the ProviderSelector if the provider has implemented
// the interface ProviderSelector. Or returns nil instead.
func (lb *LoadBalancer) ProviderSelector() ProviderSelector {
	ps, _ := lb.Provider.(ProviderSelector)
	return ps
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

func (lb *LoadBalancer) getEndpoint(req Request) (ep Endpoint) {
	raddr := req.RemoteAddrString()
	if sreq, ok := req.(RequestSession); ok {
		if sid := sreq.SessionID(); sid != "" {
			raddr = sid
		}
	}

	ep = lb.getEndpointFromSession(raddr)
	if ep != nil && !lb.Provider.IsActive(ep) {
		lb.deleteEndpointFromSession(raddr)
		ep = nil
	}

	if ep == nil {
		if ep = lb.Provider.Select(req); ep != nil {
			lb.setEndpointToSession(raddr, ep)
		}
	}

	return
}

func (lb *LoadBalancer) selectEndpoint(req Request) (ep Endpoint) {
	raddr := req.RemoteAddrString()
	if sreq, ok := req.(RequestSession); ok {
		if sid := sreq.SessionID(); sid != "" {
			raddr = sid
		}
	}

	lb.deleteEndpointFromSession(raddr)
	if ep = lb.Provider.Select(req); ep != nil {
		lb.setEndpointToSession(raddr, ep)
	}
	return
}

// RoundTrip selects an endpoint, then call it. If failed, it will retry it
// by the fail handler.
func (lb *LoadBalancer) RoundTrip(c context.Context, r Request) (Response, error) {
	endpoint := lb.getEndpoint(r)
	if endpoint == nil {
		return nil, ErrNoAvailableEndpoint
	}

	resp, err := endpoint.RoundTrip(c, r)
	if err == nil || lb.FailRetry == nil {
		return resp, err
	}

	resp, err2 := lb.FailRetry.Retry(c, r, endpoint, lbProvider{lb})
	if err2 != ErrNoAvailableEndpoint {
		err = err2
	}

	return resp, err
}

type lbProvider struct{ *LoadBalancer }

func (p lbProvider) Select(req Request) (ep Endpoint) {
	return p.LoadBalancer.selectEndpoint(req)
}
