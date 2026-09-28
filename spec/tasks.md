# Tasks

> 对应规格：`spec/spec.md`。spec 改动中即可规划的前瞻性任务以未勾选项写入本文件，执行并验收后改为勾选；仅保留未完成任务、最近已完成批次与长远有影响的批次（精简纪律见 [agents/general.md](../agents/general.md) 第 2 节）。

## 历史批次（已完成；更早批次见 git 历史与 `CHANGELOG.md`）

### spec 纪律通用化（spec/ 条款移入 general.md）
- [x] 原四条逐条落入 general.md 新第 1 节（措辞泛化为「项目以 spec/ 目录维护规格文档时」），语义无损
- [x] 引用与编号同步（AGENTS.md 章节顺延、任务循环引用更新、全仓 grep 复核无遗留旧章节号）
- [x] 验证命令全绿（纯 Markdown 变更，无 Go 代码改动）；执行提交（docs: spec 纪律通用化移入 general.md）

### workflow 触发条件按门禁/分析/发布分层
- [x] 门禁类 ci.yml 与 govulncheck.yml 全分支触发（push `branches: ["**"]`、PR 不过滤目标分支）+ 顶层 concurrency 去重；分析/发布类（codeql/sonarcloud/scorecard/docker/release-please）维持默认分支限定
- [x] spec.md 触发分层约定、决策依据与新增「开发分支合入前门禁」Scenario 同步
- [x] 验证命令全绿；执行提交

### AGENTS.md 分拆（agents/ 通用规则抽取）
- [x] 项目/语言无关条款分拆至 `agents/general.md`，Go 语言通用条款分拆至 `agents/go.md`，AGENTS.md 仅留项目特有约束，无条款丢失
- [x] 交叉引用同步（章节号引用改文件引用）
- [x] 验证命令全绿；执行提交（docs: 分拆 AGENTS.md 为项目规范与 agents/ 通用规则）

## spec 纪律 v2（本批次）
- [x] `spec/tasks.md` 重写：合并原任务/验收双清单用途，spec 改动中即规划前瞻任务（未勾选），验收后勾选；仅保留最近 3 个已完成批次，其余删除
- [x] `spec/checklist.md` 重写为固定验收动作清单（有序列表、无勾选框、随项目状态动态调整）
- [x] `spec/spec.md` 首次精简 619→397 行（≤400 达标）：GitHub 环境类 Requirement 收敛为要点、相关 Scenario 合并、历史决策依据与完整约定迁出，新建 `spec/spec.legacy.md`（160 行）承接存档
- [x] `agents/general.md` 新增第 2 节「spec 目录精简纪律」（spec.md ≤400 行、tasks.md 保留策略、checklist.md 动态调整），原第 2~6 节顺延为第 3~7 节，第 1/7 节改新分工表述
- [x] `AGENTS.md`（第 1 节摘要、任务循环第 2/4 步、章节号引用）与 README 双语目录表 spec/ 描述同步
- [x] 验证命令全绿（纯 Markdown 变更，无 Go 代码改动；交叉引用与本地路径复核通过）；执行提交（docs: 重定义 spec 三件套分工并建立精简纪律）
