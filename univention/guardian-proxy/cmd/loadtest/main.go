// SPDX-FileCopyrightText: 2026 Univention GmbH
// SPDX-License-Identifier: AGPL-3.0-only

// Command loadtest sends CheckResources requests to Cerbos or guardian-proxy
// over HTTP or gRPC and reports throughput, latency and the CPU time and
// memory of the Cerbos and proxy processes.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	effectv1 "github.com/cerbos/cerbos/api/genpb/cerbos/effect/v1"
	enginev1 "github.com/cerbos/cerbos/api/genpb/cerbos/engine/v1"
	requestv1 "github.com/cerbos/cerbos/api/genpb/cerbos/request/v1"
	responsev1 "github.com/cerbos/cerbos/api/genpb/cerbos/response/v1"
	svcv1 "github.com/cerbos/cerbos/api/genpb/cerbos/svc/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

const warmupRequests = 200

func main() {
	protocol := flag.String("protocol", "http", "http or grpc")
	addr := flag.String("addr", "http://localhost:3600", "HTTP base URL or gRPC host:port")
	roles := flag.String("roles", "user", "comma-separated principal roles")
	guardianRole := flag.String("guardian-role", "ucsschool:users:teacher&ucsschool:ou=school1", "guardianRoles value of the principal")
	clients := flag.Int("c", 1, "parallel clients")
	duration := flag.Duration("d", 8*time.Second, "duration of the measurement")
	name := flag.String("name", "", "label of the output line")
	cerbosPID := flag.Int("cerbos-pid", 0, "PID of Cerbos, for CPU per request")
	proxyPID := flag.Int("proxy-pid", 0, "PID of guardian-proxy, for CPU per request and memory")
	flag.Parse()

	req, err := newRequest(strings.Split(*roles, ","), *guardianRole)
	if err != nil {
		fail(err)
	}

	var check func() (*responsev1.CheckResourcesResponse, error)
	switch *protocol {
	case "http":
		check, err = httpCheck(*addr, req, *clients)
	case "grpc":
		check, err = grpcCheck(*addr, req)
	default:
		err = fmt.Errorf("unknown protocol %q", *protocol)
	}
	if err != nil {
		fail(err)
	}

	do := func() error {
		resp, err := check()
		if err != nil {
			return err
		}
		if effect := resp.GetResults()[0].GetActions()["view"]; effect != effectv1.Effect_EFFECT_ALLOW {
			return fmt.Errorf("view: %s", effect)
		}
		return nil
	}

	for range warmupRequests {
		if err := do(); err != nil {
			fail(fmt.Errorf("warm-up: %w", err))
		}
	}

	r := measure(do, *clients, *duration, *cerbosPID, *proxyPID)
	fmt.Printf("%-8s %-4s c=%-3d req/s=%-7.0f p50=%-6s p90=%-6s p99=%-6s errors=%d cerbosCPU/req=%s proxyCPU/req=%s proxyRSS=%s\n",
		*name, *protocol, *clients, r.rate, r.percentile(0.5), r.percentile(0.9), r.percentile(0.99),
		r.errors, cpuPerRequest(r.cerbosTicks, len(r.latencies)), cpuPerRequest(r.proxyTicks, len(r.latencies)), rss(*proxyPID))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "loadtest:", err)
	os.Exit(1)
}

func newRequest(roles []string, guardianRole string) (*requestv1.CheckResourcesRequest, error) {
	guardianRoles, err := structpb.NewValue([]any{guardianRole})
	if err != nil {
		return nil, err
	}

	return &requestv1.CheckResourcesRequest{
		RequestId: "loadtest",
		Principal: &enginev1.Principal{
			Id:    "anton",
			Roles: roles,
			Attr:  map[string]*structpb.Value{"guardianRoles": guardianRoles},
		},
		Resources: []*requestv1.CheckResourcesRequest_ResourceEntry{{
			Actions:  []string{"view"},
			Resource: &enginev1.Resource{Kind: "doc", Id: "1"},
		}},
	}, nil
}

func httpCheck(baseURL string, req *requestv1.CheckResourcesRequest, clients int) (func() (*responsev1.CheckResourcesResponse, error), error) {
	body, err := protojson.Marshal(req)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: clients}}
	url := strings.TrimSuffix(baseURL, "/") + "/api/check/resources"

	return func() (*responsev1.CheckResourcesResponse, error) {
		resp, err := client.Post(url, "application/json", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("status %d: %s", resp.StatusCode, data)
		}

		out := &responsev1.CheckResourcesResponse{}
		return out, protojson.Unmarshal(data, out)
	}, nil
}

func grpcCheck(addr string, req *requestv1.CheckResourcesRequest) (func() (*responsev1.CheckResourcesResponse, error), error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := svcv1.NewCerbosServiceClient(conn)
	return func() (*responsev1.CheckResourcesResponse, error) {
		return client.CheckResources(context.Background(), req)
	}, nil
}

type result struct {
	latencies               []time.Duration
	errors                  int
	rate                    float64
	cerbosTicks, proxyTicks int64
}

func measure(do func() error, clients int, duration time.Duration, cerbosPID, proxyPID int) result {
	var (
		mu  sync.Mutex
		r   result
		wg  sync.WaitGroup
		end = time.Now().Add(duration)
	)

	cerbos0, proxy0 := cpuTicks(cerbosPID), cpuTicks(proxyPID)
	start := time.Now()
	for range clients {
		wg.Go(func() {
			var latencies []time.Duration
			errors := 0
			for time.Now().Before(end) {
				t := time.Now()
				if err := do(); err != nil {
					errors++
					continue
				}
				latencies = append(latencies, time.Since(t))
			}

			mu.Lock()
			defer mu.Unlock()
			r.latencies = append(r.latencies, latencies...)
			r.errors += errors
		})
	}
	wg.Wait()

	r.rate = float64(len(r.latencies)) / time.Since(start).Seconds()
	r.cerbosTicks = cpuTicks(cerbosPID) - cerbos0
	r.proxyTicks = cpuTicks(proxyPID) - proxy0
	slices.Sort(r.latencies)

	return r
}

func (r result) percentile(p float64) string {
	if len(r.latencies) == 0 {
		return "-"
	}
	d := r.latencies[int(float64(len(r.latencies)-1)*p)]
	return fmt.Sprintf("%dµs", d.Microseconds())
}

// cpuTicks returns the user and system CPU time of a process in clock ticks,
// which are 10 ms on Linux.
func cpuTicks(pid int) int64 {
	if pid == 0 {
		return 0
	}

	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0
	}

	// The command name in parentheses may contain spaces.
	fields := strings.Fields(string(data[bytes.LastIndexByte(data, ')')+2:]))
	utime, _ := strconv.ParseInt(fields[11], 10, 64)
	stime, _ := strconv.ParseInt(fields[12], 10, 64)
	return utime + stime
}

func cpuPerRequest(ticks int64, requests int) string {
	if ticks == 0 || requests == 0 {
		return "-"
	}
	return fmt.Sprintf("%.0fµs", float64(ticks)*float64(10*time.Millisecond/time.Microsecond)/float64(requests))
}

func rss(pid int) string {
	if pid == 0 {
		return "-"
	}

	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return "-"
	}

	for line := range strings.Lines(string(data)) {
		if v, ok := strings.CutPrefix(line, "VmRSS:"); ok {
			return strings.TrimSpace(v)
		}
	}
	return "-"
}
