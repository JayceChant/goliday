# goliday — Chinese Public Holiday API Service

**English** | [简体中文](README-CN.md)

[![ci](https://github.com/JayceChant/goliday/actions/workflows/ci.yml/badge.svg)](https://github.com/JayceChant/goliday/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/JayceChant/goliday/graph/badge.svg)](https://codecov.io/gh/JayceChant/goliday)
[![CodeQL](https://github.com/JayceChant/goliday/actions/workflows/codeql.yml/badge.svg)](https://github.com/JayceChant/goliday/actions/workflows/codeql.yml)
[![govulncheck](https://github.com/JayceChant/goliday/actions/workflows/govulncheck.yml/badge.svg)](https://github.com/JayceChant/goliday/actions/workflows/govulncheck.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/JayceChant/goliday.svg)](https://pkg.go.dev/github.com/JayceChant/goliday)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/JayceChant/goliday/badge)](https://scorecard.dev/viewer/?uri=github.com/JayceChant/goliday)
[![SonarCloud](https://sonarcloud.io/api/project_badges/quality_gate?project=JayceChant_goliday)](https://sonarcloud.io/summary/new_code?id=JayceChant_goliday)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A holiday lookup service built on Go 1.27: it records the holiday and workday-swap arrangements published by China's State Council in **sparse, per-year config files**, derives all other dates from standard-library weekend rules, and exposes semantically identical **HTTP and gRPC** interfaces supporting date-type queries and range statistics at both coarse and fine granularity.

## Core Design

- **Two-tier final-state bitmask**: `DayType` (uint8) uses 2 mutually exclusive **base bits** (1=work, 2=rest) for coarse granularity, plus 3 mutually exclusive **adjustment bits** (4=festival, 8=adjusted rest, 16=compensating workday) combining into 5 fine-grained values (1/2/6/10/17); `|` is all-of (conjunction) semantics across all values, and coarse membership is a single bit-and (`t & 2` rest, `t & 1` work) with no priority disambiguation.
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
# {"date":"2026-02-20","type":2,"type_label":"rest","total_days":1,"stats":{"holiday":1,"workday":0}}

# Fine granularity: festival day → rest|festival = 6
curl "http://localhost:8080/api/v1/days?date=2026-02-17&detailed=true"
# {"date":"2026-02-17","type":6,"type_label":"rest|festival",...}

# Range statistics (half-open interval)
curl "http://localhost:8080/api/v1/stats?start=2026-02-14&end=2026-02-17"
# {"mode":"range",...,"total_days":3,"stats":{"workday":1,"holiday":2}}

# gRPC (Go client)
conn, _ := grpc.NewClient("localhost:50051",
    grpc.WithTransportCredentials(insecure.NewCredentials()))
client := golidayv1.NewGolidayServiceClient(conn)
resp, _ := client.GetDay(ctx, &golidayv1.GetDayRequest{Date: "2026-02-17", Detailed: true})
// resp.Type == 6, resp.TypeLabel == "rest|festival"
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

Images are published to GHCR by [GitHub Actions](.github/workflows/docker.yml) on every `v*` tag push (multi-arch `linux/amd64` + `linux/arm64`; pushes to the default branch, PRs and manual runs build for verification only, without publishing):

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

Day-type bitmask (`type_label` is exactly `DayType.String()`: legal values look up a static label table, combo labels join two segments with `|`, illegal values yield `invalid`; `|` is all-of semantics across all values, coarse granularity is the base-bit projection):

| Value | Combination | Meaning | | Value | Combination | Meaning |
|---|---|---|---|---|---|---|
| 1 | `Work` | Plain workday | | 6 | `FestivalRest` | Festival rest day |
| 2 | `Rest` | Plain weekend | | 10 | `AdjustedRestDay` | Adjusted rest day (ex-weekday) |
| 4 | — | Adjustment bit: festival | | 17 | `AdjustedWorkDay` | Compensating workday (ex-weekend) |
| 8 | — | Adjustment bit: adjusted rest | | | | |
| 16 | — | Adjustment bit: compensating work | | | | |

Coarse granularity is the base bit itself: with `detailed=false`, `type` is 1 (work) or 2 (rest); membership is just `t & 1` / `t & 2`.

Fine-grained statistics (`detailed=true`) use five MECE keys — plain workday (`ordinary`), plain weekend (`weekend`), festival rest day (`festival`), adjusted rest day (`adjusted_rest`), compensating workday (`adjusted_work`) — each counting exactly one class of day, so **the keys always sum to `total_days`**; for total rest/workday days use the coarse `stats.holiday`/`stats.workday`.

Semantics: "festivals" here are public holidays that grant time off (non-rest commemorative days are out of scope); "adjusted rest" is narrow (an ex-weekday turned into rest by arrangement, not the festival day itself — no new holiday), while a festival day always adds one new holiday; whether a festival falls on a weekday or weekend, the fine-grained type is the same `rest|festival`.

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

## Documentation Index

| Doc | Contents |
|---|---|
| [docs/API.md](docs/API.md) | Full HTTP & gRPC contract, bitmask reference, examples (Chinese) |
| [docs/CONFIG_FORMAT.md](docs/CONFIG_FORMAT.md) | Config format rationale, sparse-table principles, validation rules, algorithm (Chinese) |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Directory layout, layering, dependency constraints, data flow, testing layers, quality gates & CI (Chinese) |
| [docs/generate_prompt.md](docs/generate_prompt.md) | LLM prompt template: official announcement → yearly config (Chinese) |
| [spec/](spec/) | Requirements spec, task list and acceptance checklist (see [AGENTS.md](AGENTS.md)) |
