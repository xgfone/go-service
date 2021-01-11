// Copyright 2020 xgfone
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

	"github.com/xgfone/go-service/retry"
)

// FailRetry is used to retry the service.
type FailRetry interface {
	// String returns the name of FailRetry.
	String() string

	// Retry calls the endpoint to finish the service. If the endpoint returns
	// the error, it will retry it.
	//
	// ep is the initial endpoint, which failed to be call, so the request
	// should be retried to call by the initial endpoint or a new one from
	// Provider instead.
	Retry(c context.Context, r Request, ep Endpoint, p Provider) (Response, error)
}

/// -------------------------------------------------------------------------

// FailFast returns a fail handler without any retry.
//
// Notice: the name is "fastfail".
func FailFast() FailRetry { return failfastRetry{} }

type failfastRetry struct{}

func (r failfastRetry) String() string { return "fastfail" }
func (r failfastRetry) Retry(ctx context.Context, req Request, ep Endpoint,
	p Provider) (Response, error) {
	return ep.RoundTrip(ctx, req)
}

/// -------------------------------------------------------------------------

// FailTry returns a fail handler, which will retry the same endpoint
// until the maximum retry number.
//
// If maxnum is equal to 0, it will retry the same endpoint for the number
// of the endpoints.
//
// Notice: the name is "failtry(maxnum)".
func FailTry(maxnum int, retryf func(maxnum int) retry.Retry) FailRetry {
	if maxnum < 0 {
		panic("FailTry: the retry maximum number must not be a negative integer")
	}
	name := fmt.Sprintf("failtry(%d)", maxnum)
	return failRetry{name: name, maxnum: maxnum, retryf: retryf, sameep: true}
}

// FailOver returns a fail handler, which will retry the other endpoints
// until the maximum retry number.
//
// If maxnum is equal to 0, it will retry until all endpoints are retried.
//
// Notice: the name is "failover(maxnum)".
func FailOver(maxnum int, retryf func(maxnum int) retry.Retry) FailRetry {
	if maxnum < 0 {
		panic("FailOver: the retry maximum number must not be a negative integer")
	}
	name := fmt.Sprintf("failover(%d)", maxnum)
	return failRetry{name: name, maxnum: maxnum, retryf: retryf}
}

type failRetryArg struct {
	Request
	Endpoint
	Provider
}

type failRetry struct {
	name   string
	sameep bool

	maxnum int
	retryf func(int) retry.Retry
}

func (r failRetry) String() string { return r.name }
func (r failRetry) Retry(ctx context.Context, req Request, ep Endpoint,
	p Provider) (Response, error) {
	num := r.maxnum
	if num == 0 {
		if num = p.Len(); num == 0 {
			return nil, ErrNoAvailableEndpoint
		}
	}

	resp, err := r.retryf(num-1).Call(ctx, r.call, req, ep, p)
	if err == retry.ErrEndRetry {
		err = ErrNoAvailableEndpoint
	}
	return resp, err
}
func (r failRetry) call(c context.Context, args ...interface{}) (interface{}, error) {
	if r.sameep {
		return args[1].(Endpoint).RoundTrip(c, args[0].(Request))
	}

	req := args[0].(Request)
	if ep := args[2].(Provider).Select(req); ep != nil {
		return ep.RoundTrip(c, req)
	}

	return nil, retry.ErrEndRetry
}
