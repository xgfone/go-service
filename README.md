# go-service [![Build Status](https://travis-ci.org/xgfone/go-service.svg?branch=master)](https://travis-ci.org/xgfone/go-service) [![GoDoc](https://godoc.org/github.com/xgfone/go-service?status.svg)](http://godoc.org/github.com/xgfone/go-service) [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=flat-square)](https://raw.githubusercontent.com/xgfone/go-service/master/LICENSE)

A service library, such as LoadBalancer.

## Installation
```shell
$ go get -u github.com/xgfone/go-service
```

## Example

### `Client` Mode

```go
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"time"

	"github.com/xgfone/go-service"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type request struct {
	method string
	urlTmp string
	body   io.Reader
}

// newRequest returns a new Request instance, which can convert itself to
// *http.Request.
func newRequest(method, urlTmp string, body io.Reader) request {
	return request{method: method, urlTmp: urlTmp, body: body}
}

func (r request) Session() string      { return "" }
func (r request) RemoteAddr() net.Addr { return nil }
func (r request) ToHTTPRequest(ctx context.Context, ep service.Endpoint) (*http.Request, error) {
	url := fmt.Sprintf(r.urlTmp, ep.String())
	return http.NewRequestWithContext(ctx, r.method, url, r.body)
}

func main() {
	timeout := time.Second
	interval := time.Second * 5

	lb := service.
		NewLoadBalancer().
		SetSelector(service.RandomSelector()).
		SetFailHandler(service.FailTry(0))
	hc := service.NewHealthCheck()
	hc.AddUpdater(lb)
	hc.AddEndpoint(service.NewHTTPEndpoint("192.168.1.1:80", nil), interval, timeout)
	hc.AddEndpoint(service.NewHTTPEndpoint("192.168.1.2:80", nil), interval, timeout)
	hc.AddEndpoint(service.NewHTTPEndpoint("192.168.1.3:80", nil), interval, timeout)
	//
	// Or you can do this by using StatusLoadBalancer as follow,
	// which is the union of LoadBalancer and HealthCheck.
	//
	// lb := service.NewStatusLoadBalancer()
	// lb.SetSelector(service.RandomSelector()).SetFailHandler(service.FailTry(0))
	// lb.AddEndpoint(service.NewHTTPEndpoint("192.168.1.1:80", nil), interval, timeout)
	// lb.AddEndpoint(service.NewHTTPEndpoint("192.168.1.2:80", nil), interval, timeout)
	// lb.AddEndpoint(service.NewHTTPEndpoint("192.168.1.3:80", nil), interval, timeout)

	// Wait to check the health status of all end endpoints.
	time.Sleep(time.Second)

	fmt.Println(lb.Endpoints())
	// Output:
	// [192.168.1.1:80 192.168.1.2:80 192.168.1.3:80]

	// Send the request and get the response.
	req := newRequest(http.MethodGet, "http://%s", nil)
	res, err := lb.RoundTrip(context.Background(), req)
	if err != nil {
		fmt.Println(err)
		return
	}

	buf := bytes.NewBuffer(nil)
	resp := res.(*http.Response)
	io.CopyN(buf, resp.Body, resp.ContentLength)
	resp.Body.Close()

	fmt.Println("StatusCode:", resp.StatusCode)
	fmt.Println("Body:", buf.String())
}
```

### `Proxy` Mode

```go
package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"time"

	"github.com/xgfone/go-service"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type request struct {
	*http.Request
}

func (r request) Session() string { return "" }
func (r request) RemoteAddr() net.Addr {
	addr, _ := net.ResolveTCPAddr("tcp", r.Request.RemoteAddr)
	return addr
}
func (r request) ToHTTPRequest(ctx context.Context, ep service.Endpoint) (*http.Request, error) {
	url := fmt.Sprintf("http://%s%s", ep.String(), r.Request.RequestURI)
	req, _ := http.NewRequestWithContext(ctx, r.Request.Method, url, r.Request.Body)
	req.Header.Set("HeaderXForwardedFor", r.Request.RemoteAddr)
	req.Header.Set("Origin", url)
	// TODO: Add other headers
	return req, nil
}

func proxyHandler(lb *service.LoadBalancer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, err := lb.RoundTrip(context.Background(), request{r})
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}

		hresp := resp.(*http.Response)
		for key, value := range hresp.Header {
			w.Header()[key] = value
		}
		// TODO: Add and fix the response headers
		w.WriteHeader(hresp.StatusCode)
		if hresp.ContentLength > 0 {
			io.CopyBuffer(w, hresp.Body, make([]byte, 1024))
		}
	})
}

func main() {
	timeout := time.Second
	interval := time.Second * 5

	slb := service.NewStatusLoadBalancer()
	slb.SetSelector(service.RandomSelector()).SetFailHandler(service.FailTry(0))
	slb.AddEndpoint(service.NewHTTPEndpoint("192.168.1.1:80", nil), interval, timeout)
	slb.AddEndpoint(service.NewHTTPEndpoint("192.168.1.2:80", nil), interval, timeout)
	slb.AddEndpoint(service.NewHTTPEndpoint("192.168.1.3:80", nil), interval, timeout)

	http.ListenAndServe(":8000", proxyHandler(slb.LoadBalancer))
}
```

Then you can access `http://127.0.0.1:8000` to forward the request to any of `http://192.168.1.1:80`, `http://192.168.1.2:80`, `http://192.168.1.3:80`.
