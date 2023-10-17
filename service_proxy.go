// Copyright 2023 xgfone
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
	"log"
	"sync"
	"sync/atomic"
)

func init() { Append(DefaultProxy) }

// DefaultProxy is the default global proxy service.
var DefaultProxy = NewProxy(context.Background())

type contextInfo struct {
	context.CancelFunc
	context.Context
}

// Proxy is a service to maintain the activation status.
type Proxy struct {
	parent  context.Context
	context atomic.Value
	clock   sync.Mutex
}

// NewProxy returns a new proxy service.
//
// If parent is nil, use context.Background() instead.
func NewProxy(parent context.Context) *Proxy {
	if parent == nil {
		parent = context.Background()
	}

	p := &Proxy{parent: parent}
	p.context.Store(contextInfo{})
	return p
}

// Context returns the inner context, which will be cancelled
// when the task service is deactivated.
//
// Return nil instead if the task service is not activated.
func (p *Proxy) Context() context.Context {
	return p.context.Load().(contextInfo).Context
}

// IsActivated reports whether the task service is activated.
func (p *Proxy) IsActivated() bool { return p.Context() != nil }

// Activate implements the interface Service.
func (p *Proxy) Activate() {
	if !p.IsActivated() {
		p.clock.Lock()
		if !p.IsActivated() {
			ctx, cancel := context.WithCancel(p.parent)
			p.context.Store(contextInfo{Context: ctx, CancelFunc: cancel})
		}
		p.clock.Unlock()
		log.Printf("the service proxy is activated")
	}
}

// Deactivate implements the interface Service.
func (p *Proxy) Deactivate() {
	if p.IsActivated() {
		var cancel func()
		p.clock.Lock()
		if p.IsActivated() {
			cancel = p.context.Swap(contextInfo{}).(contextInfo).CancelFunc
		}
		p.clock.Unlock()
		if cancel != nil {
			cancel()
		}
		log.Printf("the service proxy is deactivated")
	}
}

// RunService runs the Activate method of s if the proxy service is activated.
// Or, run Deactivate instead.
func (p *Proxy) RunService(s Service) {
	if ctx := p.Context(); ctx != nil {
		s.Activate()
	} else {
		s.Deactivate()
	}
}

// RunFunc runs the activate function of s if the proxy service is activated.
// Or, run aeactivate instead.
//
// If deactivate is nil, do nothing if the proxy service is not activated.
func (p *Proxy) RunFunc(activate func(context.Context), deactivate func()) {
	if ctx := p.Context(); ctx != nil {
		activate(ctx)
	} else if deactivate != nil {
		deactivate()
	}
}

// Run is equal to p.RunFunc(activate, nil).
func (p *Proxy) Run(activate func(context.Context)) {
	p.RunFunc(activate, nil)
}
