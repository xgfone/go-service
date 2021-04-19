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
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"time"
)

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

var selectors = make(map[string]Selector)

func init() {
	RegisterSelector(RandomSelector())
	RegisterSelector(WeightSelector())
	RegisterSelector(SourceIPSelector())
	RegisterSelector(RoundRobinSelector())
	RegisterSelector(LeastConnectionSelector())
}

// RegisterSelector registers the selector, and panics if it has been registered.
//
// The package has registered the selectors named ""round_robin", "random",
// weight", "source_ip" and "least_connections". So you can use the function
// GetSelector to get them.
func RegisterSelector(selector Selector) {
	name := selector.String()
	if _, ok := selectors[name]; ok {
		panic(fmt.Errorf("the policy selector named '%s' has been registered", name))
	}
	selectors[name] = selector
}

// UnregisterSelector unregisters the selector by the name.
func UnregisterSelector(name string) { delete(selectors, name) }

// GetSelector returns the selector by the name.
//
// Return nil if the selector has not been registered.
func GetSelector(name string) Selector { return selectors[name] }

// GetSelectors returns all the registered selectors.
func GetSelectors() []Selector {
	ss := make([]Selector, 0, len(selectors))
	for _, s := range selectors {
		ss = append(ss, s)
	}
	return ss
}

// SelectorGetSetter is used to get or set the selector.
type SelectorGetSetter interface {
	// GetSelector returns the selector.
	GetSelector() Selector

	// SetSelector sets the selector.
	SetSelector(Selector)
}

// Selector is used to to select the active endpoint to be used.
type Selector interface {
	// String returns the name of the selector.
	String() string

	// Select returns the selected endpoint from endpoints by the request
	// to forward the request.
	Select(request Request, endpoints []Endpoint) Endpoint
}

type selector struct {
	name     string
	selector func(Request, Endpoints) Endpoint
}

func (s selector) String() string                            { return s.name }
func (s selector) Select(r Request, eps []Endpoint) Endpoint { return s.selector(r, eps) }

// SelectorFunc returns a new Selector with the name and the selector.
func SelectorFunc(name string, s func(Request, Endpoints) Endpoint) Selector {
	return selector{name: name, selector: s}
}

// RandomSelector returns a random selector which returns a endpoint randomly,
// whose name is "random".
func RandomSelector() Selector {
	return SelectorFunc("random", func(req Request, eps Endpoints) Endpoint {
		return eps[random.Intn(len(eps))]
	})
}

// RoundRobinSelector returns a RoundRobin selector, whose name is "round_robin".
func RoundRobinSelector() Selector {
	return roundRobinSelector(random.Intn(64))
}

func roundRobinSelector(start int) Selector {
	last := uint64(start)
	return SelectorFunc("round_robin", func(req Request, eps Endpoints) Endpoint {
		last++
		return eps[last%uint64(len(eps))]
	})
}

// SourceIPSelector returns an endpoint selector based on the source ip,
// whose name is "source_ip".
//
// If the request has implemented the interface { RemoteAddr() net.Addr },
// it will get the source ip from RemoteAddr(). Or, parse the source ip
// from RemoteAddrString().
//
// Notice: If failing to parse the remote address, it will degenerate to
// the RoundRobin selector.
func SourceIPSelector() Selector {
	rr := RoundRobinSelector()
	return SelectorFunc("source_ip", func(req Request, eps Endpoints) Endpoint {
		var ip net.IP
		if raddr, ok := req.(interface{ RemoteAddr() net.Addr }); ok {
			switch addr := raddr.RemoteAddr().(type) {
			case *net.IPAddr:
				ip = addr.IP
			case *net.TCPAddr:
				ip = addr.IP
			case *net.UDPAddr:
				ip = addr.IP
			default:
				addrs := addr.String()
				if host, _, _ := net.SplitHostPort(addrs); host == "" {
					ip = net.ParseIP(addrs)
				} else {
					ip = net.ParseIP(host)
				}
			}
		} else if host, _, _ := net.SplitHostPort(req.RemoteAddrString()); host != "" {
			ip = net.ParseIP(host)
		}

		var value uint64
		switch len(ip) {
		case net.IPv4len:
			value = uint64(binary.BigEndian.Uint32(ip))
		case net.IPv6len:
			value = binary.BigEndian.Uint64(ip[8:16])
		default:
			return rr.Select(req, eps)
		}

		return eps[value%uint64(len(eps))]
	})
}

// WeightSelector returns an endpoint selector based on the weight,
// whose name is "weight".
//
// Notice: If all the endpoints have the same weight, select one randomly.
func WeightSelector() Selector {
	getWeight := func(ep Endpoint) (weight int) {
		if we, ok := ep.(WeightEndpoint); ok {
			weight = we.Weight()
		}
		return
	}

	return SelectorFunc("weight", func(req Request, eps Endpoints) Endpoint {
		length := len(eps)
		sameWeight := true
		firstWeight := getWeight(eps[0])
		totalWeight := firstWeight

		weights := make([]int, length)
		weights[0] = firstWeight

		for i := 1; i < length; i++ {
			weight := getWeight(eps[i])
			weights[i] = weight
			totalWeight += weight
			if sameWeight && weight != firstWeight {
				sameWeight = false
			}
		}

		if !sameWeight && totalWeight > 0 {
			offset := random.Intn(totalWeight)
			for i := 0; i < length; i++ {
				if offset -= weights[i]; offset < 0 {
					return eps[i]
				}
			}
		}

		return eps[random.Intn(len(eps))]
	})
}

// LeastConnectionSelector returns a endpoint selector based on the least
// connections, whose name is "least_connections".
//
// Notice: If all the endpoints have the same number of the connections,
// select one randomly.
func LeastConnectionSelector() Selector {
	return SelectorFunc("least_connections", func(req Request, eps Endpoints) Endpoint {
		length := len(eps)
		sameConns := true
		firstConns := eps[0].State().CurrentConnections
		totalConns := firstConns

		conns := make([]int64, length)
		conns[0] = firstConns

		for i := 1; i < length; i++ {
			conn := eps[i].State().CurrentConnections
			conns[i] = conn
			totalConns += conn
			if sameConns && conn != firstConns {
				sameConns = false
			}
		}

		if !sameConns && totalConns > 0 {
			offset := totalConns - random.Int63n(totalConns)
			for i := 0; i < length; i++ {
				if offset -= conns[i]; offset < 0 {
					return eps[i]
				}
			}
		}

		return eps[random.Intn(len(eps))]
	})
}
