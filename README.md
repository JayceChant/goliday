# goliday — 中国法定节假日 API 服务

基于 Go 1.27 的节假日查询服务：以**按年组织的稀疏配置文件**记录国务院节假日办公布的放假与补班安排，其余日期由标准库周休规则推导，对外提供语义一致的 **HTTP 与 gRPC 双协议**接口，支持粗/细两级粒度的日期类型查询与区间统计。

## 特性

- **位掩码日期类型**：`DayType`（uint8）以可组合位标志表达 5 种细粒度（普通工作日/补班/周末/节日/调休）与 2 种粗粒度段值（工作日/节假日），细→粗用位运算与优先级规则（补班优先）映射。
- **稀疏配置**：每年一个 `TOML` 文件，只记录"被调整过"的日期（`off` 仅工作日变休息、`work` 仅周末变上班）；周末与普通工作日等标准库可判定的信息一律不写入，年文件仅 20~30 行、可人工审计。
- **配置优先 + 周休回退**：有配置年份以配置为准，未覆盖日期回退周六/周日判断；无配置年份整年自动回退，服务开箱即用。
- **丰富的查询形态**：单日、区间（左闭右开 `[start, end)`）、离散日期列表，以及区间+离散混合并集；统计支持细粒度组合日交叉计数。
- **双协议**：HTTP（仅标准库，Go 1.22+ `ServeMux`）与 gRPC（`proto/goliday/v1/goliday.proto`，生成代码入库，调用方无需 protoc）。
- **极小依赖面**：核心包（根包 `goliday`）仅依赖 `github.com/BurntSushi/toml`；gRPC 运行时隔离在服务入口与生成代码包中。零框架、零数据库，配置启动时一次性加载纯内存。

## 快速开始

环境要求：Go 1.27+。

```bash
# 启动（HTTP :8080，gRPC :50051）
go run ./cmd/goliday-server -addr :8080 -grpc-addr :50051 -config-dir ./configs

# 单日查询（默认粗粒度）
curl "http://localhost:8080/api/v1/days?date=2026-02-20"
# {"date":"2026-02-20","type":28,"type_label":"holiday","total_days":1,"stats":{"holiday":1,"workday":0}}

# 细粒度：节日当天调休 → festival|adjusted = 24
curl "http://localhost:8080/api/v1/days?date=2026-02-17&detailed=true"
# {"date":"2026-02-17","type":24,"type_label":"festival|adjusted",...}

# 区间统计（左闭右开）
curl "http://localhost:8080/api/v1/stats?start=2026-02-14&end=2026-02-17"
# {"mode":"range",...,"total_days":3,"stats":{"workday":1,"holiday":2}}

# gRPC（Go 客户端）
conn, _ := grpc.NewClient("localhost:50051",
    grpc.WithTransportCredentials(insecure.NewCredentials()))
client := golidayv1.NewGolidayServiceClient(conn)
resp, _ := client.GetDay(ctx, &golidayv1.GetDayRequest{Date: "2026-02-17", Detailed: true})
// resp.Type == 24, resp.TypeLabel == "festival|adjusted"
```

> 示例基于 `configs/2026.toml`（假设示例方案，非官方）。正式使用请按[年度配置更新流程](#年度配置更新)以官方公告生成。

## API 概览

| 协议 | 入口 | 说明 |
|---|---|---|
| HTTP | `GET /api/v1/days` | 单日 / 区间 / 离散 / 混合查询，含逐日明细 |
| HTTP | `GET /api/v1/stats` | 与 `/days` 统计口径一致，无明细 |
| HTTP | `GET /healthz` | 健康检查，返回已加载年份 |
| gRPC | `GolidayService` | `GetDay` / `QueryDays` / `QueryStats`，与 HTTP 一一对应，另注册 gRPC 标准健康检查 |

日期类型掩码：

| 值 | 含义 | | 值 | 含义 |
|---|---|---|---|---|
| 1 | 普通工作日 | | 3 | **粗粒度：工作日**（1\|2） |
| 2 | 补班 | | 28 | **粗粒度：节假日**（4\|8\|16） |
| 4 | 周末 | | 6 | 补班逢周末 |
| 8 | 节日 | | 12 | 节日逢周末 |
| 16 | 调休 | | 24 | 节日当天调休 |

完整契约（参数、响应结构、错误码、gRPC 调用示例、proto 再生成命令）见 [docs/API.md](docs/API.md)。

## 年度配置更新

每年 11 月左右国务院公布次年安排后：

1. 用官方公告生成草稿：`go run ./cmd/goliday-tool gen -year 2027 -out configs/2027.toml -file 公告.txt`（或使用 [docs/generate_prompt.md](docs/generate_prompt.md) 的 LLM 提示词模板）；
2. 校验：`go run ./cmd/goliday-tool validate configs/2027.toml`；
3. 人工抽查若干日期后放入 `configs/`，重启服务并经 `/healthz` 确认年份已加载。

配置格式（选型对比、字段语义、校验规则、判断算法）见 [docs/CONFIG_FORMAT.md](docs/CONFIG_FORMAT.md)，完整注释示例见 [docs/holiday_config_example.toml](docs/holiday_config_example.toml)。

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
| [docs/CONFIG_FORMAT.md](docs/CONFIG_FORMAT.md) | 配置格式选型、稀疏表原则、校验规则、判断算法 |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | 目录结构、分层、依赖约束、数据流 |
| [docs/generate_prompt.md](docs/generate_prompt.md) | 官方公告 → 年度配置的 LLM 提示词模板 |
| [spec/](spec/) | 需求规格、任务清单与验收清单（遵循 [AGENTS.md](AGENTS.md)） |
