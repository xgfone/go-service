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
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/xgfone/go-service/retry"
)

var errFailed = fmt.Errorf("error")

type failEndpoint struct {
	Addr string
	Buf  *bytes.Buffer
}

func newFailEndpoint(addr string, buf *bytes.Buffer) Endpoint {
	return failEndpoint{addr, buf}
}
func (e failEndpoint) String() string                 { return e.Addr }
func (e failEndpoint) IsHealthy(context.Context) bool { return true }
func (e failEndpoint) RoundTrip(context.Context, Request) (Response, error) {
	if e.Buf != nil {
		fmt.Fprintln(e.Buf, e.Addr)
	}
	return nil, errFailed
}

type failRequest string

func (r failRequest) RemoteAddrString() string { return string(r) }

type stringRequest string

func (r stringRequest) RemoteAddrString() string { return string(r) }

func TestFailFast(t *testing.T) {
	p := NewGeneralProvider(roundRobinSelector(0))
	ep := newFailEndpoint("1.2.3.4", nil)
	_, err := FailFast().Retry(context.TODO(), NewNoopRequest("5.6.7.8"), ep, p)
	if err != errFailed {
		t.Error(err)
	}
}

func TestFailTry(t *testing.T) {
	buf := bytes.NewBufferString("\n")
	p := NewGeneralProvider(roundRobinSelector(0))
	p.AddEndpoint(newFailEndpoint("1.1.1.1:80", buf))
	p.AddEndpoint(newFailEndpoint("2.2.2.2:80", buf))
	p.AddEndpoint(newFailEndpoint("3.3.3.3:80", buf))

	req := stringRequest("0.0.0.0")
	retry := FailTry(0, retry.DefaultRetryNewer(time.Millisecond*10))
	_, err := retry.Retry(context.TODO(), req, p.Select(req), p)
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
		"2.2.2.2:80",
		"2.2.2.2:80",
		"error",
	}

	if len(lines) != len(expectLines) {
		t.Errorf("line: expect '%d', got '%d'", len(expectLines), len(lines))
	}
	for i := 0; i < len(expectLines); i++ {
		if lines[i] != expectLines[i] {
			t.Errorf("line '%d': expect '%s', got '%s'", i, expectLines[i], lines[i])
		}
	}
}

func TestFailOver(t *testing.T) {
	buf := bytes.NewBufferString("\n")
	p := NewGeneralProvider(roundRobinSelector(0))
	p.AddEndpoint(newFailEndpoint("1.1.1.1:80", buf))
	p.AddEndpoint(newFailEndpoint("2.2.2.2:80", buf))
	p.AddEndpoint(newFailEndpoint("3.3.3.3:80", buf))

	req := stringRequest("0.0.0.0")
	retry := FailOver(0, retry.DefaultRetryNewer(time.Millisecond*10))
	_, err := retry.Retry(context.TODO(), req, p.Select(req), p)
	buf.WriteString(err.Error())

	var lines []string
	_lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	for _, line := range _lines {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}

	expectLines := []string{
		"3.3.3.3:80",
		"1.1.1.1:80",
		"2.2.2.2:80",
		"error",
	}

	if len(lines) != len(expectLines) {
		t.Errorf("line: expect '%d', got '%d'", len(expectLines), len(lines))
	}
	for i := 0; i < len(expectLines); i++ {
		if lines[i] != expectLines[i] {
			t.Errorf("line '%d': expect '%s', got '%s'", i, expectLines[i], lines[i])
		}
	}
}
