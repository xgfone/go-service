package service

import (
	"context"
	"net/http"
)

// NewHTTPEndpoint returns a HTTP Endpoint.
//
// Notice: the request must implement
//   interface{
//       ToHTTPRequest(Endpoint) (*http.Request, error)
//   }
// If it has implemented the interface
//   interface{
//       ToHTTPResponse(*http.Response) (*http.Response, error)
//   }
// it will call ToHTTPResponse to fix the response. If ToHTTPResponse returns
// an error, it should close the body of the original response.
func NewHTTPEndpoint(addr string, client *http.Client) Endpoint {
	return httpEndpoint{addr: addr, client: client}
}

type httpEndpoint struct {
	addr   string
	client *http.Client
}

func (h httpEndpoint) String() string {
	return h.addr
}

func (h httpEndpoint) IsHealthy(context.Context) bool {
	return true
}

func (h httpEndpoint) RoundTrip(ctx context.Context, req Request) (Response, error) {
	hreq, err := req.(interface {
		ToHTTPRequest(Endpoint) (*http.Request, error)
	}).ToHTTPRequest(h)
	if err != nil {
		return nil, err
	}

	if ctx != context.TODO() && ctx != context.Background() {
		hreq = hreq.WithContext(ctx)
	}

	client := h.client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(hreq)
	if err == nil {
		resp, err = req.(interface {
			ToHTTPResponse(*http.Response) (*http.Response, error)
		}).ToHTTPResponse(resp)
	}

	return resp, err
}
