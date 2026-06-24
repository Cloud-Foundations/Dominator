gRPC and REST APIs
==================

Overview
========

Each **Dominator** server exposes three protocols on a single TLS endpoint, served from the same process: **SRPC**, the native RPC protocol used between Go services; **gRPC**, an RPC protocol for type-safe SDKs in other languages; and **REST**, an HTTP/JSON surface for web UIs and `curl`-style debugging. All three protocols share the same client certificates, authorisation logic, rate limiting and metrics. SRPC is the primary internal protocol and the only one spoken between core servers; gRPC and REST exist for external integrators and non-Go clients.

Three constraints shape the design: type definitions are not duplicated across protocols (the Go structs in `messages.go` are the single source of truth), the `encoding/gob` wire format used by SRPC is not altered, and the certificate-based access-control model is uniform across all three protocols.

Background
==========

SRPC uses `encoding/gob` as its wire format and X.509 client certificates over TLS for authentication and authorisation. `gob` is efficient for native Go types (including `net.IP`, pointers and recursive structures) and requires no separate schema, which makes it well-suited to traffic between Go services but ill-suited to clients written in other languages:

- there is no machine-readable schema; an external developer would otherwise have to read `messages.go` to reverse-engineer request and response structures

- complex Go types (such as `net.IP` and `net.HardwareAddr`) have no standard JSON representation, so each client would map them by hand and silently break as the Go structs evolve

- SRPC uses a custom HTTP `CONNECT` upgrade with its own framing, which web browsers do not implement

gRPC provides a strictly typed, machine-readable contract that addresses these gaps, and grpc-gateway derives a REST/JSON surface from the same definitions wherever the method shape allows, with no hand-written routing.

Alternatives Considered
=======================

A pure REST implementation (with `net/http` handlers and an OpenAPI spec generated from the Go source) was considered. It is awkward for the long-lived streaming RPCs that several **Dominator** services rely on, and JSON encoding is significantly heavier than protobuf for the high-volume internal traffic. Using gRPC as the base and generating REST from it gives both protocols for the cost of one.

Expanding the existing Rust SRPC client and exposing it to other languages via FFI/PyO3 was also considered. It does not solve the type-discovery problem (there is still no schema) and adds a maintenance burden of FFI glue per language.

gRPC alone would have been a narrower scope, but since grpc-gateway generates the REST layer automatically, the marginal cost of supporting both is small.

High-level Design
=================

SRPC, gRPC and REST run side-by-side inside each server process, behind a single TLS endpoint, sharing the same business logic. SRPC and gRPC handlers are thin wrappers that translate transport-specific request types to and from the internal Go structs.

- SRPC handlers call the business logic directly

- gRPC handlers receive protobuf messages, convert them to the internal Go structs using auto-generated converters, and call the same business logic

- REST is provided by grpc-gateway, which translates HTTP/JSON requests into in-process gRPC calls; no REST-specific handler code is written

The mapping from gRPC to REST is not total. Unary methods become ordinary `POST` (or `GET`) endpoints. Server-streaming methods are exposed as chunked HTTP responses with one JSON object per chunk, which `curl` and most HTTP clients can consume. Client-streaming and bidirectional-streaming methods have no REST equivalent and are reachable only via gRPC; the generator detects these shapes and skips REST routing for them automatically.

A single TLS configuration terminates the connection for all three protocols, so identity extraction is uniform regardless of which protocol the client uses.

Protocol Routing on a Shared Port
---------------------------------

Each **Dominator** server listens on one fixed port, and gRPC and REST are served on that same port to avoid port proliferation. Routing is done at the HTTP layer using `http.ServeMux`:

- **SRPC**: the client issues `HTTP CONNECT /_goSRPC/` (or `/_SRPC/`); the connection is upgraded and all subsequent traffic uses SRPC framing over the upgraded TCP connection

- **gRPC**: requests with `Content-Type: application/grpc` are routed to `/<package>.<Service>/<Method>`. The shared listener serves HTTP/2 over the same TLS endpoint

- **REST**: requests matching the grpc-gateway mux paths (rooted at `/v1/<Service>/<Method>`) are dispatched to the gateway, which translates them into in-process gRPC calls

Authentication and Authorisation
================================

The certificate-based access-control model is uniform across all three protocols. Certificates are issued by **Keymaster**, signed by a trusted CA, and carry the username (Common Name), groups (carried in a custom OID) and the comma-separated list of permitted `Service.Method` names (carried in a custom OID).

For gRPC, the SRPC TLS configuration (`srpc.GetServerTlsConfig()`) is wrapped in `credentials.NewTLS(...)` and passed to `grpc.NewServer`. A unary interceptor (and a matching streaming interceptor) extracts the TLS peer information from the request context, invokes `srpc.GetAuthFromTLS` and `srpc.GetPermittedMethodsFromTLS`, and stores the resulting `*srpc.Conn` on the context. Handlers retrieve it with a small helper, call `conn.GetAuthInformation()`, and pass the result into the business logic, in the same way as SRPC handlers.

For REST, grpc-gateway is configured with a clone of the same TLS configuration, with `ClientAuth = tls.RequireAndVerifyClientCert`. A REST request therefore presents the same client certificate, terminates against the same trust chain, and reaches the business logic with the same `AuthInformation` value as the equivalent SRPC or gRPC request. There are no protocol-specific bypass paths.

Concurrency and Rate Limiting
=============================

SRPC handles one RPC at a time per connection (via a `callLock` mutex) and caps the global connection pool to a few hundred by default, configured higher for some servers. The serial execution model and the bounded pool together cap aggregate throughput on every method. A small number of methods in the **fleet-manager** and the **Dominator** layer additional explicit per-user-method concurrency limits on top, via `lib/srpc/serverutil`.

gRPC uses HTTP/2 and multiplexes an unbounded number of concurrent RPCs per connection. The implicit protections SRPC relies on do not apply, and applying them would defeat the point of using gRPC. Protections are therefore made explicit, and they apply uniformly to SRPC, gRPC and REST so that quotas cannot be bypassed by switching protocols.

The `lib/srpc/serverutil` package provides three token-bucket limiters in addition to the per-user-method concurrency limiter, each guarding against a distinct failure mode:

- a **global** limiter that caps total admitted requests per second across all users and methods, as a server-wide safety net

- a **per-method** limiter that caps total requests per second for a given method across all users, configured only for expensive operations (such as `CreateVm` or `DestroyVm`) where the aggregate cost on the backend matters even when no single user is misbehaving

- a **per-user-per-method** limiter that caps how often a single identified user may invoke a given method. The default is one request per second per `(user, method)` pair: no method is expected to be called more often than that by any one user in normal operation. Per-method overrides raise the default for the rare cases that genuinely require a higher rate (for example, a polling or listing endpoint)

Limits are loaded from configuration so they can be tuned without code changes. An illustrative configuration is:

```yaml
rate_limits:
  global:
    requests_per_second: 10000
    burst: 20000
  per_method:
    CreateVm:  {requests_per_second: 20, burst:  40}
    DestroyVm: {requests_per_second: 20, burst:  40}
  per_user_per_method:
    default:
      requests_per_second: 1
      burst: 5
    overrides:
      ListVMs:    {requests_per_second: 10, burst: 20}
      GetMdbData: {requests_per_second: 10, burst: 20}
```

Schema Generation from Go Types
===============================

The Go structs in `messages.go` are the single source of truth for every protocol. The `cmd/proto-gen` tool parses these structs (using `go/ast`) and generates the `.proto` files, the protobuf-generated `pb/` packages and bidirectional converters between the internal Go structs and the `pb/` types.

A method or type is exposed to gRPC by adding a `@grpc` annotation to its doc comment and to the comments of its request and response types. Running `cmd/proto-gen` regenerates the proto and converters, and the generated files are committed alongside the change. Methods and types without the annotation are SRPC-only; clients of those methods are unaffected by the existence of the gRPC and REST surfaces.

A unary or server-streaming method that should not be exposed over REST (for example, an internal-only operation) can be opted out by writing `@grpc no-rest` instead of `@grpc`, leaving it reachable only via gRPC.

Method names are identical across SRPC, gRPC and REST, so a method renamed in one protocol is renamed in all of them. REST paths are generated rather than declared: each method that has a REST route is reachable at `/v1/<Service>/<Method>`, derived from the protobuf service and method names. No per-method routing annotations are needed.

Field numbers in generated `.proto` files are derived from struct field order by default. Stable numbers can be pinned with a `proto:"N"` struct tag; this is required when removing or reordering fields, and CI verifies that no existing field number changes silently.

Both the source Go structs and the generated artefacts (`.proto`, `pb/` and converters) are checked into the repository. Contributors do not need the `proto-gen` tool installed for an ordinary build, CI verifies that the committed generated files match what the tool would produce, and the diff of a change is visible in code review across all three protocols.

Type Converters
---------------

Business logic operates on the internal Go structs, while gRPC traffic is encoded as `pb/` types. The generated converters bridge the two:

| Go type (internal)               | Proto type     | Mechanism                                 |
| -------------------------------- | -------------- | ----------------------------------------- |
| `string`, `int`, `bool`          | same           | direct copy                               |
| `net.IP`, `net.HardwareAddr`     | `bytes`        | zero-cost cast: `[]byte(ip)`              |
| Embedded structs                 | flat fields    | auto-flattened by the generator           |
| `map[string][]string`            | nested message | hand-written extension method             |

For the small number of types that the generator cannot handle, a hand-written converter is placed alongside the generated file and picked up by name.

Error Handling
==============

gRPC and REST require status codes (e.g. `NotFound` maps to HTTP 404, `InvalidArgument` to HTTP 400), whereas SRPC carries errors as opaque strings. A typed-error interface in `lib/errors` exposes a gRPC status code; methods that need precise status semantics return errors that satisfy this interface. A helper in the gRPC server inspects each error: if it satisfies the interface, the code is taken from it; otherwise the helper falls back to a best-effort match on the error string. The fallback exists so that methods returning untyped errors can still be exposed without being rewritten first; methods are converted to typed errors as they are touched.

Per-method error fields in request and response structs (a field literally named `Error` of type `string`, used by some SRPC methods) are excluded from the generated `.proto`. gRPC and REST clients receive the same information via the standard status mechanism.

Metrics
=======

Per-method RPC metrics are published through the **tricorder** library: permitted call count, denied call count, successful call count and call-duration distributions. Each metric carries a `protocol` dimension (`srpc`, `grpc` or `rest`) so that overall and per-protocol QPS, error rates and latencies can be queried from a single source.

A counter, `rate_limit_denied_total{method, limit_type, protocol}`, records requests rejected by a rate limiter, where `limit_type` is one of `global`, `per_method` or `per_user_per_method`. Individual denials are also logged at debug level with the user identity, the method and the limit type, to support investigation of specific clients without inflating metric cardinality.
