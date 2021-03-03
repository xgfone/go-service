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
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

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

// RemoteAddrString implements the interface Request.
func (r HTTPRequest) RemoteAddrString() string { return r.req.RemoteAddr }

// SessionID implements the interface RequestSession.
func (r HTTPRequest) SessionID() string { return r.sid }

// Request returns the inner http.Request.
func (r HTTPRequest) Request() *http.Request { return r.req }

// HTTPEndpointConfig is used to configure the HTTP endpoint.
type HTTPEndpointConfig struct {
	Client   *http.Client
	Checker  HealthChecker
	UserData interface{}

	// Request is used to fix the request before sending the http request.
	Request func(orig *http.Request) (new *http.Request)

	// Response is used to fix the response after getting the http response.
	Response func(orig *http.Response, err error) (*http.Response, error)
}

// NewHTTPEndpoint returns a new HTTP endpoint, which only replaces the host
// of the original request with the host of "eurl" and adds the two headers
// "X-Forwarded-For" and "Origin".
//
// "eurl" is used to check whether the endpoint is healthy. By default,
// it only tests whether the tcp connection is dialed with the host part.
// But you can customize it.
func NewHTTPEndpoint(eurl string, conf *HTTPEndpointConfig) (Endpoint, error) {
	u, err := url.Parse(eurl)
	if err != nil {
		return nil, err
	} else if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid http url '%s'", eurl)
	}

	var c HTTPEndpointConfig
	if conf != nil {
		c = *conf
	}

	if c.Client == nil {
		c.Client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConns:          0,
				MaxIdleConnsPerHost:   100,
				IdleConnTimeout:       30 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			}}
	}

	if c.Checker == nil {
		c.Checker = CheckEndpointHealth(time.Second, time.Millisecond*10, 1)
	}

	return httpEndpoint{
		u:    u,
		url:  u.String(),
		host: u.Host,
		conf: c,
	}, nil
}

type httpEndpoint struct {
	u    *url.URL
	url  string
	host string
	conf HTTPEndpointConfig
}

func (e httpEndpoint) String() string {
	return e.url
}

func (e httpEndpoint) Type() string {
	return "http"
}

func (e httpEndpoint) UserData() interface{} {
	return e.conf.UserData
}

func (e httpEndpoint) MetaData() map[string]interface{} {
	return map[string]interface{}{"url": e.url}
}

func (e httpEndpoint) IsHealthy(c context.Context) bool {
	return e.conf.Checker(c, e.url) == nil
}

func (e httpEndpoint) RoundTrip(c context.Context, r Request) (Response, error) {
	req := r.(interface{ Request() *http.Request }).Request().WithContext(c)
	if req.URL.Scheme == "" {
		req.URL.Scheme = e.u.Scheme
	}

	req.Host = e.host
	req.URL.Host = e.host
	req.RequestURI = ""
	req.Header.Set("X-Forwarded-For", req.RemoteAddr)
	req.Header.Set("Origin", fmt.Sprintf("%s://%s", req.URL.Scheme, req.Host))

	if e.conf.Request != nil {
		req = e.conf.Request(req)
	}

	resp, err := e.conf.Client.Do(req)
	if e.conf.Response != nil {
		resp, err = e.conf.Response(resp, err)
	}

	return resp, err
}
