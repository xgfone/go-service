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

// Package task provides some task service functions.
package task

import (
	"context"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/xgfone/go-service"
)

var (
	checkcond   atomic.Value // = checktrue
	checkintvl  atomic.Value // = time.Second * 10
	ctx, cancel = context.WithCancel(context.Background())
)

func init() {
	SetCheckCond(checktrue)
	SetCheckInterval(time.Second * 10)
}

// StartChecker starts the checker to activate or deactivate the default services
// by the check condition.
func StartChecker() {
	var activated bool
	go runforever(ctx, 0, checkintvl.Load().(time.Duration), func() {
		ok := checkcond.Load().(func(context.Context) bool)(ctx)
		if ok != activated {
			if ok {
				service.DefaultServices.Activate()
			} else {
				service.DefaultServices.Deactivate()
			}
			activated = ok
		}
	})
}

// StopChecker stops the checker.
func StopChecker() { cancel(); service.DefaultServices.Deactivate() }

// SetCheckCond resets the check condition of the monitor service.
//
// Default: a fucntion returning true always.
func SetCheckCond(cond func(context.Context) bool) { checkcond.Store(cond) }

// SetVipCheckCond is a convenient function to set the checker based on vip.
func SetVipCheckCond(vip string) { SetCheckCond(checkvip(vip)) }

// SetCheckInterval resets the check interval duration of the checker.
//
// Default: 10s
func SetCheckInterval(interval time.Duration) { checkintvl.Store(interval) }

// RunOrElse runs the task function synchronously if service.DefaultProxy is activated.
// Or, run the elsef function.
func RunOrElse(delay, interval time.Duration, taskf func(context.Context), elsef func()) {
	runforever(ctx, delay, interval, func() {
		service.DefaultProxy.RunFunc(taskf, elsef)
	})
}

// Run is equal to RunOrElse(delay, interval, taskf, nil).
func Run(delay, interval time.Duration, taskf func(context.Context)) {
	RunOrElse(delay, interval, taskf, nil)
}

// RunAlways always runs the task function synchronously and periodically.
func RunAlways(delay, interval time.Duration, task func(context.Context)) {
	runforever(ctx, delay, interval, func() { task(ctx) })
}

func runforever(ctx context.Context, delay, interval time.Duration, task func()) {
	if interval <= 0 {
		panic("interval duration must be greater than 0")
	}

	if delay > 0 {
		delay = delay + time.Duration(rand.Float64()*float64(delay))
		t := time.NewTimer(delay)
		select {
		case <-t.C:
		case <-ctx.Done():
			if !t.Stop() {
				select {
				case <-t.C:
				default:
				}
			}
			return
		}
	}

	saferun(task) // first run
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			saferun(task)
		}
	}
}

func saferun(f func()) {
	defer wrappanic()
	f()
}

func wrappanic() {
	if r := recover(); r != nil {
		log.Printf("wrap a panic: %v", r)
	}
}

func checktrue(context.Context) bool { return true }
func checkvip(vip string) func(context.Context) bool {
	if ip := net.ParseIP(vip); ip != nil {
		vip = ip.String()
	}

	return func(ctx context.Context) bool {
		if vip == "" {
			return true
		}

		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return false
		}

		for _, addr := range addrs {
			if strings.Split(addr.String(), "/")[0] == vip {
				return true
			}
		}

		return false
	}
}
