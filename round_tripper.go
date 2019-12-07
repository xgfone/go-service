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

// RoundTripperGetter is used to get the corresponding RoundTripper by the key.
//
// If no the corresponding RoundTripper, it should return nil.
type RoundTripperGetter func(key string) RoundTripper

// NewRoundTripperGetterFromMap returns a new RoundTripperGetter,
// which returns the RoundTripper from a map.
func NewRoundTripperGetterFromMap(ms map[string]RoundTripper) RoundTripperGetter {
	return func(key string) RoundTripper {
		return ms[key]
	}
}

// NewSingleRoundTripperGetter returns a new RoundTripperGetter, which compares
// the key with the request key and returns the rt if they are equal.
func NewSingleRoundTripperGetter(key string, rt RoundTripper) RoundTripperGetter {
	return func(rkey string) RoundTripper {
		if rkey == key {
			return rt
		}
		return nil
	}
}
