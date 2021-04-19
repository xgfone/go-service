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
	"sync/atomic"
	"time"
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
	// Default: NewGeneralProvider(nil)
	Provider

	// Default: NewNoopSession()
	Session Session

	// Default: FailOver(0, time.Millisecond*10)
	FailRetry FailRetry

	name    string
	timeout int64 // Session Timeout
}

// NewLoadBalancer returns a new LoadBalancer.
func NewLoadBalancer(name string, provider Provider) *LoadBalancer {
	if provider == nil {
		provider = NewGeneralProvider(nil)
	}

	return &LoadBalancer{
		Provider:  provider,
		FailRetry: FailOver(0, time.Millisecond*10),
		Session:   NewNoopSession(),
		timeout:   int64(time.Second * 30),
		name:      name,
	}
}

// Name returns the name of the loadbalancer.
func (lb *LoadBalancer) Name() string { return lb.name }

// GetSessionTimeout gets the session timeout.
func (lb *LoadBalancer) GetSessionTimeout() time.Duration {
	return time.Duration(atomic.LoadInt64(&lb.timeout))
}

// SetSessionTimeout sets the session timeout.
func (lb *LoadBalancer) SetSessionTimeout(timeout time.Duration) {
	atomic.StoreInt64(&lb.timeout, int64(timeout))
}

// String implements the interface fmt.Stringer.
func (lb *LoadBalancer) String() string {
	if name := lb.Name(); name != "" {
		return fmt.Sprintf("LoadBalancer(name=%s, provider=%s)", name, lb.Provider.String())
	}
	return fmt.Sprintf("LoadBalancer(provider=%s)", lb.Provider.String())
}

// AddEndpoint implements the interface EndpointUpdater.
func (lb *LoadBalancer) AddEndpoint(ep Endpoint) {
	lb.Provider.(EndpointUpdater).AddEndpoint(ep)
}

// DelEndpoint implements the interface EndpointUpdater.
func (lb *LoadBalancer) DelEndpoint(ep Endpoint) {
	lb.Provider.(EndpointUpdater).DelEndpoint(ep)
}

// AddEndpoints implements the interface EndpointBatchUpdater.
func (lb *LoadBalancer) AddEndpoints(eps []Endpoint) {
	lb.Provider.(EndpointBatchUpdater).AddEndpoints(eps)
}

// DelEndpoints implements the interface EndpointBatchUpdater.
func (lb *LoadBalancer) DelEndpoints(eps []Endpoint) {
	lb.Provider.(EndpointBatchUpdater).DelEndpoints(eps)
}

func (lb *LoadBalancer) getEndpointFromSession(r Request) (sid string, ep Endpoint) {
	if sid = r.SessionID(); sid == "" {
		if sid = r.RemoteAddrString(); sid == "" {
			return
		}
	}

	if ep = lb.Session.GetEndpoint(sid); ep != nil {
		if !lb.Provider.IsActive(ep) {
			lb.Session.DelEndpoint(sid)
			ep = nil
		}
	}
	return
}

// RoundTrip selects an endpoint, then call it. If failed, it will retry it
// by the fail handler if it's set.
func (lb *LoadBalancer) RoundTrip(c context.Context, r Request) (resp interface{}, err error) {
	var notsession bool
	sid, ep := lb.getEndpointFromSession(r)
	if ep == nil {
		if ep = lb.Provider.Select(r); ep == nil {
			return nil, ErrNoAvailableEndpoint
		}

		notsession = true
	}

	if resp, err = ep.RoundTrip(c, r); err != nil {
		ep, resp, err = lb.FailRetry.Retry(c, lb.Provider, r, ep, err)
		if sid != "" {
			if err == nil {
				lb.Session.SetEndpoint(sid, ep, lb.GetSessionTimeout())
			} else if !notsession {
				lb.Session.DelEndpoint(sid)
			}
		}
	} else if notsession && sid != "" {
		lb.Session.SetEndpoint(sid, ep, lb.GetSessionTimeout())
	}

	return
}
