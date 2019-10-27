# go-service [![Build Status](https://travis-ci.org/xgfone/go-service.svg?branch=master)](https://travis-ci.org/xgfone/go-service) [![GoDoc](https://godoc.org/github.com/xgfone/go-service?status.svg)](http://godoc.org/github.com/xgfone/go-service) [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=flat-square)](https://raw.githubusercontent.com/xgfone/go-service/master/LICENSE)

A service library, such as LoadBalancer.

## Installation
```shell
$ go get -u github.com/xgfone/go-service
```

## Example

```go
package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/xgfone/go-service"
)

type request http.Request

func (r *request) ToHTTPRequest(ep service.Endpoint) (*http.Request, error) {
	req := (*http.Request)(r)
	req.URL.Host = ep.String()
	return req, nil
}

func main() {
	lb := service.NewLoadBalancer().SetFailHandler(service.FailTry(0))
	endpoint1 := service.NewHTTPEndpoint("192.168.1.1:80", nil)
	endpoint2 := service.NewHTTPEndpoint("192.168.1.2:80", nil)
	endpoint3 := service.NewHTTPEndpoint("192.168.1.3:80", nil)
	lb.AddEndpoints(endpoint1, endpoint2, endpoint3)

	req := http.NewRequest()
	resp, err := lb.RoundTrip(context.Background(), req)
	if err != nil {
		fmt.Println(err)
		return
	}
	hresp := resp.(*http.Response)
	defer hresp.Body.Close()

	// TODO
}
```
