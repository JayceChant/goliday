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
- [x] README 双语语义一致、无本地绝对路径；验证命令全绿（纯文档变更，无 Go 代码改动）；执行提交（docs: README 移除质量与 CI 章节并迁移至架构文档）
