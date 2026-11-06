# Tasks

> 对应规格：`spec/spec.md`。spec 改动中即可规划的前瞻性任务以未勾选项写入本文件，执行并验收后改为勾选；仅保留未完成任务、最近已完成批次与长远有影响的批次（精简纪律见 [agents/general.md](../agents/general.md) 第 2 节）。

## 构建环境提升至 Go 1.27.3（本批次）

- [x] `go.mod` go 指令提升至 `go 1.27.3`（两个制品共用的最低工具链要求）
- [x] `Dockerfile` 构建阶段镜像提升至 `golang:1.27.3`
- [x] CI 测试矩阵最低项与 proto 作业锁定 `1.27.3`（stable 项继续覆盖最新版）
- [x] spec.md / spec.detail.md / docs/ARCHITECTURE.md / README 双语 / AGENTS.md 版本口径同步
- [x] 验证命令全绿（`make check`；ci.yml YAML 解析）；执行提交（build: 构建环境提升至 Go 1.27.3）

## 已完成批次（最近 3 个；更早批次见 git 历史与 `CHANGELOG.md`）

### spec 存档分拆（detail 与 legacy 分层）

- [x] 新建 `spec/spec.detail.md`：承接 spec.md 精简迁出且**当前仍有效**的次要细节（容器/CI/release-please 完整步骤约定与完整决策依据、次要 Scenario、核心包 API 形态清单）；spec.md 中 7 处存档指针按去向更新（3 处历史论证指 legacy、3 处有效细节指 detail、1 处总述双指）
- [x] `spec/spec.legacy.md` 收敛为**仅历史记录**：保留各 Requirement 历史决策依据完整原文（被取代方案的论证过程），移出仍生效内容
- [x] `agents/general.md` 第 2 节精简纪律同步三层存档口径（detail=有效次要细节 / legacy=过时历史 / 每轮仍需引用的内容即使超长也留 spec.md）
- [x] 已完成批次按「最近 3 个」收敛（补入 spec 纪律 v2，移除 workflow 触发分层与 AGENTS.md 分拆两节）
- [x] 验证命令全绿（纯 Markdown 变更，无 Go 代码改动；指针与本地路径复核通过）；执行提交（docs: spec 存档分拆为 detail 与 legacy）

### spec 纪律 v2（三件套重分工与首次精简）
- [x] `tasks.md` 合并原任务/验收双清单用途（前瞻任务未勾选、验收后勾选）；`checklist.md` 改为固定验收动作清单（有序列表、动态调整）
- [x] `spec.md` 首次精简 619→397 行（≤400 达标）：GitHub 环境类 Requirement 收敛为要点、相关 Scenario 合并、历史细节迁出存档
- [x] `agents/general.md` 新增第 2 节精简纪律（原 2~6 节顺延为 3~7 节）；AGENTS.md 与 README 双语同步；验证全绿；执行提交（docs: 重定义 spec 三件套分工并建立精简纪律）

### spec 纪律通用化（spec/ 条款移入 general.md）
- [x] 原四条逐条落入 general.md 新第 1 节（措辞泛化为「项目以 spec/ 目录维护规格文档时」），语义无损
- [x] 引用与编号同步（AGENTS.md 章节顺延、任务循环引用更新、全仓 grep 复核无遗留旧章节号）
- [x] 验证命令全绿（纯 Markdown 变更，无 Go 代码改动）；执行提交（docs: spec 纪律通用化移入 general.md）
