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
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var _ Provider = &SessionProvider{}
var _ EndpointManager = &SessionProvider{}
var _ SelectorManager = &SessionProvider{}
var _ SessionManager = &SessionProvider{}

// SessionProvider returns a provider with the session.
type SessionProvider struct {
	generalProvider

	lock    sync.RWMutex
	session Session
	timeout int64
}

// NewSessionProvider returns a new SessionProvider, which has implemented
// the interface Provider, EndpointManager, SelectorManager and SessionManager.
//
// By default, s is RoundRobinSelector(), sm is NewNoopSession(),
// and sessionTimeout is time.Minute.
func NewSessionProvider(s Selector, session Session, sessionTimeout time.Duration) *SessionProvider {
	if s == nil {
		s = RoundRobinSelector()
	}
	if session == nil {
		session = NewNoopSession()
	}
	if sessionTimeout <= 0 {
		sessionTimeout = time.Minute
	}

	return &SessionProvider{
		generalProvider: generalProvider{selector: s},
		timeout:         int64(sessionTimeout),
		session:         session,
	}
}

// GetSession implements the interface SessionManager.
func (p *SessionProvider) GetSession() (session Session) {
	p.lock.RLock()
	session = p.session
	p.lock.RUnlock()
	return
}

// SetSession implements the interface SessionManager.
func (p *SessionProvider) SetSession(new Session) (old Session) {
	if new == nil {
		panic("SessionProvider.SetSession: Session must not be nil")
	}

	p.lock.Lock()
	if p.session.String() != new.String() {
		old, p.session = p.session, new
	}
	p.lock.Unlock()
	return
}

// GetSessionTimeout returns the inner session timeout.
func (p *SessionProvider) GetSessionTimeout() time.Duration {
	return time.Duration(atomic.LoadInt64(&p.timeout))
}

// SetSessionTimeout sets the sesstion timeout.
func (p *SessionProvider) SetSessionTimeout(timeout time.Duration) {
	if timeout <= 0 {
		panic("SessionProvider.SetSessionTimeout: timeout must be greater than 0")
	}

	atomic.StoreInt64(&p.timeout, int64(timeout))
}

// DeleteSession deletes the session by the session id.
func (p *SessionProvider) DeleteSession(sid string) {
	if sid != "" {
		p.deleteSession(sid)
	}
}

func (p *SessionProvider) deleteSession(sid string) {
	p.lock.RLock()
	session := p.session
	p.lock.RUnlock()
	session.DelEndpoint(sid)
}

// Close reimplements the interface io.Closer.
func (p *SessionProvider) Close() error {
	p.generalProvider.Close()
	return p.GetSession().Close()
}

// String reimplements the interface fmt.Stringer.
func (p *SessionProvider) String() string {
	p.lock.RLock()
	session := p.session.String()
	p.lock.RUnlock()
	return fmt.Sprintf("SessionProvider(session=%s, strategy=%s)", session, p.Strategy())
}

// Finish reimplements the interface Provider@Finish.
func (p *SessionProvider) Finish(req Request, err error) {
	if err != nil {
		if sid := req.SessionID(); sid == "" {
			p.DeleteSession(req.RemoteAddrString())
		} else {
			p.deleteSession(sid)
		}
	}
	p.generalProvider.Finish(req, err)
}

// Select reimplements the interface Provider@Select.
func (p *SessionProvider) Select(req Request, new bool) (ep Endpoint) {
	sid := req.SessionID()
	if sid == "" {
		if sid = req.RemoteAddrString(); sid == "" {
			return p.generalProvider.Select(req, new)
		}
	}

	p.lock.RLock()
	session := p.session
	p.lock.RUnlock()

	if new {
		if ep = session.GetEndpoint(sid); ep == nil { // No session cache
			if ep = p.generalProvider.Select(req, new); ep != nil {
				session.SetEndpoint(sid, ep, p.GetSessionTimeout())
			}
		} else if p.IsActive(ep) { // Got from session cache
			return
		} else if ep = p.generalProvider.Select(req, new); ep == nil {
			session.DelEndpoint(sid) // Delete the expired session
		} else {
			session.SetEndpoint(sid, ep, p.GetSessionTimeout()) // Reset the expired session
		}
	} else {
		if ep = p.generalProvider.Select(req, new); ep == nil {
			session.DelEndpoint(sid) // Delete the expired session
		} else {
			session.SetEndpoint(sid, ep, p.GetSessionTimeout()) // Reset the expired session
		}
	}

	return
}
