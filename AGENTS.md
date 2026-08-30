# AGENTS.md — goliday 项目协作规范

本文件是本仓库所有代理/自动化任务循环（Loop）必须遵守的全局规范。开始任何任务前先通读本文件。

## 1. 规格文档（spec）

- 规格文档位于项目根目录 `spec/`：
  - `spec/spec.md` — 需求规格（枚举、API 契约、配置格式、判断算法、约束）
  - `spec/tasks.md` — 任务清单（完成后勾选）
  - `spec/checklist.md` — 验收清单（逐项核对并勾选）
- 所有任务必须遵循 spec 的规定：实现以 `spec.md` 为准，任务范围以 `tasks.md` 为准，验收以 `checklist.md` 为准。
- 实现与 spec 冲突时，先修订 spec 并获得确认，再实现；不得绕过 spec 私自变更行为。
- 任务完成时同步勾选 `tasks.md` 与 `checklist.md` 对应项。

## 2. Git 提交规范

- 每次被接受的修改（用户验收通过的变更）必须立即做一次 git commit，不得积压多个变更到一次提交。
- Message 采用中文 Conventional Commits：

  ```
  <type>: <中文摘要>

  [可选中文正文：动机与影响]
  ```

- `type` 取值：`feat` / `fix` / `docs` / `refactor` / `test` / `build` / `chore` / `perf` / `ci`。
- 摘要使用中文、祈使语气、不超过 50 字；正文说明"为什么"而非罗列文件。
- 提交内容不得包含本地路径、密钥、临时文件（见第 3 节）。
- 未经用户明确要求，不得执行 push、reset、force 等破坏性或外发操作。

## 3. 环境中立与路径规范

- 任何提交内容（代码、文档、注释、配置、commit message）中不得出现本地绝对路径（如 `/mnt/d/...`、`D:\...`、`/home/...`、`C:\Users\...`）；只允许使用相对项目根目录的路径（如 `cmd/goliday-server/`、`docs/API.md`）。
- 本地环境差异通过项目根目录的 dotfile `.env.<os/platform>` 配置（如 `.env.wsl`、`.env.win`、`.env.linux`），此类文件一律被 git 忽略，禁止提交。
- 需要本地环境信息时，优先读取当前平台对应的 `.env.*` 文件，而不是每次重新探测；文件不存在时再探测并创建。
- 环境探测结果可写入新的 `.env.<os/platform>`，但同样不得入库。

## 4. Shell 环境选择（Windows 兼容）

- 为保证命令行与编码（UTF-8）兼容性，探测到 Windows 环境时，shell 按以下优先级选择：
  1. **WSL**（首选，git 等命令行工具优先在 WSL 中执行）
  2. Git Bash
  3. cmd
  4. PowerShell
- Linux/macOS 环境直接使用本地 shell。
- 所有命令与输出统一按 UTF-8 处理；避免使用依赖平台专有行为的命令。

## 5. 技术约束（遵循 spec）

- Go 1.27；模块名 `goliday`。
- 全项目唯一第三方依赖：`github.com/BurntSushi/toml`（核心包解析 TOML 使用）；HTTP 服务与工具仅标准库。新增依赖前必须先在 spec 中论证并获准。
- 核心逻辑位于根包 `goliday`；服务 `cmd/goliday-server`；工具 `cmd/goliday-tool`。
- 提交前必须全部通过（在所选 shell 环境中执行）：

  ```sh
  go build ./... && go vet ./... && go test -count=1 ./... && gofmt -l .
  ```

  （`gofmt -l .` 输出必须为空）

## 6. 文档与语言

- 文档、注释、commit message 与用户交流使用中文（与用户最新消息语言一致）。
- 文件编码统一 UTF-8（无 BOM），换行统一 LF（由 `.gitattributes` 保证）。

## 7. 每次任务循环（Loop）流程

1. 读本文件与 `spec/`（含任务清单），确认任务范围与验收标准。
2. 按第 4 节选择 shell 环境，按第 3 节读取/创建 `.env.<os/platform>`。
3. 实现遵循 spec；最小改动，不做 spec 之外的发挥。
4. 运行第 5 节验证命令，全绿后更新 `spec/tasks.md`、`spec/checklist.md` 勾选。
5. 按第 2 节规范提交 commit（一个被接受的变更对应一次提交）。
