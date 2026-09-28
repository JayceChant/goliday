# 节假日 API 服务（goliday）Spec

## 项目定位

基于 Go 1.27 的中国节假日 API 服务：以按年组织的**稀疏**配置文件记录节假日办对日历的调整（仅写入与默认周休状态不同的日期），配合标准库周休判断，对外提供粗/细两级粒度的日期类型查询与区间统计能力，并在每年官方方案公布后可通过提示词/工具半自动生成新年度配置。

各 Requirement 中的 **决策依据** 记录关键取舍结论，后续修改前先审视，避免反复；次要有效细节（完整步骤约定、核心包 API 形态清单）存档于 [spec.detail.md](spec.detail.md)，历史论证与过时内容存档于 [spec.legacy.md](spec.legacy.md)（当前有效契约以本文件为准）。

## Requirements

### Requirement: 模块结构与依赖约束

系统 SHALL 在仓库根创建 Go module（module 名 `github.com/JayceChant/goliday`，go 指令 1.27）；核心逻辑 SHALL 位于根包 `goliday`（导入路径 `github.com/JayceChant/goliday`）；服务与工具 SHALL 分别位于 `cmd/goliday-server`、`cmd/goliday-tool`。依赖按包分级：**根包（核心包）仅引入 `github.com/BurntSushi/toml`**，不得导入 gRPC/protobuf；HTTP 处理仅标准库；gRPC 三件套（`google.golang.org/grpc`、`google.golang.org/protobuf` 及传递依赖）仅允许出现在 `proto/goliday/v1/` 生成代码包与 `cmd/` 子包。

**决策依据**：核心库保持最小依赖面，gRPC 运行时隔离在入口与生成代码中。

#### Scenario: 依赖审计通过
- **WHEN** 运行 `go list -deps .`（根包）与 `go list -m all`
- **THEN** 根包导入中无 gRPC/protobuf 模块；`go.mod` 直接依赖仅 toml、grpc、protobuf，其余为传递依赖或 go.mod `tool` 指令锁定的构建工具（buf、protoc-gen-go、protoc-gen-go-grpc，任何包均不导入）

### Requirement: DayType 位掩码枚举（终态双层编码）

系统 SHALL 定义 `type DayType uint8`，采用「终态双层」编码：粗粒度为两个**单 bit 互斥基本位**（当日最终是否上班），细粒度调整位依附粗粒度位组合成具体日期类型；全部组合值的按位或均为 **all-of（合取）**语义，全类型不存在 any-of（并集物化值）语义。

枚举取值（`daytype.go`）：

| 常量 | 值 | 层 | 含义 |
|---|---|---|---|
| `DayTypeUnknown` | 0 | 零值（默认值） | 未定义类型；**非合法细粒度取值**，判类恒 false，`String()` 输出 `unknown` |
| `DayTypeWork` | 1 | 粗粒度基本位 | 上班 |
| `DayTypeRest` | 2 | 粗粒度基本位 | 放假（未调整时必然为周末） |
| `DayTypeFestival` | 4 | 调整位 | 过节：法定节日当天（放假），当日新增法定假期 |
| `DayTypeAdjustedRest` | 8 | 调整位 | 调休：原工作日被调整为休息（非节日当天），不新增假期 |
| `DayTypeAdjustedWork` | 16 | 调整位 | 补班：原周末被调整为上班 |
| `dayTypeCoarseMask` | 3（未导出） | 粗粒度掩码 | `Work\|Rest` 之并，仅供投影/判类（`Coarse`/`IsWork`/`IsRest`），**本身非合法取值**；不导出以免被当作类型值使用 |
| `dayTypeAdjustMask` | 28（未导出） | 调整位掩码 | `Festival\|AdjustedRest\|AdjustedWork` 之并，仅供投影/判类（`Adjustment`/三个组合判断），**本身非合法取值**；同上不导出 |

调整位常量名采用动宾结构（Adjusted**Rest** 调休 / Adjusted**Work** 补班），与基本位 Rest/Work 词根对齐，消除「调的是休还是班」的宾语歧义；`type_label` 分段名与常量名逐字对应（`adjusted_rest`/`adjusted_work`），细粒度 stats 键与分段名同词。

细粒度合法组合全集（5 种，MECE，数值之和即总天数）；组合值 SHALL 提供同名义常量：

| 值 | 常量 | 语义 |
|---|---|---|
| 1 | `DayTypeWork` | 普通工作日 |
| 2 | `DayTypeRest` | 普通周休（未调整的自然周末） |
| 6 | `DayTypeFestivalRest`（`Rest\|Festival`） | 节日放假日（无论节日落在工作日还是周末，终态同为「放假\|过节」） |
| 10 | `DayTypeAdjustedRestDay`（`Rest\|AdjustedRest`） | 调休放假日（原工作日；来源含拼假挪移与节日逢周末的补休，日类型不区分） |
| 17 | `DayTypeAdjustedWorkDay`（`Work\|AdjustedWork`） | 补班日（原周末） |

粗粒度归属 SHALL 为合法值上的单次按位与：`t & DayTypeRest != 0` → 放假、`t & DayTypeWork != 0` → 上班（合法值恰含一个基本位，无歧义、无需优先级消歧）；`Coarse()` SHALL 返回 `t & dayTypeCoarseMask`，合法值上结果 ∈ {1, 2} 且幂等；`Adjustment()` SHALL 返回 `t & dayTypeAdjustMask`，合法值上结果 ∈ {0, 4, 8, 16}（无调整位时为 0）且幂等。

非法值（可编码，但校验与判定不得产生）：`0`（默认零值）与含未定义位（≥32）的值；`3`（上班∧放假矛盾）；裸调整位 `4`/`8`/`16`；`5`/`9`（过节/调休与上班矛盾）；`18`（补班与放假矛盾）；`12`/`14`/`20`/`22` 等多调整位组合（同日至多一个调整位）。`off` 必为工作日、`work` 必为周末、工作日节日必须在 `off` 由配置校验保证。

**语义约定**（SHALL 写入用户文档）：
- 「节日」均指产生法定假期的全体公民节日，不放假的纪念日不纳入；节日当天必为放假日（配置校验强制）。
- 「调休」为窄义：原工作日因安排变休息且**非节日当天**；「过节」当日新增假期，「调休」零新增，「补班」为负增量。净增假日 = 过节日数 − 补班数（审计参考，不做硬校验）。
- 「节日逢周末在他日补休」「挪移与补班成对」为公告层配对关系，单日类型不编码、不做硬校验；自然日不进入类型值，可由日期 `Weekday` 与配置推导。

**决策依据**：旧编码混用 any-of/all-of 双语义致判类歧义、需优先级补丁，本修订统一为 all-of；「自然位」方案因翻转日判类必然误判而否决（完整论证见 spec.legacy.md）。

`DayType` SHALL 提供：`IsWork()`（合法值且基本位投影为 Work）、`IsRest()`（含普通周休/节日放假日/调休放假日，不含补班）、`IsFestivalRest()`、`IsAdjustedRestDay()`、`IsAdjustedWorkDay()`（三者与 `IsWork`/`IsRest` 同构：合法值且调整位投影等于对应调整位）、`IsValid()`（∈ {1,2,6,10,17}）、`Coarse()`、`Adjustment()`、`String()`；序列化 SHALL 直接输出 int 数值。任何非法值上五个判类方法 SHALL 均返回 false（各方法内部先做合法性校验再投影判等）。`String()`：合法值与零值 SHALL 查静态标签表直接返回（表按组合常量显式索引、编译期物化；标签与常量名逐字对应：`unknown`、`work`、`rest`、`rest|festival`、`rest|adjusted_rest`、`work|adjusted_work`）；任何非法值 SHALL 统一返回 `"invalid"`——系统不产生非法值，统一词便于调用方兜底。

#### Scenario: 粗细段映射
- **WHEN** 细粒度值分别为 `Work`、`AdjustedWorkDay` 与 `Rest`、`FestivalRest`、`AdjustedRestDay`
- **THEN** `Coarse()` 分别等于 `DayTypeWork`/`DayTypeRest`；`IsWork()`/`IsRest()` 与之一致且互斥

#### Scenario: 位与判类无歧义
- **WHEN** 对全部合法值 {1, 2, 6, 10, 17} 分别执行 `t & DayTypeRest` 与 `t & DayTypeWork`
- **THEN** 恰一非零

#### Scenario: 非法值全拒
- **WHEN** 对非法值 0、3、4、5、9、12、18 分别调用 `IsWork`/`IsRest`/`IsValid`/`IsFestivalRest`/`IsAdjustedRestDay`/`IsAdjustedWorkDay`
- **THEN** 六者均返回 false（5「基本位正确而调整位矛盾」与 12「多调整位并存」也须 false，故先合法性校验再投影判等）

#### Scenario: 组合判断与调整位投影
- **WHEN** 对五个合法值分别调用 `IsFestivalRest`/`IsAdjustedRestDay`/`IsAdjustedWorkDay` 与 `Adjustment()`
- **THEN** 各判断仅对其对应调整位的合法组合值返回 true（合法值前提下的调整位投影判等，与 `IsWork`/`IsRest` 同构）；`Work`/`Rest` 投影为 0，`FestivalRest`/`AdjustedRestDay`/`AdjustedWorkDay` 分别投影为 `Festival`/`AdjustedRest`/`AdjustedWork`，且 `Adjustment().Adjustment()` 幂等

#### Scenario: 字符串表示
- **WHEN** 对 `FestivalRest`、`AdjustedWorkDay`、`Rest`、零值 `Unknown` 调用 `String()`
- **THEN** 分别返回 `"rest|festival"`、`"work|adjusted_work"`、`"rest"`、`"unknown"`

### Requirement: 稀疏配置文件格式（TOML）

配置 SHALL 采用 TOML，每年一个文件，命名 `<year>.toml`（如 `2026.toml`），服务启动时经 `-config-dir` 加载目录内全部年份文件。

**稀疏表原则**：配置只记录节假日办对日历的"调整"，即与默认周休状态不同的日期；凡可由标准日期库判定的信息（周六/周日、普通工作日）不得写入。

文件结构（完整示例见 `docs/holiday_config_example.toml`）：

```toml
year = 2026

# 节日定义：仅名称与节日当天日期（农历节日每年漂移，必须显式配置）
[[festival]]
name = "春节"
date = "2026-02-17"

[adjust]
# 放假日：被调整为休息的工作日（周一~周五），含节日当天（若为工作日）与调休日
off = [ "2026-02-16", "2026-02-17", "2026-02-20" ]  # 节期内的周末不写入
# 补班日：被调整为上班的周末（周六/周日）
work = [ "2026-02-28" ]
```

**格式选型**：TOML——注释原生支持（JSON 被排除）、无缩进敏感与隐式类型转换陷阱（YAML 的 `01-01` 隐式解析风险，LLM 与手写均易错）、扁平字符串数组贴合稀疏表、Go（BurntSushi/toml）与 Python（tomllib）生态成熟。详见 `docs/CONFIG_FORMAT.md`。

校验规则（加载与 `goliday-tool validate` 一致）：
- `year` 必须与文件名一致；所有日期合法（含闰年）且落在 `year` 年内；
- `off` 中日期必须为周一~周五；`work` 中日期必须为周六/周日（写入与默认周休一致的日期即违反稀疏原则）；
- `off`、`work` 各自无重复且两集合互斥；`work` 不得包含任何 `festival.date`（节日当天不得补班）；
- `festival.date` 为周一~周五（工作日）时必须出现在 `off` 中——节日当天必为放假日，须经 `off` 落地（否则该日在类型上无合法状态可归）；
- 违规时返回明确错误（指明文件与原因），服务启动失败。

单日期判断算法（优先级从高到低）：
1. `date == 某 festival.date` → `Rest|Festival(6)`——节日当天必放假（落周末为自然休息、落工作日由 `off` 落地，不另标调休位）；
2. `date ∈ work` → `Work|AdjustedWork(17)`（work 必为周末，校验保证）；
3. `date ∈ off` → `Rest|AdjustedRest(10)`（off 必为工作日）；
4. 周休回退：周六/周日 → `Rest(2)`，否则 `Work(1)`；
5. 该年无配置文件 → 见「年份加载强校验」：返回 `ErrYearNotLoaded`，不回退。

#### Scenario: 启动加载
- **WHEN** 服务以 `-config-dir /etc/goliday` 启动且目录含 `2025.toml`、`2026.toml`、`README.md` 及子目录
- **THEN** 仅加载两个年份文件，其余忽略（不报错）；核心包不写日志（日志由服务入口基于 `Years()` 输出，已加载年份另可经 `/healthz` 查询）

#### Scenario: 调整与回退判定
- **WHEN** `off` 含 `2026-02-20`（周五）；`work` 含 `2026-02-28`（周六）；查询 `2026-03-03`（周二，未被任何条目覆盖）
- **THEN** 分别判定为 `Rest|AdjustedRest=10`、`Work|AdjustedWork=17`、`Work=1`（周休回退）

#### Scenario: 节日判定
- **WHEN** `2026-02-17`（周二，春节当天）在 `off` 中且为 `festival.date`；另某 `festival.date` 为周日且不在 `off`/`work` 中
- **THEN** 均为 `Rest|Festival = 6`（与工作日节日同为「节日放假日」），粗粒度均为 `Rest`

#### Scenario: 校验拒绝
- **WHEN** 某 `festival.date` 为周二但不在 `off` 中；或 `off` 含周末、`work` 含工作日、两集合相交等稀疏原则违规
- **THEN** 加载返回错误，服务启动失败；`testdata/invalid/` 提供此类样例

### Requirement: 单日期查询

核心包 SHALL 提供 `Query(date time.Time) (DayType, error)`（细粒度）与 `QueryCoarse(date time.Time) (DayType, error)`；`IsWork`/`IsRest` 同步返回 `error`。`date` 年份未加载时返回包装 `ErrYearNotLoaded` 的错误（`errors.Is` 可判别，message 含年份），不回退周休判断（见「年份加载强校验」）。

**决策依据**：消除「未配置」与「真实周末」的静默混淆，不回退周休判断。不要改回静默回退。

核心包 SHALL 提供 `Stats(dates []time.Time, detailed bool) (StatsResult, error)` 统计任意日期集合：输入经内部**排序去重副本**归一（调用方无须预处理，入参切片不被修改），`Total` 为归一后的元素数；任一日期所在年份未加载时返回包装 `ErrYearNotLoaded` 的错误（未加载年份升序去重列出），空输入返回全零结果。实现为逐日单日差分，复用前缀和数据。

**决策依据**：曾以哨兵检测年份首见，首个日期恰为 0 年时失效致 panic；改为内部归一的防御式 API（详见 spec.legacy.md）。

服务层 SHALL 暴露 HTTP 单日查询：

`GET /api/v1/days?date=2026-02-20`（`detailed=true|false`，默认 false）
- 响应：`{"date":"2026-02-20","type":2,"type_label":"rest"}`；`detailed=true` 时 `{"date":"2026-02-20","type":10,"type_label":"rest|adjusted_rest"}`。

#### Scenario: 非法日期
- **WHEN** `GET /api/v1/days?date=2026-02-30`
- **THEN** 返回 400 与统一错误信息

### Requirement: 区间、离散列表与统计查询（同一接口）

同一接口 SHALL 支持区间（左闭右开 `[start, end)`）与离散列表 `dates`（逗号分隔），两者可同时提供（并集、去重、升序）：区间 `?start=2026-02-01&end=2026-02-28`、离散 `?dates=2026-02-16,2026-02-28,2026-02-17`、混合 `?start=2026-02-01&end=2026-02-03&dates=2026-03-08`。

区间响应（粗粒度，列表/混合响应 `mode` 为 `"list"`、结构同上去除区间字段）：

```json
{ "mode": "range", "start": "2026-02-01", "end": "2026-02-28", "total_days": 27,
  "days": [ { "date": "2026-02-01", "type": 2, "type_label": "rest" } ],
  "stats": { "holiday": 8, "workday": 19 } }
```

细粒度模式下 `days[].type` 为细粒度值（`days[].type` 恒为细粒度，`detailed` 仅切换 `stats` 口径），`stats` 为 `{"ordinary":n,"weekend":n,"festival":n,"adjusted_rest":n,"adjusted_work":n}`——五键 MECE（各计一类日），**之和恒等于 `total_days`**（不存在交叉计数）。

跨度限制分化：`/api/v1/days`（含 gRPC `QueryDays`）区间跨度上限 **366 天**（防响应膨胀）；`/api/v1/stats`（含 `QueryStats`）**不限跨度**（前缀和实现，见「细粒度组合计数前缀和统计」）。全部覆盖年份须已加载（见「年份加载强校验」）。`stats` 接口（`GET /api/v1/stats?start=...&end=...&detailed=...`）统计口径与 `/api/v1/days` 完全一致但不返回 `days` 明细，两接口 SHALL 复用同一处理器逻辑；混合并集统计 = 区间前缀和统计 + 列表中剔除落在区间内日期后的分段统计，二者相加。`days` 响应中的 `stats` 与同输入的 stats 接口完全一致（复用前缀和路径）。

#### Scenario: 区间与列表语义
- **WHEN** `start=2026-02-01&end=2026-02-28`；`dates=2026-02-16,2026-02-28,2026-02-17,2026-02-16`；混合 `start+end+dates`
- **THEN** 区间 `days` 覆盖 02-01 至 02-27 共 27 天不含 end；离散去重升序共 3 条 `total_days=3`；混合并集 `mode="list"` 共 3 条

#### Scenario: 细粒度统计 MECE 计数
- **WHEN** 02-17 为 `rest|festival`、02-28 为 `work|adjusted_work`
- **THEN** 该两日分别在 `festival`、`adjusted_work` 键各计 1，五键之和 == `total_days`

#### Scenario: 参数校验
- **WHEN** `end < start`、或（days 接口）区间跨度 > 366 天、或 `date`/`start+end`/`dates` 均缺省、或日期格式非法
- **THEN** 返回 400 与明确错误信息

#### Scenario: 统计一致性
- **WHEN** 同一区间分别调用 `/api/v1/stats` 与 `/api/v1/days`
- **THEN** 两者 `stats`、`total_days` 完全一致

### Requirement: HTTP 服务子包（仅标准库）

`cmd/goliday-server` SHALL 仅用标准库实现 HTTP 服务（`net/http`，Go 1.22 `ServeMux` 方法+路径路由模式）。

路由：`GET /api/v1/days`、`GET /api/v1/stats`、`GET /healthz`；中间件（函数装饰器实现）：请求日志、Panic 恢复。命令行参数（`flag`）：`-addr`（默认 `":8080"`）、`-grpc-addr`（默认 `":50051"`，空字符串禁用 gRPC）、`-config-dir`（默认 `"./configs"`）、`-v`（输出版本后退出）。HTTP 服务端 SHALL 配置 `ReadHeaderTimeout`（10s，防慢速头部连接长期占用）与 `IdleTimeout`（120s，回收 keep-alive 空闲连接）；不设读写整体超时（纯内存查询无长请求，避免干扰正常客户端）。

错误响应统一格式 `{"error":{"code":"...","message":"..."}}`；日期解析统一 `2006-01-02`。错误码：`missing_query`、`invalid_date`、`invalid_range`、`invalid_detailed`、`year_not_loaded`、`not_found`（404）、`method_not_allowed`（405）。

`GET /healthz` SHALL 返回 200 与健康信息：`status`（恒 `"ok"`）、`version`（构建期注入的运行版本，本地/`go install` 构建为 `dev`）、`years`（已加载年份，升序）、`loaded_at`（配置加载完成时刻，RFC3339 UTC；运维确认「新配置已随重启生效」的依据）。

#### Scenario: 健康检查
- **WHEN** `GET /healthz`
- **THEN** 返回 200 与 `{"status":"ok","version":"<版本>","years":[2025,2026],"loaded_at":"<RFC3339>"}`

#### Scenario: 未知路径/方法
- **WHEN** 访问未注册路径，或对 `/api/v1/days` 使用 POST
- **THEN** 分别返回 404/405 统一错误格式

### Requirement: 年度配置生成工具与提示词

系统 SHALL 提供 `cmd/goliday-tool`（Go 实现，子命令 `gen` 与 `validate`）与 `docs/generate_prompt.md` 提示词模板。

`goliday-tool gen -year 2027 -out configs/2027.toml < 公告.txt`：输入官方公告原文（stdin 或 `-file`），解析"X月X日至X月X日放假共N天""X月X日（周X）上班"等句式；输出符合稀疏表规则的 TOML 草稿（自动剔除假期中的周末日、补班周末写入 `work`、按节日名写入 `festival`）；`festival.date` 需按节日名推断（公历固定月日；农历从"正月初一"等表述或除夕/初一日期推断），无法推断时输出占位注释待人工补全。

`goliday-tool validate <file...>`：执行与加载一致的校验，通过退出码 0，否则输出错误并退出非 0。

`docs/generate_prompt.md`：LLM 提示词模板（公告原文占位 + 生成规则：稀疏原则、节日当天规则、输出 TOML 骨架），用于人工核对 `gen` 结果或替代解析。

#### Scenario: 生成草稿与校验
- **WHEN** 将 2026 年官方公告原文输入 `goliday-tool gen -year 2026`；对 `testdata/2026.toml` 与 `testdata/invalid/*.toml` 运行 `validate`
- **THEN** 生成的 `2026.toml` 草稿通过 `validate`；校验分别退出 0 与非 0

### Requirement: 容器镜像与发布（GitHub 环境）

仓库根 SHALL 提供 `Makefile`（构建入口，二进制与镜像**共用同一构建逻辑**：目标 `build`/`build-server`/`build-tool`/`download-deps`/`image`/`check`/`lint`/`fix`/`clean`；`VERSION` 经 `-ldflags` 注入、`GOOS`/`GOARCH` 透传、Windows 加 `.exe`；编译参数唯一定义于 `build-server`：`CGO_ENABLED=0` 静态、`-trimpath`、`-s -w`、版本注入）、`Dockerfile`（多阶段：`golang:1.27` 构建阶段调用 `make build-server`，运行阶段 `distroless/static-debian12:nonroot` 仅含 server 二进制；不打包 `configs/`，配置运行时挂载；`.dockerignore` 最小化构建上下文）与 `.github/workflows/docker.yml`（多架构 `linux/amd64`+`linux/arm64`，provenance/sbom 供应链证明，语义化版本标签；**仅 push tag `v*` 发布至 GHCR**，其余事件仅构建验证；`GITHUB_TOKEN` 认证，无自定义 secrets）。

版本号 SHALL 构建期注入，不得硬编码常量（release-please 的 simple 策略只维护 CHANGELOG/tag 不改代码，硬编码随发版漂移）。Makefile 与 Dockerfile 互注防递归约束：Dockerfile 只调用不触碰 docker 的目标（`download-deps`/`build-server`），不得调用 `make image`。完整目标清单、变量约定与步骤级细节见 [spec.detail.md](spec.detail.md)（有效存档，当前有效契约以本节为准）。

#### Scenario: 本地构建二进制与镜像
- **WHEN** 在仓库根执行 `make build VERSION=0.2.0`（或 `make image VERSION=0.2.0` / `docker build --build-arg VERSION=0.2.0`）
- **THEN** `bin/` 下生成两个静态二进制且 `-v` 输出版本；镜像多阶段构建成功，基于 distroless 以 nonroot 运行

#### Scenario: CI 构建与发布
- **WHEN** push tag `v0.2.0` 触发工作流
- **THEN** 构建多架构镜像并发布至 `ghcr.io/<owner>/goliday:0.2.0`（另含 `0.2`、`0`、`latest`）
- **WHEN** push 默认分支、PR 或 workflow_dispatch 触发工作流
- **THEN** 仅执行多架构构建验证，不登录、不推送任何镜像

### Requirement: 在线质量门禁与 CI 测试矩阵（GitHub 环境）

仓库 SHALL 提供以下工作流，将本地验证门禁搬上 GitHub Actions，结果以徽章与链接接入 README 双语（详细说明收录于 [docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md) 质量门禁与 CI 章节）：

- **ci.yml**：触发覆盖**所有分支 push 与全部 PR**（开发分支与 release-please Release PR 合入 master 前即受门禁），顶层 concurrency 按 `workflow-分支` 分组 + `cancel-in-progress` 去重双事件；测试作业 `1.27.x`（go.mod 最低要求）+ `stable` 双版本矩阵执行 build/vet/gofmt/`go test -race -coverprofile` 与 fuzz 冒烟（仅 stable 项，全部 5 个 fuzz 目标各 30s）；覆盖率仅 stable 项经 codecov-action 上传（`proto/` 生成代码经仓库根 `codecov.yml` 与上传前过滤双重排除，与 `.golangci.yml` 豁免同口径）；lint 作业 golangci-lint 零告警；proto 作业 buf lint（STANDARD 零豁免）/ buf breaking（仅 PR，对照远端 master）/ 再生成一致性（`buf generate` 后 `git diff --exit-code -- proto/`）。
- **govulncheck.yml**：依赖漏洞门禁，触发同 ci.yml + 每周 schedule 兜底；存在可触达漏洞即失败。
- **scorecard.yml**（OpenSSF Scorecard，评分发布 scorecard.dev、SARIF 入 code scanning）、**codeql.yml**（Go 静态安全分析，autobuild，告警入 code scanning）、**sonarcloud.yml**（质量门禁，依赖 secret `SONAR_TOKEN` 与根目录 `sonar-project.properties`，覆盖率排除口径同上）：分析类，push 默认分支 + PR 触发（scorecard 为每周 schedule）。

**决策依据**：门禁类（ci/govulncheck）全分支执行——问题在合入 master 前暴露；分析与发布类维持默认分支限定——重型扫描不随开发分支重复，其 PR→master 触发已覆盖合入前检查。Actions 一律固定完整 commit SHA。pkg.go.dev 为官方自动抓取，无需配置，README 提供徽章。各工作流触发/步骤/权限的完整约定与完整决策依据见 [spec.detail.md](spec.detail.md)（有效存档，当前有效契约以本节为准）。

#### Scenario: CI 测试矩阵与合入前门禁
- **WHEN** push 或 PR 触发 ci.yml；推送开发分支或创建 release-please Release PR
- **THEN** `1.27.x` 与 `stable` 两矩阵项各自完成 build/vet/gofmt/test，lint 作业零告警，任一步骤失败流水线标红；ci.yml 与 govulncheck.yml 照常执行全部门禁，同一分支 push 与 PR 并发运行经 concurrency 组去重仅保留最新一次

#### Scenario: 覆盖率上报
- **WHEN** `stable` 矩阵项测试通过
- **THEN** 过滤 proto 生成代码后的 `coverage.out` 上传至 Codecov，项目页统计不含生成代码；未配置 `CODECOV_TOKEN` 时公共仓库仍可上传

#### Scenario: 外部质量服务
- **WHEN** scorecard/codeql/govulncheck/sonarcloud 工作流按各自触发条件运行（SonarCloud 已配置 `SONAR_TOKEN`）
- **THEN** 评分/SARIF/告警/质量门禁状态分别上报 scorecard.dev、code scanning 与 SonarCloud 项目页；govulncheck 存在可触达漏洞时作业失败；README 徽章反映各服务状态

### Requirement: proto 定义与生成代码

系统 SHALL 提供 `proto/goliday/v1/goliday.proto`（syntax proto3，package `goliday.v1`，`option go_package = "github.com/JayceChant/goliday/proto/goliday/v1;golidayv1"`），供调用方直接引用；生成的 Go 代码 SHALL 入库于 `proto/goliday/v1/{goliday.pb.go,goliday_grpc.pb.go}`（调用方无需本地 protoc）。

proto 内容约定：
- `DayType` 掩码以 `uint32` 表达并附注释（proto3 enum 无法表达位组合），注释标明双层位值与 5 种合法组合（1/2/6/10/17），粗粒度即基本位投影、组合值 `|` 为 all-of 语义，与根包 `DayType` 完全一致；
- 消息：`Day{date,type,type_label}`、`Stats{workday,holiday,ordinary,adjusted_work,weekend,festival,adjusted_rest}`（粗粒度字段恒填充，细粒度字段仅 `detailed=true` 时填充，五键 MECE 之和恒等于 `total_days`；`adjusted_rest`/`adjusted_work` 与根包常量逐字对应）、`GetDayRequest{date,detailed}`、`GetDayResponse{date,type,type_label,total_days,stats}`、`QueryDaysRequest{start,end,dates[],detailed}`、`QueryDaysResponse{mode,start,end,total_days,days[],stats}`、`QueryStatsRequest{start,end,dates[],detailed}`（与 QueryDaysRequest 同构，独立消息以符合 buf lint 默认规则）、`QueryStatsResponse{mode,start,end,total_days,stats}`；
- 服务 `GolidayService`：`GetDay`（单日）、`QueryDays`（区间/离散/混合并集，含明细）、`QueryStats`（入参与 QueryDays 同构，不含明细）——语义与 HTTP `/api/v1/days`、`/api/v1/stats` 一一对应；
- 日期一律 `YYYY-MM-DD` 字符串；`mode` 取 `range`/`list`；
- proto 头注释写明再生成方式（buf：`go install tool` 按 go.mod `tool` 指令安装 buf 与 protoc 插件，仓库根执行 `buf generate`；工作区标准布局——模块根 `proto/`，`buf lint` STANDARD 零豁免）。

#### Scenario: 可供调用方引用与再生成
- **WHEN** 调用方查找 `proto/goliday/v1/goliday.proto` 与生成代码；在仓库根执行 `buf generate`
- **THEN** 无需 buf/protoc 即可 `import "github.com/JayceChant/goliday/proto/goliday/v1"` 或复制 .proto 生成其他语言桩代码；生成文件落盘于 `proto/goliday/v1/` 且 `gofmt`/`go build`/`buf lint` 通过

### Requirement: gRPC 服务（与 HTTP 同进程）

`cmd/goliday-server` SHALL 在同进程内提供 gRPC 服务：注册 `GolidayService` 与 gRPC 标准健康检查服务（`grpc.health.v1`）；优雅关闭 SHALL 同时覆盖 HTTP 与 gRPC。

gRPC 查询语义 SHALL 与 HTTP 完全一致（复用同一查询逻辑）：单日 `detailed` 粗/细切换、多日明细恒细粒度、区间左闭右开（`QueryDays` 跨度 ≤366 天，`QueryStats` 不限跨度）、离散去重升序、区间+离散并集 `mode=list`、`date` 与其他参数并存时 `date` 优先；参数错误 SHALL 映射为 `codes.InvalidArgument`，错误 `message` 文案与 HTTP 一致（含 `invalid_date`/`invalid_range`/`missing_query`/`invalid_detailed`/`year_not_loaded` 等标识）。

#### Scenario: 启用与禁用
- **WHEN** 以默认参数启动；以 `-grpc-addr=""` 启动
- **THEN** 分别为 HTTP `:8080` + gRPC `:50051` 双监听，与仅 HTTP（日志说明 gRPC 已禁用）

#### Scenario: 语义一致性与参数错误
- **WHEN** 同一输入分别调用 gRPC `GetDay/QueryDays/QueryStats` 与 HTTP 对应接口；gRPC 请求 `date="2026-02-30"` 或 `end<start`
- **THEN** 日期类型数值、`type_label`、`total_days`、stats 计数完全一致；参数错误返回 `codes.InvalidArgument`，message 与 HTTP 同类错误一致

### Requirement: 测试分层（白盒/黑盒）与 fuzz 测试

系统 SHALL 将单元测试按可见性分层。**白盒测试**位于与被测包同名的内部测试包，允许访问未导出标识符，覆盖分支、边界与错误路径；每个测试文件头 SHALL 以注释标注「白盒/黑盒」及测试视角。**黑盒测试** SHALL 位于根包外部测试包 `package goliday_test`，仅引用 `goliday` 导出 API，以使用方视角验证对外行为契约。文件布局：

| 包 | 文件 | 视角 |
|---|---|---|
| 根包 `goliday` | `config_test.go`（`ParseDate` + `FuzzParseDate`）、`calendar_internal_test.go`（未导出 `comboIndex` 非法组合、`NewCalendar` nil 兜底） | 白盒 |
| 根包 `goliday_test` | `daytype_test.go`（枚举契约 + 256 值穷举）、`calendar_test.go`（判定/区间/统计契约 + `FuzzQueryConsistency`）、`config_blackbox_test.go`（加载/校验契约 + `FuzzLoadYearTOML`）、`store_test.go`（目录加载契约） | 黑盒 |
| `cmd/goliday-server` | `handlers_test.go`、`grpc_test.go`、`main_test.go`（`run` 主流程）、`handlers_fuzz_test.go`（`FuzzDaysHandler`） | 白盒 |
| `cmd/goliday-tool` | `gen_test.go`、`main_test.go`（`dispatch` 分派）、`gen_fuzz_test.go`（`FuzzGenDraft`） | 白盒 |

系统 SHALL 提供原生 fuzz 测试（Go 标准 `testing.F`），种子语料内联于测试（不落盘语料目录），并保证 `go test`（非 fuzz 模式）仅执行种子即全部通过：

| Fuzz 目标 | 所属 | 不变量 |
|---|---|---|
| `FuzzParseDate` | 根包（白盒） | 解析成功 ⇔ `time.Parse("2006-01-02", s)` 接受且回环一致；成功值再解析幂等；失败必返回非 nil error |
| `FuzzQueryConsistency` | 根包（黑盒） | 已加载年份 `Query` ∈ {1,2,6,10,17} 且 `QueryCoarse == Query().Coarse()`、`IsWork/IsRest` 互斥一致、同一日不同时刻与 UTC/+08:00 表示结果不变；未加载年份返回 `ErrYearNotLoaded` |
| `FuzzLoadYearTOML` | 根包（黑盒） | `LoadYear` 成功 ⟹ `Validate()` 幂等、off 全周一~五、work 全周六/日、互斥无重复、工作日节日均在 off、全在年内；Calendar 对 off 日含 `Rest` 位（节日当天为 `Rest\|Festival`）、work 日为 `Work\|AdjustedWork` |
| `FuzzDaysHandler` | server（白盒） | 任意查询串：不 panic、状态码仅 200/400、响应恒合法 JSON；200 且含 `days` 时升序唯一且 `total_days == len(days)`；粗/细 stats 之和均 == `total_days`（MECE）；stats 路径不因跨度报错（未加载年份除外） |
| `FuzzGenDraft` | tool（白盒） | 解析条目区间有效且在年内；草稿 off/work 合法互斥；festival 非 TODO 则为合法日期；`selfCheck` 失败仅允许 TODO 占位/"日期重复"/"节日当天不得补班"/"工作日节日不在 off"；自检通过且年份合法时产物可被 `LoadYear` 加载 |

补充：DayType 为 uint8 小域，其映射不变量 SHALL 以**穷举测试**（黑盒遍历 256 个取值：`String` 输出收敛于合法值标签 ∪ `unknown` ∪ `invalid`、不 panic、`IsValid` 恰对 {1,2,6,10,17} 为 true）覆盖；对 5 个合法值另行断言 `Coarse` ∈ {`Work`, `Rest`} 且幂等、`IsWork`/`IsRest` 恰一为真、`t & Work` 与 `t & Rest` 恰一非零。不再另设 fuzz 目标。

性能取向的实现（稀疏终态表、按值计数前缀和等）SHALL 以黑盒基准测试（`calendar_bench_test.go` 五个基准，`b.Loop` + `ReportAllocs`，testdata 真实配置复用加载）提供回归基线；基准仅在显式 `-bench` 时运行，不进入 CI 门禁。

入口 `main()` SHALL 仅保留进程级胶水（参数透传与退出码），业务逻辑 SHALL 下沉至可测函数并配套测试（`cmd/goliday-server` 的 `run(args)`、`cmd/goliday-tool` 的 `dispatch(args)`）。fuzz 目标不得新增第三方依赖（仅标准库）；失败语料落盘 `testdata/fuzz/<Name>/` 后 SHALL 转写为常规回归用例再删除语料文件，保持仓库无 fuzz 语料残留。

#### Scenario: 覆盖率维持
- **WHEN** 运行 `go test -count=1 -cover ./...` 统计语句覆盖率
- **THEN** 手写生产代码整体覆盖率（排除 `proto/` 生成代码，与 Codecov/SonarCloud/`.golangci.yml` 同口径）不低于 **95%**（当前基线约 98%）
- **AND** 允许不覆盖的例外仅限：入口 `main()` 的退出胶水语句；经论证不可达的防御分支（年份已校验后核心包再报错的 500 兜底、`parseCNNum`/`lunarMarks` 中正则保证可解析的 `Atoi` 失败分支、`ParseDate` 的回格式化比对兜底）。新增生产代码 SHALL 配套测试，不得跌破门禁；确需豁免的新例外 SHALL 在本 Scenario 补充记载

#### Scenario: 黑盒仅用导出 API
- **WHEN** 检查根包外部测试文件的导入与引用
- **THEN** 其 `package` 为 `goliday_test`，且不引用 `normalizeDate`、`parseDate`、`yearFilePattern` 等未导出标识符

#### Scenario: fuzz 种子即回归与语料回填
- **WHEN** 运行 `go test ./...`（不带 `-fuzz`）；fuzz 发现失败并在 `testdata/fuzz/<Name>/` 落盘语料
- **THEN** 各目标仅以种子执行且全部通过（CI 另做 30s 短时真实 fuzz，见「在线质量门禁与 CI 测试矩阵」）；语料转写为常规回归用例后删除，`git status` 无 fuzz 语料残留

### Requirement: 年份加载强校验（year_not_loaded）

系统 SHALL 在执行任何查询（单日、区间、离散、混合）前先确定查询实际覆盖的年份集合，并要求其中每个年份均已加载配置文件；任一年份未加载即报错，错误 SHALL 列出全部未加载年份（升序、去重），不得静默回退到系统周休判断。

**决策依据**：早期版本对无配置年份整年回退周休判断，会将「未配置」与「真实周末」混淆，故改为强校验报错。不要改回静默回退。

覆盖年份集合的确定规则（统一经根包 `CoveredYears` 计算，服务层与库内同一实现，不得各自另写）：
- 单日 `date`：`{date.Year()}`；离散 `dates`：各日期年份的并集；混合并集：区间覆盖年份 ∪ 列表年份；
- 区间 `[start, end)`：`start==end`（空区间）时为空集（不报错，返回空统计）；否则为 `[start.Year(), (end-1天).Year()]` 闭区间内全部整数年份（含中间整年）。

错误契约：
- HTTP：400，`{"error":{"code":"year_not_loaded","message":"查询范围包含未加载的年份: 2027"}}`（多年份以 `", "` 分隔升序）；
- gRPC：`codes.InvalidArgument`，message 为 `"year_not_loaded: 查询范围包含未加载的年份: 2027"`（与 HTTP 同源文案）。

#### Scenario: 未加载年份报错
- **WHEN** 已加载 2025、2026，`GET /api/v1/days?date=2027-05-01`；已加载 2025、2027，`GET /api/v1/stats?start=2025-06-01&end=2027-12-31`（2026 未加载）
- **THEN** 均返回 400 `year_not_loaded`，message 分别含 `2027`/`2026`（跨年区间含中间整年）

#### Scenario: 空区间不触发
- **WHEN** `GET /api/v1/stats?start=2027-01-01&end=2027-01-01`（2027 未加载）
- **THEN** 返回 200，`total_days=0`，stats 全 0（空区间无覆盖年份）

#### Scenario: 已加载年份不受影响
- **WHEN** 查询 2025、2026 任意日期或区间
- **THEN** 行为正常（回归）

### Requirement: 细粒度组合计数前缀和统计

`Calendar` SHALL 在构造时（`NewCalendar`）为每个已加载年份构建前缀和数组，加载完成后只读、可被多个 goroutine 并发访问：

- 每年定长数组 `prefix`（`[366]`，覆盖闰年最大天数；平年仅前 365 个元素有效；无全零首元素），随年份索引结构体内联，元素为 5 种合法细粒度值（`1/2/6/10/17`）各自的年内累计天数，**uint16 存储、按值计数**（五值 MECE；单一类型年内天数有界，无调整年的普通工作日至多 262 天，uint8 会在 256 处回绕破坏 MECE 不变量）；
- `prefix[i]` 为**闭区间** `[元旦, 元旦+i天]`（含两端）的累计；年内左闭右开查询 `[a, b)` SHALL 转换为闭区间下标后差分：`prefix[idx(b)-1] - prefix[idx(a)-1]`，下标为 -1（端点为元旦或之前）时以全零参与差分；
- 跨年/多段累加 SHALL 先转入 int 宽类型累加器（`comboTotals`）再相加，禁止 uint16 直接跨段累加（会溢出）；
- 构建成本 O(年天数)，仅在构造时发生一次。

**决策依据**：统计 O(覆盖年数) 差分即可完成，故 `/stats` 解除范围限制；days 明细保留 366 天上限防响应膨胀。uint16 + 定长内联数组为内存/分配优化（完整论证见 spec.legacy.md）。

统计导出规则（由按值计数 `C(v)` 直接映射，语义与逐日统计完全等价）：
- 细粒度（五键 MECE，之和恒等于 `Total`）：`ordinary=C(1)`、`weekend=C(2)`、`festival=C(6)`、`adjusted_rest=C(10)`、`adjusted_work=C(17)`；`StatsResult.Fine` 的 map 键 SHALL 为五个合法终态组合值，不得使用 4/8/16 裸调整位（非合法 `DayType`，`String()` 输出 `invalid`，调用方难以索引与打印）；
- 粗粒度（单次位与归类）：`workday=C(1)+C(17)`、`holiday=C(2)+C(6)+C(10)`；`Total` = 覆盖天数。

#### Scenario: 差分与跨年拆分
- **WHEN** 统计 `[a, b)`（a、b 同年，b 可为次年元旦）；统计 `[s, e)` 跨多年
- **THEN** 年内结果 = `prefix[idx(b)-1] - prefix[idx(a)-1]`（下标 -1 视为全零）；跨年拆为「首年 + 若干整年 + 末年」各段差分后相加，复杂度 O(覆盖年数)

#### Scenario: 与逐日统计等价
- **WHEN** 对任意已加载年份的任意区间/日期集合分别用前缀和与逐日 `Query` 暴力统计
- **THEN** 粗、细全部计数与 `Total` 完全一致

### Requirement: 开源许可证

仓库根 SHALL 提供 `LICENSE` 文件，采用 **MIT 许可证**标准文本，版权行 `Copyright (c) 2026 Jayce Chant (陈思杰)`。README 双语 SHALL 在标题下接入 MIT License 徽章（链接 `LICENSE`）；许可证说明收录在 [docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md) 质量门禁表格。

**决策依据**：MIT 为最宽松的常用许可证，利于嵌入调用与 Scorecard 的 License 检测项；版权人保留中文名以便实名归属。

#### Scenario: License 检测通过
- **WHEN** GitHub 或 OpenSSF Scorecard 检测仓库许可证
- **THEN** 识别为 MIT；README 双语徽章均可跳转至 `LICENSE`

### Requirement: 自动化版本发布（GitHub 环境）

仓库 SHALL 提供 `.github/workflows/release-please.yml` 与根目录 `.release-please-manifest.json`，基于 Conventional Commits 自动化版本发布（release-please）：`release-type: simple`（Go 模块无内嵌版本文件，仅维护 `CHANGELOG.md` 与 tag/Release）；默认分支累积 `feat`/`fix`/`BREAKING CHANGE` 时生成 Release PR，合并后创建附注 tag 与 GitHub Release；`GITHUB_TOKEN` 的 tag push 不级联触发工作流，故经 `gh workflow run docker.yml --ref <tag>` 显式 dispatch 衔接镜像发布；权限最小化，无自定义 secrets。完整触发/权限/步骤约定见 [spec.detail.md](spec.detail.md)（有效存档，当前有效契约以本节为准）。

#### Scenario: Release PR 生成与合并
- **WHEN** 默认分支合并含 `feat` 的提交后 release-please.yml 运行
- **THEN** 生成/更新 Release PR（含 `CHANGELOG.md` 增量与版本号提升）；PR 合并后新 tag `vX.Y.Z` 与 GitHub Release 自动创建，`CHANGELOG.md` 入库

#### Scenario: 发布后镜像自动推送
- **WHEN** Release PR 合并触发 release-please 创建新 tag
- **THEN** release-please.yml dispatch docker.yml 于该 tag ref 运行，多架构镜像以对应 semver 标签发布至 GHCR（与手动推送 tag 同口径）
