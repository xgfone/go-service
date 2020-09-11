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
	"context"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/xgfone/go-service/retry"
)

// RequestSession represents a request session of the business logic.
type RequestSession interface {
	// SessionID returns the session id of the request context, which is used,
	// when enabling the session stick, to bind the requests with the same
	// session id but without the same connection to a backend endpoint
	// to be handled.
	//
	// It maybe return the remote address as the session id. In this case,
	// you does not need to implement it, it will RemoteAddrString() instead.
	//
	// Notice: it should not return an empty string.
	SessionID() string
}

// Request represents a request.
type Request interface {
	// RemoteAddrString returns the address string of the remote peer,
	// that's, the sender of the current request.
	//
	// Notice: it maybe return "" to represent that it is the originator
	// and not to forward the request.
	RemoteAddrString() string
}

type noopRequest struct{ addr string }

// NewNoopRequest returns a Noop request with the remote address,
// which may be empty.
func NewNoopRequest(addr string) Request       { return noopRequest{addr: addr} }
func (r noopRequest) RemoteAddrString() string { return r.addr }

/// -------------------------------------------------------------------------

// Response represents a response.
type Response interface{}

/// ------------------------------------------------------------------------

// Endpoint represents a service endpoint.
type Endpoint interface {
	// String returns the description of the endpoint, which is the unique
	// identity and may be the address or the url.
	String() string

	// IsHealthy reports whether the current endpoint is healthy.
	//
	// It is used to detect whether the endpoint is still active.
	// If you use other alive probe, the method maybe always return true.
	IsHealthy(context.Context) bool

	// RoundTrip sends the request to the current endpoint.
	RoundTrip(context.Context, Request) (Response, error)
}

type noopEndpoint string

// NewNoopEndpoint returns a new Noop endpoint, which does nothing.
func NewNoopEndpoint(addr string) Endpoint                                  { return noopEndpoint(addr) }
func (e noopEndpoint) String() string                                       { return string(e) }
func (e noopEndpoint) IsHealthy(context.Context) bool                       { return true }
func (e noopEndpoint) RoundTrip(context.Context, Request) (Response, error) { return nil, nil }

/// ------------------------------------------------------------------------

// Endpoints is a set of Endpoint.
type Endpoints []Endpoint

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
	eps := endpoint.String()
	for _, ep := range es {
		if ep.String() == eps {
			return true
		}
	}
	return false
}

/// ------------------------------------------------------------------------

// EndpointUnwrap is used to unwrap the inner endpoint.
type EndpointUnwrap interface {
	// Unwrap unwraps the inner endpoint, but returns nil instead if no inner
	// endpoint.
	Unwrap() Endpoint
}

// UnwrapEndpoint unwraps the endpoint until it has not implemented
// the interface EndpointUnwrap.
func UnwrapEndpoint(endpoint Endpoint) Endpoint {
	for {
		if eu, ok := endpoint.(EndpointUnwrap); ok {
			endpoint = eu.Unwrap()
		} else {
			break
		}
	}
	return endpoint
}

/// ------------------------------------------------------------------------

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

func (we weightEndpoint) Weight() int { return we.weight(we.Endpoint) }

/// -------------------------------------------------------------------------

// Middleware is a chainable behavior modifier for endpoints.
type Middleware func(next Endpoint) Endpoint

// Chain is a helper function for composing middlewares, which will be called
// in turn from first to last.
func Chain(outer Middleware, others ...Middleware) Middleware {
	return func(next Endpoint) Endpoint {
		for i := len(others) - 1; i >= 0; i-- { // reverse
			next = others[i](next)
		}
		return outer(next)
	}
}

/// -------------------------------------------------------------------------

// HealthChecker is used to check the health status of an endpoint.
type HealthChecker func(context.Context, Endpoint) error

// CheckEndpointHealth check whether the endpoint is the healthy.
//
// If the endpoint is a HTTP URL, it will extract the Host field and test it
// by the TCP connection.
//
// If failing to check the endpoint and retryNum is greater than 0, it will
// retry it, and if retryInterval is equal to 0, it will retry it immediately,
// not wait for the interval duration.
func CheckEndpointHealth(timeout, retryInterval time.Duration, retryNum int) HealthChecker {
	retry := retry.NewIntervalRetry(retryNum, retryInterval)
	return func(ctx context.Context, endpoint Endpoint) error {
		addr := endpoint.String()
		if strings.HasPrefix(addr, "http") {
			if u, err := url.Parse(addr); err != nil {
				return err
			} else if _, _, err := net.SplitHostPort(u.Host); err == nil {
				addr = u.Host
			} else if strings.HasPrefix(addr, "https") {
				addr = net.JoinHostPort(u.Host, "80")
			} else {
				addr = net.JoinHostPort(u.Host, "443")
			}
		}

		_, err := retry.Call(ctx, dialTCP, addr, timeout)
		if err != nil {
			return err.(error)
		}
		return nil
	}
}

func dialTCP(ctx context.Context, args ...interface{}) (interface{}, error) {
	addr := args[0].(string)
	timeout := args[1].(time.Duration)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err == nil {
		conn.Close()
		return nil, nil
	}
	return nil, err
}
