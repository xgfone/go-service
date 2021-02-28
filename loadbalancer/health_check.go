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
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Updater represents a updater to update the endpoint.
type Updater interface {
	Name() string
	AddEndpoint(Endpoint)
	DelEndpoint(Endpoint)
}

type updaterFunc struct {
	name   string
	update func(addOrDel bool, endpoint Endpoint)
}

func (u updaterFunc) Name() string            { return u.name }
func (u updaterFunc) String() string          { return fmt.Sprintf("Updater(%s)", u.name) }
func (u updaterFunc) AddEndpoint(ep Endpoint) { u.update(true, ep) }
func (u updaterFunc) DelEndpoint(ep Endpoint) { u.update(false, ep) }

// UpdaterFunc covnerts the function to Updater with the name.
//
// If addOrDel is true, it calls the method AddEndpoint. Or call DelEndpoint.
//
// Notice: the returned Updater is comparable.
func UpdaterFunc(name string, f func(addOrDel bool, endpoint Endpoint)) Updater {
	return updaterFunc{name: name, update: f}
}

/////////////////////////////////////////////////////////////////////////////

type endpointWrapper struct {
	Endpoint

	exit     chan struct{}
	tick     *time.Ticker
	lock     sync.RWMutex
	timeout  time.Duration
	interval time.Duration
	retryNum int
	failures int

	health uint32
}

func newEndpointWrapper(ep Endpoint, interval, timeout time.Duration, retryNum int) *endpointWrapper {
	if interval <= 0 {
		interval = time.Second * 10
	}

	return &endpointWrapper{
		Endpoint: ep,
		exit:     make(chan struct{}),
		tick:     time.NewTicker(interval),
		timeout:  timeout,
		interval: interval,
		retryNum: retryNum,
	}
}

func (epw *endpointWrapper) Unwrap() Endpoint { return epw.Endpoint }

func (epw *endpointWrapper) IsHealthy(context.Context) bool {
	return atomic.LoadUint32(&epw.health) == 1
}

func (epw *endpointWrapper) SetHealthy(healthy bool) (ok bool) {
	if healthy {
		return atomic.CompareAndSwapUint32(&epw.health, 0, 1)
	}
	return atomic.CompareAndSwapUint32(&epw.health, 1, 0)
}

func (epw *endpointWrapper) Reset(interval, timeout time.Duration, retryNum int) {
	if interval <= 0 {
		interval = time.Second * 10
	}

	epw.lock.Lock()
	defer epw.lock.Unlock()

	if epw.interval != interval {
		epw.tick.Reset(interval)
		epw.interval = interval
	}

	if epw.timeout != timeout {
		epw.timeout = timeout
	}

	if epw.retryNum != retryNum {
		epw.retryNum = retryNum
	}
}

func (epw *endpointWrapper) resetFailures() {
	epw.lock.Lock()
	epw.failures = 0
	epw.lock.Unlock()
}

func (epw *endpointWrapper) incFailures() (new int) {
	epw.lock.Lock()
	epw.failures++
	new = epw.failures
	epw.lock.Unlock()
	return
}

func (epw *endpointWrapper) Stop() { close(epw.exit) }
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
	timeout, retryNum, failures := epw.timeout, epw.retryNum, epw.failures
	epw.lock.RUnlock()

	ctx := context.Background()
	if timeout > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	healthy := epw.Endpoint.IsHealthy(ctx)
	if healthy {
		hc.updateHealthy(epw, true)
		if failures > 0 {
			epw.resetFailures()
		}
	} else if epw.incFailures() > retryNum {
		hc.updateHealthy(epw, false)
	}
}

/////////////////////////////////////////////////////////////////////////////

var _ ProviderEndpointManager = &HealthCheck{}

type endpointOp struct {
	Add      bool
	Endpoint Endpoint
}

// HealthCheck is used to manage the health of the endpoints.
type HealthCheck struct {
	Timeout  time.Duration // Default: 0s, no limit
	Interval time.Duration // Default: 10s
	RetryNum int           // Default: 0, no retry

	lock        sync.RWMutex
	updaters    map[string]Updater            // map[Updater.Name]Updater
	subscribers map[string]map[string]Updater // map[endpoint]map[Updater.Name]Updater
	endpoints   map[string]*endpointWrapper   // map[endpoint]endpointWrapper
	updatech    chan endpointOp
	exit        chan struct{}
}

// NewHealthCheck returns a new NewHealthCheck.
func NewHealthCheck() *HealthCheck {
	hc := &HealthCheck{
		updaters:    make(map[string]Updater, 4),
		subscribers: make(map[string]map[string]Updater, 16),
		endpoints:   make(map[string]*endpointWrapper, 32),
		updatech:    make(chan endpointOp, 32),
		exit:        make(chan struct{}),
	}
	go hc.updateEndpoint()
	return hc
}

// Subscribe subscribes the update of the special endpoint, that's, the updater
// is called only when the status of the special endpoint has been changed.
//
// If endpoint is "", the updater is called only if any endpoint has changed.
//
// Notice: It should be called before any endpoint is added.
func (hc *HealthCheck) Subscribe(endpoint string, updater Updater) error {
	name := updater.Name()

	hc.lock.Lock()
	defer hc.lock.Unlock()

	var ok bool
	updaters := hc.updaters
	if endpoint != "" {
		if updaters, ok = hc.subscribers[endpoint]; !ok {
			updaters = make(map[string]Updater, 2)
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
		hc.updaters = make(map[string]Updater, 2)
	} else {
		delete(hc.subscribers, endpoint)
	}
	hc.lock.Unlock()
}

// GetAllSubscribers returns all the subscribers.
func (hc *HealthCheck) GetAllSubscribers() map[string][]Updater {
	hc.lock.RLock()
	us := make(map[string][]Updater, len(hc.subscribers)+1)

	updaters := make([]Updater, 0, len(hc.updaters))
	for _, updater := range hc.updaters {
		updaters = append(updaters, updater)
	}
	us[""] = updaters

	for endpoint, eupdaters := range hc.subscribers {
		updaters := make([]Updater, 0, len(eupdaters))
		for _, updater := range eupdaters {
			updaters = append(updaters, updater)
		}
		us[endpoint] = updaters
	}

	hc.lock.RUnlock()
	return us
}

// GetSubscribers returns the subscribers of the endpoint.
func (hc *HealthCheck) GetSubscribers(endpoint string) (updaters []Updater) {
	hc.lock.RLock()
	if endpoint == "" {
		updaters = make([]Updater, 0, len(hc.updaters))
		for _, updater := range hc.updaters {
			updaters = append(updaters, updater)
		}
	} else {
		eupdaters := hc.subscribers[endpoint]
		updaters = make([]Updater, 0, len(eupdaters))
		for _, updater := range eupdaters {
			updaters = append(updaters, updater)
		}
	}
	hc.lock.RUnlock()
	return
}

// UnsubscribeByUpdater unsubscribes the special updater of all the endpoints.
func (hc *HealthCheck) UnsubscribeByUpdater(updater Updater) {
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

// HasEndpoint reports whether the endpoint has added.
func (hc *HealthCheck) HasEndpoint(endpoint Endpoint) bool {
	addr := endpoint.String()
	hc.lock.RLock()
	_, ok := hc.endpoints[addr]
	hc.lock.RUnlock()
	return ok
}

// AddEndpointWithDuration add the endpoint to check its health status.
//
// If the endpoint has been added, update it.
func (hc *HealthCheck) AddEndpointWithDuration(ep Endpoint, interval, timeout time.Duration, retryNum int) {
	addr := ep.String()
	hc.lock.Lock()
	if ew, ok := hc.endpoints[addr]; ok {
		ew.Reset(interval, timeout, retryNum)
	} else {
		ew := newEndpointWrapper(ep, interval, timeout, retryNum)
		hc.endpoints[addr] = ew
		go ew.Check(hc)
	}
	hc.lock.Unlock()
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

// AddEndpoint is equal to hc.AddEndpointWithDuration(ep, hc.Interval, hc.Timeout).
func (hc *HealthCheck) AddEndpoint(ep Endpoint) {
	hc.AddEndpointWithDuration(ep, hc.Interval, hc.Timeout, hc.RetryNum)
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
		ep.Stop()
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
