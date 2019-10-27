package service

import (
	"sync"
)

// SessionManager is used to manage the connection session.
//
// Notice: these methods must not panic.
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
func NewMemorySessionManager() SessionManager {
	return memorySessionManager{new(sync.Map)}
}

type memorySessionManager struct {
	endpoints *sync.Map
}

func (m memorySessionManager) GetEndpoint(addr string) Endpoint {
	if endpoint, ok := m.endpoints.Load(addr); ok {
		return endpoint.(Endpoint)
	}
	return nil
}

func (m memorySessionManager) SetEndpoint(addr string, endpoint Endpoint) {
	if addr == "" {
		panic("MemorySessionManager: the address must not be empty")
	} else if endpoint == nil {
		panic("MemorySessionManager: the endpoint must not be nil")
	}
	m.endpoints.Store(addr, endpoint)
}

func (m memorySessionManager) DelEndpoint(addr string) {
	m.endpoints.Delete(addr)
}
