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
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
)

type frEndpoint struct {
	state ConnectionState
	name  string
	buf   *bytes.Buffer
}

func newFailRetryEndpoint(name string, buf *bytes.Buffer) Endpoint {
	return &frEndpoint{name: name, buf: buf}
}
func (e *frEndpoint) ID() string                       { return e.name }
func (e *frEndpoint) Type() string                     { return "sleep" }
func (e *frEndpoint) State() EndpointState             { return e.state.ToEndpointState() }
func (e *frEndpoint) MetaData() map[string]interface{} { return nil }
func (e *frEndpoint) RoundTrip(context.Context, Request) (interface{}, error) {
	e.state.Inc()
	defer e.state.Dec()
	fmt.Fprintln(e.buf, e.name)
	return nil, err1
}

func TestFailTry(t *testing.T) {
	p := NewGeneralProvider(nil)
	r := newNoopRequest("127.0.0.1:12345")
	buf := bytes.NewBuffer(nil)
	ep1 := newFailRetryEndpoint("127.0.0.1:11111", buf)
	ep2 := newFailRetryEndpoint("127.0.0.1:22222", buf)
	ep3 := newFailRetryEndpoint("127.0.0.1:33333", buf)
	p.(EndpointUpdater).AddEndpoint(ep1)
	p.(EndpointUpdater).AddEndpoint(ep2)
	p.(EndpointUpdater).AddEndpoint(ep3)

	ff := FailTry(0, 0)
	if _, _, err := ff.Retry(context.TODO(), p, r, ep1, err2); err != err1 {
		t.Errorf("expect the error '%v', but got '%v'", err1, err)
	} else if total := ep1.State().TotalConnections; total != 1 {
		t.Errorf("%s: expect the total connections '%d', but got '%d'", ep1.ID(), 1, total)
	} else if total := ep2.State().TotalConnections; total != 0 {
		t.Errorf("%s: expect the total connections '%d', but got '%d'", ep2.ID(), 0, total)
	} else if total := ep3.State().TotalConnections; total != 0 {
		t.Errorf("%s: expect the total connections '%d', but got '%d'", ep3.ID(), 0, total)
	} else if ss := strings.Split(strings.TrimSpace(buf.String()), "\n"); len(ss) != 1 {
		t.Errorf("expect '%d' endpoints, but got '%d'", 1, len(ss))
	} else if ss[0] != ep1.ID() {
		t.Errorf("expect the endpoint '%s', but got '%s'", ep1.ID(), ss[0])
	}
}

func TestFailOver(t *testing.T) {
	p := NewGeneralProvider(nil)
	r := newNoopRequest("127.0.0.1:12345")
	buf := bytes.NewBuffer(nil)
	ep1 := newFailRetryEndpoint("127.0.0.1:11111", buf)
	ep2 := newFailRetryEndpoint("127.0.0.1:22222", buf)
	ep3 := newFailRetryEndpoint("127.0.0.1:33333", buf)
	p.(EndpointUpdater).AddEndpoint(ep1)
	p.(EndpointUpdater).AddEndpoint(ep2)
	p.(EndpointUpdater).AddEndpoint(ep3)

	ff := FailOver(0, 0)
	if _, _, err := ff.Retry(context.TODO(), p, r, ep1, err2); err != err1 {
		t.Errorf("expect the error '%v', but got '%v'", err1, err)
	} else if total := ep1.State().TotalConnections; total != 0 {
		t.Errorf("%s: expect the total connections '%d', but got '%d'", ep1.ID(), 0, total)
	} else if total := ep2.State().TotalConnections; total != 1 {
		t.Errorf("%s: expect the total connections '%d', but got '%d'", ep2.ID(), 1, total)
	} else if total := ep3.State().TotalConnections; total != 1 {
		t.Errorf("%s: expect the total connections '%d', but got '%d'", ep3.ID(), 1, total)
	} else if ss := strings.Split(strings.TrimSpace(buf.String()), "\n"); len(ss) != 2 {
		t.Errorf("expect '%d' endpoints, but got '%d'", 2, len(ss))
	} else if sort.Strings(ss); ss[0] != ep2.ID() {
		t.Errorf("expect the endpoint '%s', but got '%s'", ep2.ID(), ss[0])
	} else if sort.Strings(ss); ss[1] != ep3.ID() {
		t.Errorf("expect the endpoint '%s', but got '%s'", ep3.ID(), ss[1])
	}
}
