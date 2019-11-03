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

// FailRetry is used to calculate the index of the next endpoint to be used
// to retry the service. If returning -1, it will terminate to retry.
type FailRetry func(currentEndpointIndex, hasRetriedNum int) (nextEndpointIndex int)

// FailFast returns a fast fail handler, which returns the error instantly
// and no retry.
func FailFast() FailRetry { return func(index, retry int) int { return -1 } }

// FailTry returns a fail handler, which will retry the same endpoint
// until the maximum retry number.
//
// If maxnum is equal to 0, it will retry the same endpoint for the number
// of the endpoints.
func FailTry(maxnum int) FailRetry {
	if maxnum < 0 {
		panic("the retry maximum number must not be a negative integer")
	}

	return func(index, retry int) int {
		if maxnum > 0 && retry > maxnum {
			return -1
		}
		return index
	}
}

// FailOver returns a fail handler, which will retry the other endpoints
// until the maximum retry number.
//
// If maxnum is equal to 0, it will retry until all endpoints are retryied.
func FailOver(maxnum int) FailRetry {
	if maxnum < 0 {
		panic("the retry maximum number must not be a negative integer")
	}

	return func(index, retry int) int {
		if maxnum > 0 && retry > maxnum {
			return -1
		}
		return index + 1
	}
}
