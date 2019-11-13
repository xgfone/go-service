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
	"testing"
	"time"
)

type noopEndpoint string

func newNoopEndpoint(addr string) Endpoint                                  { return noopEndpoint(addr) }
func (e noopEndpoint) String() string                                       { return string(e) }
func (e noopEndpoint) IsHealthy(context.Context) bool                       { return true }
func (e noopEndpoint) RoundTrip(context.Context, Request) (Response, error) { return nil, nil }

func TestLoadBalancer_AddEndpoints(t *testing.T) {
	lb := NewLoadBalancer(nil)

	lb.EndpointManager().AddEndpoint(newNoopEndpoint("1.1.1.1:80"))
	lb.EndpointManager().AddEndpoint(newNoopEndpoint("2.2.2.2:80"))
	if eps := lb.EndpointManager().Endpoints(); len(eps) != 2 {
		t.Error(eps)
	} else if eps[0].String() != "1.1.1.1:80" {
		t.Error(eps[0].String())
	} else if eps[1].String() != "2.2.2.2:80" {
		t.Error(eps[1].String())
	}

	lb.EndpointManager().AddEndpoint(newNoopEndpoint("4.4.4.4:80"))
	lb.EndpointManager().AddEndpoint(newNoopEndpoint("3.3.3.3:80"))
	lb.EndpointManager().AddEndpoint(newNoopEndpoint("2.2.2.2:80"))
	if eps := lb.EndpointManager().Endpoints(); len(eps) != 4 {
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

type failEndpoint struct {
	Addr string
	Buf  *bytes.Buffer
}

var errFailed = fmt.Errorf("error")

func newFailEndpoint(addr string, buf *bytes.Buffer) Endpoint { return failEndpoint{addr, buf} }
func (e failEndpoint) String() string                         { return e.Addr }
func (e failEndpoint) IsHealthy(context.Context) bool         { return true }
func (e failEndpoint) RoundTrip(context.Context, Request) (Response, error) {
	fmt.Fprintln(e.Buf, e.Addr)
	return nil, errFailed
}

type failRequest string

func (r failRequest) RemoteAddrString() string { return string(r) }

type logRetryDelay struct {
	buf   *bytes.Buffer
	delay time.Duration
}

func (d logRetryDelay) NextDelay(num int, last time.Duration) (next time.Duration) {
	fmt.Fprintf(d.buf, "delay %d\n", num)
	return d.delay
}

func TestLoadBalancer(t *testing.T) {
	buf := bytes.NewBufferString("\n")
	lb := NewLoadBalancer(nil)
	lb.RetryDelay = logRetryDelay{buf, time.Millisecond * 10}.NextDelay
	lb.EndpointManager().AddEndpoint(newFailEndpoint("1.1.1.1:80", buf))
	lb.EndpointManager().AddEndpoint(newFailEndpoint("2.2.2.2:80", buf))
	lb.EndpointManager().AddEndpoint(newFailEndpoint("3.3.3.3:80", buf))
	_, err := lb.RoundTrip(context.Background(), failRequest("1"))
	buf.WriteString(err.Error())

	var lines []string
	_lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	for _, line := range _lines {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}

	expectLines := []string{
		"2.2.2.2:80",
		"delay 1",
		"3.3.3.3:80",
		"delay 2",
		"1.1.1.1:80",
		"delay 3",
		"2.2.2.2:80",
		"error",
	}

	if len(lines) != len(expectLines) {
		t.Errorf("line: expect '%d', got '%d'", len(expectLines), len(lines))
	}
	for i := 0; i < len(lines); i++ {
		if lines[i] != expectLines[i] {
			t.Errorf("line '%d': expect '%s', got '%s'", i, expectLines[i], lines[i])
		}
	}
}
