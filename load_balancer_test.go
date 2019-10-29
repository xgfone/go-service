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
	"context"
	"testing"
)

type noopEndpoint string

func newNoopEndpoint(addr string) Endpoint                                  { return noopEndpoint(addr) }
func (e noopEndpoint) String() string                                       { return string(e) }
func (e noopEndpoint) IsHealthy(context.Context) bool                       { return true }
func (e noopEndpoint) RoundTrip(context.Context, Request) (Response, error) { return nil, nil }

func TestLoadBalancer_AddEndpoints(t *testing.T) {
	lb := NewLoadBalancer()

	lb.AddEndpoints(newNoopEndpoint("1.1.1.1:80"), newNoopEndpoint("2.2.2.2:80"))
	if eps := lb.Endpoints(); len(eps) != 2 {
		t.Error(eps)
	} else if eps[0].String() != "1.1.1.1:80" {
		t.Error(eps[0].String())
	} else if eps[1].String() != "2.2.2.2:80" {
		t.Error(eps[1].String())
	}

	lb.AddEndpoints(newNoopEndpoint("4.4.4.4:80"), newNoopEndpoint("3.3.3.3:80"),
		newNoopEndpoint("2.2.2.2:80"))
	if eps := lb.Endpoints(); len(eps) != 4 {
		t.Error(eps)
	} else if eps[0].String() != "1.1.1.1:80" {
		t.Error(eps[0].String())
	} else if eps[1].String() != "2.2.2.2:80" {
		t.Error(eps[1].String())
	} else if eps[2].String() != "3.3.3.3:80" {
		t.Error(eps[2].String())
	} else if eps[3].String() != "4.4.4.4:80" {
		t.Error(eps[3].String())
	}
}
