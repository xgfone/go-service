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

package loadbalancer

import (
	"sync"
)

// SessionManager is used to manage the connection session.
//
// Notice: these methods must be thread-safe and not panic.
type SessionManager interface {
	// GetEndpoint returns the Endpoint by the request address.
	//
	// Return nil if the endpoint does not exist.
	GetEndpoint(addr string) Endpoint

	// SetEndpoint sets the Endpoint with the request address.
	SetEndpoint(addr string, endpoint Endpoint)

	// DelEndpoint deletes the endpoint from the session manager.
	DelEndpoint(addr string)
}

// NewMemorySessionManager returns a new SessionManager based on the memory.
func NewMemorySessionManager() SessionManager { return &memorySessionManager{} }

type memorySessionManager struct {
	endpoints sync.Map
}

func (m *memorySessionManager) GetEndpoint(addr string) Endpoint {
	if endpoint, ok := m.endpoints.Load(addr); ok {
		return endpoint.(Endpoint)
	}
	return nil
}

func (m *memorySessionManager) SetEndpoint(addr string, endpoint Endpoint) {
	if addr == "" {
		panic("MemorySessionManager: the address must not be empty")
	} else if endpoint == nil {
		panic("MemorySessionManager: the endpoint must not be nil")
	}
	m.endpoints.Store(addr, endpoint)
}

func (m *memorySessionManager) DelEndpoint(addr string) {
	m.endpoints.Delete(addr)
}
