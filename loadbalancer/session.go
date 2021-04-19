// Copyright 2021 xgfone
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
	"io"
	"sync"
	"time"
)

// Session is a session to manage the connection session.
//
// Notice: these methods must be thread-safe and not panic.
type Session interface {
	// Release all the cache resources.
	io.Closer

	// URL returns the url representation of the session, which represents
	// the unique session, such as "memory://", "redis://pass@ip:port/num", etc.
	URL() string

	// GetEndpoint returns the Endpoint by the session id.
	//
	// Return nil if the endpoint does not exist.
	GetEndpoint(sid string) Endpoint

	// SetEndpoint sets the Endpoint with the session id and timeout.
	//
	// If timeout is not equal to 0, which should delete the session
	// after the timeout has elapsed since the endpoint was set.
	SetEndpoint(sid string, endpoint Endpoint, timeout time.Duration)

	// DelEndpoint deletes the endpoint from the session.
	DelEndpoint(sid string)
}

// NewNoopSession returns a new no-op Session with the url "noop://".
func NewNoopSession() Session {
	return noopSession{}
}

type noopSession struct{}

func (m noopSession) Close() error                                { return nil }
func (m noopSession) URL() string                                 { return "noop://" }
func (m noopSession) GetEndpoint(string) Endpoint                 { return nil }
func (m noopSession) SetEndpoint(string, Endpoint, time.Duration) {}
func (m noopSession) DelEndpoint(string)                          {}

// NewMemorySession returns the simple Session based on memory with the url
// "memory://"
//
// tick is the clock tick to clean the expired session cache.
func NewMemorySession(tick time.Duration) Session {
	if tick < 0 {
		panic("MemorySession: tick must not be less than 0")
	} else if tick == 0 {
		tick = time.Minute
	}

	sm := &memorySession{
		exit: make(chan struct{}),
		maps: make(map[string]msmEndpoint),
	}

	go sm.clean(tick)
	return sm
}

type msmEndpoint struct {
	Endpoint Endpoint
	Timeout  time.Duration
	Last     time.Time
}

type memorySession struct {
	exit chan struct{}
	lock sync.RWMutex
	maps map[string]msmEndpoint
}

func (m *memorySession) clean(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.exit:
			m.lock.Lock()
			m.maps = nil
			m.lock.Unlock()
			return
		case now := <-ticker.C:
			m.lock.Lock()
			for sid, mep := range m.maps {
				if now.Sub(mep.Last) > mep.Timeout {
					delete(m.maps, sid)
				}
			}
			m.lock.Unlock()
		}
	}
}

func (m *memorySession) URL() string  { return "memory://" }
func (m *memorySession) Close() error { close(m.exit); return nil }

func (m *memorySession) GetEndpoint(sid string) (ep Endpoint) {
	m.lock.RLock()
	if mep, ok := m.maps[sid]; ok {
		if time.Since(mep.Last) < mep.Timeout {
			ep = mep.Endpoint
		} else {
			delete(m.maps, sid)
		}
	}
	m.lock.RUnlock()
	return
}

func (m *memorySession) SetEndpoint(sid string, ep Endpoint, timeout time.Duration) {
	m.lock.Lock()
	m.maps[sid] = msmEndpoint{Endpoint: ep, Timeout: timeout, Last: time.Now()}
	m.lock.Unlock()
}

func (m *memorySession) DelEndpoint(sid string) {
	m.lock.Lock()
	delete(m.maps, sid)
	m.lock.Unlock()
}

// SessionProxy is a session proxy, which can swap the session to a different
// thread-safely.
type SessionProxy struct {
	lock    sync.RWMutex
	session Session
}

// NewSessionProxy returns a new session proxy with the session.
func NewSessionProxy(session Session) *SessionProxy {
	if session == nil {
		panic("SessionProxy: session is nil")
	}
	return &SessionProxy{session: session}
}

// SwapSession swaps the inner session with the new if they are equal.
// Or do nothing.
func (sp *SessionProxy) SwapSession(new Session) (old Session, ok bool) {
	url := new.URL()
	sp.lock.Lock()
	if sp.session.URL() != url {
		old, sp.session, ok = sp.session, new, true
	}
	sp.lock.Unlock()
	return
}

// GetSession returns the inner session.
func (sp *SessionProxy) GetSession() (session Session) {
	sp.lock.RLock()
	session = sp.session
	sp.lock.RUnlock()
	return
}

// Close implements the interface Session.
func (sp *SessionProxy) Close() error { return sp.GetSession().Close() }

// URL implements the interface Session.
func (sp *SessionProxy) URL() string { return sp.GetSession().URL() }

// GetEndpoint implements the interface Session.
func (sp *SessionProxy) GetEndpoint(sid string) Endpoint {
	return sp.GetSession().GetEndpoint(sid)
}

// SetEndpoint implements the interface Session.
func (sp *SessionProxy) SetEndpoint(sid string, ep Endpoint, timeout time.Duration) {
	sp.GetSession().SetEndpoint(sid, ep, timeout)
}

// DelEndpoint implements the interface Session.
func (sp *SessionProxy) DelEndpoint(sid string) { sp.GetSession().DelEndpoint(sid) }
