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
