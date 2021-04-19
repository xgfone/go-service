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
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

func init() {
	if tp, ok := http.DefaultTransport.(*http.Transport); ok {
		tp.IdleConnTimeout = time.Second * 30
		tp.MaxIdleConnsPerHost = 100
		tp.MaxIdleConns = 0
	}
}

// HTTPRequest is a HTTP request.
type HTTPRequest struct {
	req *http.Request
	sid string
}

// NewHTTPRequest returns a new HTTPRequest.
//
// Notice: sessionID may be empty.
func NewHTTPRequest(r *http.Request, sessionID string) HTTPRequest {
	return HTTPRequest{req: r, sid: sessionID}
}

// SessionID implements the interface Request.
func (r HTTPRequest) SessionID() string { return r.sid }

// RemoteAddrString implements the interface Request.
func (r HTTPRequest) RemoteAddrString() string { return r.req.RemoteAddr }

// Request returns the inner http.Request.
func (r HTTPRequest) Request() *http.Request { return r.req }

type HTTPEndpointInfo struct {
	Scheme string      `json:"scheme,omitempty" xml:"scheme,omitempty"`
	Method string      `json:"method,omitempty" xml:"method,omitempty"`
	Host   string      `json:"host,omitempty" xml:"host,omitempty"`
	Path   string      `json:"path,omitempty" xml:"path,omitempty"`
	Query  url.Values  `json:"query,omitempty" xml:"query,omitempty"`
	Header http.Header `json:"header,omitempty" xml:"header,omitempty"`
}

// Validate reports whether the fields are valid if they are not empty.
func (i HTTPEndpointInfo) Validate() error {
	switch i.Scheme {
	case "", "http", "https":
	default:
		return fmt.Errorf("invalid http scheme '%s'", i.Scheme)
	}

	switch i.Method {
	case "",
		http.MethodGet, http.MethodPut, http.MethodHead, http.MethodPost,
		http.MethodPatch, http.MethodDelete, http.MethodConnect,
		http.MethodOptions, http.MethodTrace:
	default:
		return fmt.Errorf("invalid http method '%s'", i.Method)
	}

	return nil
}

// HTTPEndpointConfig is used to configure the HTTP endpoint.
type HTTPEndpointConfig struct {
	// ServerID is set the response header "X-Server-Id".
	//
	// Default: hex.EncodeToString([]byte(addr)).
	ServerID string

	// Info is the additional optional information of the endpoint
	// to forward the request.
	//
	// If Scheme is empty, it's "http" by default.
	// For others, use the corresponding information from the origin request.
	Info HTTPEndpointInfo

	// Client is used to send the http request to forward it.
	//
	// Default: http.DefaultClient
	Client *http.Client

	// If true, the implementation will append the header "X-Forwarded-For".
	//
	// Default: false
	XForwardedFor bool

	// Handler is used to allow the user to customize the http request.
	//
	// Default: client.Do(httpReq)
	Handler func(origReq Request, client *http.Client, httpReq *http.Request) (*http.Response, error)
}

func (c *HTTPEndpointConfig) init(addr string) (hostport string, err error) {
	if err = c.Info.Validate(); err != nil {
		return "", err
	}
	if c.Info.Scheme == "" {
		c.Info.Scheme = "http"
	}
	if c.Client == nil {
		client := *http.DefaultClient
		client.Transport = http.DefaultTransport
		c.Client = &client
	}

	hostport = addr
	if host, port := splitHostPort(hostport); host == "" {
		return "", fmt.Errorf("invalid http endpoint address '%s'", addr)
	} else if port == "" {
		if c.Info.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
		hostport = net.JoinHostPort(host, port)
	}

	if c.ServerID == "" {
		c.ServerID = hex.EncodeToString([]byte(hostport))
	}

	return
}

// NewHTTPEndpoint returns a new HTTP endpoint with the type "http".
//
// addr is the format HOST[:PORT] and is the id of the endpoint.
func NewHTTPEndpoint(addr string, conf *HTTPEndpointConfig) (Endpoint, error) {
	var c HTTPEndpointConfig
	if conf != nil {
		c = *conf
	}

	epid := addr
	addr, err := c.init(addr)
	if err != nil {
		return nil, err
	}
	return &httpEndpoint{conf: c, addr: addr, epid: epid}, nil
}

type httpEndpoint struct {
	state ConnectionState
	conf  HTTPEndpointConfig
	addr  string
	epid  string
}

func (e *httpEndpoint) ID() string           { return e.epid }
func (e *httpEndpoint) Type() string         { return "http" }
func (e *httpEndpoint) State() EndpointState { return e.state.ToEndpointState() }
func (e *httpEndpoint) MetaData() map[string]interface{} {
	return map[string]interface{}{"http": e.conf.Info, "addr": e.addr}
}

func (e *httpEndpoint) RoundTrip(c context.Context, r Request) (interface{}, error) {
	e.state.Inc()
	defer e.state.Dec()

	req := r.(interface{ Request() *http.Request }).Request().WithContext(c)
	if e.conf.XForwardedFor && req.RemoteAddr != "" {
		if host, _, _ := net.SplitHostPort(req.RemoteAddr); host != "" {
			if forwards := req.Header["X-Forwarded-For"]; len(forwards) == 0 {
				req.Header["X-Forwarded-For"] = []string{host}
			} else {
				req.Header["X-Forwarded-For"] = append(forwards, host)
			}
		}
	}

	req.URL.Scheme = e.conf.Info.Scheme
	if e.conf.Info.Method != "" {
		req.Method = e.conf.Info.Method
	}

	if e.conf.Info.Path != "" {
		req.URL.Path = e.conf.Info.Path
	}

	if len(e.conf.Info.Query) != 0 {
		if req.URL.RawQuery == "" {
			req.URL.RawQuery = e.conf.Info.Query.Encode()
		} else if values := req.URL.Query(); len(values) == 0 {
			req.URL.RawQuery = e.conf.Info.Query.Encode()
		} else {
			for k, vs := range e.conf.Info.Query {
				values[k] = vs
			}
			req.URL.RawQuery = values.Encode()
		}
	}

	if len(e.conf.Info.Header) != 0 {
		if len(req.Header) == 0 {
			req.Header = e.conf.Info.Header
		} else {
			for k, vs := range e.conf.Info.Header {
				req.Header[k] = vs
			}
		}
	}

	if e.conf.Info.Host != "" {
		req.Host = e.conf.Info.Host // Set the header "Host"
	}
	req.URL.Host = e.addr // Dial to the endpoint
	req.RequestURI = ""   // Pretend to be a client request.

	var err error
	var resp *http.Response
	if e.conf.Handler == nil {
		resp, err = e.conf.Client.Do(req)
	} else {
		resp, err = e.conf.Handler(r, e.conf.Client, req)
	}

	if resp != nil {
		resp.Header.Set("X-Server-Id", e.conf.ServerID)
	}
	return resp, err
}
