# Checklist

> 每次任务循环固定执行的验收动作（有序列表、无勾选框）。随项目状态动态调整：过时的项删除，新增有用的项加入（精简纪律见 [agents/general.md](../agents/general.md) 第 2 节）。

1. 改动范围与 `spec/spec.md` 一致；实现与 spec 冲突时先修订 spec 并获得确认，不得绕过 spec 私自变更行为。
2. 有代码改动时：`go fix ./...` 重复执行至无改动（新出现的典型改写回补 [agents/go.md](../agents/go.md) 第 4 节基线）。
3. 有代码改动时：`golangci-lint run ./...` 零告警。
4. 提交前门禁全绿：`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`gofmt -l .` 输出为空（等价 `make check`）。
5. 手写代码整体覆盖率不低于 95%（排除 `proto/` 生成代码口径；例外清单见 spec.md「测试分层」的覆盖率维持 Scenario）。
6. 触及 proto 时：`buf lint` 通过、`buf generate` 后 `git diff --exit-code -- proto/` 无漂移。
7. 触及 workflow/YAML 等配置文件时：经解析校验。
8. 提交内容复核：无本地绝对路径、无密钥、无临时文件；对外行为或接口变更时 README 双语、docs/、spec Scenario 已同步。
9. `spec/tasks.md` 对应项先勾选、后提交，纳入同一次提交。
10. 提交前 `git status --short` 复核无计划外改动；一个被接受的变更对应一次中文 Conventional Commits 提交。
