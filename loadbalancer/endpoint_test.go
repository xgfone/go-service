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
)

var (
	err1 = errors.New("error1")
	err2 = errors.New("error2")
)

type noopRequest string

func newNoopRequest(addr string) Request       { return noopRequest(addr) }
func (r noopRequest) RemoteAddrString() string { return string(r) }
func (r noopRequest) SessionID() string        { return string(r) }

type noopEndpoint struct{ name string }

func newNoopEndpoint(name string) Endpoint               { return &noopEndpoint{name: name} }
func (e *noopEndpoint) ID() string                       { return e.name }
func (e *noopEndpoint) Type() string                     { return "noop" }
func (e *noopEndpoint) State() (s EndpointState)         { return }
func (e *noopEndpoint) MetaData() map[string]interface{} { return nil }
func (e *noopEndpoint) RoundTrip(context.Context, Request) (interface{}, error) {
	return nil, nil
}

type errEndpoint struct {
	name string
	err  error
}

func newErrorEndpoint(name string, err error) Endpoint {
	return &errEndpoint{name: name, err: err}
}
func (e *errEndpoint) ID() string                       { return e.name }
func (e *errEndpoint) Type() string                     { return "error" }
func (e *errEndpoint) State() (s EndpointState)         { return }
func (e *errEndpoint) MetaData() map[string]interface{} { return nil }
func (e *errEndpoint) RoundTrip(context.Context, Request) (interface{}, error) {
	return nil, e.err
}
