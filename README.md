# go-service [![Build Status](https://travis-ci.org/xgfone/go-service.svg?branch=master)](https://travis-ci.org/xgfone/go-service) [![GoDoc](https://godoc.org/github.com/xgfone/go-service?status.svg)](http://godoc.org/github.com/xgfone/go-service) [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=flat-square)](https://raw.githubusercontent.com/xgfone/go-service/master/LICENSE)

A service library, such as LoadBalancer, HealthCheck or Retry.

- 1 [Installation](#1-installation)
- 2 [Example](#2-example)
    - 2.1 [`Client` Mode](#21-client-mode)
       - 2.1.1 [For HTTP Client](#211-for-http-client)
       - 2.1.2 [For TCP Client](#212-for-tcp-client)
    - 2.2 [`Proxy` Mode](#22-proxy-mode)
       - 2.2.1 [For HTTP Proxy](#221-for-http-Proxy)
       - 2.2.2 [For TCP Proxy](#222-for-tcp-Proxy)


## 1. Installation
```shell
$ go get -u github.com/xgfone/go-service
```

## 2. Example

### 2.1 `Client` Mode

#### 2.1.1 For HTTP Client
```go
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/xgfone/go-service"
)

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

func (r request) RemoteAddrString() string { return "" }
func (r request) ToHTTPRequest(ctx context.Context, ep service.Endpoint) (*http.Request, error) {
	url := fmt.Sprintf(r.urlTmp, ep.String())
	return http.NewRequestWithContext(ctx, r.method, url, r.body)
}

func main() {
	timeout := time.Second
	interval := time.Second * 5

	lb := service.NewLoadBalancer(nil)
	hc := service.NewHealthCheck()
	hc.AddUpdater(lb.EndpointManager())
	hc.AddEndpoint(service.NewHTTPEndpoint("192.168.1.1:80", nil), interval, timeout)
	hc.AddEndpoint(service.NewHTTPEndpoint("192.168.1.2:80", nil), interval, timeout)
	hc.AddEndpoint(service.NewHTTPEndpoint("192.168.1.3:80", nil), interval, timeout)
	//
	// Or you can do this by using StatusLoadBalancer as follow,
	// which is the union of LoadBalancer and HealthCheck.
	//
	// lb := service.NewStatusLoadBalancer(nil)
	// lb.AddEndpoint(service.NewHTTPEndpoint("192.168.1.1:80", nil), interval, timeout)
	// lb.AddEndpoint(service.NewHTTPEndpoint("192.168.1.2:80", nil), interval, timeout)
	// lb.AddEndpoint(service.NewHTTPEndpoint("192.168.1.3:80", nil), interval, timeout)

	// Wait to check the health status of all end endpoints.
	time.Sleep(time.Second)

	fmt.Println(lb.EndpointManager().Endpoints())
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

Or, you can use it implicitly. For example,
```go
package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/xgfone/go-service"
)

func init() {
	timeout := time.Second
	interval := time.Second * 5
	lb := service.NewStatusLoadBalancer(nil)
	lb.AddEndpoint(service.NewHTTPEndpoint("192.168.1.1:80", nil), interval, timeout)
	lb.AddEndpoint(service.NewHTTPEndpoint("192.168.1.2:80", nil), interval, timeout)
	lb.AddEndpoint(service.NewHTTPEndpoint("192.168.1.3:80", nil), interval, timeout)

	getrt := service.NewGetRoundTripperFromMap(map[string]service.RoundTripper{"127.0.0.1:80": lb})
	// For the single RoundTripper, you can also use NewSingleGetRoundTripper.
	// getrt := service.NewSingleGetRoundTripper("127.0.0.1:80", lb)
	http.DefaultClient.Transport = service.ToHTTPRoundTripper(getrt)
}

func printResponse(resp *http.Response, err error) {
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("URL: %s\n", resp.Request.URL.String())

	buf := bytes.NewBuffer(nil)
	io.CopyN(buf, resp.Body, resp.ContentLength)
	resp.Body.Close()

	fmt.Println("StatusCode:", resp.StatusCode)
	fmt.Println("Body:", buf.String())
}

func main() {
	// Wait to check the health status of all end endpoints.
	time.Sleep(time.Second)

	// 127.0.0.1:80 will be replaced with one of 192.168.1.1:80, 192.168.1.2:80, 192.168.1.3:80.
	resp, err := http.Get("http://127.0.0.1:80")
	printResponse(resp, err)

	// 127.0.0.1:8000 won't be replaced, and it will send the request to 127.0.0.1:8000 directly.
	resp, err = http.Get("http://127.0.0.1:8000")
	printResponse(resp, err)
}
```

#### 2.1.2 For TCP Client
```go
package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/xgfone/go-service"
)

func dial(ctx context.Context, address string) (net.Conn, error) { return net.Dial("tcp", address) }

type request struct{ Data string }

func (r request) SendRequest(conn *net.TCPConn) (err error) {
	buf := bytes.NewBuffer(nil)
	buf.Grow(len(r.Data) + 4)
	binary.Write(buf, binary.BigEndian, int32(len(r.Data)))
	buf.WriteString(r.Data)
	_, err = io.Copy(conn, buf)
	return
}

func (r request) ReadResponse(conn *net.TCPConn) (resp service.Response, err error) {
	var length uint32
	if err = binary.Read(conn, binary.BigEndian, &length); err != nil {
		return
	}

	buf := bytes.NewBuffer(nil)
	buf.Grow(int(length))
	if _, err = io.CopyN(buf, conn, int64(length)); err != nil {
		return
	}

	return buf.String(), nil
}

func main() {
	timeout := time.Second
	interval := time.Second * 5
	lb := service.NewStatusLoadBalancer(nil)
	lb.AddEndpoint(service.NewTCPEndpoint("192.168.1.1:80", dial, 10, time.Second), interval, timeout)
	lb.AddEndpoint(service.NewTCPEndpoint("192.168.1.2:80", dial, 10, time.Second), interval, timeout)
	lb.AddEndpoint(service.NewTCPEndpoint("192.168.1.3:80", dial, 10, time.Second), interval, timeout)

	// Wait to check the health status of all end endpoints.
	time.Sleep(time.Second)

	fmt.Println(lb.EndpointManager().Endpoints())
	// Output:
	// [192.168.1.1:80 192.168.1.2:80 192.168.1.3:80]

	// Send the request and get the response.
	req := request{Data: "THE SENT DATA"}
	res, err := lb.RoundTrip(context.Background(), service.NewTCPRequest("", req.SendRequest, req.ReadResponse))
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(res)
	}
}
```

### 2.2 `Proxy` Mode

#### 2.2.1 For HTTP Proxy
```go
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/xgfone/go-service"
)

type request http.Request

func (r *request) RemoteAddrString() string { return r.RemoteAddr }
func (r *request) ToHTTPRequest(ctx context.Context, ep service.Endpoint) (*http.Request, error) {
	url := fmt.Sprintf("http://%s%s", ep.String(), r.RequestURI)
	req, _ := http.NewRequestWithContext(ctx, r.Method, url, r.Body)
	req.Header.Set("X-Forwarded-For", r.RemoteAddr)
	req.Header.Set("Origin", url)
	// TODO: Add other headers
	return req, nil
}

func proxyHandler(lb *service.LoadBalancer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, err := lb.RoundTrip(context.Background(), (*request)(r))
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

	slb := service.NewStatusLoadBalancer(nil)
	slb.AddEndpoint(service.NewHTTPEndpoint("192.168.1.1:80", nil), interval, timeout)
	slb.AddEndpoint(service.NewHTTPEndpoint("192.168.1.2:80", nil), interval, timeout)
	slb.AddEndpoint(service.NewHTTPEndpoint("192.168.1.3:80", nil), interval, timeout)

	http.ListenAndServe(":8000", proxyHandler(slb.LoadBalancer))
}
```

Then you can access `http://127.0.0.1:8000` to forward the request to any of `http://192.168.1.1:80`, `http://192.168.1.2:80`, `http://192.168.1.3:80`.

You also implement yourself `Request` and `Endpoint` to customize the business logic. For `TCP`, you should implement `Endpoint` by yourself according to the customized protocol format.

#### 2.2.2 For TCP Proxy
```go
package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/xgfone/go-service"
)

func dial(ctx context.Context, address string) (net.Conn, error) { return net.Dial("tcp", address) }

type request struct{ Data []byte }

func (r request) Send(conn *net.TCPConn) (err error) {
	buf := bytes.NewBuffer(nil)
	buf.Grow(len(r.Data) + 4)
	binary.Write(buf, binary.BigEndian, int32(len(r.Data)))
	buf.Write(r.Data)
	_, err = io.Copy(conn, buf)
	return
}

func (r request) Read(conn *net.TCPConn) (resp service.Response, err error) {
	var length uint32
	if err = binary.Read(conn, binary.BigEndian, &length); err != nil {
		return
	}

	buf := bytes.NewBuffer(nil)
	buf.Grow(int(length))
	if _, err = io.CopyN(buf, conn, int64(length)); err != nil {
		return
	}

	return buf.Bytes(), nil
}

func proxy(lb *service.StatusLoadBalancer, conn *net.TCPConn) {
	defer conn.Close()

	raddr := conn.RemoteAddr().String()
	for {
		r := request{}

		// Read the data from the source
		resp, err := r.Read(conn)
		if err != nil {
			if err != io.EOF {
				fmt.Printf("fail to read from the source '%s': %v\n", raddr, err)
			}
			return
		}
		r.Data = resp.([]byte)

		// Write the data to the destination and get the response.
		req := service.NewTCPRequest(raddr, r.Send, r.Read)
		resp, err = lb.RoundTrip(context.Background(), req)
		if err != nil {
			fmt.Printf("fail to send the data to the destination: %v\n", err)
			return
		}

		// Write the response to the source
		r.Data = resp.([]byte)
		if err = r.Send(conn); err != nil {
			fmt.Printf("fail to send the response to the source '%s': %v\n", raddr, err)
			return
		}
	}
}

func main() {
	timeout := time.Second
	interval := time.Second * 5
	lb := service.NewStatusLoadBalancer(nil)
	lb.AddEndpoint(service.NewTCPEndpoint("192.168.1.1:80", dial, 10, time.Second), interval, timeout)
	lb.AddEndpoint(service.NewTCPEndpoint("192.168.1.2:80", dial, 10, time.Second), interval, timeout)
	lb.AddEndpoint(service.NewTCPEndpoint("192.168.1.3:80", dial, 10, time.Second), interval, timeout)

	addr, _ := net.ResolveTCPAddr("tcp", ":8000")
	ln, err := net.ListenTCP("tcp", addr)
	if err != nil {
		fmt.Println(err)
		return
	}

	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			fmt.Println(err)
			return
		}
		go proxy(lb, conn)
	}
}
```
