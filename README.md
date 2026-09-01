# goliday — Chinese Public Holiday API Service

**English** | [简体中文](README-CN.md)

[![ci](https://github.com/JayceChant/goliday/actions/workflows/ci.yml/badge.svg)](https://github.com/JayceChant/goliday/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/JayceChant/goliday/graph/badge.svg)](https://codecov.io/gh/JayceChant/goliday)
[![Go Reference](https://pkg.go.dev/badge/github.com/JayceChant/goliday.svg)](https://pkg.go.dev/github.com/JayceChant/goliday)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/JayceChant/goliday/badge)](https://scorecard.dev/viewer/?uri=github.com/JayceChant/goliday)

A holiday lookup service built on Go 1.27: it records the holiday and workday-swap arrangements published by China's State Council in **sparse, per-year config files**, derives all other dates from standard-library weekend rules, and exposes semantically identical **HTTP and gRPC** interfaces supporting date-type queries and range statistics at both coarse and fine granularity.

## Core Design

- **Bitmask day types**: `DayType` (uint8) uses composable bit flags to express 5 fine-grained types (plain workday / swapped workday / weekend / festival / adjusted rest) plus 2 coarse-grained values (workday / holiday); fine→coarse mapping is pure bit arithmetic with priority rules (swapped workdays always count as workdays).
- **Sparse config**: one TOML file per year, containing only "adjusted" dates (`off` marks weekdays turned into rest days, `work` marks weekends turned into workdays); weekends and plain weekdays are derived from the weekday, never written down — a yearly file is only 20–30 lines and human-auditable.
- **Config-first with strict year validation**: for years with a config, the config wins; uncovered dates fall back to Saturday/Sunday checks. Queries touching a year whose config is not loaded return a `year_not_loaded` error instead of silently falling back, so "not configured" can never be misread as "actually a weekend".
- **Prefix-sum statistics**: per-year prefix sums over fine-grained flag counts are built at load time, making statistics an O(years-covered) difference — the `/stats` endpoint has no span limit.
- **Minimal dependency surface**: the core package depends only on `github.com/BurntSushi/toml`; the gRPC runtime is isolated to service entrypoints and generated-code packages. Zero frameworks, zero databases; configs are loaded once at startup into pure memory.

## Quick Start

Requires Go 1.27+.

```bash
# Start the server (HTTP :8080, gRPC :50051)
go run ./cmd/goliday-server -addr :8080 -grpc-addr :50051 -config-dir ./configs

# Single-day query (coarse granularity by default)
curl "http://localhost:8080/api/v1/days?date=2026-02-20"
# {"date":"2026-02-20","type":28,"type_label":"holiday","total_days":1,"stats":{"holiday":1,"workday":0}}

# Fine granularity: adjusted rest on festival day → festival|adjusted = 24
curl "http://localhost:8080/api/v1/days?date=2026-02-17&detailed=true"
# {"date":"2026-02-17","type":24,"type_label":"festival|adjusted",...}

# Range statistics (half-open interval)
curl "http://localhost:8080/api/v1/stats?start=2026-02-14&end=2026-02-17"
# {"mode":"range",...,"total_days":3,"stats":{"workday":1,"holiday":2}}

# gRPC (Go client)
conn, _ := grpc.NewClient("localhost:50051",
    grpc.WithTransportCredentials(insecure.NewCredentials()))
client := golidayv1.NewGolidayServiceClient(conn)
resp, _ := client.GetDay(ctx, &golidayv1.GetDayRequest{Date: "2026-02-17", Detailed: true})
// resp.Type == 24, resp.TypeLabel == "festival|adjusted"
```

> Examples are based on `configs/2026.toml` (a hypothetical sample, not official). For production use, generate the config from official announcements following the [annual config update process](#annual-config-update).

## Docker

Multi-stage build: compiled as a static binary (`CGO_ENABLED=0`), the runtime image is `gcr.io/distroless/static-debian12:nonroot` (no shell, no package manager) and contains only the server binary. Year configs are **not** baked into the image — mount them at runtime.

```bash
# Build locally
docker build -t goliday .

# Run: HTTP :8080, gRPC :50051; mount the config directory read-only
docker run -p 8080:8080 -v $PWD/configs:/data:ro goliday -config-dir /data

curl "http://localhost:8080/healthz"
# {"status":"ok","years":[2025,2026]}
```

Images are published to GHCR by [GitHub Actions](.github/workflows/docker.yml) on every push to the default branch and every `v*` tag (multi-arch `linux/amd64` + `linux/arm64`):

```bash
docker pull ghcr.io/jaycechant/goliday:latest
```

## API Overview

| Protocol | Endpoint | Description |
|---|---|---|
| HTTP | `GET /api/v1/days` | Single-day / range (half-open) / discrete / mixed-union queries with per-day details; range span ≤366 days |
| HTTP | `GET /api/v1/stats` | Same statistics as `/days`, without details; no span limit |
| HTTP | `GET /healthz` | Health check, returns loaded years |
| gRPC | `GolidayService` | `GetDay` / `QueryDays` / `QueryStats`, one-to-one with HTTP; standard gRPC health checking also registered |

Day-type bitmask (`type_label` is exactly `DayType.String()`; combinations are joined by `|` from low to high bits):

| Value | Meaning | | Value | Meaning |
|---|---|---|---|---|
| 1 | Plain workday | | 3 | **Coarse: workday** (1\|2) |
| 2 | Swapped workday | | 28 | **Coarse: holiday** (4\|8\|16) |
| 4 | Weekend | | 6 | Swapped workday on weekend |
| 8 | Festival | | 12 | Festival on weekend |
| 16 | Adjusted rest | | 24 | Adjusted rest on festival day |

Fine-grained statistics (`detailed=true`) cross-count single flags: a combined day counts 1 day toward each of its flags, so the sum of keys may exceed `total_days`; for total rest/workday days use the coarse `stats.holiday`/`stats.workday`.

Full contract (parameters, response structures, error codes, gRPC examples, proto regeneration) in [docs/API.md](docs/API.md) (Chinese).

## Annual Config Update

Around November each year, after the State Council publishes next year's arrangement:

1. Generate a draft from the official announcement: `go run ./cmd/goliday-tool gen -year 2027 -out configs/2027.toml -file announcement.txt` (or use the LLM prompt template in [docs/generate_prompt.md](docs/generate_prompt.md));
2. Validate: `go run ./cmd/goliday-tool validate configs/2027.toml`;
3. Spot-check several dates manually, place the file into `configs/`, restart the service and confirm via `/healthz` that the year is loaded.

Config format (rationale, field semantics, validation rules, determination algorithm) in [docs/CONFIG_FORMAT.md](docs/CONFIG_FORMAT.md) (Chinese); fully annotated example in [docs/holiday_config_example.toml](docs/holiday_config_example.toml).

## Architecture & Dependencies

```
cmd/goliday-server (HTTP + gRPC entry)   cmd/goliday-tool (gen/validate)
        │                                      │
        └────────────► root package goliday (core lib) ◄──┘
     daytype (bitmask) → config (TOML) → store (per-year loading) → calendar (determination/range/stats)
```

Dependencies are tiered per package: the root package uses only `BurntSushi/toml` (zero gRPC imports); the gRPC trio is confined to the `proto/goliday/v1/` generated-code package and `cmd/` subpackages. See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) (Chinese).

## Development

```bash
go build ./... && go vet ./... && go test -count=1 ./... && gofmt -l .
```

Test data: `testdata/2025.toml` (real official plan), `testdata/2026.toml` (hypothetical sample), `testdata/invalid/` (invalid samples).

## Quality & CI

| Service | Result | Notes |
|---|---|---|
| [GitHub Actions](https://github.com/JayceChant/goliday/actions/workflows/ci.yml) | [![ci](https://github.com/JayceChant/goliday/actions/workflows/ci.yml/badge.svg)](https://github.com/JayceChant/goliday/actions/workflows/ci.yml) | Go 1.27.x (go.mod minimum) + stable version matrix: build, vet, gofmt, `go test -race`, plus a zero-warning golangci-lint job ([workflow](.github/workflows/ci.yml)) |
| [Codecov](https://codecov.io/gh/JayceChant/goliday) | [![codecov](https://codecov.io/gh/JayceChant/goliday/graph/badge.svg)](https://codecov.io/gh/JayceChant/goliday) | Coverage uploaded from `go test -coverprofile`, line-by-line coverage browsing |
| [pkg.go.dev](https://pkg.go.dev/github.com/JayceChant/goliday) | [![Go Reference](https://pkg.go.dev/badge/github.com/JayceChant/goliday.svg)](https://pkg.go.dev/github.com/JayceChant/goliday) | Official Go doc build & import check, refreshed automatically per module version |
| [OpenSSF Scorecard](https://scorecard.dev/viewer/?uri=github.com/JayceChant/goliday) | [![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/JayceChant/goliday/badge)](https://scorecard.dev/viewer/?uri=github.com/JayceChant/goliday) | Automated repo security-practice score, runs weekly, SARIF synced to code scanning ([workflow](.github/workflows/scorecard.yml)) |

## Documentation Index

| Doc | Contents |
|---|---|
| [docs/API.md](docs/API.md) | Full HTTP & gRPC contract, bitmask reference, examples (Chinese) |
| [docs/CONFIG_FORMAT.md](docs/CONFIG_FORMAT.md) | Config format rationale, sparse-table principles, validation rules, algorithm (Chinese) |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Directory layout, layering, dependency constraints, data flow (Chinese) |
| [docs/generate_prompt.md](docs/generate_prompt.md) | LLM prompt template: official announcement → yearly config (Chinese) |
| [spec/](spec/) | Requirements spec, task list and acceptance checklist (see [AGENTS.md](AGENTS.md)) |
