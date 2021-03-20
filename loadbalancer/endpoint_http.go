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
	"strings"
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

// HTTPEndpointHealthCheckerFunc is used to check the health of the endpoint.
type HTTPEndpointHealthCheckerFunc func(c context.Context, addr string, info HTTPEndpointInfo) bool

// HTTPEndpointHealthChecker is equal to
//   HTTPEndpointHealthCheckerWithConfig(&HTTPEndpointHealthCheckerConfig{
//       Client: client,
//       Info: info,
//   })
func HTTPEndpointHealthChecker(client *http.Client, info HTTPEndpointInfo) (
	HTTPEndpointHealthCheckerFunc, error) {
	return HTTPEndpointHealthCheckerWithConfig(&HTTPEndpointHealthCheckerConfig{
		Client: client, Info: info,
	})
}

// HTTPStatusCodeRange is the range of the http status code,
// which is semi-closure, that's, [Begin, End).
type HTTPStatusCodeRange struct {
	Begin int `json:"begin" xml:"begin"`
	End   int `json:"end" xml:"end"`
}

// HTTPEndpointHealthCheckerConfig is used to configure the health checker
// of the http endpoint.
type HTTPEndpointHealthCheckerConfig struct {
	// Default: http.DefaultClient
	Client *http.Client

	// Default: [{Begin: 0, End: 400}]
	Codes []HTTPStatusCodeRange

	// Info is the information to check the health of the http endpoint.
	//
	// Default:
	//   Scheme: "http"
	//   Method: "GET"
	//   Path:   "/"
	Info HTTPEndpointInfo
}

// HTTPEndpointHealthCheckerWithConfig returns a health checker,
// which builds and sends a url to check the status code is in the given range.
func HTTPEndpointHealthCheckerWithConfig(c *HTTPEndpointHealthCheckerConfig) (
	HTTPEndpointHealthCheckerFunc, error) {
	var conf HTTPEndpointHealthCheckerConfig
	if c != nil {
		conf = *c
	}

	if err := conf.Info.Validate(); err != nil {
		return nil, err
	}

	if conf.Info.Method == "" {
		conf.Info.Method = http.MethodGet
	}

	if conf.Info.Scheme == "" {
		conf.Info.Scheme = "http"
	}

	if conf.Info.Path == "" {
		conf.Info.Path = "/"
	}

	if len(conf.Codes) == 0 {
		conf.Codes = []HTTPStatusCodeRange{{End: 400}}
	}

	return func(c context.Context, addr string, _ HTTPEndpointInfo) bool {
		url := fmt.Sprintf("%s://%s%s", conf.Info.Scheme, addr, conf.Info.Path)
		req, err := http.NewRequestWithContext(c, conf.Info.Method, url, nil)
		if err != nil {
			return false
		}

		if conf.Info.Hostname != "" {
			req.Host = conf.Info.Hostname
		}
		if len(conf.Info.Query) != 0 {
			req.URL.RawQuery = conf.Info.Query.Encode()
		}
		if len(conf.Info.Header) != 0 {
			if len(req.Header) == 0 {
				req.Header = conf.Info.Header
			} else {
				for k, vs := range conf.Info.Header {
					req.Header[k] = vs
				}
			}
		}

		var resp *http.Response
		if conf.Client == nil {
			resp, err = http.DefaultClient.Do(req)
		} else {
			resp, err = conf.Client.Do(req)
		}

		if err != nil {
			return false
		}
		resp.Body.Close()

		for _, code := range conf.Codes {
			if code.Begin <= resp.StatusCode && resp.StatusCode < code.End {
				return true
			}
		}

		return false
	}, nil
}

type HTTPEndpointInfo struct {
	// If Scheme is set, it should be one of "http" or "https".
	Scheme   string      `json:"scheme,omitempty" xml:"scheme,omitempty"`
	Method   string      `json:"method,omitempty" xml:"method,omitempty"`
	Hostname string      `json:"hostname,omitempty" xml:"hostname,omitempty"`
	Path     string      `json:"path,omitempty" xml:"path,omitempty"`
	Query    url.Values  `json:"query,omitempty" xml:"query,omitempty"`
	Header   http.Header `json:"header,omitempty" xml:"header,omitempty"`
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
	// ID is the id of the endpoint, which will be set as the response header
	// "X-Server-ID".
	//
	// Default: hex.EncodeToString([]byte(addr)).
	ID string

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

	// Checker is used to check whether the endpoint is healthy.
	//
	// Default:
	//   HTTPEndpointHealthChecker(HTTPEndpointInfo{
	//       Scheme: HTTPEndpointConfig.Info.Scheme,
	//       Hostname: HTTPEndpointConfig.Info.Hostname,
	//   })
	//
	Checker HTTPEndpointHealthCheckerFunc

	// Handler is used to allow the user to customize the http request.
	//
	// Default: client.Do(httpReq)
	Handler func(origReq Request, client *http.Client, httpReq *http.Request) (*http.Response, error)
}

func (c *HTTPEndpointConfig) init() (err error) {
	if err = c.Info.Validate(); err != nil {
		return err
	}

	if c.Info.Scheme == "" {
		c.Info.Scheme = "http"
	}

	if c.Client == nil {
		client := *http.DefaultClient
		client.Transport = http.DefaultTransport
		c.Client = &client
	}

	if c.Checker == nil {
		info := HTTPEndpointInfo{Scheme: c.Info.Scheme, Hostname: c.Info.Hostname}
		c.Checker, err = HTTPEndpointHealthChecker(c.Client, info)
	}

	return
}

// NewHTTPEndpoint returns a new HTTP endpoint.
//
// HTTPEndpointInfo Default:
//   Scheme: "http"
//   Checker: HTTPEndpointHealthChecker(conf.Client, HTTPEndpointInfo{
//                Scheme: conf.Info.Scheme,
//                Hostname: conf.Info.Hostname,
//            })
//
func NewHTTPEndpoint(addr string, conf *HTTPEndpointConfig) (Endpoint, error) {
	var c HTTPEndpointConfig
	if conf != nil {
		c = *conf
	}

	if err := c.init(); err != nil {
		return nil, err
	}

	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		if !strings.HasPrefix(err.Error(), "missing port") {
			return nil, err
		}

		if c.Info.Scheme == "http" {
			port = "80"
		} else {
			port = "443"
		}
		addr = net.JoinHostPort(addr, port)
	}

	desc := addr
	if c.Info.Hostname != "" {
		desc = strings.Join([]string{c.Info.Hostname, addr}, "@")
	}

	if c.ID == "" {
		c.ID = hex.EncodeToString([]byte(addr))
	}

	return &httpEndpoint{conf: c, desc: desc, addr: addr, port: port}, nil
}

type httpEndpoint struct {
	state ConnectionState
	conf  HTTPEndpointConfig
	desc  string
	addr  string
	port  string
	svrid string
}

func (e *httpEndpoint) Type() string         { return "http" }
func (e *httpEndpoint) String() string       { return e.desc }
func (e *httpEndpoint) State() EndpointState { return e.state.ToEndpointState() }
func (e *httpEndpoint) IsHealthy(c context.Context) bool {
	return e.conf.Checker(c, e.addr, e.conf.Info)
}
func (e *httpEndpoint) MetaData() map[string]interface{} {
	return map[string]interface{}{
		"id":   e.conf.ID,
		"info": e.conf.Info,
		"addr": e.addr,
	}
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

	if e.conf.Info.Method != "" {
		req.Method = e.conf.Info.Method
	}
	if req.URL.Scheme == "" {
		req.URL.Scheme = e.conf.Info.Scheme
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

	if e.conf.Info.Hostname != "" {
		req.Host = e.conf.Info.Hostname // Set the header "Host"
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
		resp.Header.Set("X-Server-ID", e.svrid)
	}
	return resp, err
}
