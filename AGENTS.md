# AGENTS.md — goliday 项目协作规范

本仓库所有代理/自动化任务循环（Loop）必须遵守本规范。开始任何任务前先通读本文件与两个引用文件。

## 1. 通用协作规范（项目无关、语言无关）

见 [agents/general.md](agents/general.md)，必须遵守：规格文档纪律（spec/ 三件套口径与精简纪律、实现与 spec 冲突时先改 spec）、Git 提交纪律（中文 Conventional Commits、一变更一提交）、环境中立与路径规范（禁本地绝对路径、`.env.*` 不入库）、Shell 环境选择（Windows 兼容）、文档与语言（中文）、清单勾选纪律（先勾选后提交）。

## 2. Go 工程规范（语言相关、项目无关）

见 [agents/go.md](agents/go.md)，必须遵守：提交前门禁四件套、go fix 不动点、golangci-lint 零告警、测试覆盖率自查、现代 Go 风格基线。

## 3. 技术约束（遵循 spec）

- Go 1.27；模块名 `github.com/JayceChant/goliday`；核心逻辑位于根包 `goliday`，服务 `cmd/goliday-server`，工具 `cmd/goliday-tool`。
- 依赖按包分级：根包仅 `github.com/BurntSushi/toml`（不得导入 gRPC/protobuf）；gRPC 三件套仅限 `proto/goliday/v1/` 生成代码包与 `cmd/` 子包；HTTP 服务仅标准库。新增依赖前必须先在 spec 中论证并获准。
- 测试覆盖率门禁：手写代码（排除 `proto/` 生成代码）整体覆盖率不得低于 95%（门禁与允许的例外口径见 `spec/spec.md`「测试分层」的覆盖率维持 Scenario）。

## 4. 每次任务循环（Loop）流程

1. 读本文件与 `spec/`（含任务清单），确认任务范围与验收标准。
2. 按 [agents/general.md](agents/general.md) 第 5 节选择 shell 环境，按第 4 节读取/创建 `.env.<os/platform>`。
3. 实现遵循 spec；最小改动，不做 spec 之外的发挥。
4. **有代码改动时**：先跑 `go fix ./...`（重复直到无改动，典型改写回补到 [agents/go.md](agents/go.md) 第 4 节基线）与 `golangci-lint run ./...`，再运行提交前门禁；按 `spec/checklist.md` 完成固定验收动作，全绿后勾选 `spec/tasks.md` 对应项。
5. 按提交纪律规范提交 commit（一个被接受的变更对应一次提交）。

勾选与提交的次序约束、循环结束检查见 [agents/general.md](agents/general.md) 第 7 节。
