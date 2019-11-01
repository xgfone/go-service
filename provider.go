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
	SelectByIndex(index int) (realIndex int, endpoint Endpoint)

	// Finish is used to notice the provider that the endpoint has finished
	// to handle the request.
	Finish(index int, endpoint Endpoint)
}

// ProviderEndpointManager is an interface to manage the endpoints.
type ProviderEndpointManager interface {
	// Endpoints should returns the copy of all the endpoints.
	Endpoints() []Endpoint
	AddEndpoint(Endpoint)
	DelEndpoint(Endpoint)
	DelEndpointByString(endpoint string)
}

// ProviderEndpointEvent is used to adds the callback of the endpoint events,
// which will be called when the corresponding event occurs.
type ProviderEndpointEvent interface {
	OnAdd(func(Endpoint))
	OnDelete(func(Endpoint))
	OnSelect(func(Endpoint))
	OnFinish(func(Endpoint))
}

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

var _ Provider = &GeneralProvider{}

// GeneralProvider is a general provider of the endpoints.
type GeneralProvider struct {
	lock sync.RWMutex

	selector  Selector
	endpoints []Endpoint

	onAdds    []func(Endpoint)
	OnDeletes []func(Endpoint)
	onSelects []func(Endpoint)
	onFinishs []func(Endpoint)
}

// NewGeneralProvider returns a new GeneralProvider with the selector.
func NewGeneralProvider(selector Selector, endpoints ...Endpoint) *GeneralProvider {
	if selector == nil {
		panic("GeneralProvider: the selector must not be nil")
	}

	p := &GeneralProvider{selector: selector, endpoints: make([]Endpoint, 0, 8)}
	p.addEndpoints(endpoints...)
	return p
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

func (p *GeneralProvider) addEndpoints(eps ...Endpoint) {
	p.endpoints = append(p.endpoints, eps...)
	sort.Sort(endpoints(p.endpoints))
}

// Len returns the number of the endpoints.
func (p *GeneralProvider) Len() int {
	p.lock.RLock()
	_len := len(p.endpoints)
	p.lock.RUnlock()
	return _len
}

// Endpoints returns the copy of all the endpoints.
func (p *GeneralProvider) Endpoints() []Endpoint {
	p.lock.RLock()
	endpoints := make([]Endpoint, len(p.endpoints))
	copy(endpoints, p.endpoints)
	p.lock.RUnlock()
	return endpoints
}

// AddEndpoint adds the endpoint.
func (p *GeneralProvider) AddEndpoint(endpoint Endpoint) {
	addr := endpoint.String()
	var cbs []func(Endpoint)
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
		sort.Sort(endpoints(p.endpoints))
	}
	cbs = append([]func(Endpoint){}, p.onAdds...)
	p.lock.Unlock()

	if eps, ok := old.(EndpointStatus); ok {
		eps.Deactivate(context.Background())
	}
	if eps, ok := endpoint.(EndpointStatus); ok {
		eps.Activate(context.Background())
	}

	for _, cb := range cbs {
		cb(endpoint)
	}
}

// DelEndpoint deletes the endpoint.
func (p *GeneralProvider) DelEndpoint(endpoint Endpoint) {
	p.DelEndpointByString(endpoint.String())
}

// DelEndpointByString deletes the endpoint.
func (p *GeneralProvider) DelEndpointByString(endpoint string) {
	var cbs []func(Endpoint)
	var deleted Endpoint
	var exist bool

	p.lock.Lock()
	for i, ep := range p.endpoints {
		if ep.String() == endpoint {
			exist = true
			deleted = ep
			p.endpoints[i] = nil
			cbs = append([]func(Endpoint){}, p.OnDeletes...)
		}
	}
	if exist {
		sort.Sort(endpoints(p.endpoints))
	}
	p.lock.Unlock()

	if eps, ok := deleted.(EndpointStatus); ok {
		eps.Deactivate(context.Background())
	}

	for _, cb := range cbs {
		cb(deleted)
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
	var cbs []func(Endpoint)
	p.lock.RLock()
	if len(p.endpoints) > 0 {
		index = p.selector.Select(req, p.endpoints)
		endpoint = p.endpoints[index]
		cbs = append([]func(Endpoint){}, p.onSelects...)
	}
	p.lock.RUnlock()

	for _, cb := range cbs {
		cb(endpoint)
	}

	return
}

// SelectByIndex selects the endpoint by the index.
//
// If the index do not exist, it will return the next endpoint.
// And return (0, nil) if no active endpoints.
func (p *GeneralProvider) SelectByIndex(index int) (realIndex int, endpoint Endpoint) {
	var cbs []func(Endpoint)
	p.lock.RLock()
	if _len := len(p.endpoints); _len > 0 {
		if index >= _len {
			index = index % _len
		}
		realIndex = index
		endpoint = p.endpoints[realIndex]
		cbs = append([]func(Endpoint){}, p.onSelects...)
	}
	p.lock.RUnlock()

	for _, cb := range cbs {
		cb(endpoint)
	}

	return
}

// Finish notices the selector that the endpoint has finished to handle the request.
func (p *GeneralProvider) Finish(index int, endpoint Endpoint) {
	p.lock.RLock()
	selector := p.selector
	cbs := append([]func(Endpoint){}, p.onFinishs...)
	p.lock.RUnlock()

	selector.Finish(index, endpoint)
	for _, cb := range cbs {
		cb(endpoint)
	}
}

// OnAdd adds the callback function, which will be called when an endpoint
// is added.
func (p *GeneralProvider) OnAdd(f func(Endpoint)) {
	if f == nil {
		panic("GeneralProvider: the OnAdd callback function must not be nil")
	}

	p.lock.Lock()
	p.onAdds = append(p.onAdds, f)
	p.lock.Unlock()
}

// OnDelete adds the callback function, which will be called when an endpoint
// is deleted.
func (p *GeneralProvider) OnDelete(f func(Endpoint)) {
	if f == nil {
		panic("GeneralProvider: the OnDelete callback function must not be nil")
	}

	p.lock.Lock()
	p.OnDeletes = append(p.OnDeletes, f)
	p.lock.Unlock()
}

// OnSelect adds the callback function, which will be called when an endpoint
// is selected.
func (p *GeneralProvider) OnSelect(f func(Endpoint)) {
	if f == nil {
		panic("GeneralProvider: the OnSelect callback function must not be nil")
	}

	p.lock.Lock()
	p.onSelects = append(p.onSelects, f)
	p.lock.Unlock()
}

// OnFinish adds the callback function, which will be called when an endpoint
// has finished to handle the request.
func (p *GeneralProvider) OnFinish(f func(Endpoint)) {
	if f == nil {
		panic("GeneralProvider: the OnFinish callback function must not be nil")
	}

	p.lock.Lock()
	p.onFinishs = append(p.onFinishs, f)
	p.lock.Unlock()
}
