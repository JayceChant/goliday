# AGENTS.md — goliday 项目协作规范

本仓库所有代理/自动化任务循环（Loop）必须遵守本规范。开始任何任务前先通读。

## 1. 规格文档（spec/）

- `spec/spec.md` 需求规格（枚举、API 契约、配置格式、判断算法、约束）；`spec/tasks.md` 任务清单（完成后勾选）；`spec/checklist.md` 验收清单（逐项核对并勾选）。
- 实现以 `spec.md` 为准，任务范围以 `tasks.md` 为准，验收以 `checklist.md` 为准。
- 实现与 spec 冲突时，先修订 spec 并获得确认，再实现；不得绕过 spec 私自变更行为。
- 任务完成时同步勾选 `tasks.md` 与 `checklist.md` 对应项。

## 2. Git 提交规范

- 每次被接受的修改（用户验收通过的变更）必须立即做一次 git commit，不得积压多个变更到一次提交。
- Message 采用中文 Conventional Commits：`<type>: <中文摘要>`（祈使语气，不超过 50 字），可选中文正文说明"为什么"而非罗列文件。`type` 取值：`feat` / `fix` / `docs` / `refactor` / `test` / `build` / `chore` / `perf` / `ci`。
- 提交内容不得包含本地路径、密钥、临时文件（见第 3 节）。
- 未经用户明确要求，不得执行 push、reset、force 等破坏性或外发操作。

## 3. 环境中立与路径规范

- 任何提交内容（代码、文档、注释、配置、commit message）中不得出现本地绝对路径（如 `/mnt/d/...`、`D:\...`、`/home/...`）；只允许使用相对项目根目录的路径。
- 本地环境差异通过项目根目录的 dotfile `.env.<os/platform>` 配置（如 `.env.wsl`），一律被 git 忽略、禁止提交。
- 需要本地环境信息时，优先读取当前平台对应的 `.env.*`；文件不存在时再探测并创建。

## 4. Shell 环境选择（Windows 兼容）

- 探测到 Windows 环境时，shell 按优先级选择：**WSL**（首选，git 等命令行工具优先在 WSL 中执行）> Git Bash > cmd > PowerShell；Linux/macOS 直接使用本地 shell。
- 所有命令与输出统一按 UTF-8 处理；避免使用依赖平台专有行为的命令。

## 5. 技术约束（遵循 spec）

- Go 1.27；模块名 `github.com/JayceChant/goliday`；核心逻辑位于根包 `goliday`，服务 `cmd/goliday-server`，工具 `cmd/goliday-tool`。
- 依赖按包分级：根包仅 `github.com/BurntSushi/toml`（不得导入 gRPC/protobuf）；gRPC 三件套仅限 `proto/goliday/v1/` 生成代码包与 `cmd/` 子包；HTTP 服务仅标准库。新增依赖前必须先在 spec 中论证并获准。
- 提交前必须全部通过（在所选 shell 环境中执行）：

  ```sh
  go build ./... && go vet ./... && go test -count=1 ./... && gofmt -l .
  ```

  （`gofmt -l .` 输出必须为空）
- 自动现代化：**每次有代码改动，提交前先执行 `go fix ./...`，且重复执行直到无任何改动**（到达不动点）；期间实际落地的典型改写，须及时归纳补充到下方"现代 Go 风格基线"。新代码直接采用下述现代写法，避免过时风格。
- 静态检查：**每次有代码改动，提交前执行** `golangci-lint run ./...`，须零告警（配置见 `.golangci.yml`；本机未安装时可通过 `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest` 安装，不要求入库）。
- 测试覆盖率：新增或修改的生产代码须配套测试，提交前经 `go test -count=1 -cover ./...` 自查：手写代码整体覆盖率（排除 `proto/` 生成代码）不得低于 95%（门禁与允许的例外口径见 `spec/spec.md`「测试分层」的覆盖率维持 Scenario）。入口 `main()` 只保留进程级胶水，逻辑下沉到可测的 `run`/`dispatch` 并测试。
- 现代 Go 风格基线（go fix 实际落地的改写规则，依 Go 版本；随 go fix 产出持续补充）：
  - 整数计数循环用 range-over-int（Go 1.22+）：`for i := range 366`，不写 `for i := 0; i < 366; i++`。
  - 循环变量每轮迭代独立作用域（Go 1.22+）：禁止 `tc := tc`、`name, keywords := name, keywords` 等影子拷贝（含 `t.Run` 闭包、goroutine 捕获场景）。
  - 仅遍历、不保留结果时用零分配迭代器（Go 1.24+）：`for part := range strings.SplitSeq(s, "|")` 替代 `strings.Split`；后者仅在需要切片本身时使用。

## 6. 文档与语言

- 文档、注释、commit message 与用户交流使用中文（与用户最新消息语言一致）。
- 文件编码统一 UTF-8（无 BOM），换行统一 LF（由 `.gitattributes` 保证）。

## 7. 每次任务循环（Loop）流程

1. 读本文件与 `spec/`（含任务清单），确认任务范围与验收标准。
2. 按第 4 节选择 shell 环境，按第 3 节读取/创建 `.env.<os/platform>`。
3. 实现遵循 spec；最小改动，不做 spec 之外的发挥。
4. **有代码改动时**：先跑 `go fix ./...`（重复直到无改动，典型改写回补到第 5 节基线）与 `golangci-lint run ./...`，再运行第 5 节验证命令；全绿后更新 `spec/tasks.md`、`spec/checklist.md` 勾选。
5. 按第 2 节规范提交 commit（一个被接受的变更对应一次提交）。

**勾选与提交的次序约束**：第 4 步的全部勾选（含「执行提交」这类自指项——其含义是"本次提交将完成该动作"）必须**先勾选、后提交**，并纳入同一次提交；严禁提交后再补勾选。循环结束时必须执行 `git status --short` 确认工作区干净。
