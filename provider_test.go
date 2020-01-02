package service

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

	p.DelEndpointByString("127.0.0.1:8002")
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

func TestGeneralProvider_ProviderEndpointGate(t *testing.T) {
	p := NewGeneralProvider(RandomSelector())
	p.SetEndpointGates([]string{"127.0.0.1:8001"})
	p.AddEndpointGate("127.0.0.1:8003")

	p.AddEndpoint(NewHTTPEndpoint("127.0.0.1:8001", nil))
	p.AddEndpoint(NewHTTPEndpoint("127.0.0.1:8002", nil)) // Be Ignored
	p.AddEndpoint(NewHTTPEndpoint("127.0.0.1:8003", nil))
	p.AddEndpoint(NewHTTPEndpoint("127.0.0.1:8004", nil)) // Be Ignored

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

	p.DelEndpointGate("127.0.0.1:8001")
	if eps := p.GetEndpointGates(); len(eps) != 1 || eps[0] != "127.0.0.1:8003" {
		t.Error(eps)
	}

	p.DelEndpointGate("127.0.0.1:8003")
	if eps := p.GetEndpointGates(); len(eps) > 0 {
		t.Error(eps)
	}
}
