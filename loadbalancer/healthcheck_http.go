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
	"net/http"
	"net/url"
)

var defaultHTTPStatusCodeRanges = []HTTPStatusCodeRange{{End: 400}}

// HTTPStatusCodeRange is the range of the http status code,
// which is semi-closure, that's, [Begin, End).
type HTTPStatusCodeRange struct {
	Begin int `json:"begin" xml:"begin"`
	End   int `json:"end" xml:"end"`
}

type HTTPEndpointHealthChecker struct {
	// Addr is the address to dial.
	Addr string

	// Info is the information to check the health of the http endpoint.
	//
	// Default:
	//   Scheme: "http"
	//   Method: "GET"
	//   Path:   "/"
	Info HTTPEndpointInfo

	// Default: http.DefaultClient
	Client *http.Client

	// Default: [{Begin: 0, End: 400}]
	Codes []HTTPStatusCodeRange
}

var _ EndpointChecker = &HTTPEndpointHealthChecker{}

func NewHTTPEndpointHealthChecker(addr string) *HTTPEndpointHealthChecker {
	return &HTTPEndpointHealthChecker{Addr: addr}
}

func (c *HTTPEndpointHealthChecker) Check(ctx context.Context) (err error) {
	url := url.URL{
		Scheme:   c.Info.Scheme,
		Host:     c.Addr,
		Path:     c.Info.Path,
		RawQuery: c.Info.Query.Encode(),
	}
	if url.Scheme == "" {
		url.Scheme = "http"
	}

	method := c.Info.Method
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, url.String(), nil)
	if err != nil {
		return err
	} else if len(c.Info.Header) > 0 {
		req.Header = c.Info.Header.Clone()
	}
	if c.Info.Host != "" {
		req.Host = c.Info.Host
	}

	var resp *http.Response
	if c.Client == nil {
		resp, err = http.DefaultClient.Do(req)
	} else {
		resp, err = c.Client.Do(req)
	}

	if err != nil {
		return
	}
	resp.Body.Close()

	statusCodes := c.Codes
	if len(statusCodes) == 0 {
		statusCodes = defaultHTTPStatusCodeRanges
	}
	for _, code := range statusCodes {
		if code.Begin <= resp.StatusCode && resp.StatusCode < code.End {
			return nil
		}
	}

	return fmt.Errorf("unexpected status code '%d'", resp.StatusCode)
}
