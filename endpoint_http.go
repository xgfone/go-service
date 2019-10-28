package service

import (
	"context"
	"net"
	"net/http"
	"strings"
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

// CheckHTTPEndpointHealth check the health status of the endpoint.
//
// If the endpoint is an address, it will use the detection of TCP port.
// Or it will retry to open the url by the GET method.
func CheckHTTPEndpointHealth(ctx context.Context, endpoint Endpoint) bool {
	addr := endpoint.String()

	client := GetHTTPClientFromContext(ctx)
	if strings.HasPrefix(addr, "http") {
		req, err := http.NewRequest(http.MethodGet, addr, nil)
		if err != nil {
			return false
		}
		req = req.WithContext(ctx)

		resp, err := client.Do(req)
		if err != nil {
			return false
		}

		resp.Body.Close()
		return true
	}

	timeout := time.Second
	if client != nil && client.Timeout > 0 {
		timeout = client.Timeout
	}

	if conn, err := net.DialTimeout("tcp", addr, timeout); err == nil {
		conn.Close()
		return true
	}
	return false
}

// NewHTTPEndpoint returns a HTTP Endpoint.
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
	_checker := CheckHTTPEndpointHealth
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
