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
	"sort"
	"sync"
	"sync/atomic"
)

// Provider is a provider of the endpoints.
type Provider interface {
	// Len returns the number of the endpoints.
	Len() int

	// IsActive reports whether the endpoint is still active.
	IsActive(Endpoint) bool

	// Select selects an endpoint with its index by the Request.
	Select(Request) (index int, endpoint Endpoint)

	// SelectByIndex selects the endpoint by the index information.
	//
	// If the index does not exist, it maybe return the next endpoint.
	// And if no active endpoints, it should return nil for Endpoint.
	SelectByIndex(index int) (realIndex int, endpoint Endpoint)
}

// ProviderEndpointManager is an interface to manage the endpoints.
type ProviderEndpointManager interface {
	// Endpoints should returns the copy of all the endpoints.
	Endpoints() Endpoints
	AddEndpoint(Endpoint)
	DelEndpoint(Endpoint)
}

// ProviderSelector is used to manage the selector by the provider.
type ProviderSelector interface {
	GetSelector() Selector
	SetSelector(Selector)
}

var (
	_ Provider                = &GeneralProvider{}
	_ ProviderSelector        = &GeneralProvider{}
	_ ProviderEndpointManager = &GeneralProvider{}
)

// GeneralProvider is a general provider of the endpoints.
type GeneralProvider struct {
	lock sync.RWMutex

	selector  Selector
	endpoints Endpoints
	eplen     uint32
}

// NewGeneralProvider returns a new GeneralProvider with the selector.
func NewGeneralProvider(selector Selector, endpoints ...Endpoint) *GeneralProvider {
	if selector == nil {
		panic("GeneralProvider: the selector must not be nil")
	}

	p := &GeneralProvider{
		selector:  selector,
		endpoints: make(Endpoints, 0, 8),
	}

	p.addEndpoints(endpoints...)
	return p
}

// GetSelector returns the selector.
func (p *GeneralProvider) GetSelector() Selector {
	p.lock.RLock()
	s := p.selector
	p.lock.RUnlock()
	return s
}

// SetSelector resets the selector to s.
func (p *GeneralProvider) SetSelector(s Selector) {
	if s == nil {
		panic("GeneralProvider: the selector must not be nil")
	}

	p.lock.Lock()
	p.selector = s
	p.lock.Unlock()
}

func (p *GeneralProvider) addEndpoints(endpoints ...Endpoint) {
	p.endpoints = append(p.endpoints, endpoints...)
	sort.Sort(p.endpoints)
	p.updateLen()
}

func (p *GeneralProvider) updateLen() {
	atomic.StoreUint32(&p.eplen, uint32(len(p.endpoints)))
}

// Len returns the number of the endpoints.
func (p *GeneralProvider) Len() int {
	return int(atomic.LoadUint32(&p.eplen))
}

// Endpoints returns the copy of all the endpoints.
func (p *GeneralProvider) Endpoints() Endpoints {
	p.lock.RLock()
	endpoints := make(Endpoints, len(p.endpoints))
	copy(endpoints, p.endpoints)
	p.lock.RUnlock()
	return endpoints
}

// AddEndpoint adds the endpoint.
func (p *GeneralProvider) AddEndpoint(endpoint Endpoint) {
	addr := endpoint.String()
	var old Endpoint

	p.lock.Lock()
	for i, ep := range p.endpoints {
		if ep.String() == addr {
			p.endpoints[i] = endpoint
			old = ep
			break
		}
	}
	if old == nil {
		p.endpoints = append(p.endpoints, endpoint)
		sort.Sort(p.endpoints)
		p.updateLen()
	}
	p.lock.Unlock()

	if eps, ok := old.(EndpointStatus); ok {
		eps.Deactivate(context.Background())
	}
	if eps, ok := endpoint.(EndpointStatus); ok {
		eps.Activate(context.Background())
	}
}

// DelEndpoint deletes the endpoint.
func (p *GeneralProvider) DelEndpoint(endpoint Endpoint) {
	p.delEndpointByString(endpoint.String())
}

// DelEndpointByString deletes the endpoint.
func (p *GeneralProvider) delEndpointByString(endpoint string) {
	var deleted Endpoint
	var exist bool

	p.lock.Lock()
	for i, ep := range p.endpoints {
		if ep.String() == endpoint {
			exist = true
			deleted = ep
			p.endpoints[i] = nil
			break
		}
	}
	if exist {
		sort.Sort(p.endpoints)
		p.endpoints = p.endpoints[:len(p.endpoints)-1]
		p.updateLen()
	}
	p.lock.Unlock()

	if eps, ok := deleted.(EndpointStatus); ok {
		eps.Deactivate(context.Background())
	}
}

// IsActive reports whether the endpoint is still active.
func (p *GeneralProvider) IsActive(endpoint Endpoint) (active bool) {
	addr := endpoint.String()
	p.lock.RLock()
	for _, ep := range p.endpoints {
		if ep.String() == addr {
			active = true
		}
	}
	p.lock.RUnlock()
	return
}

// Select selects an endpoint by the selector.
func (p *GeneralProvider) Select(req Request) (index int, endpoint Endpoint) {
	p.lock.RLock()
	if len(p.endpoints) > 0 {
		index = p.selector.Select(req, p.endpoints)
		endpoint = p.endpoints[index]
	}
	p.lock.RUnlock()
	return
}

// SelectByIndex selects the endpoint by the index.
//
// If the index do not exist, it will return the next endpoint.
// And return (0, nil) if no active endpoints.
func (p *GeneralProvider) SelectByIndex(index int) (realIndex int, endpoint Endpoint) {
	p.lock.RLock()
	if _len := len(p.endpoints); _len > 0 {
		if index >= _len {
			index = index % _len
		}
		realIndex = index
		endpoint = p.endpoints[realIndex]
	}
	p.lock.RUnlock()
	return
}
