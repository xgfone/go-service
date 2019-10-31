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

import "context"

// RoundTripper is used to emit a request and to get the corresponding response.
type RoundTripper interface {
	RoundTrip(context.Context, Request) (Response, error)
}

// RoundTripperFunc is an adapter to allow the ordinary functions as RoundTripper.
type RoundTripperFunc func(context.Context, Request) (Response, error)

// RoundTrip implements RoundTripper.
func (f RoundTripperFunc) RoundTrip(c context.Context, r Request) (Response, error) {
	return f(c, r)
}
