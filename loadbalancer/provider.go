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
)

// Provider is a provider of the endpoints, which should be thread-safe.
type Provider interface {
	// Clean the underlying resource when it's destroyed.
	io.Closer

	// Return the description of the provider.
	fmt.Stringer

	// Strategy returns the name of the strategy to select the endpoint.
	Strategy() string

	// IsActive reports whether the endpoint is still active.
	IsActive(Endpoint) bool

	// Invoke calls the function with the endpoints in ascending sort order.
	Inspect(func(Endpoints))

	// Select selects an endpoint by the Request.
	Select(req Request) Endpoint
}

// NewGeneralProvider returns a new general Provider, which has also implemented
// the interface EndpointUpdater, EndpointBatchUpdater and SelectorGetSetter.
//
// selector is RoundRobinSelector() by default.
func NewGeneralProvider(selector Selector) Provider {
	if selector == nil {
		selector = RoundRobinSelector()
	}
	return &generalProvider{selector: selector}
}

var _ Provider = &generalProvider{}
var _ EndpointUpdater = &generalProvider{}
var _ EndpointBatchUpdater = &generalProvider{}
var _ SelectorGetSetter = &generalProvider{}

type generalProvider struct {
	lock      sync.RWMutex
	selector  Selector
	endpoints Endpoints
}

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

func (p *generalProvider) IsActive(endpoint Endpoint) (active bool) {
	id := endpoint.ID()
	p.lock.RLock()
	active = binarySearch(p.endpoints, id) > -1
	p.lock.RUnlock()
	return
}

func (p *generalProvider) Inspect(f func(Endpoints)) {
	p.lock.RLock()
	f(p.endpoints)
	p.lock.RUnlock()
}

func (p *generalProvider) Select(req Request) (ep Endpoint) {
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
	}
	p.lock.Unlock()
}

func (p *generalProvider) DelEndpoint(endpoint Endpoint) {
	id := endpoint.ID()
	p.lock.Lock()
	for i, _len := 0, len(p.endpoints); i < _len; i++ {
		if p.endpoints[i].ID() == id {
			copy(p.endpoints[i:], p.endpoints[i+1:])
			p.endpoints = p.endpoints[:_len-1]
			break
		}
	}
	p.lock.Unlock()
}

func (p *generalProvider) AddEndpoints(endpoints []Endpoint) {
	var ok bool
	p.lock.Lock()
	for _, endpoint := range endpoints {
		if p.endpoints.NotContains(endpoint) {
			ok = true
			p.endpoints = append(p.endpoints, endpoint)
		}
	}
	if ok {
		p.endpoints.Sort()
	}
	p.lock.Unlock()
}

func (p *generalProvider) DelEndpoints(endpoints []Endpoint) {
	p.lock.Lock()
	for j, _len := 0, len(endpoints); j < _len; j++ {
		id := endpoints[j].ID()
		for i, _len := 0, len(p.endpoints); i < _len; i++ {
			if p.endpoints[i].ID() == id {
				copy(p.endpoints[i:], p.endpoints[i+1:])
				p.endpoints = p.endpoints[:_len-1]
				break
			}
		}
	}
	p.lock.Unlock()
}

func (p *generalProvider) GetSelector() Selector {
	p.lock.RLock()
	s := p.selector
	p.lock.RUnlock()
	return s
}

func (p *generalProvider) SetSelector(selector Selector) {
	if selector == nil {
		panic("GeneralProvider: the selector must not be nil")
	}

	p.lock.Lock()
	if p.selector.String() != selector.String() {
		p.selector = selector
	}
	p.lock.Unlock()
}
