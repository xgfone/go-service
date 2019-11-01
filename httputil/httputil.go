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

package httputil

import (
	"net/http"
)

// RoundTripperMiddleware is a chainable behavior modifier for http.RoundTripper.
type RoundTripperMiddleware func(next http.RoundTripper) http.RoundTripper

// RoundTripperMiddlewareChain is a helper function for composing middlewares,
// which will be called in turn from first to last.
func RoundTripperMiddlewareChain(outer RoundTripperMiddleware, others ...RoundTripperMiddleware) RoundTripperMiddleware {
	return func(next http.RoundTripper) http.RoundTripper {
		for i := len(others) - 1; i >= 0; i-- { // reverse
			next = others[i](next)
		}
		return outer(next)
	}
}

// RoundTripperFunc converts a function to http.RoundTripper.
func RoundTripperFunc(f func(*http.Request) (*http.Response, error)) http.RoundTripper {
	return roundTripperFunc(f)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// NewRoundTripperWrapper returns the a new http.RoundTripper that will wrap rt
// by the function f.
func NewRoundTripperWrapper(rt http.RoundTripper, f func(http.RoundTripper, *http.Request) (*http.Response, error)) http.RoundTripper {
	return roundTripperWrapper{rt: rt, f: f}
}

type roundTripperWrapper struct {
	rt http.RoundTripper
	f  func(http.RoundTripper, *http.Request) (*http.Response, error)
}

func (r roundTripperWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	return r.f(r.rt, req)
}

func (r roundTripperWrapper) WrappedRoundTripper() http.RoundTripper {
	return r.rt
}
