# goliday HTTP & gRPC API

goliday-server 提供中国法定节假日日期类型查询服务：单日、区间（左闭右开）、离散列表与混合查询，粗/细两级粒度，以及统计接口。**查询覆盖的每个年份都必须已加载配置**，否则返回 `year_not_loaded` 错误（不再静默回退周休判断）。同时提供语义一致的 **gRPC 接口**（见第 7 节）。HTTP 处理仅使用 Go 标准库，配置为纯内存、启动时一次性加载，统计基于构造期前缀和（O(覆盖年数)，`/stats` 不限查询跨度）。

相关文档：[CONFIG_FORMAT.md](./CONFIG_FORMAT.md)（配置格式与 DayType 掩码详解）、[ARCHITECTURE.md](./ARCHITECTURE.md)。

---

## 1. 启动服务

```bash
go run ./cmd/goliday-server -addr :8080 -grpc-addr :50051 -config-dir ./configs
```

| flag | 默认值 | 说明 |
|---|---|---|
| `-addr` | `":8080"` | HTTP 监听地址 |
| `-grpc-addr` | `":50051"` | gRPC 监听地址；空字符串 `""` 禁用 gRPC |
| `-config-dir` | `"./configs"` | 年度配置目录（`<year>.toml`，格式见 [CONFIG_FORMAT.md](./CONFIG_FORMAT.md)） |
| `-v` | — | 输出 `goliday-server version <版本号>` 后退出（发布镜像经构建注入 tag 版本号；本地 / `go install` 构建为 `dev`） |

启动时全量加载配置目录；任一文件解析或校验失败则启动失败并打印带文件路径的错误。加载成功后输出已加载年份列表日志。收到 SIGINT/SIGTERM 后依次优雅关闭 HTTP 与 gRPC。

路由一览（Go 1.22+ `ServeMux`「方法 + 路径」模式）：

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/days` | 日期类型查询，含逐日明细 `days` |
| GET | `/api/v1/stats` | 与 `/days` 统计口径一致，不含 `days` 明细 |
| GET | `/healthz` | 健康检查，返回已加载年份 |

所有日期参数格式均为 `YYYY-MM-DD`。下文示例使用 2026 年假设配置数据（非官方方案）。

---

## 2. GET /api/v1/days

### 2.1 参数

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `date` | string | 三选一 | 单日查询 |
| `start` | string | 与 `end` 成对 | 区间起点（**含**） |
| `end` | string | 与 `start` 成对 | 区间终点（**不含**，左闭右开 `[start, end)`） |
| `dates` | string | 三选一 | 逗号分隔的离散日期列表；可与 `start+end` 同时提供（并集、去重、升序） |
| `detailed` | bool | 否，默认 `false` | `true` 时单日查询的 `type` 为细粒度值、`stats` 为细粒度五键 MECE 计数；多日明细 `days[].type` 恒为细粒度数值，本参数仅切换 `stats` 口径 |

模式判定：仅 `date` → 单日；`start+end`（可叠加 `dates`）→ 区间 / 混合；仅 `dates` → 离散列表。区间与 `dates` 同时提供时取并集去重升序，`mode` 为 `"list"`。`date` 与 `start+end`/`dates` 并存时以 `date` 为准（单日模式），其余参数被忽略；单日响应不含 `mode` 与 `days` 字段。

限制：`end` 不得早于 `start`；**区间跨度不得超过 366 天**（仅 `/days` 明细接口，防止响应膨胀；`/stats` 基于前缀和统计，不限跨度）；查询覆盖的年份必须已加载，否则返回 400 `year_not_loaded`（见第 5 节）。空区间（`start == end`）无覆盖年份，直接返回 `total_days=0` 的全零统计，不校验年份。

### 2.2 单日查询

默认粗粒度：

```bash
curl "http://localhost:8080/api/v1/days?date=2026-02-20"
```

```json
{"date": "2026-02-20", "type": 2, "type_label": "rest", "total_days": 1, "stats": {"holiday": 1, "workday": 0}}
```

2026-02-20 为周五春节调休（`off`），粗粒度为放假 `rest`（2）。

细粒度：

```bash
curl "http://localhost:8080/api/v1/days?date=2026-02-20&detailed=true"
```

```json
{"date": "2026-02-20", "type": 10, "type_label": "rest|adjusted_rest", "total_days": 1, "stats": {"adjusted_rest": 1, "adjusted_work": 0, "festival": 0, "ordinary": 0, "weekend": 0}}
```

### 2.3 区间查询

```bash
curl "http://localhost:8080/api/v1/days?start=2026-02-14&end=2026-02-18"
```

左闭右开：覆盖 2026-02-14 ~ 2026-02-17 共 4 天（不含 `end`）。

```json
{
  "mode": "range",
  "start": "2026-02-14",
  "end": "2026-02-18",
  "total_days": 4,
  "days": [
    {"date": "2026-02-14", "type": 17, "type_label": "work|adjusted_work"},
    {"date": "2026-02-15", "type": 2,  "type_label": "rest"},
    {"date": "2026-02-16", "type": 10, "type_label": "rest|adjusted_rest"},
    {"date": "2026-02-17", "type": 6,  "type_label": "rest|festival"}
  ],
  "stats": {"holiday": 3, "workday": 1}
}
```

逐日说明：02-14（周六）补班 → 粗粒度 workday（细粒度 `work|adjusted_work`）；02-15（周日）自然周末 → holiday；02-16（周一）春节调休 → holiday；02-17（周二）春节当天 → holiday（`rest|festival`，节日当天不另标调休位）。注意 `days` 明细的 `type` **恒为细粒度数值**（与 `detailed` 无关），`detailed` 仅切换 `stats` 统计口径。

细粒度版本（`&detailed=true`）：`days` 明细不变（恒为细粒度数值），仅 `stats` 换为五键 MECE 计数：

```json
{ "mode": "range", "start": "2026-02-14", "end": "2026-02-18", "total_days": 4,
  "days": [ ...同上... ],
  "stats": {"ordinary": 0, "weekend": 1, "festival": 1, "adjusted_rest": 1, "adjusted_work": 1} }
```

### 2.4 离散列表查询

```bash
curl "http://localhost:8080/api/v1/days?dates=2026-02-16,2026-02-28,2026-02-17"
```

输入允许乱序、允许重复，返回去重后升序排列：

```json
{
  "mode": "list",
  "total_days": 3,
  "days": [
    {"date": "2026-02-16", "type": 10, "type_label": "rest|adjusted_rest"},
    {"date": "2026-02-17", "type": 6,  "type_label": "rest|festival"},
    {"date": "2026-02-28", "type": 17, "type_label": "work|adjusted_work"}
  ],
  "stats": {"holiday": 2, "workday": 1}
}
```

2026-02-28 为周六补班（`work`），故粗粒度统计归入 workday（细粒度 `work|adjusted_work`，单日细粒度响应见 2.2）。

### 2.5 混合查询（区间 + 离散）

```bash
curl "http://localhost:8080/api/v1/days?start=2026-02-01&end=2026-02-03&dates=2026-03-08"
```

区间 [02-01, 02-03) 与 03-08 取并集，`mode` 为 `"list"`：

```json
{
  "mode": "list",
  "total_days": 3,
  "days": [
    {"date": "2026-02-01", "type": 2,  "type_label": "rest"},
    {"date": "2026-02-02", "type": 1,  "type_label": "work"},
    {"date": "2026-03-08", "type": 2,  "type_label": "rest"}
  ],
  "stats": {"holiday": 2, "workday": 1}
}
```

02-01 与 03-08 均为周日；02-02 为周一普通工作日。

### 2.6 细粒度统计的五键 MECE 计数

`detailed=true` 时，`stats` 的五个键（`ordinary` / `weekend` / `festival` / `adjusted_rest` / `adjusted_work`）为 **MECE 计数**：每类日各计一类键，**各键之和恒等于 `total_days`**。以 2.3 的区间为例：

- 02-14 补班日 → `adjusted_work` +1；
- 02-15 普通周休 → `weekend` +1；
- 02-16 调休放假日 → `adjusted_rest` +1；
- 02-17 节日放假日 → `festival` +1。

键与日类的对应：`ordinary`=普通工作日（1）、`weekend`=普通周休（2）、`festival`=节日放假日（6）、`adjusted_rest`=调休放假日（10）、`adjusted_work`=补班日（17）。使用建议：

- 「总休息/上班天数」→ 用粗粒度 `stats.holiday` / `stats.workday`（也可用五键直接分组求和：`weekend+festival+adjusted_rest` = 放假、`ordinary+adjusted_work` = 上班）；
- 「某类日的天数」（如整个春节假期含节日的天数）→ 用细粒度对应键。

---

## 3. GET /api/v1/stats

参数与 `/api/v1/days` **完全相同**（`date`、`start`、`end`、`dates`、`detailed`），统计口径一致，但**不返回 `days` 明细**。两接口复用同一处理器逻辑与同一统计路径，同一输入的 `total_days` 与 `stats` 保证一致。

实现上统计基于加载时构建的细粒度组合计数前缀和：年内 `prefix[end 下标-1] - prefix[start 下标-1]`（前缀为闭区间下标），跨年各年内差分后加总，粗粒度由细粒度组合线性累加导出，复杂度 O(覆盖年数)。因此 **`/stats` 不限制查询跨度**（`/days` 仍限 366 天）。

区间、粗粒度：

```bash
curl "http://localhost:8080/api/v1/stats?start=2026-02-14&end=2026-02-18"
```

```json
{
  "mode": "range",
  "start": "2026-02-14",
  "end": "2026-02-18",
  "total_days": 4,
  "stats": {"holiday": 3, "workday": 1}
}
```

离散、细粒度：

```bash
curl "http://localhost:8080/api/v1/stats?dates=2026-02-17,2026-02-28&detailed=true"
```

```json
{
  "mode": "list",
  "total_days": 2,
  "stats": {"ordinary": 0, "weekend": 0, "festival": 1, "adjusted_rest": 0, "adjusted_work": 1}
}
```

（02-17 为 `rest|festival`，02-28 为 `work|adjusted_work`，五键各计一类日，之和 == `total_days`。）

---

## 4. GET /healthz

```bash
curl "http://localhost:8080/healthz"
```

```json
{"status": "ok", "years": [2025, 2026]}
```

`years` 为已加载的年份（升序），可用于部署后确认新年度配置已被加载。

---

## 5. 错误格式

所有非 2xx 响应统一为：

```json
{"error": {"code": "invalid_date", "message": "非法日期 \"2026-02-30\"：须为 YYYY-MM-DD 格式的有效日期"}}
```

| HTTP | code | 触发条件示例 |
|---|---|---|
| 400 | `missing_query` | `date`、`start+end`、`dates` 均缺省 |
| 400 | `invalid_date` | `date=2026-02-30`（不存在的日期）或 `date=20260230`（格式非法） |
| 400 | `invalid_range` | `start`/`end` 只出现其一；`end` 早于 `start`；区间跨度超过 366 天（仅 `/days` 与 gRPC `QueryDays`；`/stats`/`QueryStats` 不限跨度） |
| 400 | `invalid_detailed` | `detailed` 不是合法布尔值（如 `detailed=abc`） |
| 400 | `year_not_loaded` | 查询覆盖未加载配置的年份（单日、区间、离散、混合；含跨年区间的中间整年），message 列出全部未加载年份 |
| 404 | `not_found` | 访问未注册路径，如 `GET /api/v1/unknown` |
| 405 | `method_not_allowed` | 对 `/api/v1/days` 使用 POST 等非 GET 方法 |

示例：

```json
{"error": {"code": "method_not_allowed", "message": "方法不被允许: POST"}}
```

`year_not_loaded` 示例（查询覆盖未加载年份，含跨年区间的中间整年；多年份以 `, ` 分隔升序列出）：

```json
{"error": {"code": "year_not_loaded", "message": "查询范围包含未加载的年份: 2027"}}
```

---

## 6. DayType 掩码数值表与 type_label 对照

`type` 字段为整数掩码（`DayType`，`uint8`），采用「终态双层」编码：基本位互斥恰一（当日最终是否上班），调整位互斥至多一（依附基本位）；全部组合的 `|` 均为 all-of（合取）语义。位定义：

| 常量 | 数值 | 层 | 含义 |
|---|---|---|---|
| `DayTypeWork` | 1 | 基本位 | 上班（单值即普通工作日） |
| `DayTypeRest` | 2 | 基本位 | 放假（单值即普通周休；未调整时必然为周末） |
| `DayTypeFestival` | 4 | 调整位 | 过节：法定节日当天（放假），当日新增法定假期 |
| `DayTypeAdjustedRest` | 8 | 调整位 | 调休：原工作日被调整为休息（非节日当天），不新增假期 |
| `DayTypeAdjustedWork` | 16 | 调整位 | 补班：原周末被调整为上班 |

响应中 `type` 的全部取值及 `type_label` 对照（`type_label` 即 `DayType.String()`）：

| type | 常量（组合） | 粗粒度归属 | `detailed=false` 时 | `detailed=true` 时 |
|---|---|---|---|---|
| 1 | `Work` | Work(1) | — | `work` |
| 2 | `Rest` | Rest(2) | — | `rest` |
| 6 | `FestivalRest`（`Rest\|Festival`） | Rest(2) | — | `rest\|festival` |
| 10 | `AdjustedRestDay`（`Rest\|AdjustedRest`） | Rest(2) | — | `rest\|adjusted_rest` |
| 17 | `AdjustedWorkDay`（`Work\|AdjustedWork`） | Work(1) | — | `work\|adjusted_work` |
| 1 | `Work`（粗粒度值本身） | — | `work` | — |
| 2 | `Rest`（粗粒度值本身） | — | `rest` | — |

细→粗投影：`Coarse() = t & dayTypeCoarseMask`（内部常量 `dayTypeCoarseMask = Work|Rest = 3`，仅供掩码用、本身非法、不导出以免被当作类型值使用），即单次按位与；调整位投影与之对应：`Adjustment() = t & dayTypeAdjustMask`（内部常量 `= Festival|AdjustedRest|AdjustedWork = 28`，不导出），合法值上 ∈ {0, 4, 8, 16}；判类仅需 `t & 1 != 0`（上班）/ `t & 2 != 0`（放假），无优先级消歧（旧编码 `6 & 28` 双命中问题已消除）。

非法值（可编码但不出现）：`0`（默认零值 `DayTypeUnknown`，未定义）与 ≥32（未定义位）；`3`（上班∧放假）；裸调整位 `4/8/16`；`5/9/18`（调整位与终态矛盾——过节/调休必放假、补班必上班）；`12/14/20/22` 等（同日至多一个调整位）。Go 客户端可用 `IsValid()` 校验，以及 `IsWork()/IsRest()` 与 `IsFestivalRest()/IsAdjustedRestDay()/IsAdjustedWorkDay()` 判类——五个方法同构（合法值前提下，分别与粗粒度掩码/调整位掩码按位与后判等对应位），非法值恒 false。

> `type_label` 由 `DayType.String()` 生成：合法值查静态标签表直返（标签与常量名逐字对应，组合值两段以 `|` 连接），故 `FestivalRest`（6）输出 `rest|festival`，`AdjustedWorkDay`（17）输出 `work|adjusted_work`；粗粒度值 1/2 天然输出 `work`/`rest`。服务端恒输出合法值标签；非法值统一返回 `invalid`，不出现在响应中。

---

## 7. gRPC 接口

gRPC 与 HTTP 同进程提供（`-grpc-addr`，默认 `:50051`，空字符串禁用），并注册 gRPC 标准健康检查服务（`grpc.health.v1`，空服务名 `""` 报告 `SERVING`）。

### 7.1 proto 定义与生成代码

| 项 | 位置 |
|---|---|
| proto 文件 | `proto/goliday/v1/goliday.proto`（package `goliday.v1`） |
| 生成代码（已入库） | `proto/goliday/v1/goliday.pb.go`、`goliday_grpc.pb.go`（包名 `golidayv1`） |

调用方无需本地 protoc：同模块直接 `import "github.com/JayceChant/goliday/proto/goliday/v1"`；跨语言可复制 `.proto` 自行生成桩代码。`DayType` 掩码在 proto 中以 `uint32` 表达并附位注释（proto3 enum 无法表达位组合），取值与第 6 节掩码表完全一致。

### 7.2 服务与方法

`GolidayService` 三方法与 HTTP 一一对应，语义完全一致（单日 `detailed` 粗/细切换、多日明细恒细粒度、`QueryDays` 区间左闭右开 ≤366 天而 `QueryStats` 不限跨度、离散去重升序、区间+离散并集 `mode=list`、`date` 优先、覆盖年份未加载返回 `year_not_loaded`、细粒度统计五键 MECE）：

| rpc 方法 | 对应 HTTP | 说明 |
|---|---|---|
| `GetDay(GetDayRequest)` | `GET /api/v1/days?date=...` | 单日，`GetDayResponse{date,type,type_label,total_days,stats}` |
| `QueryDays(QueryDaysRequest)` | `GET /api/v1/days?start=...&end=...&dates=...` | 区间/离散/混合，含 `days` 明细 |
| `QueryStats(QueryStatsRequest)` | `GET /api/v1/stats` | 入参与 QueryDays 同构，无 `days` 明细 |

`QueryDaysRequest{start, end, repeated dates, detailed}` 中日期均为 `YYYY-MM-DD` 字符串；`Stats` 消息恒填 `workday/holiday`（粗粒度），仅 `detailed=true` 时填 `ordinary/weekend/festival/adjusted_rest/adjusted_work`（细粒度五键 MECE，之和恒等于 `total_days`）。

### 7.3 错误语义

参数错误统一返回 `codes.InvalidArgument`，message 格式为 `"<code>: <中文说明>"`，code 与 HTTP 完全一致（`invalid_date` / `invalid_range` / `missing_query` / `invalid_detailed` / `year_not_loaded`）。

### 7.4 Go 调用示例

```go
conn, err := grpc.NewClient("localhost:50051",
    grpc.WithTransportCredentials(insecure.NewCredentials()))
if err != nil { panic(err) }
defer conn.Close()

client := golidayv1.NewGolidayServiceClient(conn)
resp, err := client.GetDay(ctx, &golidayv1.GetDayRequest{
    Date: "2026-02-17", Detailed: true,
})
// resp.Type == 6（Rest|Festival），resp.TypeLabel == "rest|festival"
```

### 7.5 proto 再生成

修改 `.proto` 后在仓库根执行 `buf generate`（需 `buf` 与两个本地插件在 PATH；模块与 lint/breaking 配置见 `buf.yaml`，插件与生成参数见 `buf.gen.yaml`；生成文件随仓库提交）：

```bash
buf generate
```

参考版本：buf v1.72.0、protoc-gen-go v1.36.5、protoc-gen-go-grpc v1.5.1。buf 自带编译器、不经 protoc，生成头注释中 protoc 版本显示为 `(unknown)` 属预期。
