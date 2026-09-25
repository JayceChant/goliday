# Go 工程规范（语言相关、项目无关）

Go 项目通用的工程规则，由项目根目录的 AGENTS.md 引用；Go 版本、模块布局、依赖分级、覆盖率阈值等项目特定约束以各项目的 AGENTS.md / spec 为准。

## 1. 提交前门禁

每次提交前，以下命令必须在项目选定的 shell 环境中全部通过：

```sh
go build ./... && go vet ./... && go test -count=1 ./... && gofmt -l .
```

（`gofmt -l .` 输出必须为空。）

## 2. go fix 现代化

- 每次有代码改动，提交前先执行 `go fix ./...`，且重复执行直到无任何改动（到达不动点）。
- 期间实际落地的典型改写，须及时归纳补充到第 4 节「现代 Go 风格基线」；新代码直接采用现代写法，避免过时风格。

## 3. 静态检查与测试

- 每次有代码改动，提交前执行 `golangci-lint run ./...`，须零告警（项目根目录如有 golangci 配置文件则按其执行；本机未安装时可通过 `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest` 安装，不要求入库）。
- 新增或修改的生产代码须配套测试，提交前经 `go test -count=1 -cover ./...` 自查（覆盖率统计排除生成代码）；门禁阈值与允许的例外口径由项目规定。
- 入口 `main()` 只保留进程级胶水，逻辑下沉到可测的 `run`/`dispatch` 并测试。

## 4. 现代 Go 风格基线

go fix 实际落地的改写规则（依 Go 版本；随 go fix 产出持续补充）：

- 整数计数循环用 range-over-int（Go 1.22+）：`for i := range 366`，不写 `for i := 0; i < 366; i++`。
- 循环变量每轮迭代独立作用域（Go 1.22+）：禁止 `tc := tc`、`name, keywords := name, keywords` 等影子拷贝（含 `t.Run` 闭包、goroutine 捕获场景）。
- 仅遍历、不保留结果时用零分配迭代器（Go 1.24+）：`for part := range strings.SplitSeq(s, "|")` 替代 `strings.Split`；后者仅在需要切片本身时使用。
