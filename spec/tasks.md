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
