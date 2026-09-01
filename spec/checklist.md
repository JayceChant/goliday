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
