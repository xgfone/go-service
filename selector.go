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
	"encoding/binary"
	"math/rand"
	"net"
)

// Selector is used to to select the active endpoint to be used.
//
// Notice: the selector should return the result as soon as possible.
type Selector interface {
	// String returns the name of the selector.
	String() string

	// Select returns the index of the selected endpoint from endpoints
	// by the request.
	Select(request Request, endpoints Endpoints) (index int)
}

type selector struct {
	name     string
	selector func(Request, Endpoints) int
}

func (s selector) String() string                      { return s.name }
func (s selector) Select(r Request, eps Endpoints) int { return s.selector(r, eps) }

// SelectorFunc returns a new Selector with the name and the selector.
func SelectorFunc(name string, s func(Request, Endpoints) int) Selector {
	return selector{name: name, selector: s}
}

var selectors map[string]Selector

// RegisterSelector registers a Selector.
//
// If the selector has been registered, it will be overrided.
func RegisterSelector(selector Selector) {
	selectors[selector.String()] = selector
}

// GetSelector returns the Selector named name.
//
// Return nil if the selector does not exist.
func GetSelector(name string) Selector { return selectors[name] }

// RandomSelector returns a random selector which returns a endpoint randomly,
// whose name is "random".
//
// If the endpoint has implemented the inerface WeightEndpoint, it will select
// an endpoint based on the weight.
func RandomSelector() Selector {
	getWeight := func(ep Endpoint) (weight int) {
		if we, ok := ep.(WeightEndpoint); ok {
			weight = we.Weight()
		}
		return
	}
	return SelectorFunc("random", func(req Request, endpoints Endpoints) int {
		var lastWeight int
		var totalWeight int

		sameWeight := true
		for i, ep := range endpoints {
			weight := getWeight(ep)
			totalWeight += weight
			if i == 0 {
				lastWeight = weight
			} else if sameWeight && weight != lastWeight {
				sameWeight = false
			}
		}

		if sameWeight || totalWeight == 0 {
			offset := rand.Intn(totalWeight)
			for i, ep := range endpoints {
				if offset -= getWeight(ep); offset < 0 {
					return i
				}
			}
		}

		return rand.Intn(len(endpoints))
	})
}

// RoundRobinSelector returns a RoundRobin selector, whose name is "round_robin".
func RoundRobinSelector() Selector {
	var last uint64
	return SelectorFunc("round_robin", func(req Request, endpoints Endpoints) int {
		last++
		return int(last % uint64(len(endpoints)))
	})
}

// SourceIPSelector returns an endpoint selector based on the source ip,
// whose name is "source_ip".
//
// Notice: If failing to parse the remote address, it will degenerate to
// the RoundRobin selector.
func SourceIPSelector() Selector {
	rr := RoundRobinSelector()
	return SelectorFunc("source_ip", func(req Request, endpoints Endpoints) int {
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
				ip = net.ParseIP(raddr.RemoteAddr().String())
			}
		} else if host, _, err := net.SplitHostPort(req.RemoteAddrString()); err == nil {
			ip = net.ParseIP(host)
		}

		var value uint64
		switch len(ip) {
		case net.IPv4len:
			value = uint64(binary.BigEndian.Uint32(ip))
		case net.IPv6len:
			value = binary.BigEndian.Uint64(ip[8:16])
		default:
			return rr.Select(req, endpoints)
		}

		return int(value % uint64(len(endpoints)))
	})
}
