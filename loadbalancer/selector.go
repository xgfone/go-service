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
	"encoding/binary"
	"math/rand"
	"net"
	"time"
)

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

// Selector is used to to select the active endpoint to be used.
//
// Notice: the selector should return the result as soon as possible.
type Selector interface {
	// String returns the name of the selector.
	String() string

	// Select returns the index of the selected endpoint from endpoints
	// by the request.
	//
	// Notice: it is thread-safe, so the implementation does not need
	// to use the lock. And the number of endpoints is a positive integer.
	Select(request Request, endpoints Endpoints) Endpoint
}

type selector struct {
	name     string
	selector func(Request, Endpoints) Endpoint
}

func (s selector) String() string                           { return s.name }
func (s selector) Select(r Request, eps Endpoints) Endpoint { return s.selector(r, eps) }

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
			return rr.Select(req, eps)
		}

		return eps[value%uint64(len(eps))]
	})
}

// WeightSelector returns an endpoint selector based on the weight,
// whose name is "weight".
//
// Notice: If failing to parse the remote address, it will degenerate to
// the RoundRobin selector.
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
