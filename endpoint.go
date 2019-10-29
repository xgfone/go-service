package service

import (
	"context"
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
