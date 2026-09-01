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
- 所有日期合法（含闰年）且落在 `year` 年内；
- 违规时返回明确错误（指明文件与原因），服务启动失败。

单日期判断算法（优先级从高到低）：
1. `date ∈ work` → `Compensate|Weekend`（work 必为周末）；
2. `date ∈ off` → `Adjusted`（off 必为工作日）；
3. `date == 某 festival.date` → 在周休结果上附加 `Festival` 位；
4. 周休回退：周六/周日 → `Weekend`，否则 `Ordinary`；
5. 该年无配置文件 → 见「年份加载强校验」：返回 `ErrYearNotLoaded`，不回退。

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

#### Scenario: 稀疏原则违规
- **WHEN** `off` 含周末日期（如 `2026-01-03` 周六），或 `work` 含工作日，或两集合有交集
- **THEN** 加载返回错误，服务启动失败；`testdata/invalid/` 提供此类样例

### Requirement: 单日期查询

核心包 SHALL 提供 `Query(date time.Time) (DayType, error)`（细粒度）与 `QueryCoarse(date time.Time) (DayType, error)`；`IsWorkday`/`IsHoliday` 同步返回 `error`。`date` 年份未加载时返回包装 `ErrYearNotLoaded` 的错误（`errors.Is` 可判别，message 含年份），不回退周休判断（见「年份加载强校验」）。

**决策依据**：消除「未配置」与「真实周末」的静默混淆，故由早期的"无配置年回退周休"改为强校验报错。

服务层 SHALL 暴露 HTTP 单日查询：

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

列表/混合响应：`mode` 为 `"list"`，结构同上（不含区间字段）。

细粒度模式下 `days[].type` 为细粒度掩码，`stats` 为 `{"ordinary":n,"compensate":n,"weekend":n,"festival":n,"adjusted":n}`；组合日（如 `festival|adjusted`、`compensate|weekend`）SHALL 对其含有的每个标志各计 1 天（存在交叉计数，各键之和可大于 `total_days`）。

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

#### Scenario: 细粒度统计交叉计数
- **WHEN** 02-17 为 `festival|adjusted`、02-28 为 `compensate|weekend`
- **THEN** 该两日分别在 `festival`+`adjusted`、`compensate`+`weekend` 键中各计 1

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
- 触发：`push` 默认分支、`push` tag `v*`、`pull_request`（仅构建验证，不推送）、`workflow_dispatch`；
- 权限最小化：`contents: read` + `packages: write`；
- 步骤：checkout → buildx → QEMU（多架构）→ GHCR 登录（`GITHUB_TOKEN`）→ metadata 提取标签 → build & push（`linux/amd64` + `linux/arm64`，GHA 缓存，`provenance`/`sbom` 关闭以保持镜像单 manifest）；
- 标签策略（metadata-action）：分支名（默认分支）、语义化版本 `v1.2.3` → `1.2.3` / `1.2` / `1`、tag 事件附加 `latest`；
- 无自定义 secrets：GHCR 认证仅用内置 `GITHUB_TOKEN`。

#### Scenario: 本地构建镜像
- **WHEN** 在仓库根执行 `docker build -t goliday .`
- **THEN** 多阶段构建成功，最终镜像基于 distroless 且以 nonroot 运行，`docker run goliday -v` 输出版本后退出

#### Scenario: 容器启动并挂载配置
- **WHEN** `docker run -p 8080:8080 -v $PWD/configs:/data:ro goliday -config-dir /data`
- **THEN** 服务监听 8080/50051，`GET /healthz` 返回已加载年份

#### Scenario: CI 构建与发布
- **WHEN** push tag `v0.2.0` 触发工作流
- **THEN** 构建多架构镜像并发布至 `ghcr.io/<owner>/goliday:0.2.0`（另含 `0.2`、`0`、`latest`）；PR 事件仅构建不推送

### Requirement: 在线质量门禁与 CI 测试矩阵（GitHub 环境）

仓库 SHALL 提供 `.github/workflows/ci.yml` 与 `.github/workflows/scorecard.yml`，将本地验证门禁搬上 GitHub Actions，并把结果以徽章与链接接入 README 双语版。

ci.yml 约定：
- 触发：`push` 默认分支、`pull_request`（默认分支）、`workflow_dispatch`；权限最小化 `contents: read`；
- 测试作业：`stable` 与 `oldstable` 双版本矩阵（`actions/setup-go` 自带模块缓存），步骤 checkout → setup-go → `go build ./...` → `go vet ./...` → `gofmt` 检查（`gofmt -l .` 输出非空即失败）→ `go test -count=1 -race -covermode=atomic -coverprofile`；
- 覆盖率上报：仅 `stable` 矩阵项经 `codecov/codecov-action` 上传 `coverage.out`（secrets `CODECOV_TOKEN`；公共仓库可不配置 token，上传失败不阻塞流水线）；
- lint 作业：`golangci/golangci-lint-action` 运行 `golangci-lint`（v2，配置见 `.golangci.yml`）零告警。

scorecard.yml 约定（OpenSSF Scorecard，无需注册）：
- 触发：`push` 默认分支、每周 `schedule`、`branch_protection_rule`；顶层 `permissions: read-all`，作业内最小化（`id-token: write` 供发布 OIDC 认证、`security-events: write` 供 SARIF 上传）；
- `ossf/scorecard-action` 以 `publish_results: true` 发布评分至 scorecard.dev；SARIF 产物落盘 artifact 并经 `github/codeql-action/upload-sarif` 上传至 code scanning；checkout 置 `persist-credentials: false`。

pkg.go.dev 文档为 Go 官方服务自动抓取（模块可解析、可编译即自动建页），仓库无需配置，README SHALL 提供徽章与结果链接。

README 双语 SHALL 在标题下接入 CI、Codecov、pkg.go.dev、OpenSSF Scorecard 四枚徽章，并设「质量与持续集成」章节以表格链接各服务结果页。

**决策依据**：选取表中可直接在仓库内落地的服务（Actions/Codecov/Scorecard/pkg.go.dev）；SonarCloud、Snyk/Socket 需外部注册绑定，不在仓库内预置，避免空配置导致流水线常红。

#### Scenario: CI 测试矩阵
- **WHEN** push 或 PR 触发 ci.yml
- **THEN** `stable` 与 `oldstable` 两个矩阵项各自完成 build/vet/gofmt/test，lint 作业零告警；任一步骤失败流水线标红

#### Scenario: 覆盖率上报
- **WHEN** `stable` 矩阵项测试通过
- **THEN** `coverage.out` 上传至 Codecov，Codecov 项目页可逐行查看覆盖详情；未配置 `CODECOV_TOKEN` 时公共仓库仍可上传

#### Scenario: Scorecard 评分发布
- **WHEN** scorecard.yml 在默认分支运行
- **THEN** 评分发布至 scorecard.dev 项目页，SARIF 出现在仓库 code scanning；scorecard.dev 徽章可访问

### Requirement: proto 定义与生成代码

系统 SHALL 提供 `proto/goliday/v1/goliday.proto`（syntax proto3，package `goliday.v1`，`option go_package = "github.com/JayceChant/goliday/proto/goliday/v1;golidayv1"`），供调用方直接引用；生成的 Go 代码 SHALL 入库于 `proto/goliday/v1/{goliday.pb.go,goliday_grpc.pb.go}`（调用方无需本地 protoc）。

proto 内容约定：
- `DayType` 掩码以 `uint32` 表达并附注释（proto3 enum 无法表达位组合），注释标明细粒度位值（1/2/4/8/16）、粗粒度段值（3/28）与 6 种合法组合（1/4/6/12/16/24），与根包 `DayType` 完全一致；
- 消息：`Day{date,type,type_label}`、`Stats{workday,holiday,ordinary,compensate,weekend,festival,adjusted}`（粗粒度字段恒填充，细粒度字段仅 `detailed=true` 时填充，组合日交叉计数语义与 HTTP 一致）、`GetDayRequest{date,detailed}`、`GetDayResponse{date,type,type_label,total_days,stats}`、`QueryDaysRequest{start,end,dates[],detailed}`、`QueryDaysResponse{mode,start,end,total_days,days[],stats}`、`QueryStatsRequest{start,end,dates[],detailed}`（字段与 QueryDaysRequest 同构，独立消息以符合 buf lint 默认规则）、`QueryStatsResponse{mode,start,end,total_days,stats}`；
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
| `FuzzQueryConsistency` | 根包 `package goliday_test`（黑盒） | 任意构造的 `time.Time`：已加载年份 `Query` 结果 ∈ 合法细粒度组合全集 {1,4,6,12,16,24} 且 `QueryCoarse == Query().Coarse()`、`IsWorkday/IsHoliday` 与之互斥一致、同一日不同时刻（+5h/+23h）与 UTC/+08:00 表示结果不变；未加载年份断言返回 `ErrYearNotLoaded` |
| `FuzzLoadYearTOML` | 根包 `package goliday_test`（黑盒） | 任意年份 + TOML 文本：`LoadYear` 成功 ⟹ `Validate()` 幂等通过、off 全为周一~五、work 全为周六/日、两集合互斥无重复、全部日期在 `year` 年内；经 `LoadDir` 构造的 `Calendar` 对 off 日含 `Adjusted` 位、work 日为 `Compensate\|Weekend` |
| `FuzzDaysHandler` | `cmd/goliday-server` `package main`（白盒） | 任意查询串打到 `/api/v1/days` 与 `/api/v1/stats`：不 panic、状态码仅 200/400、响应恒为合法 JSON；200 且含 `days` 时升序唯一、`total_days == len(days)`；粗粒度 stats 之和 == `total_days`，细粒度（交叉计数）之和 ≥ `total_days`；单日模式 `total_days == 1`；stats 路径不因跨度报错（未加载年份报 `year_not_loaded` 除外） |
| `FuzzGenDraft` | `cmd/goliday-tool` `package main`（白盒） | 任意年份 + 公告文本：解析条目区间有效且在年内；草稿 off 全为周一~五、work 全为周六/日、互斥无重复、全在年内；festival 日期非 TODO 则为合法 `YYYY-MM-DD`；`selfCheck` 失败仅允许 TODO 占位或"节日当天不得补班"；自检通过且文件名年份合法时 `render` 产物可被 `LoadYear` 加载 |

补充：DayType 为 uint8 小域，其映射不变量 SHALL 以**穷举测试**（黑盒遍历全部 256 个取值：`Coarse` 结果 ∈ {Workday, Holiday} 且幂等、`IsWorkday`/`IsHoliday` 恰一为真、`String` 分段均为合法名）覆盖，不再另设 fuzz 目标。

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

- 每年数组 `prefix`，长度 = 该年天数 + 1，元素为 6 种合法细粒度组合（`1/4/6/12/16/24`）各自的累计天数（**按组合计数**，非标志位交叉计数）；
- `prefix[0]` 为全零；`prefix[i] = prefix[i-1] + 第 i 天（元旦起 1-based）类型的组合计数`，即 `prefix[i]` 表示 `[元旦, 元旦+i天)`（左闭右开）的累计；
- 构建成本 O(年天数)，仅在构造时发生一次。

**决策依据**：统计 O(覆盖年数) 差分即可完成，故 `/stats` 解除范围限制；days 明细接口保留 366 天上限防响应膨胀。

统计导出规则（由组合计数 `C(v)` 线性组合，语义与逐日统计完全等价）：
- 细粒度标志位交叉计数：`ordinary=C(1)`、`compensate=C(6)`、`weekend=C(4)+C(6)+C(12)`、`festival=C(12)+C(24)`、`adjusted=C(16)+C(24)`；
- 粗粒度：`workday=C(1)+C(6)`、`holiday=C(4)+C(12)+C(16)+C(24)`；
- `Total` = 覆盖天数（区间天数或列表长度）。

#### Scenario: 年内差分
- **WHEN** 统计 `[a, b)`（a、b 同年，b 可为次年元旦）
- **THEN** 结果 = `prefix[idx(b)] - prefix[idx(a)]`，`idx(d)` = d 距当年元旦的天数

#### Scenario: 跨年拆分
- **WHEN** 统计 `[s, e)` 跨多年
- **THEN** 拆为「首年 `[s, 次年元旦)` + 若干整年 + 末年 `[当年元旦, e)`」各段差分后相加，复杂度 O(覆盖年数)

#### Scenario: 与逐日统计等价
- **WHEN** 对任意已加载年份的任意区间/日期集合分别用前缀和与逐日 `Query` 暴力统计
- **THEN** 粗、细全部计数与 `Total` 完全一致

## 附录：核心包 API 形态

```go
package goliday // 根包

// ErrYearNotLoaded 查询覆盖了未加载配置的年份；errors.Is 判别，message 含年份。
var ErrYearNotLoaded = errors.New("年份配置未加载")

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

type Calendar struct{ /* 由 Store 构造：各年索引 + 组合计数前缀和 */ }
func NewCalendar(s *Store) *Calendar
func (c *Calendar) HasYear(year int) bool
func (c *Calendar) Query(date time.Time) (DayType, error)        // 细粒度；未加载年 → ErrYearNotLoaded
func (c *Calendar) QueryCoarse(date time.Time) (DayType, error)  // 粗粒度；未加载年 → ErrYearNotLoaded
func (c *Calendar) QueryRange(start, end time.Time) ([]Dated, error) // 左闭右开逐日；未加载年 → ErrYearNotLoaded
func (c *Calendar) StatsRange(start, end time.Time, detailed bool) (StatsResult, error) // 前缀和差分，不限跨度
func (c *Calendar) Stats(dates []time.Time, detailed bool) (StatsResult, error)        // 排序去重集合分段差分
```
