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
	"net/http"
	"strings"
	"time"
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

// Response represents a response.
type Response interface{}

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

// EndpointStatus is used to manage the status of the endpoint.
type EndpointStatus interface {
	// Activate is called when the endpoint is added into the provider,
	// if the endpoint has implemented the interface.
	Activate(context.Context)

	// Deactivate is called when the endpoint is removed from the provider,
	// if the endpoint has implemented the interface.
	Deactivate(context.Context)
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

// HealthChecker is used to check the health status of an endpoint.
type HealthChecker func(context.Context, Endpoint) bool

// CheckEndpointHealth check whether the endpoint is the healthy.
//
// If the endpoint is a HTTP URL, it will use the GET method to request it
// with the http client getting from the context. Or it will treat it as
// the address and test it by the TCP connection.
func CheckEndpointHealth(timeout time.Duration) HealthChecker {
	return func(ctx context.Context, endpoint Endpoint) bool {
		addr := endpoint.String()
		if !strings.HasPrefix(addr, "http") {
			if conn, err := net.DialTimeout("tcp", addr, timeout); err == nil {
				conn.Close()
				return true
			}
			return false
		}

		var cancel func()
		req, err := http.NewRequest(http.MethodGet, addr, nil)
		if err != nil {
			return false
		} else if timeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		req = req.WithContext(ctx)
		resp, err := GetHTTPClientFromContext(ctx).Do(req)
		if err != nil {
			return false
		}

		resp.Body.Close()
		return true
	}
}

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

type noopRequest struct{ addr string }

// NewNoopRequest returns a Noop request with the remote address,
// which may be empty.
func NewNoopRequest(addr string) Request       { return noopRequest{addr: addr} }
func (r noopRequest) RemoteAddrString() string { return r.addr }

// NewWeightEndpoint returns a WeightEndpoint with the weight and the endpoint.
func NewWeightEndpoint(weight int, endpoint Endpoint) WeightEndpoint {
	return weightEndpoint{Endpoint: endpoint, weight: func(Endpoint) int { return weight }}
}

// NewDynamicWeightEndpoint returns a new WeightEndpoint with the endpoint and
// the weigthFunc that returns the weight of the endpoint.
func NewDynamicWeightEndpoint(weightFunc func(Endpoint) int, endpoint Endpoint) WeightEndpoint {
	return weightEndpoint{Endpoint: endpoint, weight: weightFunc}
}

type weightEndpoint struct {
	Endpoint

	weight func(Endpoint) int
}

func (we weightEndpoint) Weight() int { return we.weight(we.Endpoint) }
func (we weightEndpoint) Activate(ctx context.Context) {
	if eps, ok := we.Endpoint.(EndpointStatus); ok {
		eps.Activate(ctx)
	}
}
func (we weightEndpoint) Deactivate(ctx context.Context) {
	if eps, ok := we.Endpoint.(EndpointStatus); ok {
		eps.Deactivate(ctx)
	}
}
