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
	"sync/atomic"
	"testing"
	"time"
)

func TestProxy(t *testing.T) {
	parent, pcancel := context.WithCancel(context.Background())
	proxy := NewProxy(parent)
	defer proxy.Deactivate()

	runTask := func(f func(context.Context)) {
		proxy.Run(func(c context.Context) { go f(c) })
	}

	if proxy.Context() != nil {
		t.Errorf("unexpect the context")
	}
	if proxy.IsActivated() {
		t.Errorf("unexpect the task service is activated")
	}
	runTask(func(context.Context) {
		t.Errorf("unexpect the task is run")
	})

	proxy.Activate()

	if proxy.Context() == nil {
		t.Errorf("expect the context, but got nil")
	}
	if !proxy.IsActivated() {
		t.Errorf("the task service is not activated")
	}

	var run atomic.Value
	runTask(func(context.Context) { run.Store(true) })
	time.Sleep(time.Millisecond * 10)
	if v := run.Load(); v == nil || !v.(bool) {
		t.Errorf("the task is not run")
	}

	var err atomic.Value
	runTask(func(ctx context.Context) {
		<-ctx.Done()
		err.Store(ctx.Err())
	})

	proxy.Deactivate()
	time.Sleep(time.Millisecond * 10)
	if v := err.Load(); v == nil || v != context.Canceled {
		t.Errorf("expect the error context.Canceled, but got '%v'", v)
	}

	err = atomic.Value{}
	proxy.Activate()
	runTask(func(ctx context.Context) {
		<-ctx.Done()
		err.Store(ctx.Err())
	})

	pcancel()
	time.Sleep(time.Millisecond * 10)
	if v := err.Load(); v == nil || v != context.Canceled {
		t.Errorf("expect the error context.Canceled, but got '%s'", v)
	}

	if err := proxy.Context().Err(); err != context.Canceled {
		t.Errorf("expect the error context.Canceled, but got '%s'", err)
	}
}
