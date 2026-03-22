# zipweathertracing

A distributed system in Go composed of two microservices — `gateway` and `orchestrator` — that cooperate to return the current weather for a Brazilian ZIP code (CEP). The full request flow across both services is observable via **OpenTelemetry** and **Zipkin**.

---

## Table of Contents

- [Architecture](#architecture)
- [API Contract](#api-contract)
- [Running with Docker Compose](#running-with-docker-compose)
- [Accessing Zipkin](#accessing-zipkin)
- [Running Tests](#running-tests)
- [Makefile](#makefile)
- [Environment Variables](#environment-variables)

---

## Architecture

```
Client
  │  POST / {"cep": "01001000"}
  ▼
gateway :8080
  - validates CEP format
  - propagates W3C TraceContext
  │  POST / {"cep": "01001000"} + traceparent header
  ▼
orchestrator :8081
  - resolves city via ViaCEP       → span: viacep.lookup
  - fetches temperature             → span: weatherapi.fetch
  - converts C / F / K
  │
  ▼
OTEL Collector :4317
  - receives OTLP gRPC from both services
  - exports to Zipkin
  │
  ▼
Zipkin :9411
  - visualizes the full distributed trace
```

**Trace hierarchy visible in Zipkin:**

```
gateway: POST /
  └── orchestrator: POST /
        ├── viacep.lookup
        └── weatherapi.fetch
```

---

## API Contract

Send requests to the **gateway** on port `8080`.

### `POST /`

**Request body:**
```json
{"cep": "01001000"}
```

The `cep` field must be a string with exactly 8 numeric digits.

---

**`200 OK` — success**

```bash
curl -s -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -d '{"cep": "01001000"}'
```

```json
{"city": "São Paulo", "temp_C": 28.5, "temp_F": 83.3, "temp_K": 301.65}
```

**`422 Unprocessable Entity` — invalid CEP format**

```bash
curl -i -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -d '{"cep": "123"}'
```

```
HTTP/1.1 422 Unprocessable Entity
invalid zipcode
```

**`404 Not Found` — CEP not found**

```bash
curl -i -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -d '{"cep": "99999999"}'
```

```
HTTP/1.1 404 Not Found
can not find zipcode
```

---

## Running with Docker Compose

**Prerequisites:** Docker and a free API key from [WeatherAPI](https://www.weatherapi.com).

```bash
# 1. Copy the environment file and add your API key
cp .env.example .env

# 2. Start the full stack
make up
```

| Service | Role | Port |
|---|---|---|
| `gateway` | Input validation and proxy | `8080` |
| `orchestrator` | Business logic | internal |
| `otel-collector` | Telemetry pipeline | internal |
| `zipkin` | Trace visualization | `9411` |

```bash
make down   # stop all containers
make logs   # follow logs
```

---

## Accessing Zipkin

After `make up`, open **http://localhost:9411** in your browser.

1. Click **Run Query** (or press Enter)
2. Select any trace to see the full span tree:
   - `gateway` — HTTP handler span
   - `orchestrator` — child span, propagated from gateway
   - `viacep.lookup` — time spent calling ViaCEP
   - `weatherapi.fetch` — time spent calling WeatherAPI

---

## Running Tests

```bash
make test             # both services, with race detector
make test-coverage    # both services, opens HTML coverage report
```

---

## Makefile

| Command | Description |
|---|---|
| `make up` | Build images and start the full stack |
| `make down` | Stop and remove all containers |
| `make logs` | Follow logs from all containers |
| `make build` | Build Docker images for both services |
| `make test` | Run all tests with race detector |
| `make test-coverage` | Run tests and open HTML coverage report |
| `make lint` | Run `go vet ./...` on both services |
| `make clean` | Remove compiled binaries and coverage files |

---

## Environment Variables

**Root `.env`** (read by Docker Compose):

| Variable | Required |
|---|---|
| `WEATHERAPI_KEY` | Yes |

Copy `.env.example` to `.env`. Never commit `.env`.

**Gateway:**

| Variable | Default |
|---|---|
| `PORT` | `8080` |
| `ORCHESTRATOR_URL` | `http://localhost:8081` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` |

**Orchestrator:**

| Variable | Default |
|---|---|
| `PORT` | `8081` |
| `WEATHERAPI_KEY` | required |
| `VIACEP_BASE_URL` | `https://viacep.com.br` |
| `WEATHERAPI_BASE_URL` | `https://api.weatherapi.com` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` |
