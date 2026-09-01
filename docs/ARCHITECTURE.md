# goliday 架构说明（ARCHITECTURE）

goliday 是一个基于 Go 1.27 的中国法定节假日服务：以**按年组织的稀疏 TOML 配置**记录节假日办对日历的调整，配合标准库周休判断，对外提供粗/细两级粒度的日期类型查询与统计。

相关文档：[CONFIG_FORMAT.md](./CONFIG_FORMAT.md)（配置格式）、[API.md](./API.md)（HTTP 接口）、[generate_prompt.md](./generate_prompt.md)（配置生成提示词）。

---

## 1. 目录结构

```
goliday/
├── go.mod                          # module github.com/JayceChant/goliday，go 1.27
├── go.sum
├── buf.yaml                        # buf 模块/lint/breaking 配置（proto/ 属仓库布局豁免项）
├── buf.gen.yaml                    # buf generate 插件与参数（本地 protoc-gen-go/go-grpc）
├── daytype.go                      # DayType 位掩码枚举、Coarse/String 映射
├── config.go                       # TOML 解析（LoadYear）与校验（YearConfig.Validate）
├── store.go                        # 多年份配置加载与只读存储（Store/LoadDir）
├── calendar.go                     # 查询索引与判定（Calendar/Query/QueryRange/Stats）
├── config_test.go                  # 【白盒】parseDate 严格解析 + FuzzParseDate
├── daytype_test.go                 # 【黑盒】枚举契约 + 256 值穷举不变量
├── calendar_test.go                # 【黑盒】判定/区间/统计契约 + 全年校验 + FuzzQueryConsistency
├── config_blackbox_test.go         # 【黑盒】LoadYear/LoadDir/Validate 契约 + FuzzLoadYearTOML
├── store_test.go                   # 【黑盒】目录加载契约
├── cmd/
│   ├── goliday-server/             # HTTP + gRPC 服务（可执行入口）
│   │   ├── main.go                 # flag 解析、加载配置、启动/优雅关闭 HTTP 与 gRPC
│   │   ├── handlers.go             # /api/v1/days、/api/v1/stats、/healthz 处理器与中间件
│   │   ├── handlers_test.go        # 【白盒】处理器测试（httptest）
│   │   ├── handlers_fuzz_test.go   # 【白盒】FuzzDaysHandler（任意查询参数）
│   │   ├── grpc.go                 # GolidayService 实现（GetDay/QueryDays/QueryStats）+ 标准健康检查
│   │   └── grpc_test.go            # 【白盒】gRPC 测试（bufconn）
│   └── goliday-tool/               # 配置工具（可执行入口）
│       ├── main.go                 # 子命令分发：validate（校验）与 gen（公告→TOML 草稿）
│       ├── gen.go                  # gen 实现：公告解析、节日推断、草稿渲染与自检
│       ├── gen_test.go             # 【白盒】解析与子命令测试
│       └── gen_fuzz_test.go        # 【白盒】FuzzGenDraft（任意公告文本）
├── proto/
│   └── goliday/v1/
│       ├── goliday.proto           # gRPC 接口定义（package goliday.v1，供调用方引用）
│       ├── goliday.pb.go           # 生成代码（入库，调用方无需 protoc）
│       └── goliday_grpc.pb.go      # 生成代码（入库）
├── configs/                        # 生产年度配置 <year>.toml（2025.toml、2026.toml）
├── testdata/                       # 测试数据
│   ├── 2025.toml                   # 官方方案
│   ├── 2026.toml                   # 假设示例方案（非官方，供测试引用）
│   └── invalid/                    # 非法样例：dup / off_weekend / work_weekday /
│                                   #   invalid_date / year_mismatch
└── docs/                           # 文档
    ├── API.md
    ├── CONFIG_FORMAT.md
    ├── ARCHITECTURE.md
    ├── generate_prompt.md
    └── holiday_config_example.toml # 带注释的配置完整示例
```

---

## 2. 分层

```
┌─────────────────────────────────────────────────────────┐
│  cmd/goliday-server        cmd/goliday-tool             │  可执行入口层
│  HTTP + gRPC 双协议         gen / validate CLI           │
└───────────────┬─────────────────────┬───────────────────┘
                │         仅依赖根包    │
┌───────────────▼─────────────────────▼───────────────────┐
│                     根包 goliday（核心库）                 │
│  daytype（枚举与掩码映射）                                 │
│    ← config（TOML 解析 + 校验，唯一三方依赖 toml）          │
│    ← store（多年份装载，LoadDir）                          │
│    ← calendar（哈希索引 + 判定/区间/统计）                  │
└─────────────────────────────────────────────────────────┘
```

另有一个独立的生成代码包 `proto/goliday/v1`（包名 `golidayv1`）：由 `goliday.proto` 生成，仅被 `cmd/goliday-server` 引用，不反向依赖根包以外的模块。

- **根包 `goliday` 是核心库**：纯逻辑、无 I/O 副作用（除 `LoadDir`/`LoadYear` 读文件），不导入 `net/http` 与任何 gRPC/protobuf 包，可被任何 Go 程序直接嵌入。内部依赖单向：`calendar → store → config → daytype`。
- **`cmd/goliday-server`**：双协议服务入口。HTTP 侧仅标准库 `net/http`（Go 1.22+ `ServeMux`「方法 + 路径」路由模式）；gRPC 侧实现 `GolidayService`（`GetDay/QueryDays/QueryStats`，语义与 HTTP 一致，校验/查询逻辑复用同一套内部函数）并注册 gRPC 标准健康检查；中间件（请求日志、panic 恢复）以函数装饰器实现。
- **`cmd/goliday-tool`**：运维工具入口。`gen`（公告文本 → 年度 TOML 草稿）与 `validate`（与加载一致的校验）两个子命令。
- 依赖方向严格单向：`cmd/* → 根包`（server 另依赖 `proto/goliday/v1`）；根包不感知任何入口。

---

## 3. 依赖约束（按包分级）

| 项 | 约束 |
|---|---|
| Go 版本 | `go 1.27` |
| 根包（核心库） | 仅一个三方依赖：`github.com/BurntSushi/toml v1.6.0`（TOML 解析）；**不得导入 gRPC/protobuf** |
| gRPC 三件套 | `google.golang.org/grpc`、`google.golang.org/protobuf`（+ 其传递依赖如 `genproto/googleapis/rpc`、`golang.org/x/*`），**仅允许出现在 `proto/goliday/v1/` 生成代码包与 `cmd/` 子包** |
| HTTP 处理 | 仅标准库（`net/http`、`flag`、`log` 等），不引入任何 Web 框架 |
| 工具 | 仅标准库 + 经根包使用 toml |
| 审计方式 | `go list -deps . | grep google.golang.org` 应无输出（根包零 gRPC）；`go.mod` 直接依赖仅 toml + gRPC 三件套 |

零框架、零 ORM、零数据库：配置纯内存、启动时一次性加载，换来极小的二进制与部署面。gRPC 运行时被隔离在服务入口与生成代码中，核心库的依赖面不受影响。

---

## 4. 数据流

### 4.1 启动（一次性）

```
configs/<year>.toml
      │ LoadDir（遍历目录，仅接受 NNNN.toml，忽略其余文件与子目录）
      ▼
LoadYear：toml 解析 → 日期严格解析（YYYY-MM-DD、真实存在）
        → Validate（稀疏约束/互斥/年份一致，见 CONFIG_FORMAT.md 第 5 节）
        → 文件名与 year 一致性校验
      ▼
Store：map[int]*YearConfig（加载完成后只读）
      ▼
NewCalendar：为每年构建 yearIndex{off, work, festival} 三个
             以「规范化到 UTC 午夜的日期」为键的哈希集合，
             并逐日判定构建「元旦起左闭右开的细粒度组合计数前缀和」
             （长度 = 年天数+1，按组合计数）
      ▼
注册路由 → ListenAndServe
```

任一环节失败即启动失败并打印带文件路径的错误——**错误配置宁可拒绝启动，不带病运行**。

### 4.2 请求（只读、并发安全）

```
HTTP GET /api/v1/days|/api/v1/stats
      │ 解析参数（date / [start,end) / dates，并集去重升序）
      │ days 明细接口区间跨度上限 366 天；stats 不限跨度
      ▼
覆盖年份强校验（year_not_loaded）：
    单日 {date 年}；区间 [start 年, (end-1 天) 年] 全部整年（含中间整年）；
    列表各日期年；混合取并集。任一年未加载 → 400/InvalidArgument，
    不再回退周休判断。空区间（start==end）无覆盖年份，返回全零统计。
      ▼
Calendar.Query（逐日，明细/单日判定）：
    周休基线（周六/日→Weekend，否则 Ordinary）
    → work 命中：Compensate|Weekend
    → off 命中：Adjusted
    → 节日当天：追加 Festival 位
    （未加载年已在入口拦截）
      ▼
StatsRange / Stats（前缀和差分，O(覆盖年数)）：
    年内 prefix[endIdx] - prefix[startIdx]；跨年拆「首年段+整年段+末年段」
    相加；粗粒度由组合计数线性累加（workday=C(1)+C(6)、
    holiday=C(4)+C(12)+C(16)+C(24)）；混合并集 = 区间差分
    + 列表剔除区间内日期后分段统计
      ▼
JSON 序列化返回
```

关键设计点：

- **稀疏表 + 年份强校验**：配置只存「被调整过」的日期，年文件仅 20~30 行，可人工审计；其余日期由标准库按星期推导。查询覆盖未加载年份直接报 `year_not_loaded`（含跨年区间中间整年），不再静默回退周休判断，避免「未配置」被误读为「真实周末」。
- **组合计数前缀和**：`NewCalendar` 为每年构建长度 = 年天数+1 的前缀数组（`prefix[i]` 为 `[元旦, 元旦+i 天)` 的 6 种合法组合累计天数，左闭右开），构建 O(年天数) 一次完成、构建后只读；统计为年内 O(1) 差分、跨年 O(覆盖年数) 拆段相加，因此 `/stats` 无需限制查询跨度。
- **日期键规范化**：索引与查询统一用 `normalizeDate`（所在日的 UTC 午夜）作键，消除时刻与时区差异。
- **并发模型**：`Store` 以 `RWMutex` 保障并发读安全；`Calendar`（含前缀和）构建后不可变，可被任意多 goroutine 并发调用。请求路径上无锁竞争、无内存分配热点。
- **无热加载**：配置仅在启动时加载，更新配置的流程是「改文件 → 重启 → `/healthz` 确认 years」（见 CONFIG_FORMAT.md 第 7 节）。

---

## 5. gRPC 接口

- **gRPC 服务**：与 HTTP 同进程（`-grpc-addr`，默认 `:50051`，空字符串禁用），实现 `GolidayService` 三方法（`GetDay/QueryDays/QueryStats`），语义与 HTTP API 完全一致，并注册 gRPC 标准健康检查；优雅关闭同时覆盖双协议。
- **proto 定义**：`proto/goliday/v1/goliday.proto`（package `goliday.v1`）供调用方直接引用；`DayType` 掩码以 **`uint32`** 表达并附位含义注释，与 HTTP API 掩码表（见 [API.md](./API.md) 第 6 节）一致。生成代码入库于 `proto/goliday/v1/`，调用方无需本地 protoc；再生成命令见 [API.md](./API.md) 第 7.5 节。
- 接口契约、错误语义与调用示例详见 [API.md](./API.md) 第 7 节。

---

## 6. 测试分层（白盒/黑盒）与 fuzz

单元测试按可见性分层，每个测试文件头部以注释标注视角：

| 层 | 位置 | 说明 |
|---|---|---|
| **白盒** | 根包 `package goliday`：`config_test.go`；`cmd/*` 的 `package main`：`handlers_test.go`、`grpc_test.go`、`gen_test.go` 与两个 fuzz 文件 | 与被测包同名，可访问未导出标识符（如 `parseDate`、`newHandler`、`grpcServer`、`parseAnnouncement`），覆盖分支、边界与错误路径 |
| **黑盒** | 根包 `package goliday_test`：`daytype_test.go`、`calendar_test.go`、`config_blackbox_test.go`、`store_test.go` | 仅经导出 API 验证对外契约（判断算法、粗细映射、区间/统计口径、未加载年报错 `ErrYearNotLoaded`、前缀和与逐日统计等价），不引用任何未导出标识符 |

补充两类强化用例：

- **穷举不变量**：`DayType` 为 uint8 小域，`TestDayTypeExhaustiveInvariants` 遍历全部 256 个取值验证 `Coarse` 封闭且幂等、`IsWorkday/IsHoliday` 恰一为真、`String` 分段合法；`TestCalendarConfiguredYearExhaustive` 对已配置年份全年逐日验证判型 ∈ 6 种合法组合。
- **fuzz 测试**（Go 原生 `testing.F`，种子内联，`go test` 常规运行即执行种子回归）：

| 目标 | 位置 | 不变量 |
|---|---|---|
| `FuzzParseDate` | 根包（白盒） | 严格解析 ⇔ `time.Parse` 接受且回格式化一致；幂等 |
| `FuzzQueryConsistency` | 根包（黑盒） | 判型 ∈ 合法组合全集；粗细一致；同时刻/时区不变；无配置年仅周休 |
| `FuzzLoadYearTOML` | 根包（黑盒） | `LoadYear` 成功 ⟹ 稀疏约束全部成立且 Calendar 判型与配置一致 |
| `FuzzDaysHandler` | server（白盒） | 任意参数不 panic、仅 200/400、响应 JSON 结构与统计口径不变 |
| `FuzzGenDraft` | tool（白盒） | 任意公告解析出的草稿恒满足稀疏表不变量，自检通过可加载 |

冒烟：`go test -run=^$ -fuzz=Fuzz<Name> -fuzztime=10s`（逐包逐目标）。fuzz 发现的崩溃语料按 Go 惯例落盘 `testdata/fuzz/`，须转写为常规回归用例后删除，仓库不保留语料文件。
