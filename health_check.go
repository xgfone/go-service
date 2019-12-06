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

type updaterFunc struct {
	update func(addOrDel bool, endpoint Endpoint)
}

func (u *updaterFunc) AddEndpoint(ep Endpoint) { u.update(true, ep) }
func (u *updaterFunc) DelEndpoint(ep Endpoint) { u.update(false, ep) }

// UpdaterFunc covnerts the function to Updater.
//
// If addOrDel is true, it calls the method AddEndpoint. Or call DelEndpoint.
//
// Notice: the returned Updater is comparable.
func UpdaterFunc(f func(addOrDel bool, endpoint Endpoint)) Updater {
	return &updaterFunc{update: f}
}

type updaters []Updater

func (us updaters) Len() int      { return len(us) }
func (us updaters) Swap(i, j int) { us[i], us[j] = us[j], us[i] }
func (us updaters) Less(i, j int) bool {
	if us[i] == nil {
		return false
	}
	return true
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
	updatech  chan endpointOp
	updaters  []Updater
	updaterms map[string][]Updater        // endpoint => []Updater
	endpoints map[string]*endpointWrapper // endpoint => endpointWrapper
}

// NewHealthCheck returns a new NewHealthCheck.
func NewHealthCheck() *HealthCheck {
	hc := &HealthCheck{
		exit:      make(chan struct{}),
		updatech:  make(chan endpointOp, 32),
		updaterms: make(map[string][]Updater, 4),
		endpoints: make(map[string]*endpointWrapper, 32),
	}
	go hc.updateEndpoint()
	return hc
}

// GetUpdater returns the updaters by the endpoint.
func (hc *HealthCheck) GetUpdater(endpoint string) []Updater {
	hc.lock.RLock()
	updaters := hc.updaters
	if endpoint != "" {
		updaters = hc.updaterms[endpoint]
	}
	us := make([]Updater, len(hc.updaters))
	copy(us, updaters)
	hc.lock.RUnlock()
	return us
}

// AddUpdater is equal to hc.Subscribe("", updater).
func (hc *HealthCheck) AddUpdater(updater Updater) { hc.Subscribe("", updater) }

// Subscribe subscribes the update of the special endpoint, that's, the updater
// is called only when the status of the special endpoint has been changed.
//
// If endpoint is "", the updater is called only if any endpoint has changed.
//
// Notice: It should be called before any endpoint is added. And you should
// subscribe the same updater for the same endpoint.
func (hc *HealthCheck) Subscribe(endpoint string, updater Updater) {
	hc.lock.Lock()
	defer hc.lock.Unlock()

	if endpoint == "" {
		hc.updaters = append(hc.updaters, updater)
	} else {
		hc.updaterms[endpoint] = append(hc.updaterms[endpoint], updater)
	}
}

// Unsubscribe unsubscribes all the updaters of the special endpoint.
//
// Notice: the endpoint must not be empty.
func (hc *HealthCheck) Unsubscribe(endpoint string) {
	if endpoint == "" {
		panic("HealthCheck.Unsubscribe: endpoint must not be empty")
	}

	hc.lock.Lock()
	defer hc.lock.Unlock()
	delete(hc.updaterms, endpoint)
}

// UnsubscribeByUpdater unsubscribes the special updater of all the endpoints.
//
// Notice: updater must be comparable.
func (hc *HealthCheck) UnsubscribeByUpdater(updater Updater) {
	hc.lock.Lock()
	defer hc.lock.Unlock()

	var num int
	for i, u := range hc.updaters {
		if u == updater {
			num++
			hc.updaters[i] = nil
		}
	}
	if num > 0 {
		sort.Sort(updaters(hc.updaters))
		hc.updaters = hc.updaters[:len(hc.updaters)-num]
	}

	for ep, us := range hc.updaterms {
		num = 0
		for i, u := range us {
			if u == updater {
				num++
				us[i] = nil
			}
		}

		if len(us) == num {
			delete(hc.updaterms, ep)
		} else if num > 0 {
			sort.Sort(updaters(us))
			hc.updaterms[ep] = us[:len(us)-num]
		}
	}
}

type statusEnpoind struct {
	Endpoint
	healthy bool
}

func (se statusEnpoind) Unwrap() Endpoint               { return se.Endpoint }
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
	ep, ok := hc.endpoints[endpoint]
	if ok {
		delete(hc.endpoints, endpoint)
	}
	hc.lock.Unlock()

	if ok {
		close(ep.Exit)
		hc.updatech <- endpointOp{Add: false, Endpoint: ep.Endpoint}
	}
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
			hc.updateEndpointSafely(epop.Add, epop.Endpoint)
		}
	}
}

func (hc *HealthCheck) updateEndpointSafely(add bool, ep Endpoint) {
	hc.lock.RLock()
	defer hc.lock.RUnlock()

	if add {
		for _, updater := range hc.updaters {
			updater.AddEndpoint(ep)
		}
		for _, updater := range hc.updaterms[ep.String()] {
			updater.AddEndpoint(ep)
		}
	} else {
		for _, updater := range hc.updaters {
			updater.DelEndpoint(ep)
		}
		for _, updater := range hc.updaterms[ep.String()] {
			updater.DelEndpoint(ep)
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
// If you want to cache them, please use slb.LoadBalancer.EndpointManager().Endpoints().
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
