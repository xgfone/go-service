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
	"testing"
)

func TestGeneralProvider_ProviderEndpointManager(t *testing.T) {
	p := NewGeneralProvider(RandomSelector())

	p.AddEndpoint(NewHTTPEndpoint("127.0.0.1:8001", nil))
	p.AddEndpoint(NewHTTPEndpoint("127.0.0.1:8002", nil))
	p.AddEndpoint(NewHTTPEndpoint("127.0.0.1:8003", nil))
	for i, ep := range p.Endpoints() {
		switch i {
		case 0:
			if ep.String() != "127.0.0.1:8001" {
				t.Errorf("%d: %s", i, ep.String())
			}
		case 1:
			if ep.String() != "127.0.0.1:8002" {
				t.Errorf("%d: %s", i, ep.String())
			}
		case 2:
			if ep.String() != "127.0.0.1:8003" {
				t.Errorf("%d: %s", i, ep.String())
			}
		}
	}

	p.DelEndpoint(NewNoopEndpoint("127.0.0.1:8002"))
	for i, ep := range p.Endpoints() {
		switch i {
		case 0:
			if ep.String() != "127.0.0.1:8001" {
				t.Errorf("%d: %s", i, ep.String())
			}
		case 1:
			if ep.String() != "127.0.0.1:8003" {
				t.Errorf("%d: %s", i, ep.String())
			}
		}
	}

	p.AddEndpoint(NewHTTPEndpoint("127.0.0.1:8004", nil))
	for i, ep := range p.Endpoints() {
		switch i {
		case 0:
			if ep.String() != "127.0.0.1:8001" {
				t.Errorf("%d: %s", i, ep.String())
			}
		case 1:
			if ep.String() != "127.0.0.1:8003" {
				t.Errorf("%d: %s", i, ep.String())
			}
		case 2:
			if ep.String() != "127.0.0.1:8004" {
				t.Errorf("%d: %s", i, ep.String())
			}
		}
	}
}
