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
	"errors"
	"testing"
	"time"
)

func TestSessionProvider(t *testing.T) {
	p := NewSessionProvider(roundRobinSelector(0), nil, 0)
	p.SetSession(NewMemorySession(time.Second * 10))
	p.SetSessionTimeout(time.Second * 30)
	defer p.Close()

	desc := "SessionProvider(session=memory, strategy=round_robin)"
	if s := p.String(); s != desc {
		t.Errorf("expect '%s', but got '%s'", desc, s)
	}

	ep11 := newSleepEndpoint("127.0.0.1:11111", nil)
	ep22 := newSleepEndpoint("127.0.0.1:22222", nil)
	ep33 := newSleepEndpoint("127.0.0.1:33333", nil)
	p.AddEndpoint(ep11)
	p.AddEndpoint(ep22)
	p.AddEndpoint(ep33)

	req := newNoopRequest("127.0.0.1:12345")
	ep1 := p.Select(req, true) // First
	if ep1 == nil || ep1.String() != "127.0.0.1:22222" {
		t.Errorf("expect the endpoint '%s', but got '%v'", "127.0.0.1:22222", ep1)
	}
	ep2 := p.GetSession().GetEndpoint(req.SessionID())
	if ep2 == nil || ep2.String() != "127.0.0.1:22222" {
		t.Errorf("expect the endpoint '%s', but got '%v'", "127.0.0.1:22222", ep2)
	}

	ep1 = p.Select(req, false) // Retry
	if ep1 == nil || ep1.String() != "127.0.0.1:33333" {
		t.Errorf("expect the endpoint '%s', but got '%v'", "127.0.0.1:33333", ep1)
	}
	ep2 = p.GetSession().GetEndpoint(req.SessionID())
	if ep2 == nil || ep2.String() != "127.0.0.1:33333" {
		t.Errorf("expect the endpoint '%s', but got '%v'", "127.0.0.1:33333", ep2)
	}

	p.Finish(req, nil)
	ep2 = p.GetSession().GetEndpoint(req.SessionID())
	if ep2 == nil || ep2.String() != "127.0.0.1:33333" {
		t.Errorf("expect the endpoint '%s', but got '%v'", "127.0.0.1:33333", ep2)
	}

	p.Finish(req, errors.New("error"))
	ep2 = p.GetSession().GetEndpoint(req.SessionID())
	if ep2 != nil {
		t.Errorf("expect the endpoint '%v', but got '%v'", nil, ep2)
	}
}
