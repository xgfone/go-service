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

func ExampleRetry() {
	// Retry successfully
	var retry bool
	callee := func(ctx context.Context) (interface{}, error) {
		if retry {
			return retry, nil
		}
		retry = true
		return nil, fmt.Errorf("calling failed")
	}

	result, err := Retry(context.Background(), 3, time.Millisecond*10, callee)
	if err != nil {
		fmt.Printf("Fail to retry: %s\n", err)
	} else {
		fmt.Printf("Retry successfully, result is %v\n", result)
	}

	// Retry unsuccessfully
	var num int
	callee = func(ctx context.Context) (interface{}, error) {
		num++
		return nil, fmt.Errorf("calling failed")
	}

	// Call once and retry 3 times, so it's 4 times in total that callee's called.
	_, err = Retry(context.Background(), 3, 0, callee)
	fmt.Printf("Call %d times: %s\n", num, err)

	// Output:
	// Retry successfully, result is true
	// Call 4 times: calling failed
}
