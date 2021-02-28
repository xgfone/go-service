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
	"context"
	"net"
	"time"
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

// DialFunc is used to dial to open a connection to the address.
type DialFunc func(ctx context.Context, address string) (net.Conn, error)
