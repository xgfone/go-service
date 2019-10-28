package service

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

type hcEndpoint struct {
	addr    string
	healthy bool
}

func newHCEndpoint(addr string) Endpoint { return &hcEndpoint{addr: addr} }
func (e *hcEndpoint) String() string     { return e.addr }
func (e *hcEndpoint) IsHealthy(context.Context) bool {
	e.healthy = !e.healthy
	return e.healthy
}
func (e *hcEndpoint) RoundTrip(context.Context, Request) (Response, error) { return nil, nil }

type hcUpdater struct{ buf *bytes.Buffer }

func (u hcUpdater) AddEndpoint(e Endpoint) { fmt.Fprintf(u.buf, "add endpoint '%s'\n", e.String()) }
func (u hcUpdater) DelEndpoint(e Endpoint) { fmt.Fprintf(u.buf, "delete endpoint '%s'\n", e.String()) }

func TestHealthChecker(t *testing.T) {
	hc := NewHealthCheck()

	buf := bytes.NewBufferString("\n")
	hc.AddUpdater(hcUpdater{buf: buf})

	interval := time.Millisecond * 50
	hc.AddEndpoint(newHCEndpoint("1.1.1.1:80"), interval, 0)
	hc.AddEndpoint(newHCEndpoint("2.2.2.2:80"), interval, 0)

	time.Sleep(time.Second)
	hc.Stop()

	var isDelete bool
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	for i := 0; i < len(lines); i += 2 {
		line1 := strings.TrimSpace(lines[i])
		line2 := strings.TrimSpace(lines[i+1])

		if line1 == line2 {
			t.Error(line1)
		} else if isDelete {
			if line1 != "delete endpoint '1.1.1.1:80'" && line1 != "delete endpoint '2.2.2.2:80'" {
				t.Error(line1)
			} else if line2 != "delete endpoint '1.1.1.1:80'" && line2 != "delete endpoint '2.2.2.2:80'" {
				t.Error(line2)
			}
		} else {
			if line1 != "add endpoint '1.1.1.1:80'" && line1 != "add endpoint '2.2.2.2:80'" {
				t.Error(line1)
			} else if line2 != "add endpoint '1.1.1.1:80'" && line2 != "add endpoint '2.2.2.2:80'" {
				t.Error(line2)
			}
		}

		isDelete = !isDelete
	}
}
