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

func newHCEndpoint(addr string) Endpoint { return &hcEndpoint{addr: addr} }
func (e *hcEndpoint) String() string     { return e.addr }
func (e *hcEndpoint) IsHealthy(context.Context) bool {
	e.healthy = !e.healthy
	return e.healthy
}
func (e *hcEndpoint) RoundTrip(context.Context, Request) (Response, error) { return nil, nil }

type hcUpdater struct {
	sync.RWMutex
	buf *bytes.Buffer
}

func (u *hcUpdater) String() string {
	u.Lock()
	defer u.Unlock()
	return u.buf.String()
}
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

	updater := &hcUpdater{buf: bytes.NewBufferString("\n")}
	hc.AddUpdater(updater)

	interval := time.Millisecond * 50
	hc.AddEndpoint(newHCEndpoint("1.1.1.1:80"), interval, 0)
	hc.AddEndpoint(newHCEndpoint("2.2.2.2:80"), interval, 0)

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

type testUpdater struct{ name string }

func (u testUpdater) AddEndpoint(ep Endpoint) {}
func (u testUpdater) DelEndpoint(ep Endpoint) {}

func TestHealthCheck_Unsubscribe(t *testing.T) {
	hc := NewHealthCheck()

	sub1 := UpdaterFunc(func(bool, Endpoint) {})
	sub2 := UpdaterFunc(func(bool, Endpoint) {})
	sub3 := UpdaterFunc(func(bool, Endpoint) {})

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

	if len(hc.updaters) != 4 {
		t.Error(hc.updaters)
	} else if len(hc.updaters[""]) != 3 {
		t.Error(hc.updaters[""])
	} else if len(hc.updaters["endpoint1"]) != 3 {
		t.Error(hc.updaters["endpoint1"])
	} else if len(hc.updaters["endpoint2"]) != 2 {
		t.Error(hc.updaters["endpoint2"])
	} else if len(hc.updaters["endpoint3"]) != 3 {
		t.Error(hc.updaters["endpoint3"])
	}

	hc.UnsubscribeByUpdater(sub1)
	if len(hc.updaters) != 4 {
		t.Error(hc.updaters)
	} else if len(hc.updaters[""]) != 2 {
		t.Error(hc.updaters[""])
	} else if len(hc.updaters["endpoint1"]) != 2 {
		t.Error(hc.updaters["endpoint1"])
	} else if len(hc.updaters["endpoint2"]) != 1 {
		t.Error(hc.updaters["endpoint2"])
	} else if len(hc.updaters["endpoint3"]) != 2 {
		t.Error(hc.updaters["endpoint3"])
	}

	hc.UnsubscribeByUpdater(sub3)
	if len(hc.updaters) != 3 {
		t.Error(hc.updaters)
	} else if len(hc.updaters[""]) != 1 {
		t.Error(hc.updaters[""])
	} else if us := hc.updaters["endpoint1"]; len(us) != 1 || us[0] != sub2 {
		t.Error(us)
	} else if hc.updaters["endpoint2"] != nil {
		t.Error(hc.updaters["endpoint2"])
	} else if us := hc.updaters["endpoint3"]; len(us) != 1 || us[0] != sub2 {
		t.Error(us)
	}

	hc.UnsubscribeByUpdater(sub2)
	if len(hc.updaters) != 0 {
		t.Error(hc.updaters)
	}
}
