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
	"time"
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
// If the checker is missing, it is CheckEndpointHealth(time.Second) by default.
// And for the health check, the http client is stored in the context.
//
// Notice: the request must implement
//   interface{
//       ToHTTPRequest(context.Context, Endpoint) (*http.Request, error)
//   }
// If it has implemented the interface
//   interface{
//       FromHTTPResponse(*http.Response) (Response, error)
//   }
// it will call ToHTTPResponse to convert *http.Response to the response.
// Or use *http.Response as the Response.
func NewHTTPEndpoint(addr string, client *http.Client, checker ...HealthChecker) Endpoint {
	_checker := CheckEndpointHealth(time.Second)
	if len(checker) > 0 && checker[0] != nil {
		_checker = checker[0]
	}
	return httpEndpoint{addr: addr, client: client, checker: _checker}
}

type httpEndpoint struct {
	addr    string
	client  *http.Client
	checker HealthChecker
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

	client := h.client
	if client == nil {
		client = http.DefaultClient
	}

	hresp, err := client.Do(hreq)
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
