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
	"context"
	"fmt"
	"io"
	"sort"
)

// Request represents a request.
type Request interface {
	// RemoteAddrString returns the address string of the remote peer,
	// that's, the sender of the current request.
	//
	// Notice: it maybe return "" to represent that it is the originator
	// and not the remote peer.
	RemoteAddrString() string

	// SessionID returns the session id of the request context, which is used
	// to bind the requests with the same session id to the same endpoint
	// to be handled, when enabling the session stick.
	//
	// Notice: it maybe return an empty string, but use RemoteAddrString instead.
	SessionID() string
}

// EndpointState represents the running state of the endpoint.
type EndpointState struct {
	TotalConnections   int64 // The total number of all the connections.
	CurrentConnections int64 // The number of all the current connections.
}

// Endpoint represents a service endpoint.
type Endpoint interface {
	// Returns the description of the endpoint, which is the unique identity
	// and may be the address or the url.
	//
	// Notice: For the virtual host, it should contain the host domain name,
	// such as "domain@ip:port".
	fmt.Stringer

	// Type returns the type of the endpoint, such as "http", "tcp", etc.
	Type() string

	// State returns the state of the current endpoint.
	State() EndpointState

	// MetaData is the internal data of the endpoint, which had better contain
	// a key "addr" to indicate the address of the endpoint.
	MetaData() map[string]interface{}

	// IsHealthy is used to detect whether the endpoint is healthy,
	// that's, the endpoint is connected or accessed.
	IsHealthy(context.Context) bool

	// RoundTrip sends the request to the current endpoint.
	RoundTrip(c context.Context, req Request) (response interface{}, err error)
}

// EndpointManager is used to add and delete the endpoint.
type EndpointManager interface {
	AddEndpoint(Endpoint)
	DelEndpoint(Endpoint)
}

// EndpointUpdater is used to update the endpoint.
type EndpointUpdater interface {
	EndpointManager
	Name() string
	io.Closer
}

// Endpoints is a set of Endpoint.
type Endpoints []Endpoint

func (es Endpoints) Sort()         { sort.Stable(es) }
func (es Endpoints) Len() int      { return len(es) }
func (es Endpoints) Swap(i, j int) { es[i], es[j] = es[j], es[i] }
func (es Endpoints) Less(i, j int) bool {
	if es[i] == nil {
		return false
	} else if es[j] == nil {
		return true
	}
	return es[i].String() < es[j].String()
}

// Contains reports whether the endpoints contains the endpoint.
func (es Endpoints) Contains(endpoint Endpoint) bool {
	return binarySearch(es, endpoint.String()) != -1
}

// NotContains reports whether the endpoints does not contain the endpoint.
func (es Endpoints) NotContains(endpoint Endpoint) bool {
	return binarySearch(es, endpoint.String()) == -1
}

func binarySearch(eps Endpoints, addr string) int {
	for low, high := 0, len(eps)-1; low <= high; {
		mid := (low + high)
		eaddr := eps[mid].String()
		if eaddr == addr {
			return mid
		} else if addr < eaddr {
			high = mid - 1
		} else {
			low = mid + 1
		}
	}

	return -1
}

// WeightEndpoint represents an endpoint with the weight.
type WeightEndpoint interface {
	Endpoint

	// Weight returns the weight of the endpoint, which may be equal to 0,
	// but not the negative.
	//
	// The larger the weight, the higher the weight.
	Weight() int
}

// NewWeightEndpoint returns a WeightEndpoint with the weight and the endpoint.
func NewWeightEndpoint(endpoint Endpoint, weight int) WeightEndpoint {
	return NewDynamicWeightEndpoint(endpoint, func(Endpoint) int { return weight })
}

// NewDynamicWeightEndpoint returns a new WeightEndpoint with the endpoint and
// the weigthFunc that returns the weight of the endpoint.
func NewDynamicWeightEndpoint(endpoint Endpoint, weightFunc func(Endpoint) int) WeightEndpoint {
	return weightEndpoint{Endpoint: endpoint, weight: weightFunc}
}

type weightEndpoint struct {
	Endpoint
	weight func(Endpoint) int
}

func (we weightEndpoint) Weight() int              { return we.weight(we.Endpoint) }
func (we weightEndpoint) UnwrapEndpoint() Endpoint { return we.Endpoint }
func (we weightEndpoint) MetaData() map[string]interface{} {
	md := we.Endpoint.MetaData()
	md["weight"] = we.weight(we.Endpoint)
	return md
}

// EndpointUnwrap is used to unwrap the inner endpoint.
type EndpointUnwrap interface {
	// Unwrap unwraps the inner endpoint, or nil instead if no inner endpoint.
	UnwrapEndpoint() Endpoint
}

// UnwrapEndpoint unwraps the endpoint until it has not implemented
// the interface EndpointUnwrap.
func UnwrapEndpoint(endpoint Endpoint) Endpoint {
	for {
		if eu, ok := endpoint.(EndpointUnwrap); ok {
			if ep := eu.UnwrapEndpoint(); ep != nil {
				endpoint = ep
			} else {
				break
			}
		} else {
			break
		}
	}
	return endpoint
}

// EndpointHealthChecker is used to check whether the endpoint addr is healthy.
type EndpointHealthChecker func(ctx context.Context, addr string) error
