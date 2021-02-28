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
	"context"
	"testing"

	"github.com/xgfone/go-service/retry"
)

func BenchmarkLoadBalancerWithoutSession(b *testing.B) {
	lb := NewLoadBalancer(nil)
	lb.Session = nil
	lb.EndpointManager().AddEndpoint(NewNoopEndpoint("127.0.0.1:8001"))
	lb.EndpointManager().AddEndpoint(NewNoopEndpoint("127.0.0.1:8002"))
	lb.EndpointManager().AddEndpoint(NewNoopEndpoint("127.0.0.1:8003"))

	ctx := context.Background()
	req := NewNoopRequest("1.2.3.4:12345")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		lb.RoundTrip(ctx, req)
	}
}

func BenchmarkLoadBalancerWithSession(b *testing.B) {
	lb := NewLoadBalancer(nil)
	lb.EndpointManager().AddEndpoint(NewNoopEndpoint("127.0.0.1:8001"))
	lb.EndpointManager().AddEndpoint(NewNoopEndpoint("127.0.0.1:8002"))
	lb.EndpointManager().AddEndpoint(NewNoopEndpoint("127.0.0.1:8003"))

	ctx := context.Background()
	req := NewNoopRequest("1.2.3.4:12345")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		lb.RoundTrip(ctx, req)
	}
}

func BenchmarkLoadBalancerWithFailRetry(b *testing.B) {
	lb := NewLoadBalancer(nil)
	lb.FailRetry = FailOver(0, retry.DefaultRetryNewer(0))
	lb.EndpointManager().AddEndpoint(newFailOnceEndpoint("127.0.0.1:8001"))
	lb.EndpointManager().AddEndpoint(newFailOnceEndpoint("127.0.0.1:8002"))
	lb.EndpointManager().AddEndpoint(newFailOnceEndpoint("127.0.0.1:8003"))

	ctx := context.Background()
	req := NewNoopRequest("1.2.3.4:12345")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		lb.RoundTrip(ctx, req)
	}
}

type failOnceEndpoint struct {
	Addr string
}

func newFailOnceEndpoint(addr string) Endpoint {
	return &failOnceEndpoint{Addr: addr}
}

func (e *failOnceEndpoint) String() string                   { return e.Addr }
func (e *failOnceEndpoint) UserData() interface{}            { return nil }
func (e *failOnceEndpoint) MetaData() map[string]interface{} { return nil }
func (e *failOnceEndpoint) IsHealthy(context.Context) bool   { return true }
func (e *failOnceEndpoint) RoundTrip(context.Context, Request) (Response, error) {
	return nil, errFailed
}
