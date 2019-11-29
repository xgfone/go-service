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

import (
	"context"
	"fmt"
	"time"
)

// FailRetry is used to calculate the index of the next endpoint to be used
// to retry the service.
type FailRetry interface {
	// String returns the name of FailRetry.
	String() string

	// Next calculates the index of the next endpoint to be used to retry
	// the service. If returning -1, it will terminate to retry.
	Next(totalEndpointIndex, currentEndpointIndex, hasRetriedNum int) (nextEndpointIndex int)
}

type failRetry struct {
	name string
	next func(int, int, int) int
}

func (r failRetry) String() string                       { return r.name }
func (r failRetry) Next(total, current, retried int) int { return r.next(total, current, retried) }

// FailRetryFunc converts a function with the name to
func FailRetryFunc(name string, next func(total, current, retried int) (next int)) FailRetry {
	return failRetry{name: name, next: next}
}

// FailFast returns a fast fail handler, which returns the error instantly
// and no retry.
func FailFast() FailRetry {
	return FailRetryFunc("fastfail", func(total, index, retry int) int { return -1 })
}

func failRetryWithNext(name string, maxnum int, next bool) FailRetry {
	if maxnum < 0 {
		panic("the retry maximum number must not be a negative integer")
	}

	return FailRetryFunc(name, func(total, index, retry int) int {
		if maxnum == 0 {
			if retry >= total {
				return -1
			}
		} else if retry >= maxnum {
			return -1
		}

		if next {
			return index + 1
		}
		return index
	})
}

// FailTry returns a fail handler, which will retry the same endpoint
// until the maximum retry number.
//
// If maxnum is equal to 0, it will retry the same endpoint for the number
// of the endpoints.
func FailTry(maxnum int) FailRetry {
	return failRetryWithNext(fmt.Sprintf("failtry(%d)", maxnum), maxnum, false)
}

// FailOver returns a fail handler, which will retry the other endpoints
// until the maximum retry number.
//
// If maxnum is equal to 0, it will retry until all endpoints are retryied.
func FailOver(maxnum int) FailRetry {
	return failRetryWithNext(fmt.Sprintf("failover(%d)", maxnum), maxnum, true)
}

// Retry calls the callee function, which will retry it with the interval time
// for the number times when returning an error.
//
// If number is equal to 0, it won't retry it. And if interval is equal to 0,
// it will retry it immediately.
func Retry(ctx context.Context, number int, interval time.Duration,
	callee func(context.Context) (result interface{}, err error)) (
	result interface{}, err error) {

	if number < 0 {
		panic("the retry number must not be negative")
	}

	result, err = callee(ctx)
	for err != nil && number > 0 {
		number--

		if interval > 0 {
			timer := time.NewTimer(interval)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				break
			}
		} else {
			select {
			case <-ctx.Done():
				break
			default:
			}
		}

		result, err = callee(ctx)
	}

	return
}
