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
	"io"
	"net/http"
	"time"

	"github.com/xgfone/go-service/httputil"
)

// RoundTripper is used to emit a request and to get the corresponding response.
type RoundTripper interface {
	RoundTrip(context.Context, Request) (response interface{}, err error)
}

// RoundTripperFunc is an adapter to allow the ordinary functions as RoundTripper.
type RoundTripperFunc func(context.Context, Request) (response interface{}, err error)

// RoundTrip implements RoundTripper.
func (f RoundTripperFunc) RoundTrip(c context.Context, r Request) (interface{}, error) {
	return f(c, r)
}

// LoadBalancerRoundTripper is the loadbalancer round tripper.
type LoadBalancerRoundTripper interface {
	Name() string
	RoundTripper
	io.Closer
}

// HTTPRoundTripperConfig is used to configure the HTTP RoundTripper.
type HTTPRoundTripperConfig struct {
	// Timeout is the maximum timeout to forward the reqest.
	//
	// Default: 0
	Timeout time.Duration

	// Transport is the default http RoundTripper, which is used to forward
	// the request when GetRoundTripper returns nil.
	//
	// Default: http.DefaultTransport
	Transport http.RoundTripper

	// GetSessionID returns the session id from the request. But return ""
	// instead if no session id.
	//
	// Default: a noop function to return "".
	GetSessionID func(*http.Request) (sid string)

	// GetRoundTripper returns the RoundTripper by the request to forward it.
	// But return nil instead if no the corresponding RoundTripper.
	// In this case, it will use the field Transport instead.
	//
	// Mandatory.
	GetRoundTripper func(*http.Request) RoundTripper
}

// NewHTTPRoundTripper returns a new http.RoundTripper, which finds
// the loadbalancer roundtripper by the reqest to forward it. Or use the
// default transport to forward it. And you can use it to as the Transport
// of http.Client to intercept the request.
func NewHTTPRoundTripper(c HTTPRoundTripperConfig) http.RoundTripper {
	if c.GetRoundTripper == nil {
		panic("ToHTTPRoundTripper: GetRoundTripper must not be nil")
	} else if c.GetSessionID == nil {
		c.GetSessionID = func(*http.Request) string { return "" }
	}

	return httputil.RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if rt := c.GetRoundTripper(r); rt != nil {
			ctx := context.Background()
			if c.Timeout > 0 {
				var cancel func()
				ctx, cancel = context.WithTimeout(ctx, c.Timeout)
				defer cancel()
			}

			resp, err := rt.RoundTrip(ctx, NewHTTPRequest(r, c.GetSessionID(r)))
			if err != nil {
				return nil, err
			}
			return resp.(*http.Response), nil
		}

		if c.Timeout > 0 {
			ctx := r.Context()
			if _, ok := ctx.Deadline(); !ok {
				var cancel func()
				ctx, cancel = context.WithTimeout(ctx, c.Timeout)
				defer cancel()

				r = r.WithContext(ctx)
			}
		}

		return http.DefaultTransport.RoundTrip(r)
	})
}

/*
func roundTripp(lb *loadbalancer.LoadBalancer, host string) http.RoundTripper {
	return httputil.RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Host == host {
			resp, err := lb.RoundTrip(context.Background(), loadbalancer.NewHTTPRequest(r, ""))
			if err != nil {
				return nil, err
			}
			return resp.(*http.Response), nil
		}
		return http.DefaultTransport.RoundTrip(r)
	})
}
*/
