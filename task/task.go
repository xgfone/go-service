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
	"fmt"
	"time"
)

// Task represents a task.
type Task interface {
	Name() string
	Run(ctx context.Context, now time.Time) error
}

type task struct {
	name string
	run  func(context.Context, time.Time) error
}

// NewTask returns a new Task.
func NewTask(name string, run func(context.Context, time.Time) error) Task {
	return task{name: name, run: run}
}

func (t task) Run(c context.Context, n time.Time) error { return t.run(c, n) }
func (t task) Name() string                             { return t.name }
func (t task) String() string {
	return fmt.Sprintf("Task(name=%s)", t.name)
}
