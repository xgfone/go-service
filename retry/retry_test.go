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
	"errors"
	"testing"
	"time"
)

func callError(c context.Context, a interface{}) (interface{}, error) {
	return nil, errors.New("test")
}

func TestNewIntervalRetry(t *testing.T) {
	start := time.Now()
	retry := NewIntervalRetry(3, time.Millisecond*20)
	if _, err := retry.Call(context.TODO(), nil, callError); err == nil {
		t.Fail()
	} else if err.Error() != "test" {
		t.Errorf("the error is 'test': %s", err)
	}

	if cost := time.Now().Sub(start); cost < time.Millisecond*60 ||
		cost > time.Millisecond*120 {
		t.Error(cost)
	}

	start = time.Now()
	retry = NewIntervalRetry(5, 0)
	if _, err := retry.Call(context.TODO(), nil, callError); err == nil {
		t.Fail()
	} else if err.Error() != "test" {
		t.Errorf("the error is 'test': %s", err)
	}

	if cost := time.Now().Sub(start); cost > time.Millisecond*10 {
		t.Errorf("the cost of the retry call is greater than 10ms: %s", cost)
	}
}

func TestNewDoubleDelayRetry(t *testing.T) {
	start := time.Now()
	retry := NewDoubleDelayRetry(3, time.Millisecond*20, 0)
	if _, err := retry.Call(context.TODO(), nil, callError); err == nil {
		t.Fail()
	} else if err.Error() != "test" {
		t.Errorf("the error is 'test': %s", err)
	}

	if cost := time.Now().Sub(start); cost < time.Millisecond*140 {
		t.Errorf("the cost of the retry call is less than 140ms: %s", cost)
	}

	start = time.Now()
	retry = NewDoubleDelayRetry(5, time.Millisecond*10, time.Millisecond*20)
	if _, err := retry.Call(context.TODO(), nil, callError); err == nil {
		t.Fail()
	} else if err.Error() != "test" {
		t.Errorf("the error is 'test': %s", err)
	}

	if cost := time.Now().Sub(start); cost < time.Millisecond*90 ||
		cost > time.Millisecond*200 {
		t.Error(cost)
	}
}
