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
	"fmt"
	"sort"
	"time"
)

func ExampleLoadBalancerGroup() {
	ep1 := NewNoopEndpoint("127.0.0.1:8001")
	ep2 := NewNoopEndpoint("127.0.0.1:8002")
	ep3 := NewNoopEndpoint("127.0.0.1:8003")
	ep4 := NewNoopEndpoint("127.0.0.1:8004")
	ep5 := NewNoopEndpoint("127.0.0.1:8005")

	hc := NewHealthCheck()
	defer hc.Stop()

	lbg := NewLoadBalancerGroup()
	lbg.SetHealthCheck(hc, time.Millisecond*10, time.Second)
	lbg.OnEndpoint = UpdaterFunc(func(add bool, ep Endpoint) {
		if hc.HasEndpoint(ep) {
			return
		}

		if add {
			fmt.Printf("Add the endpoint '%s'\n", ep.String())
		} else {
			fmt.Printf("Delete the endpoint '%s'\n", ep.String())
		}
	})

	lbg.AddEndpoint("group1", ep1)
	lbg.AddEndpoint("group1", ep2)
	lbg.AddEndpoint("group1", ep3)
	lbg.AddEndpoint("group2", ep2)
	lbg.AddEndpoint("group2", ep3)
	lbg.AddEndpoint("group2", ep4)
	lbg.AddEndpoint("group3", ep3)
	lbg.AddEndpoint("group3", ep4)
	lbg.AddEndpoint("group3", ep5)

	time.Sleep(time.Millisecond * 100)

	groups := lbg.GetAllGroups()
	sort.Strings(groups)
	fmt.Println(groups)

	fmt.Println("group1:", lbg.GetLoadBalancer("group1").EndpointManager().Endpoints())
	fmt.Println("group2:", lbg.GetLoadBalancer("group2").EndpointManager().Endpoints())
	fmt.Println("group3:", lbg.GetLoadBalancer("group3").EndpointManager().Endpoints())

	lbg.DelEndpoint(ep2)
	time.Sleep(time.Millisecond * 50)
	fmt.Println("group1:", lbg.GetLoadBalancer("group1").EndpointManager().Endpoints())
	fmt.Println("group2:", lbg.GetLoadBalancer("group2").EndpointManager().Endpoints())
	fmt.Println("group3:", lbg.GetLoadBalancer("group3").EndpointManager().Endpoints())

	lbg.DelGroup("group2")
	lbg.DelGroup("group3")
	time.Sleep(time.Millisecond * 50)

	fmt.Println("group1:", lbg.GetLoadBalancer("group1").EndpointManager().Endpoints())
	fmt.Println("group2:", lbg.GetLoadBalancer("group2"))
	fmt.Println("group3:", lbg.GetLoadBalancer("group3"))

	eps := hc.Endpoints()
	sort.Sort(eps)
	fmt.Println(eps)

	// Test the inner state
	ss := make([]string, 0, len(lbg.eps))
	for ep := range lbg.eps {
		ss = append(ss, ep)
	}
	sort.Strings(ss)
	for _, ep := range ss {
		for group := range lbg.eps[ep].LoadBalancers {
			fmt.Printf("Endpoint(%s) -> Group(%s)\n", ep, group)
		}
	}

	// Output:
	// Add the endpoint '127.0.0.1:8001'
	// Add the endpoint '127.0.0.1:8002'
	// Add the endpoint '127.0.0.1:8003'
	// Add the endpoint '127.0.0.1:8004'
	// Add the endpoint '127.0.0.1:8005'
	// [group1 group2 group3]
	// group1: [127.0.0.1:8001 127.0.0.1:8002 127.0.0.1:8003]
	// group2: [127.0.0.1:8002 127.0.0.1:8003 127.0.0.1:8004]
	// group3: [127.0.0.1:8003 127.0.0.1:8004 127.0.0.1:8005]
	// group1: [127.0.0.1:8001 127.0.0.1:8003]
	// group2: [127.0.0.1:8003 127.0.0.1:8004]
	// group3: [127.0.0.1:8003 127.0.0.1:8004 127.0.0.1:8005]
	// group1: [127.0.0.1:8001 127.0.0.1:8003]
	// group2: <nil>
	// group3: <nil>
	// [127.0.0.1:8001 127.0.0.1:8003]
	// Endpoint(127.0.0.1:8001) -> Group(group1)
	// Endpoint(127.0.0.1:8003) -> Group(group1)
}
