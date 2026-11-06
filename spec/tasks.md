# Tasks

> 对应规格：`spec/spec.md`。spec 改动中即可规划的前瞻性任务以未勾选项写入本文件，执行并验收后改为勾选；仅保留未完成任务、最近已完成批次与长远有影响的批次（精简纪律见 [agents/general.md](../agents/general.md) 第 2 节）。

## ci 热修复：golangci-lint 升级至 v2.15.0（本批次）

- [x] 定位 lint 作业失败根因：v2.13 的 gcimporter 无法解码 Go 1.27.3 的 export data v5，typecheck 全量报错
- [x] `ci.yml` lint 作业版本由 `v2.13` 升至 `v2.15.0`，并注明与 Go 升级的联动约束
- [x] `.golangci.yml` 验证版本口径与 `spec.detail.md` lint 作业约定同步
- [x] 本地复现 v2.13 失败、验证 v2.15.0 零告警；workflow YAML 解析通过
- [x] 执行提交（ci: 升级 golangci-lint 至 v2.15.0 修复 lint 作业）

## 已完成批次（最近 3 个；更早批次见 git 历史与 `CHANGELOG.md`）

### govulncheck 热修复：golang.org/x/net 升级至 v0.62.0

- [x] 传递依赖 `golang.org/x/net` v0.58.0 → v0.62.0：清除 GO-2026-6603/6611/6612/6617 四项可触达漏洞，并一并覆盖 GO-2026-6860/6866 两项当前未触达漏洞
- [x] `go mod tidy` 后依赖分级口径不变（`golang.org/x/net` 仍为 `// indirect`，直接依赖仅 toml/grpc/protobuf）
- [x] 本地 govulncheck 复扫无可触达漏洞；`go fix` 不动点、lint 零告警、提交门禁与覆盖率 95% 全绿
- [x] 执行提交（fix: 修复 govulncheck 发现的可触达依赖漏洞）

### 构建环境提升至 Go 1.27.3

- [x] `go.mod` go 指令提升至 `go 1.27.3`（两个制品共用的最低工具链要求）
- [x] `Dockerfile` 构建阶段镜像提升至 `golang:1.27.3`
- [x] CI 测试矩阵最低项与 proto 作业锁定 `1.27.3`（stable 项继续覆盖最新版）
- [x] spec.md / spec.detail.md / docs/ARCHITECTURE.md / README 双语 / AGENTS.md 版本口径同步
- [x] 验证命令全绿（`make check`；ci.yml YAML 解析）；执行提交（build: 构建环境提升至 Go 1.27.3）

### spec 存档分拆（detail 与 legacy 分层）

- [x] 新建 `spec/spec.detail.md`：承接 spec.md 精简迁出且**当前仍有效**的次要细节（容器/CI/release-please 完整步骤约定与完整决策依据、次要 Scenario、核心包 API 形态清单）；spec.md 中 7 处存档指针按去向更新（3 处历史论证指 legacy、3 处有效细节指 detail、1 处总述双指）
- [x] `spec/spec.legacy.md` 收敛为**仅历史记录**：保留各 Requirement 历史决策依据完整原文（被取代方案的论证过程），移出仍生效内容
- [x] `agents/general.md` 第 2 节精简纪律同步三层存档口径（detail=有效次要细节 / legacy=过时历史 / 每轮仍需引用的内容即使超长也留 spec.md）
- [x] 已完成批次按「最近 3 个」收敛（补入 spec 纪律 v2，移除 workflow 触发分层与 AGENTS.md 分拆两节）
- [x] 验证命令全绿（纯 Markdown 变更，无 Go 代码改动；指针与本地路径复核通过）；执行提交（docs: spec 存档分拆为 detail 与 legacy）
