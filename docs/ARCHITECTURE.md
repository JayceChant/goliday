# goliday 架构说明（ARCHITECTURE）

goliday 是一个基于 Go 1.27 的中国法定节假日服务：以**按年组织的稀疏 TOML 配置**记录节假日办对日历的调整，配合标准库周休判断，对外提供粗/细两级粒度的日期类型查询与统计。

相关文档：[CONFIG_FORMAT.md](./CONFIG_FORMAT.md)（配置格式）、[API.md](./API.md)（HTTP 接口）、[generate_prompt.md](./generate_prompt.md)（配置生成提示词）。

---

## 1. 目录结构

```
goliday/
├── go.mod                          # module goliday，go 1.27
├── go.sum
├── daytype.go                      # DayType 位掩码枚举、Coarse/String 映射
├── config.go                       # TOML 解析（LoadYear）与校验（YearConfig.Validate）
├── store.go                        # 多年份配置加载与只读存储（Store/LoadDir）
├── calendar.go                     # 查询索引与判定（Calendar/Query/QueryRange/Stats）
├── daytype_test.go                 # 掩码与映射测试
├── config_test.go                  # 解析与校验测试
├── store_test.go                   # 目录加载测试
├── calendar_test.go                # 判定/区间/统计测试
├── cmd/
│   ├── goliday-server/             # HTTP 服务（可执行入口）
│   │   ├── main.go                 # flag 解析、加载配置、注册路由、ListenAndServe
│   │   ├── handlers.go             # /api/v1/days、/api/v1/stats、/healthz 处理器与中间件
│   │   └── handlers_test.go        # 处理器测试（httptest）
│   └── goliday-tool/               # 配置工具（可执行入口）
│       ├── main.go                 # 子命令分发：validate（校验）与 gen（公告→TOML 草稿）
│       ├── gen.go                  # gen 实现：公告解析、节日推断、草稿渲染与自检
│       └── gen_test.go             # 解析与子命令测试
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
│  HTTP API（仅标准库）       gen / validate CLI           │
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

- **根包 `goliday` 是核心库**：纯逻辑、无 I/O 副作用（除 `LoadDir`/`LoadYear` 读文件），不导入 `net/http`，可被任何 Go 程序直接嵌入。内部依赖单向：`calendar → store → config → daytype`。
- **`cmd/goliday-server`**：HTTP 服务入口。仅标准库 `net/http`（Go 1.22+ `ServeMux`「方法 + 路径」路由模式）+ `flag`；中间件（请求日志、panic 恢复）以函数装饰器实现；`/days` 与 `/stats` 复用同一处理器逻辑。
- **`cmd/goliday-tool`**：运维工具入口。`gen`（公告文本 → 年度 TOML 草稿）与 `validate`（与加载一致的校验）两个子命令。
- 依赖方向严格单向：`cmd/* → 根包`；根包不感知任何入口。

---

## 3. 依赖约束

| 项 | 约束 |
|---|---|
| Go 版本 | `go 1.27` |
| 三方依赖 | **全项目唯一一个**：`github.com/BurntSushi/toml v1.6.0`（根包解析 TOML 使用） |
| HTTP 服务 | 仅标准库（`net/http`、`flag`、`log` 等），不引入任何 Web 框架 |
| 工具 | 仅标准库 + 经根包使用 toml |
| 审计方式 | `go list -m all` 应只见 `goliday` 与 `github.com/BurntSushi/toml` |

零框架、零 ORM、零数据库：配置纯内存、启动时一次性加载，换来极小的二进制与部署面。

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
             以「规范化到 UTC 午夜的日期」为键的哈希集合
      ▼
注册路由 → ListenAndServe
```

任一环节失败即启动失败并打印带文件路径的错误——**错误配置宁可拒绝启动，不带病运行**。

### 4.2 请求（只读、并发安全）

```
HTTP GET /api/v1/days|/api/v1/stats
      │ 解析参数（date / [start,end) / dates，并集去重升序，跨度上限 366 天）
      ▼
Calendar.Query（逐日）：
    周休基线（周六/日→Weekend，否则 Ordinary）
    → work 命中：Compensate|Weekend
    → off 命中：Adjusted
    → 节日当天：追加 Festival 位
    → 该年无配置：整年周休回退
      ▼
Stats（粗粒度计数；detailed 时按单标志位交叉计数）
      ▼
JSON 序列化返回
```

关键设计点：

- **稀疏表 + 周休回退**：配置只存「被调整过」的日期，年文件仅 20~30 行，可人工审计；其余日期由标准库按星期推导，未配置的年份自动整年回退周休判断。
- **日期键规范化**：索引与查询统一用 `normalizeDate`（所在日的 UTC 午夜）作键，消除时刻与时区差异。
- **并发模型**：`Store` 以 `RWMutex` 保障并发读安全；`Calendar` 构建后不可变，可被任意多 goroutine 并发调用。请求路径上无锁竞争、无内存分配热点。
- **无热加载**：配置仅在启动时加载，更新配置的流程是「改文件 → 重启 → `/healthz` 确认 years」（见 CONFIG_FORMAT.md 第 7 节）。

---

## 5. 后续规划（TODO，本次未实现）

- **gRPC 接口**：在现有核心包之上实现 gRPC 服务（与 HTTP 服务同进程，或独立 `cmd/goliday-grpc`），接口语义与 HTTP API 一致：单日查询、区间/离散查询、统计，粗/细两级粒度。
- **proto 定义**：提供 `proto/goliday/v1/goliday.proto`（package `goliday.v1`）及生成说明，方便调用方直接引用。`DayType` 掩码在 proto 中以 **`uint32`** 表达并附位含义注释，取值与位定义和 HTTP API 掩码表（见 [API.md](./API.md) 第 6 节）保持一致。
- 本次变更**不引入** gRPC / protobuf 相关依赖；上述内容作为后续 spec 另行立项。
