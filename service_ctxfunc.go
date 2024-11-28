// Copyright 2024 xgfone
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

import "context"

// ContextFunc returns a new Service based on a context function.
func ContextFunc(f func(ctx context.Context)) Service {
	if f == nil {
		panic("service.ContextFunc: func f must not be nil")
	}
	return Lock(&single{run: f})
}

type single struct {
	context context.Context
	cancel  context.CancelFunc
	run     func(context.Context)
}

func (s *single) Activate() {
	if s.context == nil {
		s.context, s.cancel = context.WithCancel(context.Background())
		go s.run(s.context)
	}
}

func (s *single) Deactivate() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
		s.context = nil
	}
}
