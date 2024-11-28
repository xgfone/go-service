// Copyright 2024 xgfone
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
	"testing"
	"time"
)

func TestContextFunc(t *testing.T) {
	var end time.Time
	start := time.Now()
	svc := ContextFunc(func(ctx context.Context) {
		<-ctx.Done()
		end = time.Now()
	})

	svc.Activate()
	time.Sleep(time.Second)
	svc.Deactivate()
	time.Sleep(time.Millisecond * 100)

	if cost := end.Sub(start); cost < time.Second || cost > time.Second+time.Millisecond*100 {
		t.Error(cost.String())
	}
}