# Customize Error Handler Demo

A minimal Aurora app that demonstrates **custom error response format** via `contracts.ErrorHandler` and `feature.WithErrorHandler`.

Only the Server feature is registered; all handler errors are rendered with a custom JSON shape (`code`, `message`, `error`, `timestamp`, `custom`).

## Run

This demo uses only the Server feature, so only Server-related environment variables are needed. Aurora’s config loader does not use struct `envDefault`; every variable must be set explicitly.

Run from the **sample** directory (e.g. `cd sample` from the Aurora repo root). If Aurora lives inside another repo that has a `go.work` file, use **`GOWORK=off`** so this sample’s `go.mod` is used (otherwise Go may pick the parent module and fail).

### Environment variables (all required)

| Variable | Description |
|----------|-------------|
| `SERVICE_NAME` | Service name |
| `SERVICE_VERSION` | Service version |
| `RUN_LEVEL` | `local` / `stage` / `production` |
| `HOST` | Listen address (e.g. `0.0.0.0`) |
| `PORT` | Listen port (e.g. `8080`) |
| `READ_TIMEOUT` | Read timeout (e.g. `30s`) |
| `WRITE_TIMEOUT` | Write timeout (e.g. `30s`) |
| `IDLE_TIMEOUT` | Idle timeout (e.g. `60s`) |
| `SHUTDOWN_TIMEOUT` | Graceful shutdown timeout (e.g. `5s`) |

### One-line run (copy-paste)

```bash
cd sample

GOWORK=off SERVICE_NAME=customize_error_handler SERVICE_VERSION=1.0.0 RUN_LEVEL=local HOST=0.0.0.0 PORT=8080 READ_TIMEOUT=30s WRITE_TIMEOUT=30s IDLE_TIMEOUT=60s SHUTDOWN_TIMEOUT=5s go run ./customize_error_handler
```

### Using exports (or `.env` + `source`)

```bash
cd sample

export SERVICE_NAME=customize_error_handler
export SERVICE_VERSION=1.0.0
export RUN_LEVEL=local
export HOST=0.0.0.0
export PORT=8080
export READ_TIMEOUT=30s
export WRITE_TIMEOUT=30s
export IDLE_TIMEOUT=60s
export SHUTDOWN_TIMEOUT=5s
GOWORK=off go run ./customize_error_handler
```

## Test

- **Success**  
  `GET http://localhost:8080/ok`  
  Response: `{"message":"success","status":"ok"}`

- **Custom error (500)**  
  `GET http://localhost:8080/err`  
  Response body (example):  
  `{"code":500,"message":"...","error":"...","timestamp":...,"custom":true}`

- **Custom error (400)**  
  `GET http://localhost:8080/err-bad-request`  
  Same shape, with `code: 400`.

Without `WithErrorHandler`, the framework uses the default `{"message":"..."}` format; this demo shows how to control the full error JSON via a custom handler.
