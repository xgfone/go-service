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
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// Provider is a provider of the endpoints, which should be thread-safe.
type Provider interface {
	// Clean the underlying resource when it's destroyed.
	io.Closer

	// Return the description of the provider.
	fmt.Stringer

	// Strategy returns the name of the strategy to select the endpoint.
	Strategy() string

	// Len returns the number of the endpoints.
	Len() int

	// Endpoints returns the list of the inner endpoints.
	Endpoints() Endpoints

	// IsActive reports whether the endpoint is still active.
	IsActive(Endpoint) bool

	// Select selects an endpoint by the Request.
	//
	// If new is true, it represents that it's the first.
	// Or, it means to select another endpoint to retry to forward the request.
	Select(req Request, new bool) Endpoint

	// Finish is called when finishing or failing to forward the request.
	Finish(req Request, err error)
}

// NewGeneralProvider returns a new general Provider, which has also implemented
// the interface EndpointManager and SelectorManager.
//
// selector is RoundRobinSelector() by default.
func NewGeneralProvider(selector Selector) Provider {
	if selector == nil {
		selector = RoundRobinSelector()
	}

	return &generalProvider{selector: selector}
}

var _ Provider = &generalProvider{}
var _ EndpointManager = &generalProvider{}
var _ SelectorManager = &generalProvider{}

type generalProvider struct {
	lock      sync.RWMutex
	selector  Selector
	endpoints Endpoints
	eplen     uint32
}

func (p *generalProvider) updateLen() {
	atomic.StoreUint32(&p.eplen, uint32(len(p.endpoints)))
}

func (p *generalProvider) Len() int { return int(atomic.LoadUint32(&p.eplen)) }

func (p *generalProvider) Close() error { return nil }

func (p *generalProvider) String() string {
	return fmt.Sprintf("GeneralProvider(strategy=%s)", p.Strategy())
}

func (p *generalProvider) Strategy() string {
	p.lock.RLock()
	name := p.selector.String()
	p.lock.RUnlock()
	return name
}

func (p *generalProvider) Endpoints() Endpoints {
	p.lock.RLock()
	endpoints := make(Endpoints, len(p.endpoints))
	copy(endpoints, p.endpoints)
	p.lock.RUnlock()
	return endpoints
}

func (p *generalProvider) IsActive(endpoint Endpoint) (active bool) {
	addr := endpoint.String()
	p.lock.RLock()
	active = binarySearch(p.endpoints, addr) > -1
	p.lock.RUnlock()
	return
}

func (p *generalProvider) Finish(req Request, err error) {}
func (p *generalProvider) Select(req Request, new bool) (ep Endpoint) {
	p.lock.RLock()
	if len(p.endpoints) > 0 {
		ep = p.selector.Select(req, p.endpoints)
	}
	p.lock.RUnlock()
	return
}

func (p *generalProvider) AddEndpoint(endpoint Endpoint) {
	p.lock.Lock()
	if p.endpoints.NotContains(endpoint) {
		p.endpoints = append(p.endpoints, endpoint)
		p.endpoints.Sort()
		p.updateLen()
	}
	p.lock.Unlock()
}

func (p *generalProvider) DelEndpoint(endpoint Endpoint) {
	p.delEndpointByAddr(endpoint.String())
}

func (p *generalProvider) delEndpointByAddr(addr string) {
	var exist bool
	p.lock.Lock()
	for i, ep := range p.endpoints {
		if ep.String() == addr {
			exist = true
			p.endpoints[i] = nil
			break
		}
	}
	if exist {
		p.endpoints.Sort()
		p.endpoints = p.endpoints[:len(p.endpoints)-1]
		p.updateLen()
	}
	p.lock.Unlock()
}

func (p *generalProvider) GetSelector() Selector {
	p.lock.RLock()
	s := p.selector
	p.lock.RUnlock()
	return s
}

func (p *generalProvider) SetSelector(new Selector) (old Selector) {
	if new == nil {
		panic("GeneralProvider: the selector must not be nil")
	}

	p.lock.Lock()
	if p.selector.String() != new.String() {
		old, p.selector = p.selector, new
	}
	p.lock.Unlock()
	return
}
