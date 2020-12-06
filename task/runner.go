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

package task

import (
	"context"
	"net"
	"strings"
)

// Runner is a task runner.
type Runner interface {
	AddTask(Task)
	DelTask(Task)
	Stop()
}

type taskCtxType uint8

const taskCtx taskCtxType = 0

// GetTaskFromCtx returns the task from the context.
func GetTaskFromCtx(ctx context.Context) Task {
	if v := ctx.Value(taskCtx); v != nil {
		return v.(Task)
	}
	return nil
}

func setTaskToCtx(ctx context.Context, task Task) context.Context {
	return context.WithValue(ctx, taskCtx, task)
}

// CanRunVip returns a function to reports whether the task can be run
// based on vip.
//
// Return true if vip is empty.
func CanRunVip(vip string) func() bool {
	ip := net.ParseIP(strings.TrimSpace(vip))
	return func() bool {
		if isZero(ip) {
			return true
		}

		addrs, _ := net.InterfaceAddrs()
		for _, addr := range addrs {
			ipport := addr.String()
			if index := strings.IndexByte(ipport, '/'); index > 0 {
				ipport = ipport[:index]
			}

			if ip.Equal(net.ParseIP(ipport)) {
				return true
			}
		}

		return false
	}
}

func isZero(ip net.IP) bool {
	for _, b := range ip {
		if b != 0 {
			return false
		}
	}
	return true
}
