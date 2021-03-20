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
	"errors"
	"time"
)

var (
	err1 = errors.New("error1")
	err2 = errors.New("error2")
)

var sleep = time.Millisecond * 10

type noopRequest string

func newNoopRequest(addr string) Request       { return noopRequest(addr) }
func (r noopRequest) RemoteAddrString() string { return string(r) }
func (r noopRequest) SessionID() string        { return string(r) }

type sleepEndpoint struct {
	err   error
	addr  string
	state ConnectionState
}

func newSleepEndpoint(addr string, e error) Endpoint      { return &sleepEndpoint{err: e, addr: addr} }
func (e *sleepEndpoint) Type() string                     { return "noop" }
func (e *sleepEndpoint) String() string                   { return e.addr }
func (e *sleepEndpoint) State() EndpointState             { return e.state.ToEndpointState() }
func (e *sleepEndpoint) MetaData() map[string]interface{} { return nil }
func (e *sleepEndpoint) IsHealthy(context.Context) bool   { return true }
func (e *sleepEndpoint) RoundTrip(c context.Context, r Request) (interface{}, error) {
	e.state.Inc()
	defer e.state.Dec()
	time.Sleep(sleep)
	return nil, e.err
}
