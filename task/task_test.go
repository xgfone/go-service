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

package task

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestTask(t *testing.T) {
	task1 := new(int32)
	task2 := new(int32)

	SetCheckInterval(time.Second)
	SetCheckCond(func(ctx context.Context) bool { return false })
	StartChecker()
	go RunAlways(time.Millisecond*100, time.Second, func(ctx context.Context) { atomic.StoreInt32(task1, 1) })
	go Run(time.Millisecond*100, time.Second, func(ctx context.Context) { atomic.StoreInt32(task2, 1); panic("test") })

	time.Sleep(time.Second + time.Millisecond*100)
	if atomic.LoadInt32(task1) != 1 {
		t.Errorf("expect true, but got false")
	}
	if atomic.LoadInt32(task2) != 0 {
		t.Errorf("expect false, but got true")
	}

	SetVipCheckCond("127.0.0.1")
	time.Sleep(time.Second * 2)
	if atomic.LoadInt32(task1) != 1 {
		t.Errorf("expect true, but got false")
	}
	if atomic.LoadInt32(task2) != 1 {
		t.Errorf("expect true, but got false")
	}

	StopChecker()
}
