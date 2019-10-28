package service

import (
	"context"
	"net"
)

// Request represents a request.
type Request interface {
	// SessionID returns the session id of the request context,
	// which may be the remote address.
	//
	// Notice: It maybe return the empty string, and the remote address will
	// be used at the moment.
	SessionID() string

	// RemoteAddr returns the address of the remote peer, that's, the sender
	// of the current request.
	//
	// Notice: it maybe return nil to represent that it is the originator and
	// not to forward the request.
	RemoteAddr() net.Addr
}

// Response represents a response.
type Response interface{}

// Endpoint represents a service endpoint.
type Endpoint interface {
	// String returns the description of the endpoint, which maybe the address
	// for TCP or URL for HTTP.
	String() string

	// IsHealthy reports whether the current endpoint is healthy.
	IsHealthy(context.Context) bool

	// RoundTrip sends the request to the current endpoint.
	RoundTrip(context.Context, Request) (Response, error)
}

// HealthChecker is used to check the health status of an endpoint.
type HealthChecker func(context.Context, Endpoint) bool

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
