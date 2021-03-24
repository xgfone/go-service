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

// HTTPRoundTripper is used as the http RoundTripper to intercept the request,
// which finds the loadbalancer roundtripper by the reqest to forward it.
// Or use the default transport to forward it.
type HTTPRoundTripper struct {
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

var _ http.RoundTripper = &HTTPRoundTripper{}

// RoundTrip implements the interface http.RoundTripper.
func (hrt *HTTPRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	if rt := hrt.GetRoundTripper(r); rt != nil {
		ctx := context.Background()
		if hrt.Timeout > 0 {
			var cancel func()
			ctx, cancel = context.WithTimeout(ctx, hrt.Timeout)
			defer cancel()
		}

		var sid string
		if hrt.GetSessionID != nil {
			sid = hrt.GetSessionID(r)
		}

		resp, err := rt.RoundTrip(ctx, NewHTTPRequest(r, sid))
		if err != nil {
			return nil, err
		}
		return resp.(*http.Response), nil
	}

	if hrt.Timeout > 0 {
		ctx := r.Context()
		if _, ok := ctx.Deadline(); !ok {
			var cancel func()
			ctx, cancel = context.WithTimeout(ctx, hrt.Timeout)
			defer cancel()

			r = r.WithContext(ctx)
		}
	}

	if hrt.Transport == nil {
		return http.DefaultTransport.RoundTrip(r)
	}
	return hrt.Transport.RoundTrip(r)
}
