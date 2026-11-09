# guardian-proxy

Proof of concept for guardian#303:
a proxy in front of an unmodified Cerbos
that changes the principal of each decision request
before Cerbos evaluates it.

## How it works

```text
client ──HTTP :3600──▶ grpc-gateway ──┐
                                      ├──▶ proxy + PrincipalMapper ──gRPC :3593──▶ Cerbos
client ──gRPC :3601───────────────────┘
```

The proxy implements the Cerbos gRPC service, `svcv1.CerbosServiceServer`,
with the generated Cerbos API code from `api/genpb`,
and serves it twice:

- **gRPC** on `:3601` (`grpc.go`).
- **HTTP** on `:3600` with grpc-gateway (`newHandler` in `proxy.go`),
  which is how Cerbos serves its own HTTP API.
  The gateway decodes the JSON request into the same protobuf message
  and calls the proxy in-process, without a gRPC hop.
  It uses the same marshaler settings and header handling as Cerbos
  (`incomingHeaderMatcher` is a copy),
  so clients get the same JSON and the same errors as from Cerbos.

A decision request goes through these steps (`forward` and `call` in `proxy.go`):

1. The proxy passes the principal to `PrincipalMapper.MapPrincipal`,
   which changes it in place.
   If the mapper fails, the request ends here, see [The hook](#the-hook).
2. The proxy sends the changed request to Cerbos over one shared gRPC connection.
   The client's metadata goes along unchanged.
   For gRPC clients, the proxy appends the client address to `x-forwarded-for`,
   as the gateway does for HTTP clients,
   so the audit log shows who sent the request.
3. Cerbos validates and evaluates the request.
   The proxy returns the response and Cerbos's response headers,
   such as `cerbos-version`, to the client.
   Cerbos's errors pass through unchanged, including validation errors
   with their `buf.validate.Violations` details;
   `protodeps.go` registers that type so that the gateway can encode it,
   as Cerbos does in `internal/server/protodeps.go`.

What the proxy does with each request:

| Request | HTTP | gRPC |
|---|---|---|
| `CheckResources`, `CheckResourceSet`, `CheckResourceBatch`, `PlanResources` | mapped, then to Cerbos | mapped, then to Cerbos |
| `ServerInfo` | to Cerbos | to Cerbos |
| Health | `/_cerbos/health` to the Cerbos HTTP API | `grpc.health.v1` to Cerbos |
| AuthZEN | `/access/v1/` returns `404` | `Unimplemented` |
| Admin API and all other paths | to the Cerbos HTTP API, unchanged | `Unimplemented` |

The AuthZEN API is blocked
because its subject would reach Cerbos without the mapping.
Over HTTP, everything else goes through a plain reverse proxy;
gRPC has no such pass-through without a raw-bytes proxy,
so the PoC leaves the other services out.

### Files

| File | Content |
|---|---|
| `main.go` | flags, connection to Cerbos, both servers and shutdown |
| `proxy.go` | the Cerbos service, request forwarding and the HTTP routing |
| `grpc.go` | the gRPC server and the health check |
| `mapper.go` | the `PrincipalMapper` interface and the placeholder `staticMapper` |
| `protodeps.go` | registration of the error detail types |
| `proxy_test.go` | tests against a fake Cerbos over HTTP and gRPC |
| `cmd/loadtest` | the load generator, see [Performance](#performance) |

## Build

The module uses the Cerbos API code from this repository
(`replace` to `../../api/genpb` in `go.mod`),
so build it inside a checkout of the fork, with Go 1.27.3 or later.
The repository's `go.work` doesn't include this module,
so every Go command needs `GOWORK=off`;
without it, Go fails with
`main module (github.com/cerbos/cerbos) does not contain package`.
All commands in this README assume:

```sh
cd univention/guardian-proxy
export GOWORK=off
```

```sh
go build -o guardian-proxy .
go test ./...
```

For a static binary, for example for a container image:

```sh
CGO_ENABLED=0 go build -trimpath -o guardian-proxy .
```

The binary `guardian-proxy` is ignored by Git.

## Run

```sh
cerbos server --config=...   # HTTP on :3592, gRPC on :3593
go run . -role-mappings mappings.json
curl localhost:3600/api/check/resources -d @request.json
```

`go run . -help` lists the listen addresses and the Cerbos addresses.

`mappings.json` maps `guardianRoles` values to Cerbos roles:

```json
{"ucsschool:users:teacher&ucsschool:ou=school1": "teacher"}
```

## The hook

`PrincipalMapper.MapPrincipal` receives the principal and changes it in place.
The placeholder `staticMapper` adds the Cerbos role
for each known `guardianRoles` value
and keeps `guardianRoles` in the attributes,
so the decision log shows the original and the resolved roles.
The real mapping comes from guardian#302.

If the mapper returns an error,
the proxy answers with `InvalidArgument` (HTTP `400`)
and doesn't forward the request.
A mapper can return a gRPC status error to choose another code.

## Performance

Load test from 2026-11-09:
`CheckResources` with one resource,
against a policy that allows `view` only for `teacher`.
Sent directly to Cerbos, the request contains the role `teacher`;
through the proxy, `teacher` comes from the mapping.
Cerbos ran with `--log-level=warn` and without audit log.
Load generator, proxy and Cerbos shared one machine with 16 cores.
Each value is the mean of two runs of 8 seconds;
all requests succeeded.
CPU is per request, read from `/proc` with 10 ms resolution,
so it suits the comparison, not absolute numbers.

HTTP:

| Clients | Direct req/s | Proxy req/s | Direct p50 / p99 | Proxy p50 / p99 | CPU direct | CPU Cerbos + proxy | Proxy RSS |
|---|---|---|---|---|---|---|---|
| 1 | 2,130 | 2,200 | 0.45 / 1.04 ms | 0.44 / 0.84 ms | 745 µs | 447 + 322 µs | 24 MB |
| 16 | 10,120 | 8,730 (−14%) | 1.42 / 3.68 ms | 1.72 / 3.78 ms | 1,057 µs | 608 + 558 µs | 28 MB |
| 64 | 12,770 | 12,240 (−4%) | 4.60 / 11.1 ms | 4.92 / 11.1 ms | 975 µs | 590 + 378 µs | 33 MB |

gRPC:

| Clients | Direct req/s | Proxy req/s | Direct p50 / p99 | Proxy p50 / p99 | CPU direct | CPU Cerbos + proxy | Proxy RSS |
|---|---|---|---|---|---|---|---|
| 1 | 3,630 | 2,170 (−40%) | 0.26 / 0.62 ms | 0.44 / 0.85 ms | 394 µs | 460 + 362 µs | 23 MB |
| 16 | 16,910 | 10,320 (−39%) | 0.83 / 2.33 ms | 1.45 / 3.22 ms | 531 µs | 584 + 340 µs | 25 MB |
| 64 | 20,660 | 13,380 (−35%) | 2.86 / 7.06 ms | 4.52 / 9.72 ms | 513 µs | 576 + 240 µs | 27 MB |

Over HTTP the proxy costs little,
because Cerbos serves HTTP with its own grpc-gateway,
which calls the Cerbos gRPC server over a local connection.
The proxy takes over that step
instead of adding one.
Over gRPC the proxy is an extra hop:
it adds about 0.2 ms per request
and nearly doubles the CPU per request.
Through the proxy,
gRPC is about as fast as plain Cerbos over HTTP.

Not covered:
a network hop between proxy and Cerbos,
the cost of the real mapping from guardian#302,
the other decision APIs,
and more than one gRPC connection from the proxy to Cerbos.

To repeat the test,
start Cerbos and the proxy with the policy and mapping in `cmd/loadtest/testdata`,
then run `cmd/loadtest` once for each combination of
direct or proxy, HTTP or gRPC, and number of clients:

```sh
T=cmd/loadtest/testdata
cerbos server --log-level=warn --set=storage.driver=disk --set=storage.disk.directory=$T/policies &
C=$!
go build -o guardian-proxy .
./guardian-proxy -role-mappings $T/mappings.json &
P=$!

go run ./cmd/loadtest -name direct -protocol http -addr http://localhost:3592 -roles user,teacher -c 16 -cerbos-pid $C
go run ./cmd/loadtest -name proxy  -protocol http -addr http://localhost:3600 -c 16 -cerbos-pid $C -proxy-pid $P
go run ./cmd/loadtest -name direct -protocol grpc -addr localhost:3593 -roles user,teacher -c 16 -cerbos-pid $C
go run ./cmd/loadtest -name proxy  -protocol grpc -addr localhost:3601 -c 16 -cerbos-pid $C -proxy-pid $P
```

Sent directly, the request carries the role `teacher` itself (`-roles user,teacher`);
through the proxy, the mapping adds it.
The tool stops if a warm-up request isn't allowed,
so a missing mapping can't produce numbers.

## Limitations

- The AuthZEN API isn't served,
  since its subject would bypass the mapping:
  `/access/v1/` returns `404`,
  and the gRPC service returns `Unimplemented`.
- The gRPC admin API isn't forwarded.
  Admin clients talk to Cerbos directly.
- In the audit log,
  the peer address and user agent are the proxy's.
  The client address is in `forwardedFor`.
- The role mappings are read at startup;
  a change needs a restart.
- The connections to Cerbos use neither TLS nor authentication.
