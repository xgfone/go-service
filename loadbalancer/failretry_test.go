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
	"testing"
	"time"

	"github.com/xgfone/go-service/retry"
)

func TestFailRetry(t *testing.T) {
	p := NewGeneralProvider(roundRobinSelector(0))
	r := newNoopRequest("127.0.0.1:12345")
	ep1 := newSleepEndpoint("127.0.0.1:11111", err1)
	ep2 := newSleepEndpoint("127.0.0.1:22222", err1)
	ep3 := newSleepEndpoint("127.0.0.1:33333", err1)
	p.(EndpointManager).AddEndpoint(ep1)
	p.(EndpointManager).AddEndpoint(ep2)
	p.(EndpointManager).AddEndpoint(ep3)

	ff := FailFast()
	if _, err := ff.Retry(context.TODO(), p, r, ep1, err2); err != err2 {
		t.Errorf("expect the error '%v', but got '%v'", err2, err)
	} else if total := ep1.State().TotalConnections; total != 0 {
		t.Errorf("%s: expect the total connections '%d', but got '%d'", ep1, 0, total)
	} else if total := ep2.State().TotalConnections; total != 0 {
		t.Errorf("%s: expect the total connections '%d', but got '%d'", ep2, 0, total)
	} else if total := ep3.State().TotalConnections; total != 0 {
		t.Errorf("%s: expect the total connections '%d', but got '%d'", ep3, 0, total)
	}
}

func TestFailTry(t *testing.T) {
	p := NewGeneralProvider(roundRobinSelector(0))
	r := newNoopRequest("127.0.0.1:12345")
	ep1 := newSleepEndpoint("127.0.0.1:11111", err1)
	ep2 := newSleepEndpoint("127.0.0.1:22222", err1)
	ep3 := newSleepEndpoint("127.0.0.1:33333", err1)
	p.(EndpointManager).AddEndpoint(ep1)
	p.(EndpointManager).AddEndpoint(ep2)
	p.(EndpointManager).AddEndpoint(ep3)

	ff := FailTry(4, retry.DefaultRetryNewer(time.Millisecond*10))
	if _, err := ff.Retry(context.TODO(), p, r, ep1, err2); err != err1 {
		t.Errorf("expect the error '%v', but got '%v'", err1, err)
	} else if total := ep1.State().TotalConnections; total != 4 {
		t.Errorf("%s: expect the total connections '%d', but got '%d'", ep1, 4, total)
	} else if total := ep2.State().TotalConnections; total != 0 {
		t.Errorf("%s: expect the total connections '%d', but got '%d'", ep2, 0, total)
	} else if total := ep3.State().TotalConnections; total != 0 {
		t.Errorf("%s: expect the total connections '%d', but got '%d'", ep3, 0, total)
	}
}

func TestFailOver(t *testing.T) {
	p := NewGeneralProvider(roundRobinSelector(0))
	r := newNoopRequest("127.0.0.1:12345")
	ep1 := newSleepEndpoint("127.0.0.1:11111", err1)
	ep2 := newSleepEndpoint("127.0.0.1:22222", err1)
	ep3 := newSleepEndpoint("127.0.0.1:33333", err1)
	p.(EndpointManager).AddEndpoint(ep1)
	p.(EndpointManager).AddEndpoint(ep2)
	p.(EndpointManager).AddEndpoint(ep3)

	ff := FailOver(0, retry.DefaultRetryNewer(time.Millisecond*10))
	if _, err := ff.Retry(context.TODO(), p, r, ep1, err2); err != err1 {
		t.Errorf("expect the error '%v', but got '%v'", err1, err)
	} else if total := ep1.State().TotalConnections; total != 1 {
		t.Errorf("%s: expect the total connections '%d', but got '%d'", ep1, 1, total)
	} else if total := ep2.State().TotalConnections; total != 1 {
		t.Errorf("%s: expect the total connections '%d', but got '%d'", ep2, 1, total)
	} else if total := ep3.State().TotalConnections; total != 1 {
		t.Errorf("%s: expect the total connections '%d', but got '%d'", ep3, 1, total)
	}
}
