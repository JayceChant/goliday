# goliday — 中国法定节假日 API 服务

[English](README.md) | **简体中文**

[![ci](https://github.com/JayceChant/goliday/actions/workflows/ci.yml/badge.svg)](https://github.com/JayceChant/goliday/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/JayceChant/goliday/graph/badge.svg)](https://codecov.io/gh/JayceChant/goliday)
[![CodeQL](https://github.com/JayceChant/goliday/actions/workflows/codeql.yml/badge.svg)](https://github.com/JayceChant/goliday/actions/workflows/codeql.yml)
[![govulncheck](https://github.com/JayceChant/goliday/actions/workflows/govulncheck.yml/badge.svg)](https://github.com/JayceChant/goliday/actions/workflows/govulncheck.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/JayceChant/goliday.svg)](https://pkg.go.dev/github.com/JayceChant/goliday)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/JayceChant/goliday/badge)](https://scorecard.dev/viewer/?uri=github.com/JayceChant/goliday)
[![SonarCloud](https://sonarcloud.io/api/project_badges/quality_gate?project=JayceChant_goliday)](https://sonarcloud.io/summary/new_code?id=JayceChant_goliday)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

基于 Go 1.27 的节假日查询服务：以**按年组织的稀疏配置文件**记录国务院假日办公布的放假与补班安排，其余日期由标准库周休规则推导，对外提供语义一致的 **HTTP 与 gRPC 双协议**接口，支持粗/细两级粒度的日期类型查询与区间统计。

## 核心设计

- **终态双层位掩码**：`DayType`（uint8）以 2 个互斥**基本位**（1=上班、2=放假）表达粗粒度，3 个互斥**调整位**（4=过节、8=调休、16=补班）组合出 5 种细粒度值（1/2/6/10/17）；全部组合的 `|` 均为 all-of（合取）语义，粗细归属用单次按位与判定（`t & 2` 放假、`t & 1` 上班），无优先级消歧。
- **稀疏配置**：每年一个 TOML 文件，只记录"被调整过"的日期（`off` 仅工作日变休息、`work` 仅周末变上班），周末与普通工作日由标准库按星期推导、一律不写入——年文件仅 20~30 行，可人工审计。
- **配置优先 + 年份强校验**：有配置年份以配置为准，未覆盖日期回退周六/周日判断；查询覆盖未加载配置的年份时返回 `year_not_loaded` 错误而非静默回退，避免"未配置"被误读为"真实周末"。
- **前缀和统计**：加载时为每年构建细粒度组合计数前缀和，统计为 O(覆盖年数) 差分，`/stats` 接口不限查询跨度。
- **极小依赖面**：核心包仅依赖 `github.com/BurntSushi/toml`；gRPC 运行时隔离在服务入口与生成代码包中。零框架、零数据库，配置启动时一次性加载纯内存。

## 快速开始

环境要求：Go 1.27+。

```bash
# 启动（HTTP :8080，gRPC :50051）
go run ./cmd/goliday-server -addr :8080 -grpc-addr :50051 -config-dir ./configs

# 单日查询（默认粗粒度）
curl "http://localhost:8080/api/v1/days?date=2026-02-20"
# {"date":"2026-02-20","type":2,"type_label":"rest","total_days":1,"stats":{"holiday":1,"workday":0}}

# 细粒度：节日当天 → rest|festival = 6
curl "http://localhost:8080/api/v1/days?date=2026-02-17&detailed=true"
# {"date":"2026-02-17","type":6,"type_label":"rest|festival",...}

# 区间统计（左闭右开）
curl "http://localhost:8080/api/v1/stats?start=2026-02-14&end=2026-02-17"
# {"mode":"range",...,"total_days":3,"stats":{"workday":1,"holiday":2}}

# gRPC（Go 客户端）
conn, _ := grpc.NewClient("localhost:50051",
    grpc.WithTransportCredentials(insecure.NewCredentials()))
client := golidayv1.NewGolidayServiceClient(conn)
resp, _ := client.GetDay(ctx, &golidayv1.GetDayRequest{Date: "2026-02-17", Detailed: true})
// resp.Type == 6, resp.TypeLabel == "rest|festival"
```

> 示例基于 `configs/2026.toml`（假设示例方案，非官方）。正式使用请按[年度配置更新流程](#年度配置更新)以官方公告生成。

## Docker

多阶段构建：以静态二进制编译（`CGO_ENABLED=0`），运行镜像基于 `gcr.io/distroless/static-debian12:nonroot`（无 shell、无包管理器），仅包含 server 二进制。年份配置**不打入镜像**，运行时挂载。

```bash
# 本地构建
docker build -t goliday .

# 运行：HTTP :8080，gRPC :50051；配置目录只读挂载
docker run -p 8080:8080 -v $PWD/configs:/data:ro goliday -config-dir /data

curl "http://localhost:8080/healthz"
# {"status":"ok","years":[2025,2026]}
```

镜像由 [GitHub Actions](.github/workflows/docker.yml) 在推送 `v*` tag 时自动发布至 GHCR（多架构 `linux/amd64` + `linux/arm64`；默认分支、PR 与手动触发仅做构建验证，不发布）：

```bash
docker pull ghcr.io/jaycechant/goliday:latest
```

## API 概览

| 协议 | 入口 | 说明 |
|---|---|---|
| HTTP | `GET /api/v1/days` | 单日 / 区间（左闭右开）/ 离散 / 混合并集查询，含逐日明细；区间跨度 ≤366 天 |
| HTTP | `GET /api/v1/stats` | 与 `/days` 统计口径一致，无明细；不限跨度 |
| HTTP | `GET /healthz` | 健康检查，返回已加载年份 |
| gRPC | `GolidayService` | `GetDay` / `QueryDays` / `QueryStats`，与 HTTP 一一对应，另注册 gRPC 标准健康检查 |

日期类型掩码（`type_label` 即 `DayType.String()`：合法值查静态标签表直返，组合值两段以 `|` 连接，非法值统一输出 `invalid`；`|` 在全部取值上均为 all-of 语义，粗粒度即基本位投影）：

| 值 | 组合 | 含义 | | 值 | 组合 | 含义 |
|---|---|---|---|---|---|---|
| 1 | `Work` | 普通工作日 | | 6 | `FestivalRest` | 节日放假日 |
| 2 | `Rest` | 普通周休 | | 10 | `AdjustedRestDay` | 调休放假日（原工作日） |
| 4 | — | 调整位：过节 | | 17 | `AdjustedWorkDay` | 补班日（原周末） |
| 8 | — | 调整位：调休 | | | | |
| 16 | — | 调整位：补班 | | | | |

粗粒度即基本位本身：`detailed=false` 时 `type` 为 1（上班）或 2（放假），判类仅需 `t & 1` / `t & 2`。

细粒度统计（`detailed=true`）为五键 MECE 计数：普通工作日（`ordinary`）/普通周休（`weekend`）/节日放假日（`festival`）/调休放假日（`adjusted_rest`）/补班日（`adjusted_work`）各计一类日，**各键之和恒等于 `total_days`**；总休息/上班天数请用粗粒度 `stats.holiday`/`stats.workday`。

语义约定：本项目"节日"均指产生法定假期的全体公民节日（不放假的纪念日不纳入）；"调休"为窄义（原工作日因安排变休息且非节日当天，不新增假期），"过节"当日新增假期；节日无论落在工作日还是周末，细粒度同为 `rest|festival`。

完整契约（参数、响应结构、错误码、gRPC 调用示例、proto 再生成命令）见 [docs/API.md](docs/API.md)。

## 年度配置更新

每年 11 月左右国务院公布次年安排后：

1. 用官方公告生成草稿：`go run ./cmd/goliday-tool gen -year 2027 -out configs/2027.toml -file 公告.txt`（或使用 [docs/generate_prompt.md](docs/generate_prompt.md) 的 LLM 提示词模板）；
2. 校验：`go run ./cmd/goliday-tool validate configs/2027.toml`；
3. 人工抽查若干日期后放入 `configs/`，重启服务并经 `/healthz` 确认年份已加载。

配置格式（选型依据、字段语义、校验规则、判定算法）见 [docs/CONFIG_FORMAT.md](docs/CONFIG_FORMAT.md)，完整注释示例见 [docs/holiday_config_example.toml](docs/holiday_config_example.toml)。

## 架构与依赖

```
cmd/goliday-server（HTTP + gRPC 入口）   cmd/goliday-tool（gen/validate）
        │                                      │
        └────────────► 根包 goliday（核心库）◄──┘
     daytype（掩码）→ config（TOML）→ store（按年装载）→ calendar（判定/区间/统计）
```

依赖按包分级：根包仅 `BurntSushi/toml`（零 gRPC 导入）；gRPC 三件套仅限 `proto/goliday/v1/` 生成代码包与 `cmd/` 子包。详见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)。

## 开发

```bash
go build ./... && go vet ./... && go test -count=1 ./... && gofmt -l .
```

测试数据：`testdata/2025.toml`（真实官方方案）、`testdata/2026.toml`（假设示例）、`testdata/invalid/`（非法样例）。

## 文档索引

| 文档 | 内容 |
|---|---|
| [docs/API.md](docs/API.md) | HTTP 与 gRPC 完整契约、掩码对照、调用示例 |
| [docs/CONFIG_FORMAT.md](docs/CONFIG_FORMAT.md) | 配置格式选型、稀疏表原则、校验规则、判定算法 |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | 目录结构、分层、依赖约束、数据流、测试分层、质量门禁与 CI |
| [docs/generate_prompt.md](docs/generate_prompt.md) | 官方公告 → 年度配置的 LLM 提示词模板 |
| [spec/](spec/) | 需求规格、任务清单与验收清单（遵循 [AGENTS.md](AGENTS.md)） |
