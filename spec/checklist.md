# Checklist

> 历史批次验收记录；新批次验收项追加于本文件，完成勾选须先于提交（见 [AGENTS.md](../AGENTS.md) 第 7 节）。

## 首个完整版本（Task 1~8）

- [x] 验证命令全绿（`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`gofmt -l .` 为空）
- [x] DayType 枚举、稀疏配置校验与判定算法、查询/统计口径全部 Scenario 通过
- [x] HTTP 服务（days/stats/healthz、统一错误格式、中间件、404/405）全部 Scenario 通过
- [x] goliday-tool gen/validate 行为符合契约；docs 与实现一致

## gRPC 落地（Task 9~12）

- [x] proto 与生成代码入库（无需 protoc 可编译）；`-grpc-addr` 启停生效；优雅关闭覆盖双协议
- [x] gRPC 与 HTTP 语义一致、错误码映射一致；根包依赖审计通过（零 gRPC 导入）

## 测试强化（Task 13）

- [x] 白盒/黑盒分层落实（文件头标注视角；黑盒仅引用导出 API）
- [x] 256 值穷举、全年判型穷举通过；5 个 fuzz 目标种子全过、逐包冒烟通过、无语料残留

## 年份强校验 + 前缀和（Task 14~19）

- [x] 未加载年份在单日/区间（含跨年中间整年）/离散/混合下均报 `year_not_loaded`（HTTP 400 / gRPC InvalidArgument，message 同源）；空区间不触发
- [x] stats 不限跨度、days 保留 366 天上限；前缀和统计与逐日暴力统计完全一致（含跨年区间）
- [x] 全量验证全绿 + 中文 Conventional Commits 提交，提交后工作区干净

## 文档精简

- [x] README 保留用户必要信息（选型依据、算法/数据结构权衡）并修正与实现不一致的年份回退表述
- [x] spec/tasks/checklist/AGENTS/docs 精简后语义无损，无本地绝对路径；验证命令全绿（纯文档变更，无代码改动）
- [x] 按规范执行提交（docs: 精简 README 与规格文档并修正过时表述）

## README 双语化

- [x] `README-CN.md` 为原中文版内容（仅新增语言互链），`README.md` 为语义对应的英文版；开头互链可相互跳转
- [x] 验证命令全绿（纯文档变更，无代码改动）；执行提交（docs: README 改为英文版并新增中文版与语言互链）

## 模块路径迁移（Task 20）
- [x] `go.mod` module 为 `github.com/JayceChant/goliday`；全部 Go 文件 import 路径同步，`grep` 无残留旧路径
- [x] proto `go_package` 更新为 `github.com/JayceChant/goliday/proto/goliday/v1;golidayv1` 并按 spec 命令重新生成 pb 代码（protoc v29.3 / protoc-gen-go v1.36.5 / protoc-gen-go-grpc v1.5.1）
- [x] spec.md / AGENTS.md / docs（ARCHITECTURE、API）中的 module、go_package、import 示例与再生成命令全部同步
- [x] 验证命令全绿（`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`gofmt -l .` 为空）；执行提交（refactor: 模块路径迁移为 github.com/JayceChant/goliday）

## proto 工具链迁移至 buf（Task 21）
- [x] `buf.yaml`（模块根为仓库根，lint STANDARD + 4 项豁免并注明理由，breaking FILE）与 `buf.gen.yaml`（本地插件 protoc-gen-go v1.36.5 / protoc-gen-go-grpc v1.5.1）入库；`buf lint` 通过
- [x] 再生成命令统一为仓库根 `buf generate`（spec.md、docs/API.md 7.5、docs/ARCHITECTURE.md、proto 头注释同步）；生成代码入库位置与 `go_package` 不变，`buf generate` 幂等（二次生成无差异）
- [x] WSL 安装 buf v1.72.0（`go install`，本机 `~/go/bin`，不入库）；生成头注释 protoc 版本行变为 `(unknown)` 属预期并已在文档说明
- [x] 验证命令全绿（`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`gofmt -l .` 为空）；执行提交（build: proto 代码生成改用 buf 管理）

## buf lint 豁免收敛（Task 22）
- [x] `buf.yaml` 改为 v2 工作区（模块根 `proto/`），目录 `goliday/v1` 与 package `goliday.v1` 对应（buf 官方标准布局），`buf lint` 默认 STANDARD 规则零豁免通过；proto 包名与 Go import 路径不变
- [x] `QueryStats` 使用独立 `QueryStatsRequest`/`QueryStatsResponse`（字段与 QueryDaysRequest 同构，语义不变）；服务端 `grpc.go`/`grpc_test.go` 同步；spec.md 与 docs/API.md 7.2 消息清单更新；生成代码 `source:` 为模块相对路径 `goliday/v1/goliday.proto`
- [x] 验证命令全绿（`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`gofmt -l .` 为空）；执行提交（build: buf lint 收敛为零豁免并拆分 QueryStats 消息）

## 依赖升级巡检
- [x] `go list -m -u all` 巡检：直接依赖（toml/grpc/protobuf）已为最新稳定版；间接依赖 `genproto/googleapis/rpc` 升级一版，`go mod tidy` 后 go.mod/go.sum 无冗余变更
- [x] 依赖分级审计不变（根包仅 toml，gRPC 三件套限于 proto 生成包与 cmd/，HTTP 服务仅标准库），无新增依赖
- [x] 验证命令全绿（`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`gofmt -l .` 为空）；执行提交（chore: 升级间接依赖 genproto/googleapis/rpc 至最新版）

## golangci-lint 引入
- [x] `.golangci.yml`（golangci-lint v2）入库：standard 五件套（errcheck/govet/ineffassign/staticcheck/unused）为基线，补充 errorlint、gocritic、misspell、nolintlint（require-specific）、revive、unconvert、unparam 七项；goimports 以 `github.com/JayceChant/goliday` 本地前缀分组；proto 生成代码按 generated: lax 豁免
- [x] lint 告警清零（初检 9 处）：errcheck 5 处（测试与工具中 Close/Serve 显式 `_ =` 或闭包处理）、gocritic 2 处（main.go 信号 stop 改显式调用消除 exitAfterDefer；gen.go if-else 链改 switch）、revive 1 处（handleHealthz 未用参数改 `_`）、unparam 1 处（genFor 增补 2024 年真实公告用例使 year 参数多值生效）
- [x] AGENTS.md 第 5 节验证要求纳入 `golangci-lint run ./...` 零告警；验证命令全绿（`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`gofmt -l .` 为空、`golangci-lint run` 0 issues）；执行提交（build: 引入 golangci-lint 配置并按告警修复代码）

## go fix 现代化巡检
- [x] `go fix ./...` 重复执行至收敛：首轮改写 calendar_test.go / config_blackbox_test.go / daytype_test.go / store_test.go 共 5 处（`for i := 0; i < 366; i++` → `for i := range 366`；删除 3 处循环变量影子拷贝 `tc := tc` / `name, keywords := name, keywords`；`strings.Split` 遍历改 `strings.SplitSeq`）；第二轮起零修改（幂等）
- [x] 每轮改动均验证有效：`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`gofmt -l .` 为空、`golangci-lint run` 0 issues 全绿
- [x] 现代 Go 风格基线写入 AGENTS.md 第 5 节（go fix 幂等要求 + range-over-int / 禁影子拷贝 / SplitSeq 三条规则）；执行提交（refactor: 应用 go fix 现代化写法并沉淀风格基线）

## 容器镜像与发布（Task 23）
- [x] Dockerfile 为多阶段构建：build 阶段 `golang:1.27`（先 go.mod/go.sum 后源码分层，`CGO_ENABLED=0` 静态编译 `./cmd/goliday-server`）；运行阶段 `gcr.io/distroless/static-debian12:nonroot`，仅含 `/goliday-server`，`EXPOSE 8080 50051`，exec 形式 `ENTRYPOINT`；不打包 `configs/`
- [x] `.dockerignore` 排除 `.git`、`.github`、`docs`、`spec`、`testdata`、`*.md`、`.env*` 等非构建必需内容，构建上下文最小化
- [x] `.github/workflows/docker.yml`：push 默认分支/`v*` tag/PR/手动触发；权限 `contents: read` + `packages: write`；checkout → buildx → QEMU → GHCR 登录（GITHUB_TOKEN）→ metadata 标签（分支名、semver 三段、latest）→ build & push `linux/amd64`+`linux/arm64`（GHA 缓存）；PR 仅构建不推送
- [x] spec.md 新增「容器镜像与发布（GitHub 环境）」Requirement 与 Scenario；README.md / README-CN.md 新增 Docker 章节且语义一致
- [x] 验证命令全绿（`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`gofmt -l .` 为空、`golangci-lint run` 0 issues；纯新增容器/CI/文档文件，无 Go 代码改动）；本机无 docker，以相同编译参数（`CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w"`）交叉验证静态构建产物 `-v` 输出版本正常；执行提交（build: 新增容器镜像与 GHCR 发布工作流）

## 镜像发布策略收敛

- [x] `.github/workflows/docker.yml` 推送条件改为 `startsWith(github.ref, 'refs/tags/')`：仅 push tag `v*` 登录 GHCR 并发布（semver 三段 + latest）；push 默认分支、PR、workflow_dispatch 仅多架构构建验证，不登录不推送，消除 master 滚动镜像冗余（公共仓库 GHCR 存储免费，避免的是 package 页版本列表噪音）
- [x] spec.md「容器镜像与发布」工作流约定新增推送策略条款（含决策依据）、步骤与标签策略同步（分支名 / PR 编号标签仅作非推送事件构建标识）；「CI 构建与发布」Scenario 拆分 tag 与非 tag 双口径；README 双语 Docker 章节发布说明同步
- [x] YAML 解析校验通过；验证命令全绿（纯 workflow YAML 与文档变更，无 Go 代码改动）；执行提交（ci: 镜像仅在 tag 推送时发布，其余事件仅构建验证）

## 在线质量门禁与 CI 测试矩阵
- [x] spec.md 新增「在线质量门禁与 CI 测试矩阵（GitHub 环境）」Requirement（含 3 个 Scenario 与决策依据）；tasks.md 追加批次条目
- [x] `.github/workflows/ci.yml`：stable/oldstable 双版本矩阵（setup-go 缓存，persist-credentials: false）依次执行 build/vet/gofmt 检查/`go test -count=1 -race -covermode=atomic -coverprofile`；仅 stable 项经 codecov/codecov-action 上传 coverage.out（fail_ci_if_error: false，无 token 公共仓库亦可上传）；lint 作业 golangci/golangci-lint-action v2.13 与 `.golangci.yml`（version: "2"）匹配，零告警门禁
- [x] `.github/workflows/scorecard.yml`：push 默认分支/每周 cron/branch_protection_rule/workflow_dispatch 触发；顶层 `permissions: read-all`，作业内 id-token: write + security-events: write 最小化；ossf/scorecard-action publish_results: true 发布 scorecard.dev；SARIF 经 artifact 留存并由 github/codeql-action/upload-sarif 上传 code scanning
- [x] README 双语语义一致：标题下四枚徽章（Actions CI/Codecov/pkg.go.dev/OpenSSF Scorecard）+「质量与持续集成 / Quality & CI」章节表格链接各服务结果页与工作流文件；无本地绝对路径
- [x] YAML 语法经解析校验通过；验证命令全绿（`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`gofmt -l .` 为空、`golangci-lint run` 0 issues）；执行提交（ci: 新增 CI 测试矩阵与覆盖率、Scorecard 工作流并接入 README 徽章）
- [x] 修复 scorecard-action 版本解析失败（`@v2` 漂移主标签不存在）：改为固定 `@v2.4.4`（该仓库仅发布完整版本 tag）；YAML 重新校验通过；执行提交（fix: scorecard-action 固定到完整版本标签 v2.4.4）
- [x] 修复 CI 矩阵 oldstable（Go 1.26.x，低于 go.mod 要求 1.27 且 runner 默认 GOTOOLCHAIN=local 不自动升级）编译失败：矩阵改为 `1.27.x`（最低要求）+ `stable`，spec.md Requirement/Scenario 与 README 双语表述同步；YAML 重新校验通过；执行提交（fix: CI 矩阵改为 go.mod 最低版本与 stable）
- [x] 补充 CodeQL/govulncheck/SonarCloud 三项：新增 `.github/workflows/codeql.yml`（Go 静态安全分析，build-mode: none，告警入 code scanning）与 `.github/workflows/govulncheck.yml`（官方漏洞扫描门禁，text 输出可触达漏洞即失败）；spec.md Requirement 扩展两项约定 + 3 个 Scenario，SonarCloud 决策依据更新为徽章依赖 SonCloud 端启用（项目 key `JayceChant_goliday`）；README 双语徽章扩为 CI/Codecov/CodeQL/govulncheck/pkg.go.dev/Scorecard/SonarCloud（保留用户新增的 License 徽章与表格行）；YAML 校验通过；执行提交（ci: 新增 CodeQL 与 govulncheck 工作流并补齐 SonarCloud 徽章）
- [x] 修复 CodeQL init 失败（CodeQL 2.26+ 已移除 Go 的 none 构建模式）：`codeql.yml` 的 `build-mode: none` 改为 `autobuild`；spec.md codeql.yml 约定与 CodeQL Scenario 同步修订；YAML 重新校验通过；执行提交（fix: CodeQL 构建模式改为 autobuild 适配新版要求）
- [x] 修复 SonarCloud 徽章无法显示（`sonarcloud.io/images/project_badges/sonarcloud.svg` 实测 403 已失效）：README 双语共 4 处（标题下徽章行与质量表格行）改为官方现行质量门禁徽章 URL `sonarcloud.io/api/project_badges/quality_gate?project=JayceChant_goliday`（curl 实测 200 image/svg+xml，与 spec「徽章反映通过/失败」一致）；验证命令全绿（纯 README 变更，无 Go 代码改动）；执行提交（fix: 替换失效的 SonarCloud 徽章为质量门禁徽章）
- [x] 修复 SonarCloud 5 个 security issue（githubactions:S7637，外部 Action 须固定完整 commit SHA）：5 个 workflow 共 22 处 `uses` 全部由浮动 tag 改为完整 40 位 SHA（`git ls-remote` 取各 tag 当前指向的最新 patch 版本，运行行为不变）并附版本注释；YAML 校验通过；验证命令全绿（纯 workflow YAML 变更，无 Go 代码改动）；执行提交（fix: GitHub Actions 依赖固定为完整 commit SHA）

## SonarCloud CI 分析落地
- [x] 新增 `.github/workflows/sonarcloud.yml`：push 默认分支 / PR（master，opened/synchronize/reopened）触发；`permissions: contents: read`；checkout `fetch-depth: 0`（Quality Gate 新代码判定依赖提交历史）+ `persist-credentials: false`；setup-go（stable，cache: false）；`go test -covermode=atomic -coverprofile=coverage.txt ./...` 生成覆盖率；SonarSource/sonarqube-scan-action 固定完整 SHA（v8）扫描上报，`SONAR_TOKEN` 经 env 注入；无本地绝对路径
- [x] 新增根目录 `sonar-project.properties`：projectKey `JayceChant_goliday` / organization `jaycechant`；`sonar.sources=.` + `sonar.tests=.` + `sonar.test.inclusions=**/*_test.go`；`sonar.exclusions` 剔除测试文件与 `proto/**`，`sonar.coverage.exclusions=proto/**`（与 codecov.yml、.golangci.yml 生成代码豁免同口径）；`sonar.go.coverage.reportPaths=coverage.txt`
- [x] spec.md「在线质量门禁与 CI 测试矩阵」Requirement 同步：SHALL 清单纳入 sonarcloud.yml；「SonarCloud 侧不预置 workflow」条款改写为 sonarcloud.yml 约定（触发/步骤/认证/参数文件/排除口径）；「SonarCloud 质量门禁徽章」Scenario 改为 workflow 运行口径；决策依据更新（workflow 已入库，仅依赖 secret `SONAR_TOKEN`，SonarCloud 端需关闭 automatic analysis 避免重复分析）
- [x] docs/ARCHITECTURE.md 第 7 节质量门禁表格 SonarCloud 条目补充工作流链接与运行方式说明；spec/tasks.md 追加批次条目
- [x] YAML 解析校验通过；验证命令全绿（`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`gofmt -l .` 为空、`golangci-lint run` 0 issues；纯 workflow/文档变更，无 Go 代码改动）；执行提交（ci: 新增 SonarCloud 分析工作流与项目参数文件）

## MIT License
- [x] 仓库根 `LICENSE` 为 MIT 标准文本，版权行 `Copyright (c) 2026 Jayce Chant (陈思杰)`；文件 UTF-8 无 BOM、LF 换行，无本地绝对路径
- [x] spec.md 新增「开源许可证」Requirement（含决策依据与 Scenario）；tasks.md 追加批次条目
- [x] README 双语语义一致：标题下新增 MIT License 徽章（链接 `LICENSE`），「质量与持续集成 / Quality & CI」表格新增许可证条目
- [x] 验证命令全绿（`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`gofmt -l .` 为空、`golangci-lint run` 0 issues；纯新增 LICENSE/文档变更，无 Go 代码改动）；执行提交（docs: 新增 MIT 许可证并在 README 接入徽章）

## 测试覆盖率提升（含 fuzz 发现缺陷修复）
- [x] 覆盖率：根包 98.1%→99.6%、`cmd/goliday-server` 73.4%→77.2%（未覆盖项仅剩 `main()` 与「年份已校验后查询再报错」的不可达防御分支）、`cmd/goliday-tool` 85.2%→96.1%（仅剩 `main()` 与正则保证不可达的防御分支）；全仓 `-coverpkg` 口径 64.4%→83.4%
- [x] spec 分层文件表新增 `calendar_internal_test.go`（白盒）；gRPC 测试 helper 改经生产构造函数 `newGRPCServer` 挂载（0%→100%，与 main 注册一致）
- [x] fuzz 冒烟（`FuzzLoadYearTOML` 5s）发现真实缺陷：festival 当天为工作日且不在 off 时判型为 `Ordinary|Festival`（9，非法组合），`comboIndex` 返回 -1 致 `buildPrefix` 越界 panic；按 spec 第 7 节流程将语料转写为回归用例（festival 当天周一不在 off → Validate 拒绝）后删除语料，`testdata/fuzz/` 无残留
- [x] 修复：`Validate` 新增校验规则「festival.date 为周一~周五时须在 off 中」（spec 校验规则清单、CONFIG_FORMAT.md 规则 10 与注 1、generate_prompt.md 自检清单、holiday_config_example.toml 注释同步）；`configs/` 现有配置经验证工具确认仍通过
- [x] `FuzzGenDraft` 允许的 selfCheck 失败类别同步扩展（spec 不变量表与测试注释一致）；5 个 fuzz 目标冒烟复跑全部 PASS
- [x] 验证命令全绿（`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`gofmt -l .` 为空、`golangci-lint run` 0 issues）；执行提交（test: 提升测试覆盖率并修复 fuzz 发现的索引越界缺陷）

## 覆盖率统计排除 proto 生成代码
- [x] 新增仓库根 `codecov.yml`：`ignore: ["proto/"]`，注释说明排除理由（buf 生成桩代码不设测试目标，行为由 buf lint/再生成幂等/bufconn 集成测试间接保障，与 `.golangci.yml` 生成代码豁免同一口径）
- [x] `.github/workflows/ci.yml` 上传前新增过滤步骤：`grep -v` 剔除 coverage.out 中 `proto/` 行，并以 `grep -c` 自校验（残留即失败）；过滤仅 stable 矩阵项生效于上传物
- [x] spec.md「在线质量门禁与 CI 测试矩阵」Requirement 的覆盖率上报约定与「覆盖率上报」Scenario 同步双口径（CI 过滤 + Codecov ignore）
- [x] 本地验证：过滤后 profile 无 proto 记录，全仓口径 89.6%（Codecov 原 69% 系零覆盖生成代码计入分母所致）；ci.yml/codecov.yml 经 YAML 解析校验通过；验证命令全绿（build/vet/test/gofmt 空输出）；执行提交（ci: 覆盖率统计排除 proto 生成代码）

## README 移除质量与 CI 章节

- [x] README 双语删除「质量与持续集成 / Quality & CI」章节（标题下徽章保留），文档索引中 ARCHITECTURE.md 条目补充「质量门禁与 CI」描述
- [x] docs/ARCHITECTURE.md 新增第 7 节「质量门禁与 CI」：收录原 README 表格（工作流链接改为 `../` 相对路径），许可证条目按徽章语义移除
- [x] spec.md 同步修订：「在线质量门禁与 CI」与「开源许可证」Requirement 中 README 章节表述改为徽章 + 指向 ARCHITECTURE.md；tasks.md 追加批次条目
- [x] README 双语语义一致、无本地绝对路径；验证命令全绿（纯文档变更，无代码改动）；执行提交（docs: README 移除质量与 CI 章节并迁移至架构文档）

## DayType 终态双层编码重构

- [x] spec.md「DayType 位掩码枚举（终态双层编码）」Requirement（位表、五值全集、非法值清单、语义约定、决策依据）经用户确认后实现；判定算法、校验规则（工作日节日必须在 off）、统计口径（五键 MECE）、proto 注释约定、fuzz 不变量与穷举测试约定同步修订
- [x] 全部合法值 {1,2,6,10,17} 上 `t & Work` 与 `t & Rest` 恰一非零（位与判类无歧义）；`Coarse() = t & 3` 幂等；`String()` 输出 work/rest/rest|festival/rest|adjusted/work|compensate
- [x] 新增校验规则落地并有 invalid 样例回归（`testdata/invalid/festival_workday_no_off.toml`），2025/2026 真实配置仍通过校验
- [x] 细粒度统计五键之和 == `total_days`（HTTP/gRPC/根包三层一致，fuzz 不变量同步收紧）；前缀和与逐日暴力统计全年/跨年/离散完全一致
- [x] proto 注释与生成代码同步（`buf generate` 幂等、`buf lint` 通过）；README 双语、docs/API.md、CONFIG_FORMAT.md、ARCHITECTURE.md、holiday_config_example.toml、configs/testdata 头注释无旧编码残留
- [x] 验证命令全绿（`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`gofmt -l .` 为空、`golangci-lint run` 0 issues、`go fix ./...` 幂等）；执行提交（refactor: DayType 重构为终态双层位编码并统一语义）

## DayType 命名与判定方法 API 增强

- [x] 调整位常量更名 `DayTypeAdjustedRest`/`DayTypeAdjustedWork`（动宾结构，用户选定方案），`type_label` 分段名 adjusted/compensate 不变；组合常量 `DayTypeFestivalRest`/`DayTypeAdjustedRestDay`/`DayTypeAdjustedWorkDay` 与「= 基本位|调整位」等值断言
- [x] `IsWorkday/IsHoliday` → `IsWork/IsRest`（DayType 与 Calendar 两层同步）；非法值（0/3/5/9/18）上 `IsWork/IsRest/IsValid` 全 false 的回归用例（实现为 `IsValid() && t&3 == 位`，修复 3 双 true 与 5 误报 true 两类缺陷）；`IsFestivalRest/IsAdjustedRestDay/IsAdjustedWorkDay` 组合值精确判等用例；`IsValid` 穷举 256 值恰五值 true
- [x] spec.md 常量表（动宾命名与组合常量列）、方法清单与新增「非法值全拒」「组合判断」Scenario、附录 API 形态、fuzz 不变量同步；proto 注释（ADJUSTED_REST/ADJUSTED_WORK 与组合值名）再生成且 `buf lint` 通过
- [x] docs（API/CONFIG_FORMAT/ARCHITECTURE）、README 双语表格、configs/testdata 与 holiday_config_example 头注释的组合记法统一为常量名；无旧常量名残留（grep 验证）
- [x] 验证命令全绿（build/vet/test 3 包 ok、`gofmt -l .` 为空、`golangci-lint run` 0 issues、`go fix` 幂等、FuzzParseDate/FuzzQueryConsistency 各 2s 冒烟通过）；执行提交（refactor: DayType 调整位命名动宾化并补齐判定方法）

## 粗粒度掩码常量化

- [x] 未导出常量 `dayTypeCoarseMask = DayTypeWork|DayTypeRest = 3` 落地：注释与 spec 明示「仅供掩码、本身非法、不导出以免被当作类型值使用」；`Coarse`/`IsWork`/`IsRest` 三处投影与 `calendar_internal_test` 的 19 构造改用该常量（黑盒测试以 `Work|Rest` 位运算表达同值并断言其非合法取值）；测试断言其数值为 3 且 `IsValid()` 为 false
- [x] spec.md 常量表新增掩码行、投影条款改用命名常量；docs/API.md 与 CONFIG_FORMAT.md 投影表述同步；验证命令全绿（build/vet/test 3 包 ok、gofmt 空、lint 0 issues、go fix 幂等）；执行提交（refactor: 粗粒度掩码常量化并收敛为未导出）

## 组合判断掩码化

- [x] 未导出常量 `dayTypeAdjustMask = DayTypeFestival|DayTypeAdjustedRest|DayTypeAdjustedWork = 28` 落地；`IsFestivalRest/IsAdjustedRestDay/IsAdjustedWorkDay` 改为「`IsValid()` 前置 + `t&dayTypeAdjustMask` 投影判等对应调整位」，与 `IsWork/IsRest` 五方法同构；全 256 值行为不变（4 裸调整位、5 矛盾值、12/20 多调整位等非法值均 false）
- [x] 黑盒回归：`TestComboPredicates` 补非法值 4/5/9/12/18/20 组合判断全 false、`Festival|AdjustedRest|AdjustedWork = 28` 数值断言及其 `IsValid()` 为 false
- [x] spec.md 常量表加 `dayTypeAdjustMask` 行、方法清单（五 Is 方法同构表述）与「非法值全拒」（扩 12 与三个组合方法）「组合判断」Scenario、docs/API.md 判类表述同步；验证命令全绿（build/vet/test 3 包 ok、gofmt 空、lint 0 issues、go fix 幂等）
- [x] `Adjustment()` 调整位投影方法（与 `Coarse()` 对应）落地：黑盒 `TestAdjustment`（五合法值 ∈ {0, Festival, AdjustedRest, AdjustedWork}、非法值 12 原值返回）；spec 投影条款、方法清单、附录签名与「调整位投影」Scenario、常量表用途表述、docs（API/CONFIG_FORMAT）投影表述同步；验证命令全绿；执行提交（refactor: 组合判断改为调整位投影判等并新增 Adjustment 方法）

## 判类与命名语义统一

- [x] 五个判类方法内裸掩码表达式（`t&dayTypeCoarseMask`/`t&dayTypeAdjustMask`）改为 `t.Coarse()`/`t.Adjustment()` 方法调用；`dayTypeNames` 改按位常量显式索引（编译期跟随位序/常量名调整），`String()` 保持手写（用户确认：位操作特殊 + 不输出 DayType 前缀为需求，不引入 stringer）
- [x] `type_label` 分段名动宾化与常量名逐字对应：`adjusted`→`adjusted_rest`、`compensate`→`adjusted_work`；stats 五键同步（JSON 键、proto Stats 字段 `adjusted`/`compensate` → `adjusted_rest`/`adjusted_work`，`buf lint` 通过、再生成完成，wire 序号不变）；handlers/grpc 映射与 handlers_test/grpc_test 断言（label + 键名 + Get 调用）全量更新；`TestString`/穷举测试合法分段名集合同步
- [x] spec（分段名条款、String 条款、API 响应样例、stats 键、proto Stats 字段清单、统计导出规则）、docs（API/CONFIG_FORMAT 全部样例与注）、README 双语五键名同步；全仓 grep 无 `compensate`/旧键名残留（历史勾选记录除外）；验证命令全绿（build/vet/test 3 包 ok、gofmt 空、lint 0 issues、go fix 幂等）；执行提交（refactor: 判类复用投影方法并统一 label 与 stats 键名）

## 零值常量与 String 查表

- [x] `DayTypeUnknown = 0` 落地：零值/默认值语义（`var zero DayType` 即 Unknown）、非合法取值（`IsValid()` false）、判类五方法恒 false、`Coarse()/Adjustment()` 均 0、`String()` 输出 `unknown`；黑盒 `TestUnknown` 全维度断言
- [x] `String()` 查表化：初始化期预计算 `dayTypeStrings`（由 `dayTypeNames` 物化 0~31 全 5 位组合域，`joinNames` 为构建原语），域内值直接查表返回（合法值与零值无逐次运行时构建），域外值（≥32）运行时 `joinNames` 逐位构建；穷举测试继续兜底 256 值输出不变
- [x] spec 常量表加 `DayTypeUnknown` 行、非法值条款（0 标注默认零值）、String 条款（预计算表约定）、「字符串表示」Scenario 扩 Unknown；docs/API.md 与 CONFIG_FORMAT.md 常量表及非法值表述同步；验证命令全绿；执行提交（feat: 新增 DayTypeUnknown 零值并预计算 String 查表）

## String 契约收紧（非法值统一 invalid）

- [x] `String()` 收紧落地：标签表 `dayTypeStrings` 改为六个组合常量显式索引（Unknown/work/rest/FestivalRest/AdjustedRestDay/AdjustedWorkDay），合法值与零值查表直返；任何非法值统一返回 `"invalid"`；删除 `joinNames()` 与按位名表 `dayTypeNames`，根包 strings 导入移除（依赖面缩小）
- [x] 测试同步：穷举测试改为 256 值全收敛断言（六标签 ∪ invalid，含越界值 33）、删除分段名白名单逻辑；`TestString` 补非法值 3/5/12/33 统一 invalid 回归；`TestUnknown` 的 unknown 断言不变（0 → unknown 语义延续）
- [x] spec String 条款（静态标签表 + invalid 统一词及其理由）与「字符串表示」Scenario、docs/API.md 与 CONFIG_FORMAT.md 的 label 机制说明同步；验证命令全绿（build/vet/test 3 包 ok、gofmt 空、lint 0 issues、go fix 幂等）；执行提交（refactor: String 非法值统一返回 invalid 并改静态标签表）
- [x] DayType 影响面终检：旧名零残留（历史勾选记录除外）；补修 README 双语 type_label 表述、spec fuzz 不变量条款、docs/ARCHITECTURE.md 穷举描述、calendar_test.go Fine 键注释措辞共 5 处遗漏；验证命令全绿；执行提交（docs: DayType 影响面终检并补修机制描述遗漏）

## v0.1.0 发布

- [x] 附注 tag `v0.1.0` 已推送至 origin（触发 docker.yml 发布多架构镜像至 GHCR）；中英双语 release notes 已交付（用户自行在 GitHub 创建 Release）
- [x] README 双语同步补充：go install 安装方式（server/tool）、server 启动参数表（-addr/-grpc-addr/-config-dir/-v）、configs 内置 2025 官方方案与 2026 假设示例说明、明文 HTTP/gRPC 安全提示（不暴露公网）、「作为 Go 库使用」最小示例（LoadDir/NewCalendar/Query/IsWork/StatsRange）、gRPC proto 文件位置（非 Go 客户端生成入口）、镜像拉取示例改为 v0.1.0 版本 tag、Docker 章节补充配置自备与年度更新流程链接
- [x] 双语语义一致、无本地绝对路径；验证命令全绿（`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`gofmt -l .` 为空；纯文档变更，无 Go 代码改动）；执行提交（docs: README 补齐安装方式、库使用与部署提示等用户信息）

## 前缀和存储优化（uint8 + 闭区间下标）

- [x] 前缀和元素改 uint8 存储：年内单一类型天数有界（普通工作日至多 248 天）uint8 足以存下；跨年/多段累加不直接在 uint8 上进行，先经新增宽类型累加器 `comboTotals`（int）转换再相加，无溢出
- [x] 前缀数组长度改为与年天数一致（去掉无意义的全零 0 下标），`prefix[i]` 为闭区间 `[元旦, 元旦+i天]` 累计；左闭右开查询统一转换为闭区间下标差分（`cumulationAt` 处理下标 -1 归零），统计结果与改造前完全一致（既有前缀和 vs 暴力统计一致性测试全过）
- [x] spec.md「细粒度组合计数前缀和统计」Requirement（数组约定、年内差分 Scenario、决策依据）与 docs/ARCHITECTURE.md、docs/API.md 表述同步
- [x] `go fix ./...` 幂等无改动、`golangci-lint run ./...` 0 issues；验证命令全绿（`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`gofmt -l .` 为空）；执行提交（perf: 前缀和改 uint8 存储并收敛为闭区间下标）

## 查询索引与前缀和存储再优化（整数键 + 稀疏终态表 + 定长内联）

- [x] `normalizeDate` 改返回「(年份, 年内 0-based 天序)」整数键（壁钟语义不变，按 t 自身时区取年与 YearDay-1）；三张 time.Time 键哈希集合合并为单一 `map[int]DayType` 稀疏终态表，构建期按 off→work→festival 依序覆写（festival 最后，工作日节日同落 off 与 festival 时终态收敛 FestivalRest），与判定优先级一致；未命中回退周休判断
- [x] 前缀和改定长 `[366]comboCounts` 内联数组（前 days 项有效、平年尾部闲置不参与差分；每年省一次独立堆分配与切片头，访问少一次间接寻址）；区间终点恰为次年元旦（天序 0）时折叠为上一年末（天序 = 该年天数），不进入未加载的终点年（回归于跨年暴力一致性测试覆盖）
- [x] 判定/区间/离散统计行为与重构前完全一致（黑盒/白盒测试零改动全过，`FuzzQueryConsistency` 15s 冒烟通过）；spec.md 前缀和条款与决策依据、docs/ARCHITECTURE.md（NewCalendar 流程、Query 请求流、按值计数前缀和、日期键规范化）同步
- [x] 验证命令全绿（`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`gofmt -l .` 为空、`golangci-lint run` 0 issues、`go fix ./...` 幂等）；执行提交（perf: 查询索引与前缀和改整数键稀疏表与定长内联数组）

## yearDays 改公历闰年直判

- [x] `yearDays` 改为整数直判（被 4 整除且不被 100 整除，或被 400 整除 → 366，否则 365），消除两次 `time.Date` 构造与浮点除法；正确性与「元旦至次年元旦差值」恒等价（Go time 包为外推公历，无闰年规则以外的日期调整）
- [x] 等价性固化为白盒回归 `TestYearDaysMatchesTimeCalc`：1000~9999（四位年份文件名全集）逐点断言直判结果与 time 包计算一致，世纪年（1900/2100 平年、2000/2400 闰年）随之覆盖
- [x] 验证命令全绿（`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`gofmt -l .` 为空、`golangci-lint run` 0 issues、`go fix ./...` 幂等）；执行提交（refactor: yearDays 改为公历闰年直判并固化等价回归）

## release-please 自动化发布

- [x] `.github/workflows/release-please.yml` 入库：push master / workflow_dispatch 触发；`permissions` 最小化（contents: write、pull-requests: write、issues: write、actions: write）；`googleapis/release-please-action` 固定完整 SHA（v5.0.0，与全仓 Action 固定策略一致），`release-type: simple`（Go 模块无内嵌版本文件，仅维护 CHANGELOG.md 与 tag/Release）；`releases_created` 为真时 `gh workflow run docker.yml --ref <tag_name>` 衔接镜像发布（`GITHUB_TOKEN` 的 tag push 不级联触发其他工作流，workflow_dispatch 是例外可触发事件；docker.yml 零改动）；无自定义 secrets，无本地绝对路径
- [x] 根目录 `.release-please-manifest.json` 记录版本基线 `"." : "0.1.0"`（与已发布的 v0.1.0 tag 对齐，下版从 0.1.0 起算增量）
- [x] spec.md 新增「自动化版本发布（GitHub 环境）」Requirement：工作流约定 6 条（触发/策略/版本基线/权限/镜像衔接/无 secrets）+「Release PR 生成与合并」「发布后镜像自动推送」2 个 Scenario；tasks.md 追加批次条目
- [x] README 双语 Docker 章节同步 tag 自动化来源说明（语义一致）；docs/ARCHITECTURE.md 第 7 节质量门禁表格新增 release-please 条目（含工作流链接）
- [x] YAML 解析校验通过；验证命令全绿（`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`gofmt -l .` 为空、`golangci-lint run` 0 issues；纯 workflow/文档变更，无 Go 代码改动，go fix 无输入）；执行提交（ci: 新增 release-please 自动化版本发布工作流）

## 复查修复（代码审查问题批次）

- [x] 前缀和计数元素 `comboCounts` 改 uint16：`TestStatsRangeNoAdjustmentOverflow` 以 2024 空调整年（闰年、元旦周一）断言 ordinary=262、weekend=104、五键之和 == Total（uint8 回绕时 ordinary 得 6、之和 110 ≠ 366）；spec.md 前缀和 Requirement 与决策依据（上界估计漏算无调整年）、docs/ARCHITECTURE.md 两处表述同步；`go fix` 幂等、`golangci-lint run` 0 issues；验证命令全绿；执行提交（fix: 前缀和计数改 uint16 修复无调整年溢出回绕）
- [x] Stats 防御式归一：内部 `slices.Clone` + 排序 + 去重（入参不被修改），年份校验显式首见标记（年份 0 不再漏判，回归断言返回 ErrYearNotLoaded 而非 panic）；`TestStatsDefensiveNormalization` 覆盖乱序/重复输入一致性、入参不变性、年份 0；spec.md 核心包 Stats 条款与决策依据、附录注释同步；`go fix` 幂等、`golangci-lint run` 0 issues；验证命令全绿；执行提交（fix: Stats 改内部排序去重并修复年份哨兵漏判）
- [x] 版本号构建期注入：`main.go` `const version = "0.1.0"` 改 `var version = "dev"`（-X 注入点），Dockerfile `ARG VERSION=dev` + `-ldflags "-s -w -X main.version=${VERSION}"`，docker.yml 新增「Derive binary version」步骤（tag 事件去 v 前缀，非 tag 不注入保持 dev）并传 `build-args`；spec.md 容器 Requirement 构建阶段/工作流步骤、docs/API.md `-v` 描述同步；YAML 解析校验通过、`go fix` 幂等、`golangci-lint run` 0 issues；验证命令全绿；执行提交（fix: 服务版本号改为构建期注入消除发版漂移）
- [x] 500 兜底错误记录底层原因：新增 `internalQueryErr(op, err)`（log.Printf 操作名 + 错误链后返回统一 `errInternalQuery`），handlers.go 单日 Query/Stats、多日明细 Query、区间 StatsRange、列表 Stats 5 处与 grpc.go GetDay 2 处、QueryDays 1 处兜底分支改经该函数；对外响应文案与状态码不变；`go fix` 幂等、`golangci-lint run` 0 issues；验证命令全绿；执行提交（fix: 500 兜底错误记录底层原因便于排查）
- [x] Fine 键改终态组合值：`comboTotals.result` 的 Fine map 键改 `DayTypeFestivalRest`/`DayTypeAdjustedRestDay`/`DayTypeAdjustedWorkDay`（ordinary/weekend 两键不变），`String()` 打印 Fine 不再出现 invalid 键；handlers.go `fillStats`、grpc.go `statsProto`、黑盒 `fineKeyOf`（简化为恒等）/`wantSameStats` 键集合/`TestCalendarStats` 期望表同步；HTTP JSON 键与 proto 字段名对外不变（既有 handlers/grpc 测试零改动通过）；spec.md 统计导出规则补键约定；`go fix` 幂等、`golangci-lint run` 0 issues；验证命令全绿；执行提交（refactor: 统计细粒度键改用合法终态组合值）
- [x] Store 移除 RWMutex：`Store` 无任何写方法，删除 `mu` 字段与三处 RLock/RUnlock（CI `-race` 继续验证并发安全），结构注释改「构建后不可变」；`Years()` `sort.Ints` 改 `slices.Sort`；docs/ARCHITECTURE.md 并发模型（RWMutex→不可变无锁）与数据流 Store 行、spec.md 附录 LoadDir 注释同步；`go fix` 幂等、`golangci-lint run` 0 issues；验证命令全绿（含 -race）；执行提交（refactor: Store 移除冗余读写锁并声明不可变）
- [x] LoadDir 移除库内日志：删除 `log.Printf`（及 log/strconv 导入），加载信息由服务入口日志与 `/healthz` 承担（原启动重复两行）；spec.md「启动加载」Scenario 改为「核心包不写日志，由服务入口基于 Years() 输出」、docs/API.md 启动说明同步；`go fix` 幂等、`golangci-lint run` 0 issues；验证命令全绿；执行提交（refactor: LoadDir 移除库内日志输出）
- [x] HTTP 服务端超时：`http.Server` 增加 `ReadHeaderTimeout: 10s`（防慢速头部连接占坑）与 `IdleTimeout: 120s`（回收 keep-alive 空闲连接），以具名常量落库；不设整体 Read/WriteTimeout（纯内存查询无长请求，避免干扰正常客户端）；spec.md HTTP 服务子包 Requirement 超时条款、docs/API.md 启动说明同步；`go fix` 幂等、`golangci-lint run` 0 issues；验证命令全绿；执行提交（fix: HTTP 服务端补充请求头与空闲连接超时）
- [x] proto CI 作业：ci.yml 新增 `proto (buf)` 作业（checkout persist-credentials: false → setup-go → `go install` buf@v1.72.0 + protoc-gen-go@v1.36.5 + protoc-gen-go-grpc@v1.5.1 → `buf lint` → `buf breaking`（仅 PR，对照远端 master）→ `buf generate && git diff --exit-code -- proto/`）；三项检查本机实测通过（lint OK / breaking OK / 再生成无差异）；spec.md ci.yml 约定新增 proto 作业条款；YAML 解析校验通过；执行提交（ci: 新增 buf lint 破坏性变更与再生成一致性检查）
- [x] fuzz CI 冒烟：ci.yml 测试作业新增「Fuzz smoke test」步骤（仅 stable 项，`go test -run '^$' -fuzz '^<目标>$' -fuzztime 30s` 逐目标执行 FuzzParseDate/FuzzQueryConsistency/FuzzLoadYearTOML/FuzzDaysHandler/FuzzGenDraft 共 5×30s）；命令形态本机以 3s 实测可运行；spec.md 测试作业步骤与「fuzz 种子即回归」Scenario 同步；YAML 解析校验通过；执行提交（ci: 测试矩阵加入全部 fuzz 目标短时冒烟）
- [x] 基准测试：新增 `calendar_bench_test.go`（黑盒，`package goliday_test`；`sync.OnceValue` 复用加载 testdata 配置），5 个基准覆盖 Query/QueryCoarse（单日判定，0 alloc）、StatsRangeFullYear（前缀和差分）、StatsList（排序去重 + 逐日差分）、QueryRangeFullYear（逐日明细分配）；采用 `b.Loop()`（Go 1.24+）与 `ReportAllocs`，本机 `-benchtime 100x` 冒烟通过；spec.md 测试分层 Requirement 补性能基线条款；`go fix` 幂等、`golangci-lint run` 0 issues；验证命令全绿；执行提交（test: 新增日历核心基准测试）
- [x] docker.yml checkout 补 `persist-credentials: false`：全仓 8 处显式 actions/checkout 中唯 docker.yml 遗漏（release-please.yml/govulncheck.yml 无显式 checkout 步骤，其 action 内部自管凭证，无需设置）；spec.md 容器工作流步骤条款补注；YAML 解析校验通过；执行提交（ci: docker 工作流 checkout 不保留凭证）
- [x] 供应链证明恢复：docker.yml build-push-action 显式 `provenance: mode=min` + `sbom: true`（attestation 附着于多架构镜像索引，ghcr.io OCI 1.1 展示；distroless 基础镜像与静态二进制不变，`docker pull`/`docker run` 兼容不受影响）；spec.md 容器工作流 build 条款同步；YAML 解析校验通过；执行提交（ci: 镜像构建恢复 provenance 与 sbom 供应链证明）
- [x] 排序统一 slices：gen.go `sort.Slice`/`sort.SliceStable` 改 `slices.SortFunc`（`time.Time.Compare`）/`slices.SortStableFunc`（bool 比较改显式 -1/1，字符串用 `strings.Compare`），删除 sort 导入；全仓不再有 sort 包调用（store.go 已于前次提交迁移）；行为等价（off/work 键唯一，festival 稳定序不变，gen 测试全过）；`go fix` 幂等、`golangci-lint run` 0 issues；验证命令全绿；执行提交（refactor: 排序统一改用 slices 包）
- [x] 解析与星期名收敛：根包导出 `ParseDate`（严格 YYYY-MM-DD，config.go 原内部实现上移导出）与 `WeekdayCN`（中文星期名）；handlers.go `parseDate` 改为包装 `goliday.ParseDate` → invalid_date API 错误（文案与配置加载同源）；gen.go 删除本地 `weekdayName` 表改用 `goliday.WeekdayCN`；config_test.go 白盒改用导出名；spec.md 附录 API 形态补两函数、测试分层表措辞同步；对外 HTTP 错误文案不变（invalid_date 消息逐字一致，handlers 测试零改动通过）；`go fix` 幂等、`golangci-lint run` 0 issues；验证命令全绿；执行提交（refactor: 严格日期解析与中文星期名收敛为根包导出）
- [x] 覆盖年份收敛：根包新增导出 `CoveredYears(start, end)`（空/倒置区间返回 nil，其余复用既有 int 内核 `coveredYears`，元旦折叠语义不变）；handlers.go `multiQuery.coveredYears` 区间部分改调 `goliday.CoveredYears`（无区间时置 nil 再并入列表年份）；黑盒 `TestCoveredYears` 六场景（空/倒置/同年/跨年/终点元旦折叠/起点元旦）覆盖；spec.md 年份强校验覆盖规则标注统一经 `CoveredYears`、附录 API 形态补该函数；既有 handlers/grpc 测试零改动通过；`go fix` 幂等、`golangci-lint run` 0 issues；验证命令全绿；执行提交（refactor: 覆盖年份判定收敛为库导出函数）
- [x] healthz 增强：`Store` 新增 `loadedAt` 字段与 `LoadedAt()`（`LoadDir` 末尾 `time.Now()`，构建后不可变）；`/healthz` 响应增加 `version`（构建期注入变量）与 `loaded_at`（RFC3339 UTC）；`TestHealthzAndRouting` 断言 version == 构建变量与 loaded_at 非零可解析；docs/API.md 第 4 节示例与路由表、README 双语 Docker 示例与 API 总览表、spec.md HTTP 服务子包健康检查条款/Scenario/附录 `LoadedAt` 同步；`go fix` 幂等、`golangci-lint run` 0 issues；验证命令全绿；执行提交（feat: healthz 增加版本与配置加载时间字段）

## 构建入口统一（Makefile）

- [x] `Makefile` 入库：目标 `all/build/build-server/build-tool/download-deps/image/check/lint/fix/clean` 全部 `.PHONY`（产物经 go 构建缓存兜底，改版本号不误用旧产物）；`VERSION ?= dev` 经 `-ldflags "-s -w -X main.version=$(VERSION)"` 注入；`GOOS`/`GOARCH` 环境变量透传，Windows 目标自动加 `.exe`（`file` 验证 PE32+ 产物）；`make check` 为 AGENTS 四件套（gofmt 非空即失败并列出文件）
- [x] Dockerfile 引用 Makefile：构建阶段 `apt-get install make`（golang 镜像不含 make）→ `COPY Makefile go.mod go.sum` → `make download-deps` → `COPY . .` → `make build-server VERSION=${VERSION} OUT_DIR=/out`；docker.yml 的 `--build-arg VERSION` 流向不变（ARG → make 变量）；本机无 docker，镜像路径未实机验证（同命令经本地 make build 验证），CI 验证构建将复核
- [x] `bin/` 入 `.gitignore`；`.dockerignore` 补排除 `bin`、`coverage.out`、`coverage.txt`（Makefile 保留在构建上下文中）；本机实测：`make build VERSION=test-0.2.0`（server `-v` 输出 test-0.2.0）、`make check` 全绿、`make clean` 清理
- [x] spec.md 容器与发布 Requirement 增 Makefile 约定（目标清单/变量/编译命令唯一定义）、Dockerfile 构建阶段改写、`.dockerignore` 条款与「本地构建二进制」「本地构建镜像」Scenario 更新；README 双语开发章节（make check/build/image）与 Docker 章节构建注释、docs/ARCHITECTURE.md 目录树与「构建入口统一」说明同步；执行提交（build: 新增 Makefile 统一二进制与镜像构建入口）
- [x] 防递归约束注释：Makefile `image` 目标注明「Dockerfile 只允许引用不触碰 docker 的目标，不得引用本目标，否则 make image → docker build → 容器内 make image 构成真实循环」，Dockerfile 构建阶段注释对称注明「只调用 download-deps/build-server，不得调用 make image」——互引是分层委托（执行路径 DAG），本约束是其成立前提；纯注释变更，无代码改动；验证命令全绿；执行提交（docs: Makefile 与 Dockerfile 互注防构建递归约束）
- [x] 修复 ci.yml proto 作业安装失败：`go install` 的 pkg@version 形式要求实参同一 module 同一版本，buf/protoc-gen-go/protoc-gen-go-grpc 分属三个 module，单条命令必失败（go 1.27 报 "all arguments must refer to packages in the same module"）；拆为逐 module 三条安装命令并注明原因，三条命令本机实测安装成功（buf 1.72.0 / protoc-gen-go v1.36.5 / protoc-gen-go-grpc v1.5.1）；YAML 解析校验通过；验证命令全绿（纯 workflow 变更，无 Go 代码改动）；执行提交（fix: 拆分 go install 逐模块安装 buf 与 protoc 插件）
