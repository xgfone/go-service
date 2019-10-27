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
