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
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type hcEndpoint struct {
	addr    string
	healthy bool
}

func newHCEndpoint(addr string) Endpoint               { return &hcEndpoint{addr: addr} }
func (e *hcEndpoint) Type() string                     { return "healthcheck" }
func (e *hcEndpoint) String() string                   { return e.addr }
func (e *hcEndpoint) UserData() interface{}            { return nil }
func (e *hcEndpoint) MetaData() map[string]interface{} { return nil }
func (e *hcEndpoint) IsHealthy(context.Context) bool {
	e.healthy = !e.healthy
	return e.healthy
}
func (e *hcEndpoint) RoundTrip(context.Context, Request) (Response, error) { return nil, nil }

type hcUpdater struct {
	name string
	sync.RWMutex
	buf *bytes.Buffer
}

func newHcUpdater(name string) *hcUpdater {
	return &hcUpdater{name: name, buf: bytes.NewBufferString("\n")}
}
func (u *hcUpdater) Name() string   { return u.name }
func (u *hcUpdater) String() string { u.Lock(); defer u.Unlock(); return u.buf.String() }
func (u *hcUpdater) AddEndpoint(e Endpoint) {
	u.Lock()
	defer u.Unlock()
	fmt.Fprintf(u.buf, "add endpoint '%s'\n", e.String())
}
func (u *hcUpdater) DelEndpoint(e Endpoint) {
	u.Lock()
	defer u.Unlock()
	fmt.Fprintf(u.buf, "delete endpoint '%s'\n", e.String())
}

func TestHealthChecker(t *testing.T) {
	hc := NewHealthCheck()

	updater := newHcUpdater("all")
	hc.Subscribe("", updater)

	hc.Interval = time.Millisecond * 50
	hc.AddEndpoint(newHCEndpoint("1.1.1.1:80"))
	hc.AddEndpoint(newHCEndpoint("2.2.2.2:80"))

	time.Sleep(time.Second)
	hc.Stop()
	time.Sleep(time.Millisecond)

	var isDelete bool
	lines := strings.Split(strings.TrimSpace(updater.String()), "\n")
	for i := 0; i+1 < len(lines); i += 2 {
		line1 := strings.TrimSpace(lines[i])
		line2 := strings.TrimSpace(lines[i+1])

		if line1 == line2 {
			t.Error(line1)
		} else if isDelete {
			if line1 != "delete endpoint '1.1.1.1:80'" && line1 != "delete endpoint '2.2.2.2:80'" {
				t.Error(line1)
			} else if line2 != "delete endpoint '1.1.1.1:80'" && line2 != "delete endpoint '2.2.2.2:80'" {
				t.Error(line2)
			}
		} else {
			if line1 != "add endpoint '1.1.1.1:80'" && line1 != "add endpoint '2.2.2.2:80'" {
				t.Error(line1)
			} else if line2 != "add endpoint '1.1.1.1:80'" && line2 != "add endpoint '2.2.2.2:80'" {
				t.Error(line2)
			}
		}

		isDelete = !isDelete
	}
}

func TestHealthCheck_Unsubscribe(t *testing.T) {
	hc := NewHealthCheck()

	sub1 := UpdaterFunc("updater1", func(bool, Endpoint) {})
	sub2 := UpdaterFunc("updater2", func(bool, Endpoint) {})
	sub3 := UpdaterFunc("updater3", func(bool, Endpoint) {})

	hc.Subscribe("", sub1)
	hc.Subscribe("", sub2)
	hc.Subscribe("", sub3)
	hc.Subscribe("", sub3)
	hc.Subscribe("endpoint1", sub1)
	hc.Subscribe("endpoint1", sub2)
	hc.Subscribe("endpoint1", sub3)
	hc.Subscribe("endpoint2", sub1)
	hc.Subscribe("endpoint2", sub1)
	hc.Subscribe("endpoint2", sub3)
	hc.Subscribe("endpoint3", sub1)
	hc.Subscribe("endpoint3", sub2)
	hc.Subscribe("endpoint3", sub3)

	if len(hc.updaters) != 3 {
		t.Error(hc.updaters)
	} else if len(hc.subscribers["endpoint1"]) != 3 {
		t.Error(hc.subscribers["endpoint1"])
	} else if len(hc.subscribers["endpoint2"]) != 2 {
		t.Error(hc.subscribers["endpoint2"])
	} else if len(hc.subscribers["endpoint3"]) != 3 {
		t.Error(hc.subscribers["endpoint3"])
	}

	hc.UnsubscribeByUpdater(sub1)
	if len(hc.updaters) != 2 {
		t.Error(hc.updaters)
	} else if len(hc.subscribers["endpoint1"]) != 2 {
		t.Error(hc.subscribers["endpoint1"])
	} else if len(hc.subscribers["endpoint2"]) != 1 {
		t.Error(hc.subscribers["endpoint2"])
	} else if len(hc.subscribers["endpoint3"]) != 2 {
		t.Error(hc.subscribers["endpoint3"])
	}

	hc.UnsubscribeByUpdater(sub3)
	if len(hc.updaters) != 1 {
		t.Error(hc.updaters)
	} else if us := hc.subscribers["endpoint1"]; len(us) != 1 || us["updater2"] == nil {
		t.Error(us)
	} else if hc.subscribers["endpoint2"] != nil {
		t.Error(hc.subscribers["endpoint2"])
	} else if us := hc.subscribers["endpoint3"]; len(us) != 1 || us["updater2"] == nil {
		t.Error(us)
	}

	hc.UnsubscribeByUpdater(sub2)
	if len(hc.updaters) != 0 {
		t.Error(hc.updaters)
	} else if len(hc.subscribers) != 0 {
		t.Error(hc.subscribers)
	}
}

func TestHealthCheck_ReferenceCount(t *testing.T) {
	hc := NewHealthCheck()
	defer hc.Stop()

	hc.AddEndpoint(NewNoopEndpoint("ep1"))
	hc.AddEndpoint(NewNoopEndpoint("ep2"))
	hc.AddEndpoint(NewNoopEndpoint("ep2"))
	hc.AddEndpoint(NewNoopEndpoint("ep3"))
	hc.AddEndpoint(NewNoopEndpoint("ep3"))
	hc.AddEndpoint(NewNoopEndpoint("ep3"))

	time.Sleep(time.Millisecond * 10)
	if rc := hc.ReferenceCount("ep1"); rc != 1 {
		t.Errorf("Endpoint(%s) ReferenceCount: expect '%d', but got '%d'", "ep1", 1, rc)
	}
	if rc := hc.ReferenceCount("ep2"); rc != 2 {
		t.Errorf("Endpoint(%s) ReferenceCount: expect '%d', but got '%d'", "ep2", 2, rc)
	}
	if rc := hc.ReferenceCount("ep3"); rc != 3 {
		t.Errorf("Endpoint(%s) ReferenceCount: expect '%d', but got '%d'", "ep3", 3, rc)
	}

	time.Sleep(time.Millisecond * 10)
	hc.DelEndpointByString("ep1")
	hc.DelEndpointByString("ep2")
	hc.DelEndpointByString("ep3")
	if rc := hc.ReferenceCount("ep1"); rc != 0 {
		t.Errorf("Endpoint(%s) ReferenceCount: expect '%d', but got '%d'", "ep1", 0, rc)
	}
	if rc := hc.ReferenceCount("ep2"); rc != 1 {
		t.Errorf("Endpoint(%s) ReferenceCount: expect '%d', but got '%d'", "ep2", 1, rc)
	}
	if rc := hc.ReferenceCount("ep3"); rc != 2 {
		t.Errorf("Endpoint(%s) ReferenceCount: expect '%d', but got '%d'", "ep3", 2, rc)
	}

	time.Sleep(time.Millisecond * 10)
	hc.DelEndpointByForce("ep3")
	if rc := hc.ReferenceCount("ep3"); rc != 0 {
		t.Errorf("Endpoint(%s) ReferenceCount: expect '%d', but got '%d'", "ep3", 0, rc)
	}
}
