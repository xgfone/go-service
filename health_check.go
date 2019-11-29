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
	"sync"
	"time"
)

type endpointWrapper struct {
	Exit     chan struct{}
	Timeout  time.Duration
	Interval time.Duration
	Endpoint Endpoint
	Health   bool
}

type endpointOp struct {
	Add      bool
	Endpoint Endpoint
}

// Updater represents a updater to update the endpoint.
type Updater interface {
	AddEndpoint(Endpoint)
	DelEndpoint(Endpoint)
}

// HealthCheck is used to manage the health of the endpoints.
type HealthCheck struct {
	lock sync.RWMutex

	exit      chan struct{}
	updaters  []Updater
	updatech  chan endpointOp
	endpoints map[string]*endpointWrapper
}

// NewHealthCheck returns a new NewHealthCheck.
func NewHealthCheck() *HealthCheck {
	hc := &HealthCheck{
		exit:      make(chan struct{}),
		updatech:  make(chan endpointOp, 32),
		endpoints: make(map[string]*endpointWrapper, 32),
	}
	go hc.updateEndpoint()
	return hc
}

// AddUpdater adds a updater to monitor the change of the health status
// of the endpoint.
//
// It should be called before any endpoint is added.
func (hc *HealthCheck) AddUpdater(updater Updater) {
	hc.updaters = append(hc.updaters, updater)
}

type statusEnpoind struct {
	Endpoint
	healthy bool
}

func (se statusEnpoind) IsHealthy(context.Context) bool { return se.healthy }

// Endpoints returns the copy of all the endpoints, which cannot be cached.
func (hc *HealthCheck) Endpoints() Endpoints {
	hc.lock.RLock()
	eps := make(Endpoints, 0, len(hc.endpoints))
	for _, ep := range hc.endpoints {
		eps = append(eps, statusEnpoind{Endpoint: ep.Endpoint, healthy: ep.Health})
	}
	hc.lock.RUnlock()
	return eps
}

// AddEndpoint add the endpoint to check its health status.
func (hc *HealthCheck) AddEndpoint(ep Endpoint, interval, timeout time.Duration) {
	ew := &endpointWrapper{
		Exit:     make(chan struct{}),
		Timeout:  timeout,
		Interval: interval,
		Endpoint: ep,
	}

	addr := ep.String()
	hc.lock.Lock()
	hc.endpoints[addr] = ew
	hc.lock.Unlock()
	go hc.check(ew)
}

// DelEndpoint deletes the endpoint in order not to monitor it.
func (hc *HealthCheck) DelEndpoint(endpoint Endpoint) {
	hc.DelEndpointByString(endpoint.String())
}

// DelEndpointByString deletes the endpoint in order not to monitor it.
func (hc *HealthCheck) DelEndpointByString(endpoint string) {
	hc.lock.Lock()
	if ep, ok := hc.endpoints[endpoint]; ok {
		close(ep.Exit)
		hc.updatech <- endpointOp{Add: false, Endpoint: ep.Endpoint}
		delete(hc.endpoints, endpoint)
	}
	hc.lock.Unlock()
}

// Stop stops the check of the health status of all the endpoints.
func (hc *HealthCheck) Stop() {
	hc.lock.RLock()
	for _, ew := range hc.endpoints {
		close(ew.Exit)
	}
	hc.lock.RUnlock()
	hc.exit <- struct{}{}
}

func (hc *HealthCheck) cancelNothing() {}

func (hc *HealthCheck) getContext(ew *endpointWrapper) (context.Context, context.CancelFunc) {
	if ew.Timeout > 0 {
		return context.WithTimeout(context.Background(), ew.Timeout)
	}
	return context.Background(), hc.cancelNothing
}

func (hc *HealthCheck) check(ew *endpointWrapper) {
	ctx, cancel := hc.getContext(ew)
	hc.checkEndpoint(ctx, ew)
	cancel()

	ticker := time.NewTicker(ew.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-hc.exit:
			cancel()
			return
		case <-ew.Exit:
			cancel()
			return
		case <-ticker.C:
			ctx, cancel = hc.getContext(ew)
			hc.checkEndpoint(ctx, ew)
			cancel()
		}
	}
}

func (hc *HealthCheck) checkEndpoint(ctx context.Context, ew *endpointWrapper) {
	if health := ew.Endpoint.IsHealthy(ctx); health != ew.Health {
		ew.Health = health
		if health {
			hc.updatech <- endpointOp{Add: true, Endpoint: ew.Endpoint}
		} else {
			hc.updatech <- endpointOp{Add: false, Endpoint: ew.Endpoint}
		}
	}
}

func (hc *HealthCheck) updateEndpoint() {
	for {
		select {
		case <-hc.exit:
			return
		case epop := <-hc.updatech:
			if epop.Add {
				for _, updater := range hc.updaters {
					updater.AddEndpoint(epop.Endpoint)
				}
			} else {
				for _, updater := range hc.updaters {
					updater.DelEndpoint(epop.Endpoint)
				}
			}
		}
	}
}

// StatusLoadBalancer is the union of LoadBalancer and HealthCheck,
// which will monitor the health status of the added endpoints and add/remove
// the endpoint to/from LoadBalancer.
type StatusLoadBalancer struct {
	*LoadBalancer
	*HealthCheck
}

// NewStatusLoadBalancer returns a new StatusLoadBalancer.
func NewStatusLoadBalancer(provider Provider) *StatusLoadBalancer {
	lb := NewLoadBalancer(provider)
	hc := NewHealthCheck()
	hc.AddUpdater(lb.EndpointManager())
	return &StatusLoadBalancer{
		LoadBalancer: lb,
		HealthCheck:  hc,
	}
}

// Endpoints returns the copy of all the endpoints, which cannot be cached.
//
// If you want to cache them, please use slb.LoadBalancer.EndpointManager.Endpoints().
func (slb *StatusLoadBalancer) Endpoints() Endpoints {
	return slb.HealthCheck.Endpoints()
}

// AddEndpoint is the proxy of slb.HealthCheck.AddEndpoint.
func (slb *StatusLoadBalancer) AddEndpoint(ep Endpoint, interval, timeout time.Duration) {
	slb.HealthCheck.AddEndpoint(ep, interval, timeout)
}

// DelEndpoint is the proxy of slb.HealthCheck.DelEndpoint.
func (slb *StatusLoadBalancer) DelEndpoint(endpoint Endpoint) {
	slb.HealthCheck.DelEndpoint(endpoint)
}

// DelEndpointByString is the proxy of slb.HealthCheck.DelEndpointByString.
func (slb *StatusLoadBalancer) DelEndpointByString(endpoint string) {
	slb.HealthCheck.DelEndpointByString(endpoint)
}
