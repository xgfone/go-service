# go-service [![Build Status](https://travis-ci.org/xgfone/go-service.svg?branch=master)](https://travis-ci.org/xgfone/go-service) [![GoDoc](https://godoc.org/github.com/xgfone/go-service?status.svg)](https://pkg.go.dev/github.com/xgfone/go-service) [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=flat-square)](https://raw.githubusercontent.com/xgfone/go-service/master/LICENSE)

A service library, such as Task Runner, LoadBalancer, HealthCheck or Retry, support `Go1.15+`.

- 1 [Installation](#1-installation)
- 2 [Example](#2-example)
    - 2.1 [Task Runner](#21-task-runner)
    - 2.2 [Load Balancer](#22-load-balancer)
        - 2.2.1 [`Client` Mode](#221-client-mode)
        - 2.2.2 [`Proxy` Mode](#222-proxy-mode)


## 1. Installation
```shell
$ go get -u github.com/xgfone/go-service
```

## 2. Example

### 2.1 Task Runner
```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/xgfone/go-service/task"
)

type tasker struct {
	name string
}

func (t tasker) Name() string { return t.name }
func (t tasker) Run(ctx context.Context, now time.Time) (err error) {
	fmt.Printf("[%s] run task '%s'\n", now.Format(time.RFC3339), t.name)
	return
}

func newTask(name string) task.Task { return tasker{name: name} }

func runTask(ctx context.Context, now time.Time) (err error) {
	t := task.GetTaskFromCtx(ctx)
	fmt.Printf("[%s] run task '%s'\n", now.Format(time.RFC3339), t.Name())
	return
}

func main() {
	// Default Tick:     1s
	// Default Interval: 3s
	config := task.IntervalRunnerConfig{Interval: time.Second * 3}
	runner := task.NewIntervalRunner(config)

	// Add all the tasks
	runner.AddTask(newTask("task1"))                                     // Use Default Interval: 3s
	runner.AddTask(task.NewIntervalTask2("task2", 0, runTask))           // Use Default Interval: 3s
	runner.AddTask(task.NewIntervalTask2("task3", time.Second, runTask)) // Use Special Interval: 1s

	// We only run the tasks for 10s.
	time.Sleep(time.Second * 10)
	runner.Stop()

	// Output:
	// [2020-12-06T10:17:57+08:00] run task 'task2'
	// [2020-12-06T10:17:57+08:00] run task 'task1'
	// [2020-12-06T10:17:57+08:00] run task 'task3'
	// [2020-12-06T10:17:59+08:00] run task 'task3'
	// [2020-12-06T10:18:01+08:00] run task 'task2'
	// [2020-12-06T10:18:01+08:00] run task 'task1'
	// [2020-12-06T10:18:01+08:00] run task 'task3'
	// [2020-12-06T10:18:03+08:00] run task 'task3'
	// [2020-12-06T10:18:04+08:00] run task 'task1'
	// [2020-12-06T10:18:04+08:00] run task 'task3'
	// [2020-12-06T10:18:04+08:00] run task 'task2'
}
```

### 2.2 Load Balancer

#### 2.2.1 `Client` Mode

```go
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/xgfone/go-service/httputil"
	"github.com/xgfone/go-service/loadbalancer"
)

func roundTripp(lb *loadbalancer.LoadBalancer, host string) http.RoundTripper {
	return httputil.RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Host == host {
			resp, err := lb.RoundTrip(context.Background(), loadbalancer.NewHTTPRequest(r, ""))
			if err != nil {
				return nil, err
			}
			return resp.(*http.Response), nil
		}
		return http.DefaultTransport.RoundTrip(r)
	})
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
	lb := loadbalancer.NewLoadBalancer("default", nil)
	defer lb.Close()

	hc := loadbalancer.NewHealthCheck()
	hc.AddUpdater(lb.Name(), lb)
	defer hc.Stop()

	http.DefaultClient.Transport = roundTripp(lb, "127.0.0.1:80")
	ep1, _ := loadbalancer.NewHTTPEndpoint("192.168.1.1", nil)
	ep2, _ := loadbalancer.NewHTTPEndpoint("192.168.1.2", nil)
	ep3, _ := loadbalancer.NewHTTPEndpoint("192.168.1.3", nil)
	duration := loadbalancer.EndpointCheckerDuration{Interval: time.Second * 10}
	hc.AddEndpoint(ep1, loadbalancer.NewHTTPEndpointHealthChecker(ep1.ID()), duration)
	hc.AddEndpoint(ep2, loadbalancer.NewHTTPEndpointHealthChecker(ep2.ID()), duration)
	hc.AddEndpoint(ep3, loadbalancer.NewHTTPEndpointHealthChecker(ep3.ID()), duration)

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

#### 2.2.2 `Proxy` Mode
```go
package main

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/xgfone/go-service/loadbalancer"
)

func proxyHandler(lb *loadbalancer.LoadBalancer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Add other headers
		resp, err := lb.RoundTrip(context.Background(), loadbalancer.NewHTTPRequest(r, r.Header.Get("SessionID")))
		if err != nil {
			w.WriteHeader(502)
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
	lb := loadbalancer.NewLoadBalancer("default", nil)
	defer lb.Close()

	hc := loadbalancer.NewHealthCheck()
	hc.AddUpdater(lb.Name(), lb)
	defer hc.Stop()

	ep1, _ := loadbalancer.NewHTTPEndpoint("192.168.1.1", nil)
	ep2, _ := loadbalancer.NewHTTPEndpoint("192.168.1.2", nil)
	ep3, _ := loadbalancer.NewHTTPEndpoint("192.168.1.3", nil)
	duration := loadbalancer.EndpointCheckerDuration{Interval: time.Second * 10}
	hc.AddEndpoint(ep1, loadbalancer.NewHTTPEndpointHealthChecker(ep1.ID()), duration)
	hc.AddEndpoint(ep2, loadbalancer.NewHTTPEndpointHealthChecker(ep2.ID()), duration)
	hc.AddEndpoint(ep3, loadbalancer.NewHTTPEndpointHealthChecker(ep3.ID()), duration)

	http.ListenAndServe(":80", proxyHandler(lb))
}
```
