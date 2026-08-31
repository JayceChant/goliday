# Tasks

> 对应 spec：`implement-holiday-api`。所有任务完成后须 `go build ./...`、`go vet ./...`、`go test ./...` 全部通过。

- [x] Task 1: 初始化 Go module 与工程骨架
  - [x] 1.1 仓库根执行 `go mod init goliday`，go 指令 1.27，创建 `.gitignore`
  - [x] 1.2 创建目录骨架：`cmd/goliday-server/`、`cmd/goliday-tool/`、`configs/`、`docs/`、`testdata/invalid/`
  - [x] 1.3 验证：`go build ./...` 通过

- [x] Task 2: 实现 DayType 位掩码枚举（根包 `daytype.go`）
  - [x] 2.1 定义 `type DayType uint8` 与常量：`Ordinary=1`、`Compensate=2`、`Weekend=4`、`Festival=8`、`Adjusted=16`，粗粒度 `Workday=3`、`Holiday=28`
  - [x] 2.2 实现方法：`IsWorkday()`、`IsHoliday()`、`Coarse()`（补班优先规则）、`String()`（按位序 `|` 连接小写名；粗值 `workday`/`holiday`）
  - [x] 2.3 单元测试 `daytype_test.go` 覆盖 spec 全部枚举 Scenario（合法组合全集 6 种、Coarse 映射、String）
  - [x] 2.4 验证：`go test ./...` 通过

- [x] Task 3: 实现稀疏配置模型、校验与加载（根包 `config.go` + `store.go`）
  - [x] 3.1 定义 `YearConfig{Year, Name, Festivals, Adjust}`、`Festival{Name, Date}`、`Adjust{Off, Work []time.Time}`；TOML 中 `date`/`off`/`work` 为 `YYYY-MM-DD` 字符串，解析后转 `time.Time`
  - [x] 3.2 实现 `Validate()`：year 与文件名一致；off 仅周一~周五、work 仅周六/周日；两集合无重复且互斥；work 不含 festival.date；日期合法且在年内。违规错误需指明文件与原因
  - [x] 3.3 实现 `LoadDir(dir)`：遍历目录加载 `<year>.toml`（忽略非 toml 与子目录），`Has(year)`；引入 `github.com/BurntSushi/toml`（全项目唯一三方依赖）
  - [x] 3.4 测试 `config_test.go`/`store_test.go`：使用 `testdata/2025.toml`、`testdata/2026.toml`、`testdata/invalid/*.toml` 覆盖加载成功、忽略杂项、各类非法样例报错
  - [x] 3.5 验证：`go test ./...` 通过；`go list -m all` 仅含 BurntSushi/toml 一个三方依赖

- [x] Task 4: 实现 Calendar 核心判断（根包 `calendar.go`）
  - [x] 4.1 实现 `Query(date)` 细粒度：work 命中→`Compensate|Weekend`；off 命中→`Adjusted`；节日当天→附加 `Festival`；否则周休回退（Weekend/Ordinary）；无配置年整年回退
  - [x] 4.2 实现 `QueryCoarse`、`QueryRange(start, end)`（左闭右开，上限 366 天）、`Stats(dates, detailed)`（细粒度组合日交叉计数）
  - [x] 4.3 测试 `calendar_test.go` 覆盖 spec 全部 Scenario：工作日调休、节日当天 `Festival|Adjusted`、周末补班 `Compensate|Weekend`、节日恰逢周末 `Festival|Weekend`、未覆盖回退、无该年配置回退、区间边界
  - [x] 4.4 验证：`go test ./...` 通过

- [x] Task 5: 测试数据与示例配置（可与 Task 3/4 并行，开发期可先用内联数据）
  - [x] 5.1 `testdata/2025.toml`、`testdata/2026.toml`：完整年度稀疏表样例（festival 定义 + off/work），2026 需覆盖 spec 示例（春节 02-16~02-20 off、02-28 work）
  - [x] 5.2 `testdata/invalid/`：off 含周末、work 含工作日、两集合交集、year 与文件名不一致、非法日期（02-30）样例
  - [x] 5.3 `configs/` 提供同内容可用示例；`docs/holiday_config_example.toml` 完整注释版
  - [x] 5.4 验证：被 Task 3/4/5/7 测试实际引用

- [x] Task 6: 实现 HTTP 服务子包（`cmd/goliday-server/`，仅标准库）
  - [x] 6.1 `main.go`：flag 解析（`-addr` 默认 `:8080`、`-config-dir` 默认 `./configs`、`-v`）、LoadDir、Go 1.22 `ServeMux` 路由、优雅关闭
  - [x] 6.2 `handlers.go`：`GET /api/v1/days`（date 单日 / start+end 区间 / dates 列表 / 混合并集去重升序；`detailed` 参数）、`GET /api/v1/stats`（与 days 同一统计逻辑，无 days 明细）、`GET /healthz`（含已加载年份）；中间件（函数装饰器）：日志、Panic 恢复；统一错误 `{"error":{"code","message"}}`；日期 `2006-01-02`；区间 ≤366 校验
  - [x] 6.3 测试 `handlers_test.go`（httptest）覆盖全部 API Scenario：粗/细单日、区间左闭右开、离散去重排序、混合并集、交叉计数统计、stats/days 一致性、400 校验、404/405、healthz
  - [x] 6.4 验证：`go build ./...`、`go vet ./...`、`go test ./...` 通过

- [x] Task 7: 实现 goliday-tool 生成与校验工具（`cmd/goliday-tool/`）
  - [x] 7.1 `validate` 子命令：复用根包校验逻辑，校验指定文件，错误输出文件与原因，退出码 0/非 0
  - [x] 7.2 `gen` 子命令：`-year`、`-out`、`-file`（缺省 stdin），解析公告句式（“X月X日至X月X日放假共N天”、“X月X日（周X）上班”、节日名），生成稀疏 TOML 草稿（自动剔除假期内周末、补班写入 work、festival 按公历固定/农历推断，无法推断输出占位注释）
  - [x] 7.3 测试/验证：`goliday-tool validate testdata/2026.toml` 通过、invalid 样例非 0；`gen` 对 2026 公告样例生成文件可通过 validate
  - [x] 7.4 验证：`go build ./...`、`go vet ./...`、`go test ./...` 通过

- [x] Task 8: 编写文档（`docs/`）
  - [x] 8.1 `docs/CONFIG_FORMAT.md`：TOML vs YAML vs JSON 选型对比（结论 TOML）、稀疏表原则与字段语义、校验规则、判断算法
  - [x] 8.2 `docs/API.md`：全部端点请求/响应示例（粗/细、range/list/混合、错误格式）
  - [x] 8.3 `docs/ARCHITECTURE.md`：目录结构、根包/cmd 分层、依赖约束（唯一三方依赖 BurntSushi/toml，服务仅标准库）
  - [x] 8.4 `docs/generate_prompt.md`：公告 → 年度 TOML 的 LLM 提示词模板（含稀疏原则与生成规则、TOML 骨架占位）
  - [x] 8.5 验证：文档示例与实际实现一致（17 处偏差已按代码修正）

# Task Dependencies
- Task 1 最先；Task 2 依赖 Task 1
- Task 3 依赖 Task 1；Task 5 可与 Task 3 并行（Task 3 开发期可用内联数据，Task 5 完成后替换）
- Task 4 依赖 Task 2、Task 3
- Task 6 依赖 Task 4 与 Task 5（测试数据）
- Task 7 依赖 Task 3（复用校验）、Task 5（公告样例与 testdata）
- Task 8 依赖 Task 2~7 定型接口，可与 Task 7 部分并行
- 追加任务（gRPC TODO 落地）：Task 9 → Task 10 → Task 11；Task 12 依赖 Task 10

# 追加任务（gRPC TODO 落地）
- [x] Task 9: 编写 proto 定义并生成代码入库
  - [x] 9.1 `proto/goliday/v1/goliday.proto`（package `goliday.v1`、go_package、DayType uint32 位注释、GolidayService 三方法、消息契约见 spec）
  - [x] 9.2 工具链：protoc（用户级安装，不入库）+ protoc-gen-go/protoc-gen-go-grpc（`go install`）
  - [x] 9.3 生成 `proto/goliday/v1/{goliday.pb.go,goliday_grpc.pb.go}` 入库，`go build ./...` 通过
- [x] Task 10: 实现 gRPC 服务（`cmd/goliday-server/grpc.go`，同进程）
  - [x] 10.1 `-grpc-addr` flag（默认 `:50051`，空串禁用）、注册 GolidayService 与 grpc 标准健康检查、优雅关闭覆盖双协议
  - [x] 10.2 查询逻辑与 HTTP 复用/对齐（单日/区间/离散/混合、detailed、跨度校验），参数错误 → `codes.InvalidArgument`，message 文案与 HTTP 一致
  - [x] 10.3 `grpc_test.go`（bufconn）覆盖：单日粗/细、区间左闭右开、离散去重排序、混合并集、QueryStats 无明细、与 HTTP 结果一致性、InvalidArgument 错误
- [x] Task 11: 更新文档（`docs/`）
  - [x] 11.1 `docs/API.md` 新增 gRPC 章节（proto 路径、服务与方法、-grpc-addr、再生成命令）
  - [x] 11.2 `docs/ARCHITECTURE.md` 更新依赖约束（按包分级）与目录树（proto/）
- [ ] Task 12: 全量验证与提交
  - [x] 12.1 `go build ./... && go vet ./... && go test -count=1 ./... && gofmt -l .` 全绿；根包依赖审计（无 gRPC 导入）
  - [x] 12.2 冒烟：gRPC 端口监听/禁用行为、健康检查
  - [x] 12.3 按 AGENTS.md 规范 git commit（中文 Conventional Commits）
