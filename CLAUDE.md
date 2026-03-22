# CLAUDE.md

## Context

Distributed system in Go with two microservices (`gateway` and `orchestrator`) that cooperate to return weather data for a Brazilian ZIP code (CEP). The defining feature is **distributed tracing** via OpenTelemetry and Zipkin.

---

## Architecture boundaries

```
gateway/
├── cmd/server/main.go        # OTEL bootstrap, wiring, server start
└── internal/handler/         # POST /: validate CEP, proxy to orchestrator

orchestrator/
├── cmd/server/main.go        # OTEL bootstrap, wiring, server start
└── internal/
    ├── domain/               # types, sentinel errors, pure functions — zero I/O
    ├── ports/                # LocationPort, WeatherPort interfaces
    └── adapters/
        ├── http/             # primary: HTTP handler
        ├── viacep/           # secondary: ViaCEP client + manual span
        └── weatherapi/       # secondary: WeatherAPI client + manual span
```

**Dependency rule (orchestrator only):**
```
adapters → ports + domain
ports    → domain
domain   → nothing
main     → everything (only place with concrete wiring)
```

The `gateway` has no hexagonal architecture — it is a thin proxy and abstractions would be over-engineering.

---

## Coding rules — never do

- Never use `panic` in application code
- Never ignore errors — handle all explicitly
- Never hardcode configuration values — use environment variables
- Never write business logic inside adapters
- Never use `http.DefaultClient` without an explicit timeout
- Never leak internal errors to clients — return only contract messages (`invalid zipcode`, `can not find zipcode`)
- Never use `context.Background()` in secondary adapters — always propagate the received context
- Never add external dependencies without clear necessity — prefer stdlib
- Never use an HTTP framework (Gin, Echo, Chi, etc.)
- Never commit `.env`

---

## OpenTelemetry rules — never do

- Never call `otel.GetTracerProvider()` inside adapters — inject `trace.Tracer` via constructor
- Never create a span without `defer span.End()`
- Never start a span without using the returned `ctx` in subsequent calls
- Never wrap the HTTP mux without `otelhttp.NewHandler` — this is how server spans are created
- Never use a plain `http.Client` in the gateway — wrap with `otelhttp.NewTransport` for trace propagation

**Span names:** `"viacep.lookup"`, `"weatherapi.fetch"`
**Service names:** `"gateway"`, `"orchestrator"`
**Propagation:** W3C TraceContext (`traceparent` header)

---

## Context propagation chain

```
r.Context() [gateway handler]
  → http.NewRequestWithContext [gateway → orchestrator, via otelhttp.NewTransport]
    → r.Context() [orchestrator handler]
      → ports.GetLocation(ctx) → viacep adapter creates child span
      → ports.GetTemperature(ctx) → weatherapi adapter creates child span
```

Every I/O call must carry the request context. Cancellation propagates: client disconnects → context cancelled → in-flight external calls abort.

---

## Naming conventions

- Ports: `LocationPort`, `WeatherPort`
- Adapters: `ViaCEPClient`, `WeatherAPIClient`
- Constructors: `New{Type}`
- Sentinel errors: `ErrNotFound`, `ErrInvalidCEP`
- Logging: `slog` with JSON handler — never `fmt.Println`

---

## Testing

Write tests whenever adding or changing behaviour. Tests live next to the code they cover (`_test.go` in the same package, using the `_test` external-package convention).

### What to test and how

| Layer | Tool | Notes |
|---|---|---|
| `domain/` | pure unit tests | no I/O, no mocks — just call the function and assert |
| `orchestrator/adapters/http` | `httptest.NewRecorder` + mock ports | mock `LocationPort` and `WeatherPort` via local structs |
| `orchestrator/adapters/viacep` | `httptest.NewServer` + noop tracer | fake the ViaCEP HTTP server; use `noop.NewTracerProvider().Tracer("")` |
| `orchestrator/adapters/weatherapi` | `httptest.NewServer` + noop tracer | fake the WeatherAPI HTTP server; use `noop.NewTracerProvider().Tracer("")` |
| `gateway/internal/handler` | `httptest.NewServer` (fake orchestrator) | start a fake orchestrator; validate CEP rejection without hitting the network |

### Noop tracer for adapter tests

Secondary adapters receive a `trace.Tracer` via constructor. In tests, always pass a noop tracer — never a real SDK tracer:

```go
import "go.opentelemetry.io/otel/trace/noop"

tracer := noop.NewTracerProvider().Tracer("")
client := viacep.NewClient(srv.URL, httpClient, tracer)
```

### Rules

- Never mock the real external APIs (ViaCEP, WeatherAPI) by pointing at them in tests — always use `httptest.NewServer`
- Never skip tests because a feature "obviously works" — if you add behaviour, add a test
- Tests for adapters must cover: success path, non-OK HTTP status, context cancellation
- Tests for handlers must cover: success, invalid CEP (too short, non-numeric, bad JSON), not found (404), upstream error (500)

---

## Git workflow

- One branch per feature, from `main` — prefix: `feat/`, `fix/`, `test/`, `chore/`, `docs/`
- Atomic commits; present to the user for approval before committing
- `git add <specific files>` — never `git add .`
- Commit messages: English, Conventional Commits (`feat:`, `fix:`, `chore:`, etc.)
- One logical change per commit — never mention Claude or AI in commit messages
- Merge to `main` only after explicit user approval

---

## Commit checklist

- [ ] All errors handled explicitly
- [ ] No hardcoded values — everything from environment variables
- [ ] `context.Context` propagated through the full call chain
- [ ] `defer span.End()` on every manually created span
- [ ] `otelhttp.NewTransport` on the gateway's HTTP client
- [ ] `otelhttp.NewHandler` wrapping each service's mux
- [ ] Tracer injected via constructor — not via `otel.GetTracerProvider()`
- [ ] `domain/` has zero external imports
- [ ] Orchestrator handler depends only on port interfaces, never concrete adapters
- [ ] Tests added or updated for the changed behaviour
- [ ] `go test ./...` passes in both `gateway/` and `orchestrator/`
- [ ] `go vet ./...` passes in both `gateway/` and `orchestrator/`
- [ ] `go mod tidy` run in both service directories
- [ ] `.env` is not staged
