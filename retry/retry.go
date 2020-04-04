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

package retry

import (
	"context"
	"time"
)

// Caller is the caller function.
type Caller func(context.Context) (result interface{}, err error)

// Retry is used to retry a function call when it returns an error.
type Retry interface {
	// Call calls the caller and returns it result, which will retry it
	// when the caller returns the error.
	Call(context.Context, Caller) (result interface{}, err error)
}

// NewIntervalRetry returns a new Retry to call the caller, which will retry
// to call it until the caller returns nil or is called for number times.
//
// If number is equal to 0, it won't retry it. And If interval is 0, the caller
// will be called to retry immediately.
func NewIntervalRetry(number int, interval time.Duration) Retry {
	return intervalRetry{number: number, interval: interval}
}

type intervalRetry struct {
	number   int
	interval time.Duration
}

func (r intervalRetry) Call(c context.Context, f Caller) (interface{}, error) {
	result, err := f(c)

FOR:
	for number := r.number; err != nil && number > 0; number-- {
		if r.interval > 0 {
			timer := time.NewTimer(r.interval)
			select {
			case <-timer.C:
			case <-c.Done():
				timer.Stop()
				break FOR
			}
		} else {
			select {
			case <-c.Done():
				break FOR
			default:
			}
		}

		result, err = f(c)
	}

	return result, err
}

// NewDoubleDelayRetry returns a new Retry to call the caller, which will retry
// to call it until the caller returns nil or is called for number times.
//
// If number is equal to 0, it won't retry it. start is the start delay
// to retry it, which must not be equal to 0. But, end may be equal to 0.
// If greater than 0, it is the maximum delay duration to retry it.
func NewDoubleDelayRetry(number int, start, end time.Duration) Retry {
	return doubleDelayRetry{number: number, start: start, end: end}
}

type doubleDelayRetry struct {
	number int
	start  time.Duration
	end    time.Duration
}

func (r doubleDelayRetry) Call(c context.Context, f Caller) (interface{}, error) {
	result, err := f(c)

FOR:
	for number, start := r.number, r.start; err != nil && number > 0; number-- {
		timer := time.NewTimer(start)
		select {
		case <-timer.C:
		case <-c.Done():
			timer.Stop()
			break FOR
		}

		if result, err = f(c); err != nil {
			if start = start * 2; r.end > 0 && start > r.end {
				start = r.end
			}
		}
	}

	return result, err
}
