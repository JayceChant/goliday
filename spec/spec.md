# 节假日 API 服务（goliday）Spec

## Why
空白仓库，需从零实现一个基于 Go 1.27 的中国节假日 API 服务：以按年组织的**稀疏**配置文件记录节假日办对日历的调整（仅写入与默认周休状态不同的日期），配合标准库周休判断，对外提供粗/细两级粒度的日期类型查询与区间统计能力，并在每年官方方案公布后可通过提示词/工具半自动生成新年度配置。

## What Changes
- 新建 Go module `goliday`（go 1.27），核心逻辑置于根包 `goliday`，HTTP 服务置于 `cmd/goliday-server` 子包，配置生成/校验工具置于 `cmd/goliday-tool` 子包。
- 定义日期类型位掩码枚举 `DayType`（**uint8**，取值范围小，可组合位标志）：
  - 粗粒度：`Workday 工作日`、`Holiday 节假日`。
  - 细粒度：`Ordinary 普通工作日`、`Compensate 补班`、`Weekend 周末`、`Festival 节日`、`Adjusted 调休`。
  - 细→粗映射用位运算与优先级规则（补班优先）实现。
- 核心查询能力：
  - 单日期查询，默认粗粒度，参数开启细粒度。
  - 区间查询（左闭右开 `[start, end)`）与离散日期列表，可混合（并集、去重、升序）。
  - 统计：按粒度返回各类天数（细粒度组合日交叉计数）。
- 配置体系（**稀疏表原则**）：
  - 格式 **TOML**，每年一个文件 `<config_dir>/<year>.toml`，启动时全量加载（参数指定目录），纯内存，不用数据库。
  - **只记录节假日办“调整过”的日期**：放假日 `off`（仅周一~周五的工作日变休息）与补班日 `work`（仅周六/周日变上班）；凡标准日期库可判定的信息（周末、普通工作日）一律不写入配置。
  - 节日定义 `[[festival]]` 仅含名称与节日当天日期（农历节日每年漂移，无法由标准库获得）。
  - 判断顺序：work 命中 → 补班；off 命中 → 调休；节日当天 → 附加 Festival 标志；否则退回标准库周休判断；无该年文件整年退回周休。
- 生成工具与文档：`docs/generate_prompt.md` 提示词模板；`cmd/goliday-tool` 提供 `gen`（公告文本 → 年度 TOML 草稿）与 `validate`（校验）子命令。
- 依赖策略：核心包仅引入一个 TOML 解析库（`github.com/BurntSushi/toml`）；HTTP 服务**仅标准库**（`net/http` + Go 1.22 `ServeMux` 路由模式）；生成工具仅标准库 + TOML 库。
- API 契约与配置格式、选型、目录结构、示例配置见 `docs/API.md`、`docs/CONFIG_FORMAT.md`、`docs/ARCHITECTURE.md`、`docs/holiday_config_example.toml`。

## Impact
- Affected specs: 全部为本变更新增（无既有 spec 受影响）。
- Affected code（均为新建）：
  - 根包：`daytype.go`、`config.go`、`store.go`、`calendar.go` 及对应 `*_test.go`
  - 服务子包：`cmd/goliday-server/{main.go,handlers.go,handlers_test.go}`
  - 工具子包：`cmd/goliday-tool/main.go`
  - 文档：`docs/{API.md,CONFIG_FORMAT.md,ARCHITECTURE.md,generate_prompt.md,holiday_config_example.toml}`
  - 测试数据：`testdata/{2025.toml,2026.toml,invalid/*.toml}`
  - 工程：`go.mod`、`.gitignore`

## ADDED Requirements

### Requirement: 模块结构与依赖约束
系统 SHALL 在仓库根创建 Go module（module 名 `goliday`，go 指令 1.27）；核心逻辑 SHALL 位于根包 `goliday`；服务与工具 SHALL 分别位于 `cmd/goliday-server`、`cmd/goliday-tool`。依赖约束：整个项目 SHALL 仅引入一个第三方依赖 `github.com/BurntSushi/toml`（核心包解析 TOML 使用），HTTP 服务与工具不得引入其他第三方模块。

#### Scenario: 依赖审计通过
- **WHEN** 运行 `go list -m all`
- **THEN** 除 `goliday` 与 `github.com/BurntSushi/toml` 外无任何第三方模块

### Requirement: DayType 位掩码枚举
系统 SHALL 定义 `type DayType uint8` 与可组合位标志常量，细粒度值可通过位运算映射为粗粒度段值（uint8 足够容纳 5 个细粒度位及组合）。

枚举取值（`daytype.go`）：

| 常量 | 值 | 含义 |
|---|---|---|
| `DayTypeOrdinary` | 1 << 0 | 普通工作日（细） |
| `DayTypeCompensate` | 1 << 1 | 补班（细，归工作日段） |
| `DayTypeWeekend` | 1 << 2 | 周末（细，归节假日段） |
| `DayTypeFestival` | 1 << 3 | 节日（细，归节假日段） |
| `DayTypeAdjusted` | 1 << 4 | 调休（细，归节假日段） |
| `DayTypeWorkday` | (1<<0)\|(1<<1) = 3 | 粗粒度：工作日段 |
| `DayTypeHoliday` | (1<<2)\|(1<<3)\|(1<<4) = 28 | 粗粒度：节假日段 |

粗粒度化规则（`Coarse()`，优先级消歧）：
1. 含 `Compensate` 位 → `Workday`（补班必为工作日，即使该日是周末）；
2. 否则含 `Weekend|Festival|Adjusted` 任一位 → `Holiday`；
3. 否则 → `Workday`。

细粒度合法组合全集（由配置与周休推断产生）：
`Ordinary(1)`、`Compensate|Weekend(6)`、`Weekend(4)`、`Festival|Weekend(12)`、`Adjusted(16)`、`Festival|Adjusted(24)`。

`DayType` SHALL 提供 `IsWorkday()`、`IsHoliday()`、`Coarse()`、`String()`；序列化 SHALL 直接输出 int 数值。`String()`：单/组合标志按位序以 `|` 连接小写名（如 `"festival|adjusted"`、`"compensate|weekend"`）；粗粒度值返回 `"workday"`/`"holiday"`。

#### Scenario: 工作日段映射
- **WHEN** 细粒度值分别为 `DayTypeOrdinary`、`DayTypeCompensate|DayTypeWeekend`
- **THEN** `Coarse()` 均等于 `DayTypeWorkday`，`IsWorkday()` 为 true

#### Scenario: 节假日段映射
- **WHEN** 细粒度值分别为 `DayTypeWeekend`、`DayTypeFestival|DayTypeWeekend`、`DayTypeAdjusted`、`DayTypeFestival|DayTypeAdjusted`
- **THEN** `Coarse()` 均等于 `DayTypeHoliday`

#### Scenario: 字符串表示
- **WHEN** 对 `DayTypeFestival|DayTypeAdjusted` 与 `DayTypeCompensate|DayTypeWeekend` 调用 `String()`
- **THEN** 分别返回 `"festival|adjusted"`、`"compensate|weekend"`；对粗粒度 `DayTypeWorkday` 返回 `"workday"`

### Requirement: 稀疏配置文件格式（TOML）
配置 SHALL 采用 TOML，每年一个文件，命名 `<year>.toml`（如 `2026.toml`），服务启动时经 `-config-dir` 加载目录内全部年份文件。

**稀疏表原则**：配置只记录节假日办对日历的“调整”，即与默认周休状态不同的日期；凡可由标准日期库判定的信息（周六/周日、普通工作日）不得写入。

文件结构：

```toml
year = 2026

# 节日定义：仅名称与节日当天日期（农历节日每年漂移，必须显式配置）
[[festival]]
name = "元旦"
date = "2026-01-01"

[[festival]]
name = "春节"
date = "2026-02-17"

# 日期调整稀疏表
[adjust]
# 放假日：被调整为休息的工作日（周一~周五），含节日当天（若为工作日）与调休日
off = [
  "2026-01-01", "2026-01-02",            # 元旦（01-03 为周六，不写入）
  "2026-02-16", "2026-02-17", "2026-02-18",
  "2026-02-19", "2026-02-20",            # 春节（02-15/21/22 为周末，不写入）
]
# 补班日：被调整为上班的周末（周六/周日）
work = [ "2026-01-24", "2026-02-28" ]
```

**格式选型（详见 `docs/CONFIG_FORMAT.md`）**：TOML。理由：注释原生支持（JSON 被排除）；无缩进敏感与隐式类型转换陷阱（YAML 的 `01-01`/时间戳隐式解析风险，LLM 与手写均易错）；扁平字符串数组最贴合稀疏表；Go（BurntSushi/toml）与 Python 3.11+（tomllib）生态成熟。

校验规则（加载与 `goliday-tool validate` 一致）：
- `year` 必须与文件名一致；
- `off` 中日期必须为周一~周五（非周末）——写入周末日即违反稀疏原则；
- `work` 中日期必须为周六/周日；
- `off`、`work` 各自无重复且两集合互斥；
- `work` 不得包含任何 `festival.date`（节日当天不得补班）；
- 所有日期合法（含闰年）且落在 `year` 年内；
- 违规时返回明确错误（指明文件与原因），服务启动失败。

单日期判断算法（优先级从高到低）：
1. `date ∈ work` → `Compensate|Weekend`（work 必为周末）；
2. `date ∈ off` → `Adjusted`（off 必为工作日）；
3. `date == 某 festival.date` → 在周休结果上附加 `Festival` 位；
4. 周休回退：周六/周日 → `Weekend`，否则 `Ordinary`；
5. 该年无配置文件 → 整年按 4 回退。

等价表达：`t = 周末 ? Weekend : Ordinary`；`if off 命中 { t = Adjusted }`；`if work 命中 { t = Compensate|Weekend }`；`if 节日当天 { t |= Festival }`。

#### Scenario: 启动加载
- **WHEN** 服务以 `-config-dir /etc/goliday` 启动且目录含 `2025.toml`、`2026.toml`、`README.md` 及子目录
- **THEN** 仅加载两个年份文件，其余忽略（不报错），日志输出已加载年份

#### Scenario: 工作日调休（配置优先）
- **WHEN** `2026.toml` 的 `off` 含 `2026-02-20`（周五）
- **THEN** 2026-02-20 判定为 `Holiday`（细：`Adjusted=16`），而非默认的 `Workday`

#### Scenario: 节日当天
- **WHEN** `2026-02-17`（周二，春节当天）在 `off` 中且为 `festival.date`
- **THEN** 细粒度为 `Festival|Adjusted = 24`，粗粒度为 `Holiday`

#### Scenario: 周末补班
- **WHEN** `2026.toml` 的 `work` 含 `2026-02-28`（周六）
- **THEN** 细粒度为 `Compensate|Weekend = 6`，粗粒度为 `Workday`

#### Scenario: 节日恰逢周末
- **WHEN** 某 `festival.date` 为周日、且不在 `off`/`work` 中
- **THEN** 细粒度为 `Festival|Weekend = 12`，粗粒度为 `Holiday`

#### Scenario: 未覆盖日期回退
- **WHEN** 查询 2026-03-03（周二，未被任何条目覆盖）
- **THEN** 判定为 `Workday`（细：`Ordinary=1`）

#### Scenario: 无该年配置
- **WHEN** 配置目录仅有 `2026.toml`，查询 2027-05-01（周六）
- **THEN** 判定为 `Holiday`（细：`Weekend=4`）

#### Scenario: 稀疏原则违规
- **WHEN** `off` 含周末日期（如 `2026-01-03` 周六），或 `work` 含工作日，或两集合有交集
- **THEN** 加载返回错误，服务启动失败；`testdata/invalid/` 提供此类样例

### Requirement: 单日期查询
核心包 SHALL 提供 `Query(date time.Time) DayType`（细粒度）与 `QueryCoarse(date time.Time) DayType`；服务层 SHALL 暴露 HTTP 单日查询。

`GET /api/v1/days?date=2026-02-20`（`detailed=true|false`，默认 false）
- 响应：`{"date":"2026-02-20","type":28,"type_label":"holiday"}`；`detailed=true` 时 `{"date":"2026-02-20","type":16,"type_label":"adjusted"}`。

#### Scenario: 非法日期
- **WHEN** `GET /api/v1/days?date=2026-02-30`
- **THEN** 返回 400 与统一错误信息

### Requirement: 区间与离散列表查询（同一接口）
同一接口 SHALL 支持（左闭右开 `[start, end)`）与离散列表 `dates`（逗号分隔），两者可同时提供（并集、去重、升序）。

- 区间模式：`GET /api/v1/days?start=2026-02-01&end=2026-02-28`
- 离散模式：`GET /api/v1/days?dates=2026-02-16,2026-02-28,2026-02-17`
- 混合模式：`GET /api/v1/days?start=2026-02-01&end=2026-02-03&dates=2026-03-08`

区间响应（粗粒度）：
```json
{
  "mode": "range",
  "start": "2026-02-01",
  "end": "2026-02-28",
  "total_days": 27,
  "days": [ { "date": "2026-02-01", "type": 28, "type_label": "holiday" } ],
  "stats": { "holiday": 8, "workday": 19 }
}
```

列表/混合响应：
```json
{
  "mode": "list",
  "total_days": 3,
  "days": [ ... ],
  "stats": { "holiday": 2, "workday": 1 }
}
```

细粒度模式下 `days[].type` 为细粒度掩码，`stats` 为 `{"ordinary":n,"compensate":n,"weekend":n,"festival":n,"adjusted":n}`；组合日（如 `festival|adjusted`、`compensate|weekend`）SHALL 对其含有的每个标志各计 1 天（存在交叉计数，各键之和可大于 `total_days`）。

#### Scenario: 区间左闭右开
- **WHEN** `start=2026-02-01&end=2026-02-28`
- **THEN** `days` 覆盖 2026-02-01 至 2026-02-27 共 27 天，不含 end

#### Scenario: 离散去重排序
- **WHEN** `dates=2026-02-16,2026-02-28,2026-02-17,2026-02-16`
- **THEN** `days` 依次为 02-16、02-17、02-28 共 3 条，`total_days=3`

#### Scenario: 混合并集
- **WHEN** `start=2026-02-01&end=2026-02-03&dates=2026-03-08`
- **THEN** `days` 为 02-01、02-02、03-08 共 3 条，`mode="list"`

#### Scenario: 细粒度统计交叉计数
- **WHEN** 02-17 为 `festival|adjusted`、02-28 为 `compensate|weekend`
- **THEN** 该两日分别在 `festival`+`adjusted`、`compensate`+`weekend` 键中各计 1

#### Scenario: 参数校验
- **WHEN** `end < start`、或区间跨度 > 366 天、或 `date`/`start+end`/`dates` 均缺省、或日期格式非法
- **THEN** 返回 400 与明确错误信息

### Requirement: 统计接口
系统 SHALL 提供 `GET /api/v1/stats?start=...&end=...&detailed=...`，统计口径与 `/api/v1/days` 完全一致但不返回 `days` 明细；两接口 SHALL 复用同一处理器逻辑。

#### Scenario: 统计一致性
- **WHEN** 同一区间分别调用 `/api/v1/stats` 与 `/api/v1/days`
- **THEN** 两者 `stats`、`total_days` 完全一致

### Requirement: HTTP 服务子包（仅标准库）
`cmd/goliday-server` SHALL 仅用标准库实现 HTTP 服务（`net/http`，Go 1.22 `ServeMux` 方法+路径路由模式）。

命令行参数（`flag`）：`-addr`（默认 `":8080"`）、`-config-dir`（默认 `"./configs"`）、`-v`（输出版本后退出）。

路由：`GET /api/v1/days`、`GET /api/v1/stats`、`GET /healthz`；中间件（函数装饰器实现）：请求日志、Panic 恢复。

错误响应统一格式 `{"error":{"code":"...","message":"..."}}`；日期解析统一 `2006-01-02`。

#### Scenario: 健康检查
- **WHEN** `GET /healthz`
- **THEN** 返回 200 与 `{"status":"ok","years":[2025,2026]}`（已加载年份）

#### Scenario: 未知路径/方法
- **WHEN** 访问未注册路径，或对 `/api/v1/days` 使用 POST
- **THEN** 分别返回 404/405 统一错误格式

### Requirement: 年度配置生成工具与提示词
系统 SHALL 提供 `cmd/goliday-tool`（Go 实现，子命令 `gen` 与 `validate`）与 `docs/generate_prompt.md` 提示词模板。

`goliday-tool gen -year 2027 -out configs/2027.toml < 公告.txt`：
- 输入官方公告原文（stdin 或 `-file`），解析“X月X日至X月X日放假共N天”“X月X日（周X）上班”等句式；
- 输出符合稀疏表规则的 TOML 草稿：自动剔除假期中的周末日（不写入 `off`）、将补班周末写入 `work`、按节日名写入 `festival`；
- `festival.date` 需按节日名推断（公历节日固定月日；农历节日从公告中的“正月初一”等表述或除夕/初一日期推断），无法推断时输出占位注释待人工补全。

`goliday-tool validate <file...>`：执行与加载一致的校验（年份一致、稀疏原则、互斥、合法性），通过退出码 0，否则输出错误并退出非 0。

`docs/generate_prompt.md`：LLM 提示词模板（公告原文占位 + 生成规则：稀疏原则、节日当天规则、输出 TOML 骨架），用于人工核对 `gen` 结果或替代解析。

#### Scenario: 生成草稿
- **WHEN** 将 2026 年官方公告原文输入 `goliday-tool gen -year 2026`
- **THEN** 生成 `2026.toml` 草稿，且通过 `goliday-tool validate`

#### Scenario: 校验工具
- **WHEN** `goliday-tool validate testdata/2026.toml` 与对 `testdata/invalid/*.toml`
- **THEN** 分别退出 0 与非 0

## TODO（后续规划，不在本次实现范围）
- **gRPC 接口**：后续在现有核心包之上实现 gRPC 服务（与 HTTP 服务同进程或独立 `cmd/goliday-grpc`），接口与 HTTP API 语义一致（单日查询、区间/离散查询、统计，粗/细粒度）。
- **proto 文档**：同时提供 `.proto` 定义（建议 `proto/goliday/v1/goliday.proto`，package `goliday.v1`）及生成说明，方便调用方直接引用；`DayType` 掩码在 proto 中以 `uint32` 表达并附注释，粗/细粒度与位含义与本 spec 枚举取值保持一致。
- 本次变更不引入 gRPC/proto 相关依赖；上述内容作为后续 spec 立项。

## MODIFIED Requirements
（无——全新能力。）

## REMOVED Requirements
（无。）

## 附录：核心包 API 形态

```go
package goliday // 根包

type DayType uint8
func (t DayType) IsWorkday() bool
func (t DayType) IsHoliday() bool
func (t DayType) Coarse() DayType
func (t DayType) String() string

type YearConfig struct { Year int; Name string; Festivals []Festival; Adjust Adjust }
type Festival  struct { Name string; Date time.Time }
type Adjust    struct { Off, Work []time.Time }

func LoadDir(dir string) (*Store, error)   // 加载 <year>.toml
func (s *Store) Has(year int) bool

type Calendar struct{ /* 由 Store 构造 */ }
func NewCalendar(s *Store) *Calendar
func (c *Calendar) Query(date time.Time) DayType       // 细粒度
func (c *Calendar) QueryCoarse(date time.Time) DayType // 粗粒度
func (c *Calendar) QueryRange(start, end time.Time) []Dated // 左闭右开
func (c *Calendar) Stats(dates []time.Time, detailed bool) StatsResult
```
