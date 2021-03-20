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
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

type endpointWrapper struct {
	Endpoint

	exit chan struct{}
	tick *time.Ticker
	lock sync.RWMutex
	hcop HealthCheckOption
	fail int

	count  int32
	health uint32
}

func newEndpointWrapper(ep Endpoint, o HealthCheckOption) *endpointWrapper {
	if o.Interval <= 0 {
		o.Interval = time.Second * 10
	}

	return &endpointWrapper{
		Endpoint: ep,
		count:    1,
		hcop:     o,
		exit:     make(chan struct{}),
		tick:     time.NewTicker(o.Interval),
	}
}

func (epw *endpointWrapper) Stop()                    { close(epw.exit) }
func (epw *endpointWrapper) IncCount()                { atomic.AddInt32(&epw.count, 1) }
func (epw *endpointWrapper) DecCount() int32          { return atomic.AddInt32(&epw.count, -1) }
func (epw *endpointWrapper) ReferenceCount() int32    { return atomic.LoadInt32(&epw.count) }
func (epw *endpointWrapper) UnwrapEndpoint() Endpoint { return epw.Endpoint }

func (epw *endpointWrapper) IsHealthy(context.Context) bool {
	return atomic.LoadUint32(&epw.health) == 1
}

func (epw *endpointWrapper) SetHealthy(healthy bool) (ok bool) {
	if healthy {
		return atomic.CompareAndSwapUint32(&epw.health, 0, 1)
	}
	return atomic.CompareAndSwapUint32(&epw.health, 1, 0)
}

func (epw *endpointWrapper) Reset(o HealthCheckOption) {
	if o.Interval <= 0 {
		o.Interval = time.Second * 10
	}

	epw.lock.Lock()
	if epw.hcop != o {
		epw.hcop = o
		epw.tick.Reset(o.Interval)
	}
	epw.lock.Unlock()
}

func (epw *endpointWrapper) resetFailures() {
	epw.lock.Lock()
	epw.fail = 0
	epw.lock.Unlock()
}

func (epw *endpointWrapper) incFailures() (new int) {
	epw.lock.Lock()
	epw.fail++
	new = epw.fail
	epw.lock.Unlock()
	return
}

func (epw *endpointWrapper) Check(hc *HealthCheck) {
	defer epw.tick.Stop()
	epw.check(hc)
	for {
		select {
		case <-hc.exit:
			return
		case <-epw.exit:
			return
		case <-epw.tick.C:
			epw.check(hc)
		}
	}
}

func (epw *endpointWrapper) check(hc *HealthCheck) {
	defer func() { // Prevent IsHealthy from panicking.
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()

	epw.lock.RLock()
	o, failures := epw.hcop, epw.fail
	epw.lock.RUnlock()

	ctx := context.Background()
	if o.Timeout > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, o.Timeout)
		defer cancel()
	}

	if epw.Endpoint.IsHealthy(ctx) {
		hc.updateHealthy(epw, true)
		if failures > 0 {
			epw.resetFailures()
		}
	} else if epw.incFailures() > o.RetryNum {
		hc.updateHealthy(epw, false)
	}
}

/////////////////////////////////////////////////////////////////////////////

var _ EndpointManager = &HealthCheck{}

type endpointOp struct {
	Add      bool
	Endpoint Endpoint
}

// HealthCheckOption is the duation option to check the endpoint.
type HealthCheckOption struct {
	Timeout  time.Duration `json:"timeout,omitempty" xml:"timeout,omitempty"`
	Interval time.Duration `json:"interval,omitempty" xml:"interval,omitempty"`
	RetryNum int           `json:"retrynum,omitempty" xml:"retrynum,omitempty"`
}

// HealthCheck is used to manage the health of the endpoints.
type HealthCheck struct {
	lock        sync.RWMutex
	option      HealthCheckOption
	updaters    map[string]EndpointUpdater            // map[EndpointUpdater.Name]EndpointUpdater
	subscribers map[string]map[string]EndpointUpdater // map[endpoint]map[EndpointUpdater.Name]EndpointUpdater
	endpoints   map[string]*endpointWrapper           // map[endpoint]endpointWrapper
	updatech    chan endpointOp
	exit        chan struct{}
}

// NewHealthCheck returns a new NewHealthCheck with the option.
//
//   HealthCheckOption{Interval: time.Second * 10}
//
func NewHealthCheck() *HealthCheck {
	hc := &HealthCheck{
		updaters:    make(map[string]EndpointUpdater, 4),
		subscribers: make(map[string]map[string]EndpointUpdater, 16),
		endpoints:   make(map[string]*endpointWrapper, 32),
		updatech:    make(chan endpointOp, 32),
		exit:        make(chan struct{}),
	}
	hc.SetDefaultOption(HealthCheckOption{Interval: time.Second * 10})
	go hc.updateEndpoint()
	return hc
}

// GetDefaultOption returns the default option of the health check.
func (hc *HealthCheck) GetDefaultOption() HealthCheckOption {
	hc.lock.RLock()
	option := hc.option
	hc.lock.RUnlock()
	return option
}

// SetDefaultOption sets the default option of the health check to o,
// which will be used by the method AddEndpoint as the health check option.
func (hc *HealthCheck) SetDefaultOption(o HealthCheckOption) {
	hc.lock.Lock()
	hc.option = o
	hc.lock.Unlock()
}

// Subscribe subscribes the update of the special endpoint, that's, the updater
// is called only when the status of the special endpoint has been changed.
//
// If endpoint is "", the updater is called only if any endpoint has changed.
//
// Notice: It should be called before any endpoint is added.
func (hc *HealthCheck) Subscribe(endpoint string, updater EndpointUpdater) error {
	name := updater.Name()

	hc.lock.Lock()
	defer hc.lock.Unlock()

	var ok bool
	updaters := hc.updaters
	if endpoint != "" {
		if updaters, ok = hc.subscribers[endpoint]; !ok {
			updaters = make(map[string]EndpointUpdater, 2)
			hc.subscribers[endpoint] = updaters
		}
	}

	if _, exist := updaters[name]; exist {
		return fmt.Errorf("the updater '%s' for the endpoint '%s' has been subscribed", name, endpoint)
	}

	updaters[name] = updater
	return nil
}

// Unsubscribe unsubscribes all the updaters of the special endpoint.
func (hc *HealthCheck) Unsubscribe(endpoint string) {
	hc.lock.Lock()
	if endpoint == "" {
		hc.updaters = make(map[string]EndpointUpdater, 2)
	} else {
		delete(hc.subscribers, endpoint)
	}
	hc.lock.Unlock()
}

// GetAllSubscribers returns all the subscribers.
func (hc *HealthCheck) GetAllSubscribers() map[string][]EndpointUpdater {
	hc.lock.RLock()
	us := make(map[string][]EndpointUpdater, len(hc.subscribers)+1)

	updaters := make([]EndpointUpdater, 0, len(hc.updaters))
	for _, updater := range hc.updaters {
		updaters = append(updaters, updater)
	}
	us[""] = updaters

	for endpoint, eupdaters := range hc.subscribers {
		updaters := make([]EndpointUpdater, 0, len(eupdaters))
		for _, updater := range eupdaters {
			updaters = append(updaters, updater)
		}
		us[endpoint] = updaters
	}

	hc.lock.RUnlock()
	return us
}

// GetSubscribers returns the subscribers of the endpoint.
func (hc *HealthCheck) GetSubscribers(endpoint string) (updaters []EndpointUpdater) {
	hc.lock.RLock()
	if endpoint == "" {
		updaters = make([]EndpointUpdater, 0, len(hc.updaters))
		for _, updater := range hc.updaters {
			updaters = append(updaters, updater)
		}
	} else {
		eupdaters := hc.subscribers[endpoint]
		updaters = make([]EndpointUpdater, 0, len(eupdaters))
		for _, updater := range eupdaters {
			updaters = append(updaters, updater)
		}
	}
	hc.lock.RUnlock()
	return
}

// UnsubscribeByUpdater unsubscribes the special updater of all the endpoints.
func (hc *HealthCheck) UnsubscribeByUpdater(updater EndpointUpdater) {
	name := updater.Name()
	hc.lock.Lock()

	delete(hc.updaters, name)
	for endpoint, eupdaters := range hc.subscribers {
		delete(eupdaters, name)
		if len(eupdaters) == 0 {
			delete(hc.subscribers, endpoint)
		}
	}

	hc.lock.Unlock()
}

/////////////////////////////////////////////////////////////////////////////

// SetHealthy sets the healthy of the endpoint, and returns whether it
// successfully updates the health status of the endpoint or not.
func (hc *HealthCheck) SetHealthy(endpoint string, healthy bool) (ok bool) {
	hc.lock.RLock()
	ep := hc.endpoints[endpoint]
	hc.lock.RUnlock()
	return hc.updateHealthy(ep, healthy)
}

// IsHealthy reports whether the endpoint is healthy.
func (hc *HealthCheck) IsHealthy(endpoint string) (yes bool) {
	hc.lock.RLock()
	if ep, ok := hc.endpoints[endpoint]; ok {
		yes = ep.IsHealthy(context.TODO())
	}
	hc.lock.RUnlock()
	return
}

// ReferenceCount returns the reference count of the endpoint.
//
// If the endpoint does not exist, return 0.
func (hc *HealthCheck) ReferenceCount(endpoint string) (rc int32) {
	hc.lock.RLock()
	if ep, ok := hc.endpoints[endpoint]; ok {
		rc = ep.ReferenceCount()
	}
	hc.lock.RUnlock()
	return
}

// Endpoint returns the monitored endpoint.
//
// If the endpoint does not exist, return nil.
func (hc *HealthCheck) Endpoint(endpoint string) Endpoint {
	hc.lock.RLock()
	ep := hc.endpoints[endpoint]
	hc.lock.RUnlock()
	return ep
}

// HasEndpoint reports whether the endpoint has added.
func (hc *HealthCheck) HasEndpoint(endpoint string) bool {
	hc.lock.RLock()
	_, ok := hc.endpoints[endpoint]
	hc.lock.RUnlock()
	return ok
}

// GetEndpointsByUpdater returns the list of the endpoints by the updater.
func (hc *HealthCheck) GetEndpointsByUpdater(u EndpointUpdater) (eps Endpoints) {
	name := u.Name()
	hc.lock.RLock()
	for endpoint, updaters := range hc.subscribers {
		if _, ok := updaters[name]; ok {
			if ep, ok := hc.endpoints[endpoint]; ok {
				eps = append(eps, ep)
			}
		}
	}
	hc.lock.RUnlock()
	return
}

// Endpoints returns the copy of all the endpoints.
func (hc *HealthCheck) Endpoints() Endpoints {
	hc.lock.RLock()
	eps := make(Endpoints, 0, len(hc.endpoints))
	for _, ep := range hc.endpoints {
		eps = append(eps, ep)
	}
	hc.lock.RUnlock()
	return eps
}

// AddEndpointWithDuration add the endpoint to check its health status.
//
// If the endpoint has been added, update it.
func (hc *HealthCheck) AddEndpointWithDuration(ep Endpoint, o HealthCheckOption) {
	addr := ep.String()
	hc.lock.Lock()
	if ew, ok := hc.endpoints[addr]; ok {
		ew.IncCount()
		ew.Reset(o)
	} else {
		ew := newEndpointWrapper(ep, o)
		hc.endpoints[addr] = ew
		go ew.Check(hc)
	}
	hc.lock.Unlock()
}

// AddEndpoint is equal to hc.AddEndpointWithDuration(ep, hc.GetOption()).
func (hc *HealthCheck) AddEndpoint(ep Endpoint) {
	hc.AddEndpointWithDuration(ep, hc.GetDefaultOption())
}

// DelEndpoint is equal to DelEndpointByString(endpoint.String()).
func (hc *HealthCheck) DelEndpoint(endpoint Endpoint) {
	hc.DelEndpointByString(endpoint.String())
}

// DelEndpointByString deletes the endpoint in order not to monitor it.
//
// Notice: In order to really delete the endpoint, call the same times
// of DelEndpoint as AddEndpoint.
func (hc *HealthCheck) DelEndpointByString(endpoint string) {
	hc.delEndpointSafe(endpoint, false)
}

// DelEndpointByForce ignores the reference count of the endpoint
// deletes it forcibly.
func (hc *HealthCheck) DelEndpointByForce(endpoint string) {
	hc.delEndpointSafe(endpoint, true)
}

// DelEndpointsByUpdater deletes the endpoints by the updater.
func (hc *HealthCheck) DelEndpointsByUpdater(u EndpointUpdater) {
	name := u.Name()
	hc.lock.Lock()
	for endpoint, updaters := range hc.subscribers {
		if _, ok := updaters[name]; ok {
			hc.delEndpoint(endpoint, false)
		}
	}
	hc.lock.Unlock()
}

func (hc *HealthCheck) delEndpointSafe(endpoint string, force bool) {
	hc.lock.Lock()
	hc.delEndpoint(endpoint, force)
	hc.lock.Unlock()
}

func (hc *HealthCheck) delEndpoint(endpoint string, force bool) {
	ep, ok := hc.endpoints[endpoint]
	if ok = ok && (force || ep.DecCount() <= 0); ok {
		ep.Stop()
		delete(hc.endpoints, endpoint)
		hc.updatech <- endpointOp{Add: false, Endpoint: ep.Endpoint}
	}
}

// Stop stops the check of the health status of all the endpoints.
func (hc *HealthCheck) Stop() {
	hc.lock.RLock()
	for _, ew := range hc.endpoints {
		ew.Stop()
	}
	hc.lock.RUnlock()
	close(hc.exit)
}

func (hc *HealthCheck) updateHealthy(ew *endpointWrapper, healthy bool) (ok bool) {
	if ew == nil {
		return
	}

	if ok = ew.SetHealthy(healthy); ok {
		hc.updatech <- endpointOp{Add: healthy, Endpoint: ew.Endpoint}
	}
	return
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
	addr := ep.String()
	hc.lock.RLock()
	defer hc.lock.RUnlock()

	if add {
		for _, updater := range hc.updaters {
			updater.AddEndpoint(ep)
		}

		for _, updater := range hc.subscribers[addr] {
			updater.AddEndpoint(ep)
		}
	} else {
		for _, updater := range hc.updaters {
			updater.DelEndpoint(ep)
		}

		for _, updater := range hc.subscribers[addr] {
			updater.DelEndpoint(ep)
		}
	}
}
