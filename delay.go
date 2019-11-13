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

import "time"

// Delay is used to get the Nth delay duration.
//
// retryNumber starts with 1, and lastDelay starts with 0.
type Delay func(retryNumber int, lastDelay time.Duration) (nextDelay time.Duration)

// NewFixedDelay returns a Delay that always returns the same delay duration.
func NewFixedDelay(delay time.Duration) Delay {
	return func(int, time.Duration) time.Duration { return delay }
}

// NewMultipleDelay returns a delay that will increase the
func NewMultipleDelay(start, end time.Duration) Delay {
	if start < 1 || end < 1 {
		panic("MultipleDelay: the start or end duration must be an positive integer")
	} else if start > end {
		panic("MultipleDelay: the start must not be greater than the end")
	}

	return func(num int, last time.Duration) time.Duration {
		if num == 1 {
			return start
		} else if last == end {
			return end
		}

		if next := last * 2; next < end {
			return next
		}
		return end
	}
}
