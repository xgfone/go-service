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
	"io"
	"io/ioutil"
	"net/http"
)

func ExampleLoadBalancer() {
	ep1, _ := NewHTTPEndpoint("127.0.0.1:11111", nil)
	ep2, _ := NewHTTPEndpoint("127.0.0.1:22222", nil)
	ep3, _ := NewHTTPEndpoint("127.0.0.1:33333", nil)

	lb := NewLoadBalancer("", nil)
	lb.AddEndpoint(ep1)
	lb.AddEndpoint(ep2)
	lb.AddEndpoint(ep3)
	defer lb.Close()

	// 1. Send the request as the client mode.
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:80", nil)
	resp, err := lb.RoundTrip(context.Background(), NewHTTPRequest(req, "session_id"))
	if err != nil {
		fmt.Printf("Got an error: %v\n", err)
	} else {
		hresp := resp.(*http.Response)
		defer hresp.Body.Close()

		fmt.Printf("Response:\n")
		fmt.Printf("    StatusCode: %d\n", hresp.StatusCode)

		for key, values := range hresp.Header {
			fmt.Printf("    Headers:\n")
			fmt.Printf("        %s -> %s\n", key, values)
		}

		data, _ := ioutil.ReadAll(hresp.Body)
		fmt.Printf("    Body:\n")
		fmt.Printf("        %s\n", string(data))
	}

	// 2. Send the request as the proxy mode.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		sid := r.Header.Get("SessioinID")
		resp, err := lb.RoundTrip(context.Background(), NewHTTPRequest(r, sid))
		if err != nil {
			fmt.Printf("Got an error: %v\n", err)
			w.WriteHeader(502)
			io.WriteString(w, err.Error())
		} else {
			hresp := resp.(*http.Response)
			defer hresp.Body.Close()

			for key, values := range hresp.Header {
				w.Header()[key] = values
			}

			w.WriteHeader(hresp.StatusCode)
			io.CopyN(w, hresp.Body, hresp.ContentLength)
		}
	})
}
