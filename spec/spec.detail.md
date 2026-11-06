# Detail（当前有效的次要规格细节存档）

> 本文件存放 `spec.md` 精简时迁出、**当前仍然有效**但重要程度较低的细节（步骤级约定、完整清单、次要 Scenario）：修改相关区域前可来此查阅完整口径，日常任务循环无须通读。与 `spec.md` 冲突时以 `spec.md` 为准；不设长度上限，随 spec.md 精简持续追加。
>
> 已过时的内容不进本文件——精简后转入 [spec.legacy.md](spec.legacy.md)（仅历史记录）。

## 容器镜像与发布（GitHub 环境）——完整约定

仓库根 SHALL 提供 `Makefile`（构建入口）、`Dockerfile`（多阶段构建，构建阶段调用 Makefile）与 `.dockerignore`，GitHub Actions SHALL 提供 `.github/workflows/docker.yml` 自动构建并发布镜像至 GHCR（`ghcr.io/<owner>/goliday`）。

Makefile 约定（二进制与镜像**共用同一构建逻辑**，编译命令唯一定义）：
- 目标：`build`（= `build-server` + `build-tool`，产物到 `bin/`，Windows 交叉编译自动加 `.exe` 后缀）、`download-deps`（`go mod download`，供 Dockerfile 依赖层）、`image`（`docker build --build-arg VERSION=<VERSION>`，缺省 tag `goliday:$(VERSION)`）、`check`（AGENTS.md 提交门禁四件套 build/vet/test/gofmt）、`lint`、`fix`、`clean`；
- 变量：`VERSION`（缺省 `dev`，经 `-ldflags "-s -w -X main.version=$(VERSION)"` 注入）、`OUT_DIR`（缺省 `bin`）；`GOOS`/`GOARCH` 经环境变量透传供交叉编译；
- 编译参数唯一定义于 `build-server`（`CGO_ENABLED=0` 静态、`-trimpath`、`-s -w`、版本注入），Dockerfile 与本地 `make build` 调用同一目标，产物出自同一命令；
- 防递归约束：`image` 目标注明 Dockerfile 只允许引用不触碰 docker 的目标（`download-deps`/`build-server`），不得引用 `make image`，否则构成 make image → docker build → 容器内 make image 的真实循环。

Dockerfile 约定：
- 构建阶段：`golang:1.27.3`（`AS build`，基于 Debian 须先 `apt-get install make`），仅复制 `Makefile`/`go.mod`/`go.sum` 后 `make download-deps`（层缓存友好），再复制源码，`make build-server VERSION=${VERSION} OUT_DIR=/out` 编译；`ARG VERSION=dev` 透传给 Makefile（本地/`go install` 构建为 `dev`）。
- 运行阶段：`gcr.io/distroless/static-debian12:nonroot`；仅复制 server 二进制至 `/goliday-server`；`USER nonroot`（镜像内已内置）；`EXPOSE 8080 50051`；`ENTRYPOINT ["/goliday-server"]`。
- 不打包 `configs/`：配置与镜像解耦，运行时经 volume 挂载后以 `-config-dir` 指向；distroless 无 shell，容器内一切命令参数走 exec 形式。
- 构建上下文最小化：`.dockerignore` 排除 `.git`、`.github`、`docs`、`spec`、`testdata`、`bin`、覆盖率产物、`*.md`、`.env*` 等非构建必需内容（`Makefile`、Go 源码与 `go.mod`/`go.sum` 保留）。

docker.yml 工作流约定：
- 触发：`push` 默认分支、`push` tag `v*`、`pull_request`、`workflow_dispatch`；
- 推送策略：仅 `push` tag `v*` 事件发布镜像至 GHCR；其余事件仅构建验证不推送（master 滚动镜像无消费场景，保留只会产生冗余版本记录；构建可行性由验证构建保障）；
- 权限最小化：`contents: read` + `packages: write`；
- 步骤：checkout（`persist-credentials: false`）→ buildx → QEMU（多架构）→ 版本号派生（tag 事件去 `v` 前缀为 `VERSION` 构建参数，其余事件不注入）→ GHCR 登录（`GITHUB_TOKEN`，仅 tag 事件执行）→ metadata 提取标签 → build（`linux/amd64` + `linux/arm64`，GHA 缓存，`provenance: mode=min` 与 `sbom: true` 生成供应链证明，attestation 附着镜像索引，ghcr.io 支持 OCI 1.1 展示；仅 tag 事件 push）；
- 标签策略（metadata-action）：语义化版本 `v1.2.3` → `1.2.3` / `1.2` / `1`、tag 事件附加 `latest`；分支名 / PR 编号标签仅作非推送事件的构建标识，不发布；
- 无自定义 secrets：GHCR 认证仅用内置 `GITHUB_TOKEN`。

#### Scenario: 本地构建镜像
- **WHEN** 在仓库根执行 `make image VERSION=0.2.0`（或 `docker build --build-arg VERSION=0.2.0 -t goliday .`）
- **THEN** 多阶段构建成功（镜像内二进制经 `make build-server` 产出，与本地同一构建逻辑），最终镜像基于 distroless 且以 nonroot 运行，`docker run goliday -v` 输出版本后退出

#### Scenario: 容器启动并挂载配置
- **WHEN** `docker run -p 8080:8080 -v $PWD/configs:/data:ro goliday -config-dir /data`
- **THEN** 服务监听 8080/50051，`GET /healthz` 返回已加载年份

## 在线质量门禁与 CI 测试矩阵（GitHub 环境）——完整约定

ci.yml 约定：
- 触发：`push` 所有分支（`branches: ["**"]`，匹配含斜杠的分支名并排除 tag——开发分支无需先合入或开 PR 即可执行门禁）、`pull_request`（不设目标分支过滤——叠加 PR 与 release-please Release PR 同受门禁）、`workflow_dispatch`；权限最小化 `contents: read`；
- 冗余抑制：顶层 `concurrency` 以 `${{ github.workflow }}-${{ github.head_ref || github.ref_name }}` 分组并 `cancel-in-progress: true`——同一分支的 push 与 pull_request 双事件运行共享并发组（PR 的 head_ref 与分支推送的 ref_name 同名），仅保留最新一次，不重复执行；
- 测试作业：`1.27.3`（go.mod 最低要求）与 `stable` 双版本矩阵（`actions/setup-go` 自带模块缓存；不用 `oldstable`——其版本低于 go.mod 要求且 runner 默认 `GOTOOLCHAIN=local` 不自动升级工具链，必然编译失败），步骤 checkout → setup-go → `go build ./...` → `go vet ./...` → `gofmt` 检查（输出非空即失败）→ `go test -count=1 -race -covermode=atomic -coverprofile` → fuzz 冒烟（仅 `stable` 项：对全部 5 个 fuzz 目标各 `-fuzztime 30s` 短时真实 fuzz，普通测试仅覆盖种子语料）；
- 覆盖率上报：仅 `stable` 矩阵项经 `codecov/codecov-action` 上传 `coverage.out`（secrets `CODECOV_TOKEN`；公共仓库可不配置 token，上传失败不阻塞流水线）；上传前过滤 profile 中 `proto/goliday/v1` 生成代码的记录，并经仓库根 `codecov.yml`（`ignore: proto/`）在 Codecov 端同步排除——生成代码不设测试目标，避免零覆盖记录拉低统计（与 `.golangci.yml` 对生成代码的豁免同一口径）；
- lint 作业：`golangci/golangci-lint-action` 运行 `golangci-lint`（v2，配置见 `.golangci.yml`）零告警；
- proto 作业：`go install tool` 经 go.mod `tool` 指令 + go.sum 锁定版本安装 buf（v1.72.0）与 protoc-gen-go（v1.36.12）/protoc-gen-go-grpc（v1.5.1，满足 SonarQube 依赖版本可预测性——lock 文件强制；三个工具分属三个 module，`go install` 须逐 module 三条命令）→ `buf lint`（STANDARD 零豁免）→ `buf breaking --against <远端 master,subdir=proto>`（FILE 级，仅 PR 事件——master push 对照自身无意义）→ 再生成一致性（`buf generate` 后 `git diff --exit-code -- proto/`，入库生成代码与 proto 内容不得漂移）。

scorecard.yml 约定（OpenSSF Scorecard，无需注册）：
- 触发：`push` 默认分支、每周 `schedule`、`branch_protection_rule`；顶层 `permissions: read-all`，作业内最小化（`id-token: write` 供发布 OIDC 认证、`security-events: write` 供 SARIF 上传）；
- `ossf/scorecard-action` 以 `publish_results: true` 发布评分至 scorecard.dev；SARIF 产物落盘 artifact 并经 `github/codeql-action/upload-sarif` 上传至 code scanning；checkout 置 `persist-credentials: false`。

codeql.yml 约定（CodeQL 静态安全分析，公共仓库免费、无需注册）：
- 触发：`push` 默认分支、`pull_request`、每周 `schedule`、`workflow_dispatch`；作业权限最小化（`security-events: write` + 只读）；
- 语言 `go`、`build-mode: autobuild`（新版 CodeQL 已移除 Go 的 none 模式），结果上传 code scanning（Security 标签页）。

govulncheck.yml 约定（Go 官方依赖漏洞扫描，无需注册）：
- 触发：与 ci.yml 同口径——`push` 所有分支、`pull_request`（不过滤目标分支，依赖升级引入的可触达漏洞合入 master 前即失败）、每周 `schedule`（兜底：漏洞库新披露对既有依赖的影响）、`workflow_dispatch`；`contents: read`；concurrency 去重同 ci.yml；
- `golang/govulncheck-action@v1` 以 text 输出扫描 `./...`，仅当存在可被实际调用路径触达的漏洞时作业失败（作为门禁）。

sonarcloud.yml 约定（SonarCloud 静态分析：代码异味/安全漏洞/重复率/覆盖率质量门禁）：
- 触发：`push` 默认分支、`pull_request`（默认分支，opened/synchronize/reopened）；权限最小化 `contents: read`；
- 步骤：checkout 拉取完整历史（`fetch-depth: 0`，Quality Gate 的新代码判定依赖提交历史）→ `go test -covermode=atomic -coverprofile=coverage.txt ./...` 生成覆盖率 → `SonarSource/sonarqube-scan-action` 扫描上报；
- 认证依赖仓库 secret `SONAR_TOKEN`（未配置时扫描步骤失败，SonarCloud 端绑定仓库生成 token 后即生效）；项目参数（projectKey/organization、源码与测试目录、覆盖率路径、排除规则）入库于根目录 `sonar-project.properties`；`proto/` 生成代码经 `sonar.coverage.exclusions` 从覆盖率统计排除，与 Codecov、`.golangci.yml` 对生成代码的豁免同一口径。

pkg.go.dev 文档为 Go 官方服务自动抓取（模块可解析、可编译即自动建页），仓库无需配置，README SHALL 提供徽章与结果链接。

README 双语 SHALL 在标题下接入 CI、Codecov、CodeQL、govulncheck、pkg.go.dev、OpenSSF Scorecard、SonarCloud 徽章；各服务的说明与结果页链接 SHALL 收录于 [docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md)（质量门禁与 CI 章节），README 不设「质量与持续集成」章节——徽章保持可发现性，详细内容仅面向维护者，避免对使用者构成干扰。

**决策依据（完整）**：Actions/Codecov/Scorecard/CodeQL/govulncheck/pkg.go.dev 均可直接在仓库内落地且无需注册；SonarCloud 徽章仅依赖其在 SonarCloud 端启用项目；Snyk/Socket 需 GitHub App 绑定，不在仓库内预置，避免空配置导致流水线常红。触发分层：门禁类（ci/govulncheck）随分支推送与 PR 全量执行——公共仓库 Actions 免费且经 concurrency 消除双事件重复，问题在合入 master 前暴露，不必进入 master 后才发现；分析与发布类（codeql/sonarcloud/scorecard/docker/release-please）维持默认分支限定——重型扫描与多架构构建不随开发分支推送重复执行（其 PR→master 触发已覆盖合入前检查），Scorecard 评估对象是仓库级实践、发布链路仅对 master/tag 有意义。

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

## 自动化版本发布（GitHub 环境）——完整约定

- 触发：`push` 默认分支、`workflow_dispatch`；
- `googleapis/release-please-action`（完整 commit SHA 固定，与全仓 Action 固定策略一致）以 `release-type: simple` 运行——Go 模块无内嵌版本文件，无需更新产物版本，仅维护 `CHANGELOG.md` 与 tag/Release；
- 版本基线取 `.release-please-manifest.json`：有 `feat`/`fix`/`BREAKING CHANGE` 累积时生成 Release PR（汇总提交并更新 `CHANGELOG.md`）；Release PR 合并后创建附注 tag `vX.Y.Z` 与 GitHub Release；
- 权限最小化：`contents: write`（提交 CHANGELOG、打 tag、建 Release）+ `pull-requests: write` + `issues: write`（Release PR 标签）+ `actions: write`（下游工作流 dispatch）；
- 镜像发布衔接：`GITHUB_TOKEN` 产生的 tag push 不触发其他工作流（GitHub 防递归约定），release-please 输出 `releases_created` 为真时以 `gh workflow run docker.yml --ref <tag_name>` 显式 dispatch（`workflow_dispatch` 是 GITHUB_TOKEN 可触发的例外事件），docker.yml 零改动复用既有 tag 发布口径；
- 无自定义 secrets：全部使用内置 `GITHUB_TOKEN`。

## 附录：核心包 API 形态

```go
package goliday // 根包

// ErrYearNotLoaded 查询覆盖了未加载配置的年份；errors.Is 判别，message 含年份。
var ErrYearNotLoaded = errors.New("年份配置未加载")

// ParseDate 严格解析 YYYY-MM-DD（拒绝格式错误与不存在的日期）；配置加载与服务层共用。
func ParseDate(s string) (time.Time, error)
// WeekdayCN 返回中文星期名（错误信息与工具输出共用）。
func WeekdayCN(t time.Time) string
// CoveredYears 返回 [start, end) 覆盖的年份（升序含中间整年；空区间 nil）；
// 服务层年份校验与库内同一实现。
func CoveredYears(start, end time.Time) []int

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

func LoadDir(dir string) (*Store, error)   // 加载 <year>.toml；构建后不可变，并发读安全无锁
func (s *Store) Has(year int) bool
func (s *Store) LoadedAt() time.Time       // 配置加载完成时刻（/healthz loaded_at 来源）

type Calendar struct{ /* 由 Store 构造：各年索引 + 组合计数前缀和 */ }
func NewCalendar(s *Store) *Calendar
func (c *Calendar) HasYear(year int) bool
func (c *Calendar) Query(date time.Time) (DayType, error)        // 细粒度；未加载年 → ErrYearNotLoaded
func (c *Calendar) QueryCoarse(date time.Time) (DayType, error)  // 粗粒度；未加载年 → ErrYearNotLoaded
func (c *Calendar) IsWork(date time.Time) (bool, error)          // 未加载年 → ErrYearNotLoaded
func (c *Calendar) IsRest(date time.Time) (bool, error)          // 未加载年 → ErrYearNotLoaded
func (c *Calendar) QueryRange(start, end time.Time) ([]Dated, error) // 左闭右开逐日；未加载年 → ErrYearNotLoaded
func (c *Calendar) StatsRange(start, end time.Time, detailed bool) (StatsResult, error) // 前缀和差分，不限跨度
func (c *Calendar) Stats(dates []time.Time, detailed bool) (StatsResult, error)        // 内部排序去重后分段差分
```
