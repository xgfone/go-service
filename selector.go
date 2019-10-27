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
func SourceIPSelector() Selector {
	return func(req Request, endpoints []Endpoint) int {
		var ip net.IP
		switch addr := req.RemoteAddr().(type) {
		case *net.IPAddr:
			ip = addr.IP
		case *net.TCPAddr:
			ip = addr.IP
		case *net.UDPAddr:
			ip = addr.IP
		default:
			ip = net.ParseIP(req.RemoteAddr().String())
		}

		var value uint64
		switch len(ip) {
		case net.IPv4len:
			value = uint64(binary.BigEndian.Uint32(ip))
		case net.IPv6len:
			value = binary.BigEndian.Uint64(ip[8:16])
		default:
			return 0
		}

		return int(value % uint64(len(endpoints)))
	}
}
