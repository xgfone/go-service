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

func BenchmarkGeneralProviderWithError(b *testing.B) {
	e := errors.New("error")
	r := newNoopRequest("127.0.0.1:12345")
	p := NewGeneralProvider(nil)
	p.(EndpointManager).AddEndpoint(newSleepEndpoint("127.0.0.1:11111", nil))
	p.(EndpointManager).AddEndpoint(newSleepEndpoint("127.0.0.1:22222", nil))
	p.(EndpointManager).AddEndpoint(newSleepEndpoint("127.0.0.1:33333", nil))
	defer p.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Select(r, true)
		p.Finish(r, e)
	}
}

func BenchmarkGeneralProviderWithoutError(b *testing.B) {
	r := newNoopRequest("127.0.0.1:12345")
	p := NewGeneralProvider(nil)
	p.(EndpointManager).AddEndpoint(newSleepEndpoint("127.0.0.1:11111", nil))
	p.(EndpointManager).AddEndpoint(newSleepEndpoint("127.0.0.1:22222", nil))
	p.(EndpointManager).AddEndpoint(newSleepEndpoint("127.0.0.1:33333", nil))
	defer p.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Select(r, true)
		p.Finish(r, nil)
	}
}

func BenchmarkNoopSessionProviderWithError(b *testing.B) {
	r := newNoopRequest("127.0.0.1:12345")
	p := NewSessionProvider(nil, nil, 0)
	p.AddEndpoint(newSleepEndpoint("127.0.0.1:11111", nil))
	p.AddEndpoint(newSleepEndpoint("127.0.0.1:22222", nil))
	p.AddEndpoint(newSleepEndpoint("127.0.0.1:33333", nil))
	defer p.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Select(r, true)
		p.Finish(r, nil)
	}
}

func BenchmarkNoopSessionProviderWithoutError(b *testing.B) {
	e := errors.New("error")
	r := newNoopRequest("127.0.0.1:12345")
	p := NewSessionProvider(nil, nil, 0)
	p.AddEndpoint(newSleepEndpoint("127.0.0.1:11111", nil))
	p.AddEndpoint(newSleepEndpoint("127.0.0.1:22222", nil))
	p.AddEndpoint(newSleepEndpoint("127.0.0.1:33333", nil))
	defer p.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Select(r, true)
		p.Finish(r, e)
	}
}

func BenchmarkMemorySessionProviderWithError(b *testing.B) {
	e := errors.New("error")
	r := newNoopRequest("127.0.0.1:12345")
	p := NewSessionProvider(nil, NewMemorySession(time.Hour), 0)
	p.AddEndpoint(newSleepEndpoint("127.0.0.1:11111", nil))
	p.AddEndpoint(newSleepEndpoint("127.0.0.1:22222", nil))
	p.AddEndpoint(newSleepEndpoint("127.0.0.1:33333", nil))
	defer p.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Select(r, true)
		p.Finish(r, e)
	}
}

func BenchmarkMemorySessionProviderWithoutError(b *testing.B) {
	r := newNoopRequest("127.0.0.1:12345")
	p := NewSessionProvider(nil, NewMemorySession(time.Hour), 0)
	p.AddEndpoint(newSleepEndpoint("127.0.0.1:11111", nil))
	p.AddEndpoint(newSleepEndpoint("127.0.0.1:22222", nil))
	p.AddEndpoint(newSleepEndpoint("127.0.0.1:33333", nil))
	defer p.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Select(r, true)
		p.Finish(r, nil)
	}
}
