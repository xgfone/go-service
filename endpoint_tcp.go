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
	"io"
	"net"
	"time"

	"github.com/xgfone/vitess-go/pools"
)

// SetTimeout sets the timeout of the connection if timeout is not the ZERO.
func SetTimeout(conn net.Conn, timeout time.Duration) (err error) {
	if timeout > 0 {
		err = conn.SetDeadline(time.Now().Add(timeout))
	}
	return
}

// SetReadTimeout sets the read timeout of the connection if timeout is not
// the ZERO.
func SetReadTimeout(conn net.Conn, timeout time.Duration) (err error) {
	if timeout > 0 {
		err = conn.SetReadDeadline(time.Now().Add(timeout))
	}
	return
}

// SetWriteTimeout sets the write timeout of the connection if timeout is not
// the ZERO.
func SetWriteTimeout(conn net.Conn, timeout time.Duration) (err error) {
	if timeout > 0 {
		err = conn.SetWriteDeadline(time.Now().Add(timeout))
	}
	return
}

// TCPRequest represents the TCP request.
type TCPRequest interface {
	Request

	SendRequest(conn *net.TCPConn) (err error)
	ReadResponse(conn *net.TCPConn) (resp Response, err error)
}

// NewTCPRequest returns a TCPRequest.
func NewTCPRequest(remoteAddr string, sendReq func(*net.TCPConn) error,
	readResp func(*net.TCPConn) (Response, error)) TCPRequest {
	return tcpRequest{RemoteAddr: remoteAddr, SendReq: sendReq, ReadResp: readResp}
}

type tcpRequest struct {
	RemoteAddr string
	SendReq    func(*net.TCPConn) error
	ReadResp   func(*net.TCPConn) (Response, error)
}

func (r tcpRequest) RemoteAddrString() string {
	return r.RemoteAddr
}

func (r tcpRequest) SendRequest(conn *net.TCPConn) (err error) {
	return r.SendReq(conn)
}

func (r tcpRequest) ReadResponse(conn *net.TCPConn) (resp Response, err error) {
	return r.ReadResp(conn)
}

// DialFunc is used to represent a dialer.
type DialFunc func(ctx context.Context, address string) (net.Conn, error)

// Closer is a wrapper of io.Closer.
type Closer struct{ io.Closer }

// Close closes the closer, but returns nothing.
func (c Closer) Close() { c.Closer.Close() }

type tcpEndpoint struct {
	addr string
	dial DialFunc
	pool *pools.ResourcePool
}

// NewTCPEndpoint returns a new TCP endpoint.
//
// Notice: the TCP request must implement the interface TCPRequest.
func NewTCPEndpoint(addr string, dial DialFunc, maxConcurrency int, idleTimeout time.Duration) Endpoint {
	ep := &tcpEndpoint{addr: addr, dial: dial}
	pool := pools.NewResourcePool(ep.factory, maxConcurrency, maxConcurrency, idleTimeout, 0)
	ep.pool = pool
	return ep
}

func (e *tcpEndpoint) factory() (r pools.Resource, err error) {
	conn, err := e.dial(context.Background(), e.addr)
	if err == nil {
		r = Closer{conn}
	}
	return
}

func (e *tcpEndpoint) String() string {
	return e.addr
}

func (e *tcpEndpoint) IsHealthy(ctx context.Context) bool {
	if conn, _ := e.dial(ctx, e.addr); conn != nil {
		conn.Close()
		return true
	}
	return false
}

func (e *tcpEndpoint) RoundTrip(ctx context.Context, req Request) (resp Response, err error) {
	res, err := e.pool.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get the connection: %s", err)
	}
	defer func() { e.pool.Put(res) }()

	conn := res.(Closer).Closer.(*net.TCPConn)
	if err = req.(TCPRequest).SendRequest(conn); err != nil {
		conn.Close()
		res = nil
		return
	}

	if resp, err = req.(TCPRequest).ReadResponse(conn); err != nil {
		conn.Close()
		res = nil
	}

	return
}
