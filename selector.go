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

var selectors = make(map[string]Selector, 8)

func init() {
	RegisterSelector("random", RandomSelector())
	RegisterSelector("sourceip", SourceIPSelector())
	RegisterSelector("roundrobin", RoundRobinSelector())
}

// Selector is used to to select the service endpoint to be used,
// which will return the index of the used endpoint.
//
// It won't be called concurrently.
type Selector func(req Request, endpoints []Endpoint) (index int)

// RegisterSelector registers the selector with the name, which will reset
// the selector and return true if the name has been registered.
//
// It has registered the "random", "roundrobin", "sourceip" selectors by the default.
func RegisterSelector(name string, selector Selector) bool {
	_, ok := selectors[name]
	selectors[name] = selector
	return ok
}

// GetSelector returns the registered selector by the name.
//
// Return nil if there is no selector.
func GetSelector(name string) Selector {
	return selectors[name]
}

// RandomSelector returns a random selector which returns a endpoint randomly.
func RandomSelector() Selector {
	return func(req Request, endpoints []Endpoint) int {
		return rand.Intn(len(endpoints))
	}
}

// RoundRobinSelector returns a RoundRobin selector.
func RoundRobinSelector() Selector {
	var last uint64
	return func(req Request, endpoints []Endpoint) int {
		last++
		return int(last % uint64(len(endpoints)))
	}
}

// SourceIPSelector returns an endpoint selector based on the source ip.
//
// Notice: If failing to parse the remote address, it will degenerate to
// the RoundRobin selector.
func SourceIPSelector() Selector {
	rr := RoundRobinSelector()
	return func(req Request, endpoints []Endpoint) int {
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
			return rr(req, endpoints)
		}

		return int(value % uint64(len(endpoints)))
	}
}
