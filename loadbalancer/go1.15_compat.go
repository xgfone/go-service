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

// +build !go1.15

package loadbalancer

import (
	"sync"
	"time"
)

type ticker struct {
	lock sync.RWMutex
	tick *time.Ticker
}

func newTicker(interval time.Duration) (t ticker) {
	t.tick = time.NewTicker(interval)
	return
}

func (t *ticker) TimeChan() <-chan time.Time {
	t.lock.RLock()
	tick := t.tick
	t.lock.RUnlock()
	return tick.C
}

func (t *ticker) Reset(d time.Duration) {
	t.lock.Lock()
	t.tick.Stop()
	t.tick = time.NewTicker(d)
	t.lock.Unlock()
}

func (t *ticker) Stop() {
	t.lock.RLock()
	t.tick.Stop()
	t.lock.RUnlock()
}
