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

// Package httputil supplies the middleware and the wrapper of http.RoundTripper.
package httputil

import "net/http"

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

// RoundTripperFunc is the function type of http.RoundTripper.
type RoundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip implements the interface http.RoundTripper.
func (f RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// RoundTripperWrapperFunc is the function to wrap http.RoundTripper.
type RoundTripperWrapperFunc func(http.RoundTripper, *http.Request) (*http.Response, error)

// RoundTripperWrapper is a wrapper of http.RoundTripper.
type RoundTripperWrapper interface {
	WrappedRoundTripper() http.RoundTripper
	http.RoundTripper
}

// NewRoundTripperWrapper returns the a new http.RoundTripper that will wrap rt
// by the function f.
func NewRoundTripperWrapper(rt http.RoundTripper, f RoundTripperWrapperFunc) RoundTripperWrapper {
	return roundTripperWrapper{rt: rt, wf: f}
}

type roundTripperWrapper struct {
	rt http.RoundTripper
	wf RoundTripperWrapperFunc
}

func (r roundTripperWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	return r.wf(r.rt, req)
}

func (r roundTripperWrapper) WrappedRoundTripper() http.RoundTripper {
	return r.rt
}
