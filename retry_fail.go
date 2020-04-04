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
	"fmt"
)

// FailRetry is used to calculate the index of the next endpoint to be used
// to retry the service.
type FailRetry interface {
	// String returns the name of FailRetry.
	String() string

	// Next calculates the next endpoint to retry the service.
	//
	// totalNum is the total number of all the endpoints.
	// hasRetriedNum is the retried times, starting with 0.
	//
	// Return -1, it will terminate to retry.
	// Return  0, it will select the next endpoint to retry.
	// Return  1, it will use the same endpoint to retry.
	Next(totalNum, hasRetriedNum int) int
}

type failRetry struct {
	name string
	next func(int, int) int
}

func (r failRetry) String() string              { return r.name }
func (r failRetry) Next(total, retried int) int { return r.next(total, retried) }

// FailRetryFunc converts a function with the name to FailRetry.
func FailRetryFunc(name string, next func(total, retried int) int) FailRetry {
	return failRetry{name: name, next: next}
}

// FailFast returns a fast fail handler, which returns the error instantly
// and no retry.
//
// Notice: the name is "fastfail".
func FailFast() FailRetry {
	return FailRetryFunc("fastfail", func(total, retry int) int { return -1 })
}

func failRetryWithNext(name string, maxnum, next int) FailRetry {
	if maxnum < 0 {
		panic("the retry maximum number must not be a negative integer")
	}

	return FailRetryFunc(name, func(total, retried int) int {
		if maxnum == 0 && retried >= total {
			return -1
		} else if maxnum > 0 && retried >= maxnum {
			return -1
		}
		return next
	})
}

// FailTry returns a fail handler, which will retry the same endpoint
// until the maximum retry number.
//
// If maxnum is equal to 0, it will retry the same endpoint for the number
// of the endpoints.
//
// Notice: the name is "failtry(maxnum)".
func FailTry(maxnum int) FailRetry {
	return failRetryWithNext(fmt.Sprintf("failtry(%d)", maxnum), maxnum, 1)
}

// FailOver returns a fail handler, which will retry the other endpoints
// until the maximum retry number.
//
// If maxnum is equal to 0, it will retry until all endpoints are retried.
//
// Notice: the name is "failover(maxnum)".
func FailOver(maxnum int) FailRetry {
	return failRetryWithNext(fmt.Sprintf("failover(%d)", maxnum), maxnum, 0)
}
