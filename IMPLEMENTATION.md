# Implementation Notes

Architectural decisions, tradeoffs, and non-obvious implementation details.

---

## Why two separate services?

The `gateway` exists to decouple input validation from business logic. In a real system this boundary allows each service to evolve, scale, and be deployed independently. For this project it also creates the multi-service trace that demonstrates distributed tracing.

---

## Why gateway has no hexagonal architecture

The gateway does exactly one thing: validate a CEP and proxy the request. Introducing ports, adapters, and domain layers for a single HTTP pass-through would be premature abstraction. A `Handler` struct with an HTTP client is all it needs.

---

## Why orchestrator uses hexagonal architecture

The orchestrator has meaningful domain logic (city resolution, temperature conversion, error mapping) that is completely independent of HTTP and external APIs. Hexagonal architecture keeps the domain testable in isolation and makes it trivial to swap the ViaCEP or WeatherAPI adapters without touching business rules.

---

## Why services export to an OTEL Collector instead of Zipkin directly

The Collector decouples the services from the observability backend. If the team wants to switch from Zipkin to Jaeger, Grafana Tempo, or send data to multiple backends simultaneously, only the Collector configuration changes — the services are untouched. This is the standard production pattern for OTEL deployments.

The `otel/opentelemetry-collector-contrib` image is required (not the base `otel/opentelemetry-collector`) because the Zipkin exporter is only available in the contrib distribution.

---

## Why W3C TraceContext and not B3

W3C TraceContext (`traceparent` header) is the current IETF standard and the default in the OpenTelemetry Go SDK. B3 is a legacy format from Zipkin/Brave. Zipkin supports W3C natively via the OTEL Collector, so there is no reason to use B3.

---

## How context propagation works end-to-end

The `otelhttp.NewHandler` middleware on each service server extracts the `traceparent` header from incoming requests and attaches the parent span to `r.Context()`. Every downstream call that receives this context becomes a child span automatically.

On the outbound side, `otelhttp.NewTransport` on the gateway's HTTP client reads the active span from the context and injects `traceparent` into the outgoing request headers before it reaches the orchestrator.

The key rule is that `r.Context()` must be passed to every I/O call — this is what carries the span through the chain.

---

## Why the tracer is injected via constructor, not via `otel.Tracer()`

Calling `otel.Tracer()` inside a struct method creates a hidden dependency on the global OTEL state, making it impossible to test the adapter without a real TracerProvider. Injecting `trace.Tracer` via the constructor allows tests to pass a `noop.NewTracerProvider().Tracer("")` — zero infrastructure required.

---

## Why spans are created in adapters, not in the handler

The handler knows only port interfaces. Creating spans in the handler would mean the handler needs to know about OTEL, violating the hexagonal boundary. The adapters are the correct location: they are the ones performing I/O and can instrument their own operations.

---

## ViaCEP response quirk

ViaCEP returns HTTP 200 for every request, including non-existent CEPs. A missing CEP is signaled by a `"erro": "true"` field in the JSON body — not by an error status code. The adapter must check this field explicitly and map it to `domain.ErrNotFound`.

---

## WeatherAPI city encoding

City names returned by ViaCEP may contain accented characters (`São Paulo`) or spaces. These must be URL-encoded before being used as a query parameter in the WeatherAPI request — otherwise the request fails or returns incorrect results.

---

## Graceful shutdown and trace flushing

Both services implement `signal.NotifyContext` to handle `SIGTERM`. The `TracerProvider.Shutdown` must be deferred in `main.go` — the `BatchSpanProcessor` queues spans asynchronously, and without an explicit shutdown the last batch is lost when the process exits.
