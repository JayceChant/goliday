# 节假日 API 服务（goliday）Spec

## 项目定位

基于 Go 1.27 的中国节假日 API 服务：以按年组织的**稀疏**配置文件记录节假日办对日历的调整（仅写入与默认周休状态不同的日期），配合标准库周休判断，对外提供粗/细两级粒度的日期类型查询与区间统计能力，并在每年官方方案公布后可通过提示词/工具半自动生成新年度配置。

各 Requirement 中的 **决策依据** 记录关键取舍原因，后续修改前先审视，避免反复。

## Requirements

### Requirement: 模块结构与依赖约束

系统 SHALL 在仓库根创建 Go module（module 名 `github.com/JayceChant/goliday`，go 指令 1.27）；核心逻辑 SHALL 位于根包 `goliday`（导入路径 `github.com/JayceChant/goliday`）；服务与工具 SHALL 分别位于 `cmd/goliday-server`、`cmd/goliday-tool`。依赖按包分级：**根包（核心包）仅引入 `github.com/BurntSushi/toml`**，不得导入 gRPC/protobuf；HTTP 处理仅标准库；gRPC 三件套（`google.golang.org/grpc`、`google.golang.org/protobuf` 及传递依赖）仅允许出现在 `proto/goliday/v1/` 生成代码包与 `cmd/` 子包。

**决策依据**：核心库保持最小依赖面，gRPC 运行时隔离在入口与生成代码中。

#### Scenario: 依赖审计通过
- **WHEN** 运行 `go list -deps .`（根包）与 `go list -m all`
- **THEN** 根包导入中无 gRPC/protobuf 模块；`go.mod` 直接依赖仅 `github.com/BurntSushi/toml`、`google.golang.org/grpc`、`google.golang.org/protobuf`，其余第三方模块均为 gRPC 的传递依赖

### Requirement: DayType 位掩码枚举（终态双层编码）

系统 SHALL 定义 `type DayType uint8`，采用「终态双层」编码：粗粒度为两个**单 bit 互斥基本位**（当日最终是否上班），细粒度调整位依附粗粒度位组合成具体日期类型；全部组合值的按位或均为 **all-of（合取）**语义，全类型不存在 any-of（并集物化值）语义。

枚举取值（`daytype.go`）：

| 常量 | 值 | 层 | 含义 |
|---|---|---|---|
| `DayTypeUnknown` | 0 | 零值（默认值） | 未定义类型；**非合法细粒度取值**，判类恒 false，`String()` 输出 `unknown` |
| `DayTypeWork` | 1 << 0 = 1 | 粗粒度基本位 | 上班 |
| `DayTypeRest` | 1 << 1 = 2 | 粗粒度基本位 | 放假（未调整时必然为周末） |
| `DayTypeFestival` | 1 << 2 = 4 | 调整位 | 过节：法定节日当天（放假），当日新增法定假期 |
| `DayTypeAdjustedRest` | 1 << 3 = 8 | 调整位 | 调休：原工作日被调整为休息（非节日当天），不新增假期 |
| `DayTypeAdjustedWork` | 1 << 4 = 16 | 调整位 | 补班：原周末被调整为上班 |
| `dayTypeCoarseMask` | `Work\|Rest` = 3 | 粗粒度掩码（未导出） | 两个基本位之并，仅供内部投影/判类（`Coarse`/`IsWork`/`IsRest`）使用，**不是合法的 DayType 取值**（单值 3 非法）；不导出以免被当作类型值使用 |
| `dayTypeAdjustMask` | `Festival\|AdjustedRest\|AdjustedWork` = 28 | 调整位掩码（未导出） | 三个调整位之并，仅供内部投影/判类（`Adjustment`/`IsFestivalRest`/`IsAdjustedRestDay`/`IsAdjustedWorkDay`）使用，**不是合法的 DayType 取值**（多调整位并存非法）；不导出以免被当作类型值使用 |

调整位常量名采用动宾结构（Adjusted**Rest** 调休 / Adjusted**Work** 补班），与基本位 Rest/Work 词根对齐，消除「调的是休还是班」的宾语歧义；`type_label` 分段名与常量名逐字对应（去 `DayType` 前缀的小写蛇形：`adjusted_rest`/`adjusted_work`），细粒度 stats 键与分段名同词（见统计口径 Requirement）。

细粒度合法组合全集（5 种，MECE，由配置与周休判定产生，数值之和即总天数）；组合值 SHALL 提供同名义常量：

| 值 | 常量 | 语义 |
|---|---|---|
| 1 | `DayTypeWork` | 普通工作日 |
| 2 | `DayTypeRest` | 普通周休（未调整的自然周末） |
| 6 | `DayTypeFestivalRest`（`Rest\|Festival`） | 节日放假日（无论节日落在工作日还是周末，终态同为「放假\|过节」） |
| 10 | `DayTypeAdjustedRestDay`（`Rest\|AdjustedRest`） | 调休放假日（原工作日；来源含拼假挪移与节日逢周末的补休，日类型不区分） |
| 17 | `DayTypeAdjustedWorkDay`（`Work\|AdjustedWork`） | 补班日（原周末） |

粗粒度归属 SHALL 为合法值上的单次按位与：`t & DayTypeRest != 0` → 放假、`t & DayTypeWork != 0` → 上班（合法值恰含一个基本位，无歧义、无需优先级消歧）；`Coarse()` SHALL 返回 `t & dayTypeCoarseMask`（内部常量，`= DayTypeWork|DayTypeRest = 3`），合法值上结果 ∈ {1, 2} 且幂等；`Adjustment()` SHALL 返回调整位投影 `t & dayTypeAdjustMask`（内部常量，`= Festival|AdjustedRest|AdjustedWork = 28`），与 `Coarse()` 相对应，合法值上结果 ∈ {0, 4, 8, 16}（无调整位时为 0）且幂等。

非法值（可编码，但校验与判定不得产生）：`0`（默认零值 `DayTypeUnknown`，未定义类型）与含未定义位（≥32）的值；`3`（上班∧放假矛盾）；裸调整位 `4`/`8`/`16`（调整位必须依附基本位）；`5`/`9`（过节/调休与上班矛盾——两者必为放假）；`18`（补班与放假矛盾）；`12`/`14`/`20`/`22` 等含两个及以上调整位的组合（同日至多一个调整位）。调整动作与自然日的对应（`off` 必为工作日、`work` 必为周末、工作日节日必须在 `off`）由配置校验保证。

**语义约定**（SHALL 写入用户文档）：
- 「节日」均指产生法定假期的全体公民节日，不放假的纪念日不纳入本系统；节日当天必为放假日（配置校验强制）。
- 「调休」为窄义：原工作日因安排变休息且**非节日当天**；「过节」当日新增假期，「调休」零新增，「补班」为负增量。净增假日 = 过节日数 − 补班数（审计参考，不做硬校验）。
- 「节日逢周末在他日补休」「挪移与补班成对」为公告层配对关系，单日类型不编码、不做硬校验。
- 自然日（调整前是工作日还是周末）不进入类型值，可由日期 `Weekday` 与配置推导。

**决策依据**：旧编码在同一类型中混用两种按位或语义——粗值 3/28 为「互斥并集（any-of）」物化值、细组合值 6/12/24 为「属性合取（all-of）」——导致 `6 & 28 ≠ 0` 双命中、`Coarse()` 需优先级规则消歧，且「按可达值取并」与「按名义位取并」不一致（`1|6=7 ≠ Workday=3`）。本修订将粗粒度改为单 bit 基本枚举、组合值全部退化为 all-of，位运算语义单一，粗/细归属均可用单次按位与表达；曾评估「自然位」方案（组合 {1,2,5,6,9,18}，区分节日逢周末/工作日），因翻转日（补班/调休/过节）的位与判类必然误判且无静态掩码可补救而否决。

`DayType` SHALL 提供：`IsWork()`（合法值且基本位投影为 Work；方法名与基本位 Work 对齐——英语 holiday 与 weekend 为并列概念，普通周末不称 holiday）、`IsRest()`（合法值且投影为 Rest；含普通周休/节日放假日/调休放假日，不含补班）、`IsFestivalRest()`、`IsAdjustedRestDay()`、`IsAdjustedWorkDay()`（三者与 `IsWork`/`IsRest` 同构：合法值且调整位投影 `t & dayTypeAdjustMask` 等于对应调整位 `Festival`/`AdjustedRest`/`AdjustedWork`，非组合值精确判等）、`IsValid()`（∈ 合法值全集 {1,2,6,10,17}）、`Coarse()`、`Adjustment()`、`String()`；序列化 SHALL 直接输出 int 数值。任何非法值上 `IsWork`/`IsRest`/`IsFestivalRest`/`IsAdjustedRestDay`/`IsAdjustedWorkDay` SHALL 均返回 false（如 3 同含两基本位、5/9 调整位与终态矛盾、12 等多调整位并存）。`String()`：合法值与零值 SHALL 查静态标签表直接返回（表按组合常量显式索引、编译期物化，位序/常量名调整随编译检查自动跟随；标签与常量名逐字对应：`DayTypeUnknown` → `"unknown"`、`Work` → `"work"`、`Rest` → `"rest"`、`FestivalRest` → `"rest|festival"`、`AdjustedRestDay` → `"rest|adjusted_rest"`、`AdjustedWorkDay` → `"work|adjusted_work"`），无逐次运行时构建；任何非法值 SHALL 统一返回 `"invalid"`——系统不产生非法值，逐位拼接的伪标签会暗示有效状态，统一词便于调用方兜底（数值本身可经序列化 int 或 `%d` 查看）。

#### Scenario: 上班段映射
- **WHEN** 细粒度值分别为 `DayTypeWork`、`DayTypeAdjustedWorkDay`
- **THEN** `Coarse()` 均等于 `DayTypeWork`，`IsWork()` 为 true、`IsRest()` 为 false

#### Scenario: 放假段映射
- **WHEN** 细粒度值分别为 `DayTypeRest`、`DayTypeFestivalRest`、`DayTypeAdjustedRestDay`
- **THEN** `Coarse()` 均等于 `DayTypeRest`，`IsRest()` 为 true

#### Scenario: 非法值全拒
- **WHEN** 对非法值 0、3、4、5、9、12、18 分别调用 `IsWork`/`IsRest`/`IsValid`/`IsFestivalRest`/`IsAdjustedRestDay`/`IsAdjustedWorkDay`
- **THEN** 六者均返回 false（5 这类「基本位正确而调整位矛盾」与 12 这类「多调整位并存」的值也须为 false，故各方法内部先做合法性校验再投影判等）

#### Scenario: 组合判断
- **WHEN** 对五个合法值分别调用 `IsFestivalRest`/`IsAdjustedRestDay`/`IsAdjustedWorkDay`
- **THEN** 各方法仅对其对应调整位（`Festival`/`AdjustedRest`/`AdjustedWork`）的合法组合值返回 true（实现为合法值前提下的调整位投影判等，与 `IsWork`/`IsRest` 同构）

#### Scenario: 位与判类无歧义
- **WHEN** 对全部合法值 {1, 2, 6, 10, 17} 分别执行 `t & DayTypeRest` 与 `t & DayTypeWork`
- **THEN** 恰一非零（旧编码 `Compensate|Weekend & Holiday = 4` 双命中问题消除）

#### Scenario: 调整位投影
- **WHEN** 对五个合法值分别调用 `Adjustment()`
- **THEN** `Work`/`Rest` 返回 0（无调整位），`FestivalRest`/`AdjustedRestDay`/`AdjustedWorkDay` 分别返回 `Festival`/`AdjustedRest`/`AdjustedWork`；且 `Adjustment().Adjustment()` 幂等（与 `Coarse()` 对应）

#### Scenario: 字符串表示
- **WHEN** 对 `DayTypeFestivalRest` 与 `DayTypeAdjustedWorkDay` 调用 `String()`
- **THEN** 分别返回 `"rest|festival"`、`"work|adjusted_work"`；对 `DayTypeRest` 返回 `"rest"`；对零值 `DayTypeUnknown` 返回 `"unknown"`

### Requirement: 稀疏配置文件格式（TOML）

配置 SHALL 采用 TOML，每年一个文件，命名 `<year>.toml`（如 `2026.toml`），服务启动时经 `-config-dir` 加载目录内全部年份文件。

**稀疏表原则**：配置只记录节假日办对日历的"调整"，即与默认周休状态不同的日期；凡可由标准日期库判定的信息（周六/周日、普通工作日）不得写入。

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

**格式选型**：TOML。理由：注释原生支持（JSON 被排除）；无缩进敏感与隐式类型转换陷阱（YAML 的 `01-01`/时间戳隐式解析风险，LLM 与手写均易错）；扁平字符串数组最贴合稀疏表；Go（BurntSushi/toml）与 Python 3.11+（tomllib）生态成熟。详见 `docs/CONFIG_FORMAT.md`。

校验规则（加载与 `goliday-tool validate` 一致）：
- `year` 必须与文件名一致；
- `off` 中日期必须为周一~周五（非周末）——写入周末日即违反稀疏原则；
- `work` 中日期必须为周六/周日；
- `off`、`work` 各自无重复且两集合互斥；
- `work` 不得包含任何 `festival.date`（节日当天不得补班）；
- `festival.date` 为周一~周五（工作日）时必须出现在 `off` 中——节日当天必为放假日，工作日节日须经 `off` 落地（否则该日在类型上无合法状态可归）；
- 所有日期合法（含闰年）且落在 `year` 年内；
- 违规时返回明确错误（指明文件与原因），服务启动失败。

单日期判断算法（优先级从高到低）：
1. `date == 某 festival.date` → `Rest|Festival(6)`——节日当天必放假（落周末为自然休息、落工作日由 `off` 落地，统称「节日放假日」，不另标调休位）；
2. `date ∈ work` → `Work|Compensate(17)`（work 必为周末，校验保证）；
3. `date ∈ off` → `Rest|Adjusted(10)`（off 必为工作日）；
4. 周休回退：周六/周日 → `Rest(2)`，否则 `Work(1)`；
5. 该年无配置文件 → 见「年份加载强校验」：返回 `ErrYearNotLoaded`，不回退。

等价表达：`t = 节日当天 ? Rest|Festival : work 命中 ? Work|Compensate : off 命中 ? Rest|Adjusted : (周末 ? Rest : Work)`。

#### Scenario: 启动加载
- **WHEN** 服务以 `-config-dir /etc/goliday` 启动且目录含 `2025.toml`、`2026.toml`、`README.md` 及子目录
- **THEN** 仅加载两个年份文件，其余忽略（不报错），日志输出已加载年份

#### Scenario: 工作日调休（配置优先）
- **WHEN** `2026.toml` 的 `off` 含 `2026-02-20`（周五）
- **THEN** 2026-02-20 判定为 `Rest`（细：`Rest|Adjusted = 10`），而非默认的 `Work`

#### Scenario: 节日当天
- **WHEN** `2026-02-17`（周二，春节当天）在 `off` 中且为 `festival.date`
- **THEN** 细粒度为 `Rest|Festival = 6`，粗粒度为 `Rest`（节日当天必为放假日，不另标调休位）

#### Scenario: 周末补班
- **WHEN** `2026.toml` 的 `work` 含 `2026-02-28`（周六）
- **THEN** 细粒度为 `Work|Compensate = 17`，粗粒度为 `Work`

#### Scenario: 节日恰逢周末
- **WHEN** 某 `festival.date` 为周日、且不在 `off`/`work` 中
- **THEN** 细粒度为 `Rest|Festival = 6`（与工作日节日同为「节日放假日」），粗粒度为 `Rest`

#### Scenario: 工作日节日未写入 off
- **WHEN** 某 `festival.date` 为周二、但不在 `off` 中
- **THEN** 加载返回错误，服务启动失败（节日当天必为放假日，工作日节日须经 `off` 落地）

#### Scenario: 未覆盖日期回退
- **WHEN** 查询 2026-03-03（周二，未被任何条目覆盖）
- **THEN** 判定为 `Work`（细：`Work = 1`）

#### Scenario: 稀疏原则违规
- **WHEN** `off` 含周末日期（如 `2026-01-03` 周六），或 `work` 含工作日，或两集合有交集
- **THEN** 加载返回错误，服务启动失败；`testdata/invalid/` 提供此类样例

### Requirement: 单日期查询

核心包 SHALL 提供 `Query(date time.Time) (DayType, error)`（细粒度）与 `QueryCoarse(date time.Time) (DayType, error)`；`IsWork`/`IsRest` 同步返回 `error`。`date` 年份未加载时返回包装 `ErrYearNotLoaded` 的错误（`errors.Is` 可判别，message 含年份），不回退周休判断（见「年份加载强校验」）。

**决策依据**：消除「未配置」与「真实周末」的静默混淆，故由早期的"无配置年回退周休"改为强校验报错。

服务层 SHALL 暴露 HTTP 单日查询：

`GET /api/v1/days?date=2026-02-20`（`detailed=true|false`，默认 false）
- 响应：`{"date":"2026-02-20","type":2,"type_label":"rest"}`；`detailed=true` 时 `{"date":"2026-02-20","type":10,"type_label":"rest|adjusted_rest"}`。

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
  "days": [ { "date": "2026-02-01", "type": 2, "type_label": "rest" } ],
  "stats": { "holiday": 8, "workday": 19 }
}
```

列表/混合响应：`mode` 为 `"list"`，结构同上（不含区间字段）。

细粒度模式下 `days[].type` 为细粒度值（`days[].type` 恒为细粒度，`detailed` 仅切换 `stats` 口径），`stats` 为 `{"ordinary":n,"weekend":n,"festival":n,"adjusted_rest":n,"adjusted_work":n}`——五键 MECE（各计一类日），**之和恒等于 `total_days`**（不存在交叉计数）。

跨度限制分化：`/api/v1/days`（含 gRPC `QueryDays`）区间跨度上限 **366 天**（防响应膨胀）；`/api/v1/stats`（含 `QueryStats`）**不限跨度**（前缀和实现，见「细粒度组合计数前缀和统计」）。全部覆盖年份须已加载（见「年份加载强校验」）。`days` 响应中的 `stats` 与同输入的 stats 接口完全一致（复用前缀和路径）。

#### Scenario: 区间左闭右开
- **WHEN** `start=2026-02-01&end=2026-02-28`
- **THEN** `days` 覆盖 2026-02-01 至 2026-02-27 共 27 天，不含 end

#### Scenario: 离散去重排序
- **WHEN** `dates=2026-02-16,2026-02-28,2026-02-17,2026-02-16`
- **THEN** `days` 依次为 02-16、02-17、02-28 共 3 条，`total_days=3`

#### Scenario: 混合并集
- **WHEN** `start=2026-02-01&end=2026-02-03&dates=2026-03-08`
- **THEN** `days` 为 02-01、02-02、03-08 共 3 条，`mode="list"`

#### Scenario: 细粒度统计 MECE 计数
- **WHEN** 02-17 为 `rest|festival`、02-28 为 `work|adjusted_work`
- **THEN** 该两日分别在 `festival`、`adjusted_work` 键各计 1，五键之和 == `total_days`

#### Scenario: 参数校验
- **WHEN** `end < start`、或（days 接口）区间跨度 > 366 天、或 `date`/`start+end`/`dates` 均缺省、或日期格式非法
- **THEN** 返回 400 与明确错误信息

### Requirement: 统计接口

系统 SHALL 提供 `GET /api/v1/stats?start=...&end=...&detailed=...`，统计口径与 `/api/v1/days` 完全一致但不返回 `days` 明细；两接口 SHALL 复用同一处理器逻辑。实现为前缀和差分（见「细粒度组合计数前缀和统计」），**不限查询跨度**；混合并集统计 = 区间前缀和统计 + 列表中剔除落在区间内日期后的分段统计，二者相加。

#### Scenario: 统计一致性
- **WHEN** 同一区间分别调用 `/api/v1/stats` 与 `/api/v1/days`
- **THEN** 两者 `stats`、`total_days` 完全一致

### Requirement: HTTP 服务子包（仅标准库）

`cmd/goliday-server` SHALL 仅用标准库实现 HTTP 服务（`net/http`，Go 1.22 `ServeMux` 方法+路径路由模式）。

命令行参数（`flag`）：`-addr`（默认 `":8080"`）、`-grpc-addr`（默认 `":50051"`，空字符串禁用 gRPC）、`-config-dir`（默认 `"./configs"`）、`-v`（输出版本后退出）。

路由：`GET /api/v1/days`、`GET /api/v1/stats`、`GET /healthz`；中间件（函数装饰器实现）：请求日志、Panic 恢复。

错误响应统一格式 `{"error":{"code":"...","message":"..."}}`；日期解析统一 `2006-01-02`。错误码：`missing_query`、`invalid_date`、`invalid_range`、`invalid_detailed`、`year_not_loaded`、`not_found`（404）、`method_not_allowed`（405）。

#### Scenario: 健康检查
- **WHEN** `GET /healthz`
- **THEN** 返回 200 与 `{"status":"ok","years":[2025,2026]}`（已加载年份）

#### Scenario: 未知路径/方法
- **WHEN** 访问未注册路径，或对 `/api/v1/days` 使用 POST
- **THEN** 分别返回 404/405 统一错误格式

### Requirement: 年度配置生成工具与提示词

系统 SHALL 提供 `cmd/goliday-tool`（Go 实现，子命令 `gen` 与 `validate`）与 `docs/generate_prompt.md` 提示词模板。

`goliday-tool gen -year 2027 -out configs/2027.toml < 公告.txt`：
- 输入官方公告原文（stdin 或 `-file`），解析"X月X日至X月X日放假共N天""X月X日（周X）上班"等句式；
- 输出符合稀疏表规则的 TOML 草稿：自动剔除假期中的周末日（不写入 `off`）、将补班周末写入 `work`、按节日名写入 `festival`；
- `festival.date` 需按节日名推断（公历节日固定月日；农历节日从公告中的"正月初一"等表述或除夕/初一日期推断），无法推断时输出占位注释待人工补全。

`goliday-tool validate <file...>`：执行与加载一致的校验（年份一致、稀疏原则、互斥、合法性），通过退出码 0，否则输出错误并退出非 0。

`docs/generate_prompt.md`：LLM 提示词模板（公告原文占位 + 生成规则：稀疏原则、节日当天规则、输出 TOML 骨架），用于人工核对 `gen` 结果或替代解析。

#### Scenario: 生成草稿
- **WHEN** 将 2026 年官方公告原文输入 `goliday-tool gen -year 2026`
- **THEN** 生成 `2026.toml` 草稿，且通过 `goliday-tool validate`

#### Scenario: 校验工具
- **WHEN** `goliday-tool validate testdata/2026.toml` 与对 `testdata/invalid/*.toml`
- **THEN** 分别退出 0 与非 0

### Requirement: 容器镜像与发布（GitHub 环境）

仓库根 SHALL 提供 `Dockerfile`（多阶段构建）与 `.dockerignore`，GitHub Actions SHALL 提供 `.github/workflows/docker.yml` 自动构建并发布镜像至 GHCR（`ghcr.io/<owner>/goliday`）。

Dockerfile 约定：
- 构建阶段：`golang:1.27`（`AS build`），仅复制 `go.mod`/`go.sum` 后 `go mod download`（层缓存友好），再复制源码；`CGO_ENABLED=0` 静态编译 `cmd/goliday-server`（distroless 无动态 loader，必须静态链接）。
- 运行阶段：`gcr.io/distroless/static-debian12:nonroot`；仅复制 server 二进制至 `/goliday-server`；`USER nonroot`（镜像内已内置）；`EXPOSE 8080 50051`；`ENTRYPOINT ["/goliday-server"]`。
- 不打包 `configs/`：配置与镜像解耦，运行时经 volume 挂载后以 `-config-dir` 指向；distroless 无 shell，容器内一切命令参数走 exec 形式。
- 构建上下文最小化：`.dockerignore` 排除 `.git`、`.github`、`docs`、`spec`、`testdata`、`*.md`、`.env*` 等非构建必需内容。

工作流约定：
- 触发：`push` 默认分支、`push` tag `v*`、`pull_request`、`workflow_dispatch`；
- 推送策略：仅 `push` tag `v*` 事件发布镜像至 GHCR；`push` 默认分支、`pull_request`、`workflow_dispatch` 仅构建验证不推送（master 滚动镜像无消费场景，保留只会产生冗余版本记录；构建可行性由验证构建保障）；
- 权限最小化：`contents: read` + `packages: write`；
- 步骤：checkout → buildx → QEMU（多架构）→ GHCR 登录（`GITHUB_TOKEN`，仅 tag 事件执行）→ metadata 提取标签 → build（`linux/amd64` + `linux/arm64`，GHA 缓存，`provenance`/`sbom` 关闭以保持镜像单 manifest；仅 tag 事件 push）；
- 标签策略（metadata-action）：语义化版本 `v1.2.3` → `1.2.3` / `1.2` / `1`、tag 事件附加 `latest`；分支名 / PR 编号标签仅作非推送事件的构建标识，不发布；
- 无自定义 secrets：GHCR 认证仅用内置 `GITHUB_TOKEN`。

#### Scenario: 本地构建镜像
- **WHEN** 在仓库根执行 `docker build -t goliday .`
- **THEN** 多阶段构建成功，最终镜像基于 distroless 且以 nonroot 运行，`docker run goliday -v` 输出版本后退出

#### Scenario: 容器启动并挂载配置
- **WHEN** `docker run -p 8080:8080 -v $PWD/configs:/data:ro goliday -config-dir /data`
- **THEN** 服务监听 8080/50051，`GET /healthz` 返回已加载年份

#### Scenario: CI 构建与发布
- **WHEN** push tag `v0.2.0` 触发工作流
- **THEN** 构建多架构镜像并发布至 `ghcr.io/<owner>/goliday:0.2.0`（另含 `0.2`、`0`、`latest`）
- **WHEN** push 默认分支、PR 或 workflow_dispatch 触发工作流
- **THEN** 仅执行多架构构建验证，不登录、不推送任何镜像

### Requirement: 在线质量门禁与 CI 测试矩阵（GitHub 环境）

仓库 SHALL 提供 `.github/workflows/ci.yml`、`.github/workflows/scorecard.yml` 与 `.github/workflows/sonarcloud.yml`，将本地验证门禁搬上 GitHub Actions，并把结果以徽章与链接接入 README 双语版。

ci.yml 约定：
- 触发：`push` 默认分支、`pull_request`（默认分支）、`workflow_dispatch`；权限最小化 `contents: read`；
- 测试作业：`1.27.x`（go.mod 最低要求）与 `stable` 双版本矩阵（`actions/setup-go` 自带模块缓存；不用 `oldstable`——其版本低于 go.mod 要求且 runner 默认 `GOTOOLCHAIN=local` 不自动升级工具链，必然编译失败），步骤 checkout → setup-go → `go build ./...` → `go vet ./...` → `gofmt` 检查（`gofmt -l .` 输出非空即失败）→ `go test -count=1 -race -covermode=atomic -coverprofile`；
- 覆盖率上报：仅 `stable` 矩阵项经 `codecov/codecov-action` 上传 `coverage.out`（secrets `CODECOV_TOKEN`；公共仓库可不配置 token，上传失败不阻塞流水线）；上传前过滤 profile 中 `proto/goliday/v1` 生成代码的记录，并经仓库根 `codecov.yml`（`ignore: proto/`）在 Codecov 端同步排除——生成代码不设测试目标，避免零覆盖记录拉低统计（与 `.golangci.yml` 对生成代码的豁免同一口径）；
- lint 作业：`golangci/golangci-lint-action` 运行 `golangci-lint`（v2，配置见 `.golangci.yml`）零告警。

scorecard.yml 约定（OpenSSF Scorecard，无需注册）：
- 触发：`push` 默认分支、每周 `schedule`、`branch_protection_rule`；顶层 `permissions: read-all`，作业内最小化（`id-token: write` 供发布 OIDC 认证、`security-events: write` 供 SARIF 上传）；
- `ossf/scorecard-action` 以 `publish_results: true` 发布评分至 scorecard.dev；SARIF 产物落盘 artifact 并经 `github/codeql-action/upload-sarif` 上传至 code scanning；checkout 置 `persist-credentials: false`。

codeql.yml 约定（CodeQL 静态安全分析，公共仓库免费、无需注册）：
- 触发：`push` 默认分支、`pull_request`、每周 `schedule`、`workflow_dispatch`；作业权限最小化（`security-events: write` + 只读）；
- 语言 `go`、`build-mode: autobuild`（新版 CodeQL 已移除 Go 的 none 模式），结果上传 code scanning（Security 标签页）。

govulncheck.yml 约定（Go 官方依赖漏洞扫描，无需注册）：
- 触发：`push` 默认分支、每周 `schedule`、`workflow_dispatch`；`contents: read`；
- `golang/govulncheck-action@v1` 以 text 输出扫描 `./...`，仅当存在可被实际调用路径触达的漏洞时作业失败（作为门禁）。

pkg.go.dev 文档为 Go 官方服务自动抓取（模块可解析、可编译即自动建页），仓库无需配置，README SHALL 提供徽章与结果链接。

sonarcloud.yml 约定（SonarCloud 静态分析：代码异味/安全漏洞/重复率/覆盖率质量门禁）：
- 触发：`push` 默认分支、`pull_request`（默认分支，opened/synchronize/reopened）；权限最小化 `contents: read`；
- 步骤：checkout 拉取完整历史（`fetch-depth: 0`，Quality Gate 的新代码判定依赖提交历史）→ `go test -covermode=atomic -coverprofile=coverage.txt ./...` 生成覆盖率 → `SonarSource/sonarqube-scan-action` 扫描上报；
- 认证依赖仓库 secret `SONAR_TOKEN`（未配置时扫描步骤失败，SonarCloud 端绑定仓库生成 token 后即生效）；项目参数（projectKey/organization、源码与测试目录、覆盖率路径、排除规则）入库于根目录 `sonar-project.properties`；`proto/` 生成代码经 `sonar.coverage.exclusions` 从覆盖率统计排除，与 Codecov、`.golangci.yml` 对生成代码的豁免同一口径。

README 双语 SHALL 在标题下接入 CI、Codecov、CodeQL、govulncheck、pkg.go.dev、OpenSSF Scorecard、SonarCloud 徽章；各服务的说明与结果页链接 SHALL 收录于 [docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md)（质量门禁与 CI 章节），README 不设「质量与持续集成」章节——徽章保持可发现性，详细内容仅面向维护者，避免对使用者构成干扰。

**决策依据**：Actions/Codecov/Scorecard/CodeQL/govulncheck/pkg.go.dev 均可直接在仓库内落地且无需注册；SonarCloud 徽章仅依赖其在 SonCloud 端启用项目；Snyk/Socket 需 GitHub App 绑定，不在仓库内预置，避免空配置导致流水线常红。

#### Scenario: CI 测试矩阵
- **WHEN** push 或 PR 触发 ci.yml
- **THEN** `1.27.x` 与 `stable` 两个矩阵项各自完成 build/vet/gofmt/test，lint 作业零告警；任一步骤失败流水线标红

#### Scenario: 覆盖率上报
- **WHEN** `stable` 矩阵项测试通过
- **THEN** 过滤 proto 生成代码后的 `coverage.out` 上传至 Codecov，Codecov 项目页可逐行查看覆盖详情且统计不含生成代码；未配置 `CODECOV_TOKEN` 时公共仓库仍可上传

#### Scenario: Scorecard 评分发布
- **WHEN** scorecard.yml 在默认分支运行
- **THEN** 评分发布至 scorecard.dev 项目页，SARIF 出现在仓库 code scanning；scorecard.dev 徽章可访问

#### Scenario: CodeQL 静态分析
- **WHEN** push 或 PR 触发 codeql.yml
- **THEN** Go 代码经 CodeQL 分析（autobuild 构建后提取），告警出现在仓库 code scanning；workflow 徽章反映运行状态

#### Scenario: govulncheck 漏洞门禁
- **WHEN** govulncheck.yml 扫描 `./...`
- **THEN** 无可触达漏洞时通过；存在可被调用路径触达的已知漏洞时作业失败并在日志给出修复版本

#### Scenario: SonarCloud 质量门禁徽章
- **WHEN** sonarcloud.yml 在默认分支 push 或 PR 上运行（仓库已配置 `SONAR_TOKEN`）
- **THEN** 分析结果上报 SonarCloud 项目页并生成质量门禁状态，README 徽章反映通过/失败

### Requirement: proto 定义与生成代码

系统 SHALL 提供 `proto/goliday/v1/goliday.proto`（syntax proto3，package `goliday.v1`，`option go_package = "github.com/JayceChant/goliday/proto/goliday/v1;golidayv1"`），供调用方直接引用；生成的 Go 代码 SHALL 入库于 `proto/goliday/v1/{goliday.pb.go,goliday_grpc.pb.go}`（调用方无需本地 protoc）。

proto 内容约定：
- `DayType` 掩码以 `uint32` 表达并附注释（proto3 enum 无法表达位组合），注释标明双层位值（粗粒度基本位：1=上班、2=放假；调整位：4=过节、8=调休、16=补班）与 5 种合法组合（1/2/6/10/17），并说明粗粒度即基本位投影、组合值 `|` 为 all-of 语义，与根包 `DayType` 完全一致；
- 消息：`Day{date,type,type_label}`、`Stats{workday,holiday,ordinary,adjusted_work,weekend,festival,adjusted_rest}`（粗粒度字段恒填充，细粒度字段仅 `detailed=true` 时填充，五键 MECE 之和恒等于 `total_days`，与 HTTP 一致；`adjusted_rest`/`adjusted_work` 与根包调整位常量 AdjustedRest/AdjustedWork 逐字对应）、`GetDayRequest{date,detailed}`、`GetDayResponse{date,type,type_label,total_days,stats}`、`QueryDaysRequest{start,end,dates[],detailed}`、`QueryDaysResponse{mode,start,end,total_days,days[],stats}`、`QueryStatsRequest{start,end,dates[],detailed}`（字段与 QueryDaysRequest 同构，独立消息以符合 buf lint 默认规则）、`QueryStatsResponse{mode,start,end,total_days,stats}`；
- 服务 `GolidayService`：`GetDay`（单日）、`QueryDays`（区间/离散/混合并集，含明细）、`QueryStats`（入参与 QueryDays 同构，不含 days 明细）——语义与 HTTP `/api/v1/days`、`/api/v1/stats` 一一对应；
- 日期一律 `YYYY-MM-DD` 字符串；`mode` 取 `range`/`list`；
- proto 头注释写明再生成方式（buf：在仓库根执行 `buf generate`，需 buf 与 protoc-gen-go、protoc-gen-go-grpc 在 PATH；buf 工作区为标准布局——模块根 `proto/`，`buf lint` 默认 STANDARD 规则零豁免）。

#### Scenario: proto 可供调用方引用
- **WHEN** 调用方获取本仓库后查找 `proto/goliday/v1/goliday.proto` 与生成代码
- **THEN** 无需 buf/protoc 即可 `import "github.com/JayceChant/goliday/proto/goliday/v1"`（同模块）或复制 .proto 生成其他语言桩代码

#### Scenario: 再生成
- **WHEN** 在仓库根执行 `buf generate`
- **THEN** 生成文件落盘于 `proto/goliday/v1/` 且 `gofmt`/`go build` 通过；`buf lint` 通过

### Requirement: gRPC 服务（与 HTTP 同进程）

`cmd/goliday-server` SHALL 在同进程内提供 gRPC 服务：注册 `GolidayService` 与 gRPC 标准健康检查服务（`grpc.health.v1`）；优雅关闭 SHALL 同时覆盖 HTTP 与 gRPC。

gRPC 查询语义 SHALL 与 HTTP 完全一致（复用同一查询逻辑）：单日 `detailed` 粗/细切换、多日明细恒细粒度、区间左闭右开（`QueryDays` 跨度 ≤366 天，`QueryStats` 不限跨度）、离散去重升序、区间+离散并集 `mode=list`、`date` 与其他参数并存时 `date` 优先；参数错误 SHALL 映射为 `codes.InvalidArgument`，错误 `message` 文案与 HTTP 一致（含 `invalid_date`/`invalid_range`/`missing_query`/`invalid_detailed`/`year_not_loaded` 等标识）。

#### Scenario: 启用与禁用
- **WHEN** 以默认参数启动
- **THEN** HTTP 监听 `:8080`、gRPC 监听 `:50051`；以 `-grpc-addr=""` 启动时仅 HTTP，日志说明 gRPC 已禁用

#### Scenario: 语义一致性
- **WHEN** 同一输入分别调用 gRPC `GetDay/QueryDays/QueryStats` 与 HTTP 对应接口
- **THEN** 日期类型数值、`type_label`、`total_days`、stats 计数完全一致

#### Scenario: 参数错误
- **WHEN** gRPC 请求 `date="2026-02-30"` 或 `end<start`
- **THEN** 返回 `codes.InvalidArgument`，message 与 HTTP 同类错误一致

### Requirement: 测试分层（白盒/黑盒）与 fuzz 测试

系统 SHALL 将单元测试按可见性分层。**白盒测试**位于与被测包同名的内部测试包，允许访问未导出标识符，覆盖分支、边界与错误路径；每个测试文件头 SHALL 以注释标注「白盒/黑盒」及测试视角。**黑盒测试** SHALL 位于根包外部测试包 `package goliday_test`，仅引用 `goliday` 导出 API（`LoadYear`/`LoadDir`/`Store`/`NewCalendar`/`Calendar`/`YearConfig.Validate`/`DayType` 常量与方法），不引用任何未导出标识符，以使用方视角验证对外行为契约。文件布局：

| 包 | 文件 | 视角 |
|---|---|---|
| 根包 `goliday` | `config_test.go`（未导出 `parseDate` 的严格解析 + `FuzzParseDate`） | 白盒 |
| 根包 `goliday` | `calendar_internal_test.go`（未导出 `comboIndex` 非法组合、`NewCalendar` nil 配置兜底） | 白盒 |
| 根包 `goliday_test` | `daytype_test.go`（枚举契约 + 256 值穷举不变量） | 黑盒 |
| 根包 `goliday_test` | `calendar_test.go`（判定/区间/统计契约 + `FuzzQueryConsistency`） | 黑盒 |
| 根包 `goliday_test` | `config_blackbox_test.go`（LoadYear/LoadDir/Validate 契约 + `FuzzLoadYearTOML`，含黑盒公用 helper 与内联 TOML 常量） | 黑盒 |
| 根包 `goliday_test` | `store_test.go`（目录加载契约） | 黑盒 |
| `cmd/goliday-server`（`package main`） | `handlers_test.go`、`grpc_test.go`、`handlers_fuzz_test.go`（`FuzzDaysHandler`） | 白盒 |
| `cmd/goliday-tool`（`package main`） | `gen_test.go`、`gen_fuzz_test.go`（`FuzzGenDraft`） | 白盒 |

系统 SHALL 提供原生 fuzz 测试（Go 标准 `testing.F`），种子语料内联于测试（不落盘语料目录），并保证 `go test`（非 fuzz 模式）仅执行种子即全部通过：

| Fuzz 目标 | 所属 | 不变量 |
|---|---|---|
| `FuzzParseDate` | 根包 `package goliday`（白盒） | 任意字符串：解析成功 ⇔ `time.Parse("2006-01-02", s)` 接受且 `Format` 回环一致；成功值再解析幂等；失败必须返回非 nil error |
| `FuzzQueryConsistency` | 根包 `package goliday_test`（黑盒） | 任意构造的 `time.Time`：已加载年份 `Query` 结果 ∈ 合法细粒度值全集 {1,2,6,10,17} 且 `QueryCoarse == Query().Coarse()`、`IsWork/IsRest` 与之互斥一致、同一日不同时刻（+5h/+23h）与 UTC/+08:00 表示结果不变；未加载年份断言返回 `ErrYearNotLoaded` |
| `FuzzLoadYearTOML` | 根包 `package goliday_test`（黑盒） | 任意年份 + TOML 文本：`LoadYear` 成功 ⟹ `Validate()` 幂等通过、off 全为周一~五、work 全为周六/日、两集合互斥无重复、工作日节日均在 off、全部日期在 `year` 年内；经 `LoadDir` 构造的 `Calendar` 对 off 日含 `Rest` 位（节日当天为 `Rest\|Festival`，其余为 `Rest\|Adjusted`）、work 日为 `Work\|Compensate` |
| `FuzzDaysHandler` | `cmd/goliday-server` `package main`（白盒） | 任意查询串打到 `/api/v1/days` 与 `/api/v1/stats`：不 panic、状态码仅 200/400、响应恒为合法 JSON；200 且含 `days` 时升序唯一、`total_days == len(days)`；粗粒度 stats 之和 == `total_days`，细粒度五键之和 == `total_days`（MECE）；单日模式 `total_days == 1`；stats 路径不因跨度报错（未加载年份报 `year_not_loaded` 除外） |
| `FuzzGenDraft` | `cmd/goliday-tool` `package main`（白盒） | 任意年份 + 公告文本：解析条目区间有效且在年内；草稿 off 全为周一~五、work 全为周六/日、互斥无重复、全在年内；festival 日期非 TODO 则为合法 `YYYY-MM-DD`；`selfCheck` 失败仅允许 TODO 占位、"festival 日期重复"、"节日当天不得补班"或"节日当天为工作日但不在 off"；自检通过且文件名年份合法时 `render` 产物可被 `LoadYear` 加载 |

补充：DayType 为 uint8 小域，其映射不变量 SHALL 以**穷举测试**（黑盒遍历全部 256 个取值：`String` 输出收敛于合法值标签 ∪ `unknown` ∪ `invalid`、不 panic、`IsValid` 恰对 {1,2,6,10,17} 为 true）覆盖；对 5 个合法值 {1,2,6,10,17} 另行断言：`Coarse` 结果 ∈ {`DayTypeWork`, `DayTypeRest`} 且幂等、`IsWork`/`IsRest` 恰一为真、`t & DayTypeWork` 与 `t & DayTypeRest` 恰一非零。不再另设 fuzz 目标。

约束：fuzz 目标不得新增第三方依赖（仅 `testing`/`time`/标准库）；失败语料按 Go 惯例落盘 `testdata/fuzz/<Name>/` 后 SHALL 转写为常规回归用例（普通 Test 或种子）再删除语料文件，保持仓库无 fuzz 语料残留。

#### Scenario: 黑盒仅用导出 API
- **WHEN** 检查根包外部测试文件 `goliday_test.go` 的导入与引用
- **THEN** 其 `package` 为 `goliday_test`，且不引用 `normalizeDate`、`parseDate`、`yearFilePattern` 等未导出标识符

#### Scenario: fuzz 种子即回归
- **WHEN** 运行 `go test ./...`（不带 `-fuzz`）
- **THEN** 各 Fuzz 目标仅以种子语料执行且全部通过；`go test -fuzz=Fuzz -fuzztime=10s ./...`（逐包）无 crash

#### Scenario: 崩溃语料回填
- **WHEN** fuzz 发现失败并在 `testdata/fuzz/<Name>/` 落盘语料
- **THEN** 该语料转写为常规回归用例（种子或普通 Test）后删除语料文件，`git status` 无 fuzz 语料残留

### Requirement: 年份加载强校验（year_not_loaded）

系统 SHALL 在执行任何查询（单日、区间、离散、混合）前先确定查询实际覆盖的年份集合，并要求其中每个年份均已加载配置文件；任一年份未加载即报错，错误 SHALL 列出全部未加载年份（升序、去重），不得静默回退到系统周休判断。

**决策依据**：早期版本对无配置年份整年回退周休判断，会将「未配置」与「真实周末」混淆，故改为强校验报错。不要改回静默回退。

覆盖年份集合的确定规则：
- 单日 `date`：`{date.Year()}`；
- 区间 `[start, end)`：`start==end`（空区间）时为空集（不报错，返回空统计）；否则为 `[start.Year(), (end-1天).Year()]` 闭区间内全部整数年份（含中间整年）；
- 离散 `dates`：各日期年份的并集；
- 混合并集：区间覆盖年份 ∪ 列表年份。

错误契约：
- HTTP：400，`{"error":{"code":"year_not_loaded","message":"查询范围包含未加载的年份: 2027"}}`（多年份以 `", "` 分隔升序）；
- gRPC：`codes.InvalidArgument`，message 为 `"year_not_loaded: 查询范围包含未加载的年份: 2027"`（与 HTTP 同源文案）。

#### Scenario: 单日未加载年份
- **WHEN** 已加载 2025、2026，`GET /api/v1/days?date=2027-05-01`
- **THEN** 返回 400 `year_not_loaded`，message 含 `2027`

#### Scenario: 跨年区间含未加载中间年
- **WHEN** 已加载 2025、2027，`GET /api/v1/stats?start=2025-06-01&end=2027-12-31`（2026 未加载）
- **THEN** 返回 400 `year_not_loaded`，message 含 `2026`

#### Scenario: 空区间不触发
- **WHEN** `GET /api/v1/stats?start=2027-01-01&end=2027-01-01`（2027 未加载）
- **THEN** 返回 200，`total_days=0`，stats 全 0（空区间无覆盖年份）

#### Scenario: 已加载年份不受影响
- **WHEN** 查询 2025、2026 任意日期或区间
- **THEN** 行为正常（回归）

### Requirement: 细粒度组合计数前缀和统计

`Calendar` SHALL 在构造时（`NewCalendar`）为每个已加载年份构建前缀和数组，加载完成后只读、可被多个 goroutine 并发访问：

- 每年定长数组 `prefix`（`[366]`，覆盖闰年最大天数；平年仅前 365 个元素有效，尾部 1 个元素闲置不用；无全零首元素），随年份索引结构体内联（无独立堆分配与切片头），元素为 5 种合法细粒度值（`1/2/6/10/17`）各自的年内累计天数，以 **uint16 存储**（单一类型年内天数有界：无调整年的普通工作日至多 262 天——闰年且元旦为周一~周四，366 减 104 个周末；超出 uint8 上界 255，uint8 存储会在累计到 256 时回绕破坏 MECE 不变量；**按值计数**，五值 MECE）；
- `prefix[i]` 为**闭区间** `[元旦, 元旦+i天]`（含两端，即元旦起前 i+1 天）的累计；年内左闭右开查询 `[a, b)` SHALL 转换为闭区间下标后差分：`prefix[idx(b)-1] - prefix[idx(a)-1]`，下标为 -1（端点为元旦或之前）时以全零参与差分；
- 跨年/多段累加 SHALL 先将各段年内差分转入 int 宽类型累加器（`comboTotals`）再相加，禁止 uint16 直接跨段累加（会溢出）；
- 构建成本 O(年天数)，仅在构造时发生一次。

**决策依据**：统计 O(覆盖年数) 差分即可完成，故 `/stats` 解除范围限制；days 明细接口保留 366 天上限防响应膨胀。前缀元素以 uint16 存储（uint8 的「工作日至多 248 天」上界估计漏算了无调整年：空 off/work 配置合法，闰年且元旦为周一~周四时普通工作日达 262 天，uint8 累计到 256 回绕会破坏五键之和 == Total 的 MECE 不变量）、数组定长 `[366]` 随年份索引结构体内联（省去切片头与独立堆分配、访问少一次间接寻址；平年尾部闲置 10B 为代价，闭区间下标省去无意义的全零首元素）压缩内存与分配数；uint16 只约束年内单段，跨年累加经 int 转换保持溢出安全。

统计导出规则（由按值计数 `C(v)` 直接映射，语义与逐日统计完全等价）：
- 细粒度（五键 MECE，之和恒等于 `Total`）：`ordinary=C(1)`、`weekend=C(2)`、`festival=C(6)`、`adjusted_rest=C(10)`、`adjusted_work=C(17)`；
- 粗粒度（单次位与归类）：`workday=C(1)+C(17)`、`holiday=C(2)+C(6)+C(10)`；
- `Total` = 覆盖天数（区间天数或列表长度）。

#### Scenario: 年内差分
- **WHEN** 统计 `[a, b)`（a、b 同年，b 可为次年元旦）
- **THEN** 结果 = `prefix[idx(b)-1] - prefix[idx(a)-1]`（下标 -1 视为全零），`idx(d)` = d 距当年元旦的天数

#### Scenario: 跨年拆分
- **WHEN** 统计 `[s, e)` 跨多年
- **THEN** 拆为「首年 `[s, 次年元旦)` + 若干整年 + 末年 `[当年元旦, e)`」各段差分后相加，复杂度 O(覆盖年数)

#### Scenario: 与逐日统计等价
- **WHEN** 对任意已加载年份的任意区间/日期集合分别用前缀和与逐日 `Query` 暴力统计
- **THEN** 粗、细全部计数与 `Total` 完全一致

### Requirement: 开源许可证

仓库根 SHALL 提供 `LICENSE` 文件，采用 **MIT 许可证**标准文本，版权行 `Copyright (c) 2026 Jayce Chant (陈思杰)`。

README 双语 SHALL 在标题下接入 MIT License 徽章（链接 `LICENSE`）；许可证说明收录在 [docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md) 质量门禁表格的条目中。

**决策依据**：MIT 为最宽松的常用许可证，利于嵌入调用与 Scorecard 的 License 检测项；版权人保留中文名以便实名归属。

#### Scenario: License 检测通过
- **WHEN** GitHub 或 OpenSSF Scorecard 检测仓库许可证
- **THEN** 识别为 MIT；README 双语徽章均可跳转至 `LICENSE`

### Requirement: 自动化版本发布（GitHub 环境）

仓库 SHALL 提供 `.github/workflows/release-please.yml` 与根目录 `.release-please-manifest.json`，基于 Conventional Commits 自动化版本发布（release-please）。

工作流约定：
- 触发：`push` 默认分支、`workflow_dispatch`；
- `googleapis/release-please-action`（完整 commit SHA 固定，与全仓 Action 固定策略一致）以 `release-type: simple` 运行——Go 模块无内嵌版本文件，无需更新产物版本，仅维护 `CHANGELOG.md` 与 tag/Release；
- 版本基线取 `.release-please-manifest.json`（当前 `"." : "0.1.0"`）：有 `feat`/`fix`/`BREAKING CHANGE` 累积时生成 Release PR（汇总提交并更新 `CHANGELOG.md`）；Release PR 合并后创建附注 tag `vX.Y.Z` 与 GitHub Release；
- 权限最小化：`contents: write`（提交 CHANGELOG、打 tag、建 Release）+ `pull-requests: write` + `issues: write`（Release PR 标签）+ `actions: write`（下游工作流 dispatch）；
- 镜像发布衔接：`GITHUB_TOKEN` 产生的 tag push 不触发其他工作流（GitHub 防递归约定），release-please 输出 `releases_created` 为真时以 `gh workflow run docker.yml --ref <tag_name>` 显式 dispatch（`workflow_dispatch` 是 GITHUB_TOKEN 可触发的例外事件），docker.yml 零改动复用既有 tag 发布口径；
- 无自定义 secrets：全部使用内置 `GITHUB_TOKEN`。

#### Scenario: Release PR 生成与合并
- **WHEN** 默认分支合并含 `feat` 的提交后 release-please.yml 运行
- **THEN** 生成/更新 Release PR（含 `CHANGELOG.md` 增量与版本号提升）；PR 合并后新 tag `vX.Y.Z` 与 GitHub Release 自动创建，`CHANGELOG.md` 入库

#### Scenario: 发布后镜像自动推送
- **WHEN** Release PR 合并触发 release-please 创建新 tag
- **THEN** release-please.yml dispatch docker.yml 于该 tag ref 运行，多架构镜像以对应 semver 标签发布至 GHCR（与手动推送 tag 同口径）

## 附录：核心包 API 形态

```go
package goliday // 根包

// ErrYearNotLoaded 查询覆盖了未加载配置的年份；errors.Is 判别，message 含年份。
var ErrYearNotLoaded = errors.New("年份配置未加载")

type DayType uint8
func (t DayType) IsWork() bool
func (t DayType) IsRest() bool
func (t DayType) IsFestivalRest() bool
func (t DayType) IsAdjustedRestDay() bool
func (t DayType) IsAdjustedWorkDay() bool
func (t DayType) IsValid() bool
func (t DayType) Coarse() DayType
func (t DayType) Adjustment() DayType
func (t DayType) String() string

type YearConfig struct { Year int; Name string; Festivals []Festival; Adjust Adjust }
type Festival  struct { Name string; Date time.Time }
type Adjust    struct { Off, Work []time.Time }

func LoadDir(dir string) (*Store, error)   // 加载 <year>.toml
func (s *Store) Has(year int) bool

type Calendar struct{ /* 由 Store 构造：各年索引 + 组合计数前缀和 */ }
func NewCalendar(s *Store) *Calendar
func (c *Calendar) HasYear(year int) bool
func (c *Calendar) Query(date time.Time) (DayType, error)        // 细粒度；未加载年 → ErrYearNotLoaded
func (c *Calendar) QueryCoarse(date time.Time) (DayType, error)  // 粗粒度；未加载年 → ErrYearNotLoaded
func (c *Calendar) IsWork(date time.Time) (bool, error)          // 未加载年 → ErrYearNotLoaded
func (c *Calendar) IsRest(date time.Time) (bool, error)          // 未加载年 → ErrYearNotLoaded
func (c *Calendar) QueryRange(start, end time.Time) ([]Dated, error) // 左闭右开逐日；未加载年 → ErrYearNotLoaded
func (c *Calendar) StatsRange(start, end time.Time, detailed bool) (StatsResult, error) // 前缀和差分，不限跨度
func (c *Calendar) Stats(dates []time.Time, detailed bool) (StatsResult, error)        // 排序去重集合分段差分
```
