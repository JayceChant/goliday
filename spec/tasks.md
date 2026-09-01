# Tasks

> 对应规格：`spec/spec.md`。新任务按批次追加于本文件；完成勾选须先于提交（见 [AGENTS.md](../AGENTS.md) 第 7 节）。

## 已完成批次总览

- [x] Task 1~8（首个完整版本）：module 骨架与目录、DayType 位掩码、稀疏 TOML 配置模型/校验/加载、Calendar 判定与区间统计、测试数据（testdata）、HTTP 服务（仅标准库）、goliday-tool（gen/validate）、docs。
- [x] Task 9~12（gRPC TODO 落地）：`proto/goliday/v1/goliday.proto` 与生成代码入库、同进程 gRPC 服务（`-grpc-addr`，语义与 HTTP 一致）、文档同步、依赖分级审计（根包零 gRPC 导入）。
- [x] Task 13（测试强化）：根包测试黑盒化（`package goliday_test`，仅用导出 API）、DayType 全 256 取值穷举、已配置年份全年逐日穷举、5 个原生 fuzz 目标（种子内联，无语料残留）。
- [x] Task 14~19（年份强校验 + 统计前缀和化）：`ErrYearNotLoaded` / `year_not_loaded`（四种查询形态，不再静默回退周休）、构造期组合计数前缀和（统计 O(覆盖年数)）、stats 取消 366 天跨度限制（days 保留上限防响应膨胀）、测试与文档同步。
- [x] 文档精简：README 改为用户视角并修正过时的「无配置年回退」表述；spec.md 合并 ADDED/MODIFIED 为单一当前契约（保留决策依据）；tasks/checklist 压缩为批次总览；AGENTS.md 轻度精简并同步依赖分级表述；docs 修正过时条目与重复示例。
- [x] README 双语化：原中文版移至 `README-CN.md`，`README.md` 改为英文版，两文件开头提供语言切换互链；纯文档变更，无代码改动。
- [x] Task 20（模块路径迁移）：module 由 `goliday` 改为 `github.com/JayceChant/goliday`；同步全部 import 路径、proto `go_package` 与重新生成 pb 代码、spec/AGENTS/docs 引用；验证命令全绿。
- [x] Task 21（proto 工具链迁移至 buf）：新增 `buf.yaml`（模块/lint/breaking，豁免项附理由）与 `buf.gen.yaml`（本地 protoc-gen-go/go-grpc，版本固定）；再生成命令改为仓库根 `buf generate`，`buf lint` 通过；spec/API.md/ARCHITECTURE.md/proto 头注释同步；产物与 protoc 仅差生成头 protoc 版本行（`(unknown)`，预期）。
- [x] Task 22（buf lint 豁免收敛）：`buf.yaml` 改为 v2 工作区（模块根 `proto/`，buf 官方标准布局，目录与 package `goliday.v1` 对应），消除 PACKAGE_DIRECTORY_MATCH 豁免；`QueryStats` 拆分独立 `QueryStatsRequest`/`QueryStatsResponse`（字段与 QueryDaysRequest 同构，符合 buf BP「每 RPC 独立消息」），消除 RPC_* 三条豁免；`buf lint` STANDARD 零豁免通过；服务端代码与测试、spec/API 文档同步。
- [x] 依赖升级巡检：`go list -m -u all` 确认直接依赖（toml v1.6.0 / grpc v1.83.2 / protobuf v1.36.12）均为最新稳定版；仅间接依赖 `google.golang.org/genproto/googleapis/rpc` 升至 `v0.0.0-20260831171406-18b4a7587f8a`；依赖分级不变，无新增依赖；验证命令全绿。
- [x] golangci-lint 引入：新增 `.golangci.yml`（v2 格式，standard 五件套为基线，按"高价值低噪音"原则补充 errorlint/gocritic/misspell/nolintlint/revive/unconvert/unparam；goimports 以本地前缀分组；proto 生成代码豁免）；按 lint 结果清零 9 处告警；AGENTS.md 验证命令纳入 `golangci-lint run`；验证命令全绿。
- [x] go fix 现代化巡检：重复执行 `go fix ./...` 至幂等（首轮改写 4 个测试文件共 5 处：range-over-int 循环、删除 Go 1.22 前的循环变量影子拷贝、`strings.Split` 改 `SplitSeq`），每轮验证命令全绿；AGENTS.md 第 5 节写入现代 Go 风格基线（go fix 幂等要求 + 三条改写规则），避免再写过时风格代码。
- [x] Task 23（容器镜像与发布，GitHub 环境）：`Dockerfile` 多阶段构建（`golang:1.27` CGO_ENABLED=0 静态编译 → `distroless/static-debian12:nonroot`，仅打包 goliday-server，configs 运行时挂载）与 `.dockerignore`；`.github/workflows/docker.yml`（push 默认分支/`v*` tag/PR/workflow_dispatch 触发，buildx + QEMU 多架构发布 GHCR，GITHUB_TOKEN 认证，metadata 标签策略）；spec 新增「容器镜像与发布」Requirement，README 双语 Docker 章节；验证命令全绿。
- [x] 在线质量门禁与 CI 测试矩阵：spec 新增对应 Requirement；新增 `.github/workflows/ci.yml`（Go stable/oldstable 双版本矩阵 build/vet/gofmt/`go test -race -coverprofile`，stable 项经 codecov-action 上报覆盖率，golangci-lint-action 零告警作业）与 `.github/workflows/scorecard.yml`（OpenSSF Scorecard 每周评分，publish_results 发布 scorecard.dev，SARIF 上传 code scanning）；README 双语接入 CI/Codecov/pkg.go.dev/Scorecard 四枚徽章与「质量与持续集成」章节；SonarCloud/Snyk/Socket 需外部注册绑定，不在仓库内预置（spec 决策依据已记录）。
- [x] MIT License：仓库根新增 `LICENSE`（MIT 标准文本，Copyright (c) 2026 Jayce Chant（陈思杰））；spec 新增「开源许可证」Requirement；README 双语标题下接入 License 徽章并在「质量与持续集成」表格新增许可证条目；纯文档/法务文件变更，无代码改动。
- [x] 测试覆盖率提升：根包 98.1%→99.6%（新增白盒 `calendar_internal_test.go`：`comboIndex` 非法组合、`NewCalendar` nil 配置兜底；黑盒补 `toYearConfig` festival/work 日期非法分支）；`cmd/goliday-server` 73.4%→77.2%（`newGRPCServer` 经测试 helper 直挂 0%→100%、recover/日志中间件兜底、start/end 成对性与非法日期参数分支、混合列表全落区间内）；`cmd/goliday-tool` 85.2%→96.1%（`writeOut` stdout、`runGen` 各结果路径全分支、`parseCNNum` 边界穷尽、`lunarMarks` 首次优先、重复条目/重复补班去重、TODO 排序、`selfCheck` 非法日期）。
- [x] 覆盖率统计排除 proto 生成代码：新增仓库根 `codecov.yml`（`ignore: proto/`，附排除理由注释）；`ci.yml` 上传前新增「Filter coverage profile」步骤剔除 coverage.out 中 `proto/` 记录并自校验过滤结果；spec.md「在线质量门禁」Requirement 的覆盖率上报约定与 Scenario 同步；过滤后全仓口径 89.6%（原 69% 因零覆盖生成代码计入分母被拉低）；YAML 均经解析校验通过。
- [x] README 精简「质量与持续集成」章节：面向维护者的 CI/质量服务表格迁移至 docs/ARCHITECTURE.md（新增第 7 节），README 双语仅保留标题下徽章；spec.md 对应 Requirement 同步修订；纯文档变更，无代码改动。
- [x] 镜像发布策略收敛：docker.yml 改为仅 push tag `v*` 推送 GHCR（login 同步收紧至 tag 事件），push 默认分支 / PR / workflow_dispatch 仅构建验证不推送，消除 master 滚动镜像冗余；spec.md「容器镜像与发布」约定与 Scenario、README 双语发布说明同步；纯 workflow YAML 与文档变更，无 Go 代码改动。
