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
	"fmt"
	"math/rand"
	"time"
)

// FailRetry is used to retry to forward the request.
type FailRetry interface {
	// String returns the name of FailRetry.
	String() string

	// Retry retries to forward the request again.
	//
	// Endpoint is the initial endpoint, which failed to forward the request.
	// error is the error which is returned by Endpoint.
	//
	// The implementation maybe retry the initial endpoint or a new one
	// from the provider instead.
	Retry(context.Context, Provider, Request, Endpoint, error) (
		respEndpoint Endpoint, response interface{}, err error)
}

// FailFast returns a fast failure retry without any retry.
//
// Notice: the name is "fastfail".
func FailFast() FailRetry { return failfastRetry{} }

type failfastRetry struct{}

func (f failfastRetry) String() string { return "fastfail" }
func (f failfastRetry) Retry(ctx context.Context, provider Provider,
	request Request, endpoint Endpoint, err error) (Endpoint, interface{}, error) {
	return endpoint, nil, err
}

// FailTry returns a failure retry, which will retry the same endpoint
// until the maximum retry number.
//
// If maxnum is equal to 0, it is equivalent to 1.
//
// Notice: the name is "failtry(maxnum)".
func FailTry(maxnum int, interval time.Duration) FailRetry {
	return failTryRetry{maxnum: maxnum, interval: interval}
}

type failTryRetry struct {
	interval time.Duration
	maxnum   int
}

func (r failTryRetry) String() string { return fmt.Sprintf("failtry(%d)", r.maxnum) }
func (r failTryRetry) Retry(ctx context.Context, provider Provider,
	request Request, endpoint Endpoint, err error) (Endpoint, interface{}, error) {
	num := r.maxnum
	if num <= 0 {
		num = 1
	}

	var resp interface{}
	for i := 0; i < num && err != nil; i++ {
		if r.interval > 0 {
			ticker := time.NewTimer(r.interval)
			select {
			case <-ctx.Done():
				ticker.Stop()
				return endpoint, resp, err
			case <-ticker.C:
				ticker.Stop()
			}
		}

		resp, err = endpoint.RoundTrip(ctx, request)
	}

	return endpoint, resp, err
}

// FailOver returns a failure retry, which will retry the other endpoints
// until the maximum retry number or all endpoints are retried.
//
// If maxnum is equal to 0, it will retry until all endpoints are retried.
//
// Notice: the name is "failover(maxnum)".
func FailOver(maxnum int, interval time.Duration) FailRetry {
	return failOverRetry{
		maxnum:   maxnum,
		interval: interval,
		random:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

type failOverRetry struct {
	random   *rand.Rand
	interval time.Duration
	maxnum   int
}

func (r failOverRetry) String() string { return fmt.Sprintf("failover(%d)", r.maxnum) }
func (r failOverRetry) Retry(ctx context.Context, provider Provider,
	request Request, endpoint Endpoint, err error) (Endpoint, interface{}, error) {
	var resp interface{}
	var failedEndpoints Endpoints
	for i := 0; (r.maxnum <= 0 || i < r.maxnum) && err != nil; i++ {
		if r.interval > 0 {
			ticker := time.NewTimer(r.interval)
			select {
			case <-ctx.Done():
				ticker.Stop()
				return endpoint, resp, err
			case <-ticker.C:
				ticker.Stop()
			}
		}

		if failedEndpoints == nil {
			failedEndpoints = Endpoints{endpoint}
		} else {
			failedEndpoints = append(failedEndpoints, endpoint)
		}

		gone := true
		provider.Inspect(func(eps Endpoints) {
			if _len := len(eps); _len > 0 {
				start := r.random.Intn(_len)
				for end := start + _len; start < end; start++ {
					if ep := eps[start%_len]; failedEndpoints.NotContains(ep) {
						endpoint, gone = ep, false
						return
					}
				}
			}
		})
		if gone {
			break
		}

		resp, err = endpoint.RoundTrip(ctx, request)
	}

	return endpoint, resp, err
}
