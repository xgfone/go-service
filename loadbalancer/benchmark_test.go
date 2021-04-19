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
)

func BenchmarkLoadBalancer(b *testing.B) {
	r := newNoopRequest("127.0.0.1:12345")
	lb := NewLoadBalancer("benchmark", nil)
	lb.AddEndpoint(newNoopEndpoint("127.0.0.1:11111"))
	lb.AddEndpoint(newNoopEndpoint("127.0.0.1:22222"))
	lb.AddEndpoint(newNoopEndpoint("127.0.0.1:33333"))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lb.RoundTrip(context.Background(), r)
	}
}

func BenchmarkLoadBalancerWithFailTry(b *testing.B) {
	r := newNoopRequest("127.0.0.1:12345")
	lb := NewLoadBalancer("benchmark", nil)
	lb.FailRetry = FailTry(3, 0)
	lb.AddEndpoint(newErrorEndpoint("127.0.0.1:11111", err1))
	lb.AddEndpoint(newErrorEndpoint("127.0.0.1:22222", err1))
	lb.AddEndpoint(newErrorEndpoint("127.0.0.1:33333", err1))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lb.RoundTrip(context.Background(), r)
	}
}

func BenchmarkLoadBalancerWithFailOver1(b *testing.B) {
	r := newNoopRequest("127.0.0.1:12345")
	lb := NewLoadBalancer("benchmark", nil)
	lb.FailRetry = FailOver(1, 0)
	lb.AddEndpoint(newErrorEndpoint("127.0.0.1:11111", err1))
	lb.AddEndpoint(newErrorEndpoint("127.0.0.1:22222", err1))
	lb.AddEndpoint(newErrorEndpoint("127.0.0.1:33333", err1))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lb.RoundTrip(context.Background(), r)
	}
}

func BenchmarkLoadBalancerWithFailOverAll(b *testing.B) {
	r := newNoopRequest("127.0.0.1:12345")
	lb := NewLoadBalancer("benchmark", nil)
	lb.FailRetry = FailOver(0, 0)
	lb.AddEndpoint(newErrorEndpoint("127.0.0.1:11111", err1))
	lb.AddEndpoint(newErrorEndpoint("127.0.0.1:22222", err1))
	lb.AddEndpoint(newErrorEndpoint("127.0.0.1:33333", err1))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lb.RoundTrip(context.Background(), r)
	}
}
