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
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/xgfone/go-service/httputil"
)

type endpointCtxKey uint8

const httpClientCtxKey endpointCtxKey = 0

// GetHTTPClientFromContext gets and returns *http.Client from the context.
//
// Return nil if the http.Client does not exist.
func GetHTTPClientFromContext(ctx context.Context) *http.Client {
	if v := ctx.Value(httpClientCtxKey); v != nil {
		return v.(*http.Client)
	}
	return nil
}

// SetHTTPClientToContext sets *http.Client into Context.
func SetHTTPClientToContext(ctx context.Context, client *http.Client) context.Context {
	return context.WithValue(ctx, httpClientCtxKey, client)
}

// NewHTTPEndpoint returns a HTTP Endpoint.
//
// If the checker is missing, it is CheckEndpointHealth(time.Second, time.Second, 1)
// by default. And for the health check, the http client is stored
// in the context.
//
// The HTTP endpoint has implemented the interface { URL() *url.URL },
// and if addr is a URL, it will be parsed earlier and returned by URL().
// Or, URL() returns nil.
//
// Notice: the request must implement
//   interface{
//       ToHTTPRequest(context.Context, Endpoint) (*http.Request, error)
//   }
//
// If the request has also implemented the http.RoundTripper interface,
// the RoundTripper will be used to send the request instead of http.Client.
//
// If the request has also implemented the interface
//   interface{
//       FromHTTPResponse(*http.Response) (Response, error)
//   }
// it will call ToHTTPResponse to convert *http.Response to the response.
// Or use *http.Response as the Response.
func NewHTTPEndpoint(addr string, client *http.Client, checker ...HealthChecker) Endpoint {
	_checker := CheckEndpointHealth(time.Second, time.Second, 1)
	if len(checker) > 0 && checker[0] != nil {
		_checker = checker[0]
	}

	_url, _ := url.Parse(addr)
	return httpEndpoint{url: _url, addr: addr, client: client, checker: _checker}
}

type httpEndpoint struct {
	url     *url.URL
	addr    string
	client  *http.Client
	checker HealthChecker
}

func (h httpEndpoint) URL() *url.URL {
	return h.url
}

func (h httpEndpoint) String() string {
	return h.addr
}

func (h httpEndpoint) IsHealthy(ctx context.Context) bool {
	client := h.client
	if client == nil {
		client = http.DefaultClient
	}
	return h.checker(SetHTTPClientToContext(ctx, client), h)
}

func (h httpEndpoint) RoundTrip(ctx context.Context, req Request) (resp Response, err error) {
	hreq, err := req.(interface {
		ToHTTPRequest(context.Context, Endpoint) (*http.Request, error)
	}).ToHTTPRequest(ctx, h)
	if err != nil {
		return nil, err
	}

	var hresp *http.Response
	if rt, ok := req.(http.RoundTripper); ok {
		hresp, err = rt.RoundTrip(hreq)
	} else {
		client := h.client
		if client == nil {
			client = http.DefaultClient
		}

		hresp, err = client.Do(hreq)
	}

	if err == nil {
		if toResp, ok := req.(interface {
			FromHTTPResponse(*http.Response) (Response, error)
		}); ok {
			if resp, err = toResp.FromHTTPResponse(hresp); err != nil {
				hresp.Body.Close()
			}
		} else {
			resp = hresp
		}
	}

	return
}

// HTTPRequest wraps the http.Request to Request.
type httpRequest struct {
	*http.Request
}

// NewHTTPRequest returns a new HTTP Request, which implements the interface
//   interface {
//       ToHTTPRequest(context.Context, Endpoint) (*http.Request, error)
//   }
// it will replace the URL or Host of the http request with the endpoint:
//
//   1. If the endpoint has implemented the interface { URL() *url.URL },
//      it will replace the URL of the request with endpoint.URL().
//   2. If the endpoint starts with "http" or "https", it will be parsed
//      by url.Parse(endpoint.String()) to replace the URL of the request.
//   3. Or, it will replace the Host of the request with endpoint.String().
//
func NewHTTPRequest(req *http.Request) Request { return httpRequest{req} }
func (r httpRequest) RemoteAddrString() string { return r.RemoteAddr }
func (r httpRequest) ToHTTPRequest(ctx context.Context, ep Endpoint) (*http.Request, error) {
	if urlf, ok := ep.(interface{ URL() *url.URL }); ok {
		if _url := urlf.URL(); _url != nil {
			_url.Host = strings.TrimSuffix(_url.Host, ":")
			r.Request.URL = _url
			r.Request.Host = _url.Host
			return r.Request, nil
		}
	}

	if eps := ep.String(); strings.HasPrefix(eps, "http") {
		_url, err := url.Parse(eps)
		if err != nil {
			return nil, err
		}
		_url.Host = strings.TrimSuffix(_url.Host, ":")
		r.Request.URL = _url
		r.Request.Host = _url.Host
	} else {
		eps = strings.TrimSuffix(eps, ":")
		r.Request.Host = eps
		r.Request.URL.Host = eps
	}

	if _, ok := ctx.Deadline(); (ok || ctx.Done() != nil) && r.Context() != ctx {
		r.Request = r.Request.WithContext(ctx)
	}

	return r.Request, nil
}

// ToHTTPRoundTripper returns a HTTP RoundTripper, which will use LoadBalance
// RoundTripper that is obtained by req.URL.Host to send the http request,
// that's, it replaces the URL or Host of the request with the HTTP endpoint
// by NewHTTPRequest and send the request by the HTTP endpoint
//
// If no the corresponding LoadBalance RoundTripper, it will use
// defaultHTTPRoundTripper instead, which is http.DefaultClient.Transport
// or http.DefaultTransport.
func ToHTTPRoundTripper(getRoundTripper GetRoundTripper,
	defaultHTTPRoundTripper ...http.RoundTripper) http.RoundTripper {
	var hrt http.RoundTripper
	if len(defaultHTTPRoundTripper) > 0 && defaultHTTPRoundTripper[0] != nil {
		hrt = defaultHTTPRoundTripper[0]
	} else if http.DefaultClient.Transport != nil {
		hrt = http.DefaultClient.Transport
	} else {
		hrt = http.DefaultTransport
	}

	return httputil.RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if rt := getRoundTripper(r.URL.Host); rt != nil {
			resp, err := rt.RoundTrip(r.Context(), NewHTTPRequest(r))
			if err != nil {
				return nil, err
			}
			return resp.(*http.Response), nil
		}
		return hrt.RoundTrip(r)
	})
}
