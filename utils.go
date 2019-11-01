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
	"sync"
)

type eventCallbacks struct {
	sync.RWMutex
	funcs []func(Endpoint)
}

func newEventCallbacks() *eventCallbacks {
	return &eventCallbacks{}
}

func (e *eventCallbacks) Append(f func(Endpoint)) {
	if f == nil {
		panic("the event callback function must not be nil")
	}

	e.Lock()
	e.funcs = append(e.funcs, f)
	e.Unlock()
}

func (e *eventCallbacks) Call(endpoint Endpoint) {
	if endpoint == nil {
		return
	}

	e.RLock()
	defer e.RUnlock()
	for _, f := range e.funcs {
		f(endpoint)
	}
}
