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

// EndpointCheckerDurationZero is the ZERO value of EndpointCheckerDuration.
var EndpointCheckerDurationZero EndpointCheckerDuration

// EndpointChecker is used to check the health status of the endpoint.
type EndpointChecker interface {
	Check(context.Context) error
}

// EndpointCheckerDuration is the check duation of the endpoint.
type EndpointCheckerDuration struct {
	Timeout  time.Duration `json:"timeout,omitempty"  xml:"timeout,omitempty"`
	Interval time.Duration `json:"interval,omitempty" xml:"interval,omitempty"`
	RetryNum int           `json:"retrynum,omitempty" xml:"retrynum,omitempty"`
}

// EndpointCheckerInfo is the information of the endpoint.
type EndpointCheckerInfo struct {
	Endpoint Endpoint
	Duration EndpointCheckerDuration
	Checker  EndpointChecker
	Healthy  bool
}

type healthChecker struct {
	Endpoint

	lock     sync.RWMutex
	checker  EndpointChecker
	duration EndpointCheckerDuration

	failures int
	healthy  int32

	exit chan struct{}
	tick *time.Ticker
}

var zeroDuration EndpointCheckerDuration

func newHealthChecker(endpoint Endpoint, checker EndpointChecker,
	duration EndpointCheckerDuration) *healthChecker {
	if duration.Interval <= 0 {
		duration.Interval = time.Second * 10
	}

	return &healthChecker{
		Endpoint: endpoint,
		duration: duration,
		checker:  checker,
		exit:     make(chan struct{}),
		tick:     time.NewTicker(duration.Interval),
	}
}

func (c *healthChecker) Stop()           { close(c.exit) }
func (c *healthChecker) IsHealthy() bool { return atomic.LoadInt32(&c.healthy) == 1 }

func (c *healthChecker) GetChecker() (EndpointChecker, EndpointCheckerDuration) {
	c.lock.RLock()
	checker, duration := c.checker, c.duration
	c.lock.RUnlock()
	return checker, duration
}

func (c *healthChecker) ResetChecker(ec EndpointChecker, d EndpointCheckerDuration) {
	c.lock.Lock()
	if d != zeroDuration {
		if d.Interval <= 0 {
			d.Interval = time.Second * 10
		}
		if d.Interval != c.duration.Interval {
			c.tick.Reset(d.Interval)
		}

		c.duration = d
	}
	c.checker = ec
	c.lock.Unlock()
}

func (c *healthChecker) Check(hc *HealthCheck) {
	defer c.tick.Stop()
	c.check(hc)
	for {
		select {
		case <-hc.exit:
			return
		case <-c.exit:
			return
		case <-c.tick.C:
			c.check(hc)
		}
	}
}

func (c *healthChecker) check(hc *HealthCheck) {
	defer func() { // Prevent IsHealthy from panicking.
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()

	c.lock.RLock()
	duration, checker := c.duration, c.checker
	c.lock.RUnlock()

	ctx := context.Background()
	if duration.Timeout > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, duration.Timeout)
		defer cancel()
	}

	if checker.Check(ctx) == nil {
		c.update(hc, true)
		if c.failures > 0 {
			c.failures = 0
		}
	} else if c.failures++; c.failures > duration.RetryNum {
		c.update(hc, false)
	}
}

func (c *healthChecker) update(hc *HealthCheck, healthy bool) {
	if healthy {
		if atomic.CompareAndSwapInt32(&c.healthy, 0, 1) {
			hc.updateEndpointHealth(c.Endpoint, true)
		}
	} else if atomic.CompareAndSwapInt32(&c.healthy, 1, 0) {
		hc.updateEndpointHealth(c.Endpoint, false)
	}
}

/////////////////////////////////////////////////////////////////////////////

// HealthCheck is used to manage all the health checker.
type HealthCheck struct {
	exit chan struct{}

	uplock   sync.RWMutex
	updaters map[string]EndpointUpdater

	chlock   sync.RWMutex
	checkers map[string]*healthChecker
}

// NewHealthCheck returns a new NewHealthCheck with the duration.
func NewHealthCheck() *HealthCheck {
	return &HealthCheck{
		exit:     make(chan struct{}),
		updaters: make(map[string]EndpointUpdater, 4),
		checkers: make(map[string]*healthChecker, 32),
	}
}

func (hc *HealthCheck) updateEndpointHealth(ep Endpoint, healthy bool) {
	hc.uplock.RLock()
	if healthy {
		for _, updater := range hc.updaters {
			updater.AddEndpoint(ep)
		}
	} else {
		for _, updater := range hc.updaters {
			updater.DelEndpoint(ep)
		}
	}
	hc.uplock.RUnlock()
}

// AddUpdater sets the health status updater, which will be called
// when the health status has been changed.
func (hc *HealthCheck) AddUpdater(name string, updater EndpointUpdater) (err error) {
	hc.uplock.Lock()
	if _, ok := hc.updaters[name]; ok {
		err = fmt.Errorf("the updater '%s' has been added", name)
	} else {
		hc.updaters[name] = updater
	}
	hc.uplock.Unlock()
	return
}

// DelUpdater deletes and returns the updater named name.
//
// If the observer doest not exist, do nothing and return nil.
func (hc *HealthCheck) DelUpdater(name string) (updater EndpointUpdater) {
	hc.uplock.Lock()
	updater, ok := hc.updaters[name]
	if ok {
		delete(hc.updaters, name)
	}
	hc.uplock.Unlock()
	return
}

// GetUpdaterr returns the updater named name.
//
// If the observer does not exist, return nil.
func (hc *HealthCheck) GetUpdaterr(name string) (updater EndpointUpdater) {
	hc.uplock.RLock()
	updater = hc.updaters[name]
	hc.uplock.RUnlock()
	return
}

// GetUpdaters returns all the updaters.
func (hc *HealthCheck) GetUpdaters() (updaters map[string]EndpointUpdater) {
	hc.uplock.RLock()
	updaters = make(map[string]EndpointUpdater, len(hc.updaters))
	for name, updater := range hc.updaters {
		updaters[name] = updater
	}
	hc.uplock.RUnlock()
	return
}

////////////////////////////////////////////////////////////////////////////

// Stop stops all the health checkers.
func (hc *HealthCheck) Stop() { close(hc.exit) }

// IsHealthy reports whether the health checker is healthy.
func (hc *HealthCheck) IsHealthy(endpointID string) (yes bool) {
	hc.chlock.RLock()
	if ch, ok := hc.checkers[endpointID]; ok {
		yes = ch.IsHealthy()
	}
	hc.chlock.RUnlock()
	return
}

// GetEndpoint returns the endpoint information.
func (hc *HealthCheck) GetEndpoint(endpointID string) (ep EndpointCheckerInfo, ok bool) {
	hc.chlock.RLock()
	c, ok := hc.checkers[endpointID]
	if ok {
		ep.Checker, ep.Duration = c.GetChecker()
		ep.Endpoint, ep.Healthy = c.Endpoint, c.IsHealthy()
	}
	hc.chlock.RUnlock()
	return
}

// GetEndpoints returns the information of all the endpoints.
func (hc *HealthCheck) GetEndpoints() []EndpointCheckerInfo {
	hc.chlock.RLock()
	eps := make([]EndpointCheckerInfo, 0, len(hc.checkers))
	for _, c := range hc.checkers {
		checker, duration := c.GetChecker()
		eps = append(eps, EndpointCheckerInfo{
			Endpoint: c.Endpoint,
			Duration: duration,
			Checker:  checker,
			Healthy:  c.IsHealthy(),
		})
	}
	hc.chlock.RUnlock()
	return eps
}

// AddEndpoint adds the endpoint with the checker and the duration.
//
// If duration.Interval is ZERO, it is time.Second*10 by default.
//
// If the checker has been added, return an error.
func (hc *HealthCheck) AddEndpoint(endpoint Endpoint, checker EndpointChecker,
	duration EndpointCheckerDuration) (err error) {
	id := endpoint.ID()
	hc.chlock.Lock()
	if _, ok := hc.checkers[id]; ok {
		err = fmt.Errorf("the endpoint '%s' has been added", id)
	} else {
		c := newHealthChecker(endpoint, checker, duration)
		hc.checkers[id] = c
		go c.Check(hc)
	}
	hc.chlock.Unlock()
	return
}

// SetEndpointChecker resets the checker and the duration of the endpoint.
//
// If the endpoint does not exist, do nothing.
// If checker is nil, do not reset it and use the old.
// If duration is ZERO, do not reset it and use the old.
func (hc *HealthCheck) SetEndpointChecker(epid string, checker EndpointChecker,
	duration EndpointCheckerDuration) {
	hc.chlock.Lock()
	if c, ok := hc.checkers[epid]; ok {
		c.ResetChecker(checker, duration)
	}
	hc.chlock.Lock()
}

// DelEndpoint is equal to DelEndpointByID(ep.ID())
func (hc *HealthCheck) DelEndpoint(ep Endpoint) { hc.DelEndpointByID(ep.ID()) }

// DelEndpoint deletes the endpoint by the id.
//
// If the endpoint does not exist, do nothing.
func (hc *HealthCheck) DelEndpointByID(endpointID string) {
	hc.chlock.Lock()
	if c, ok := hc.checkers[endpointID]; ok {
		delete(hc.checkers, endpointID)
		c.Stop()
	}
	hc.chlock.Unlock()
}
