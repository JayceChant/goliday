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
- 追加任务（测试强化）：Task 13 依赖 Task 2~12（存量测试与实现定型）
- 追加任务（年份强校验 + 前缀和统计）：Task 14 → Task 15；Task 14 → Task 16 → Task 17；Task 18 依赖 Task 14~17 接口定型；Task 19 最后

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
- [x] Task 12: 全量验证与提交
  - [x] 12.1 `go build ./... && go vet ./... && go test -count=1 ./... && gofmt -l .` 全绿；根包依赖审计（无 gRPC 导入）
  - [x] 12.2 冒烟：gRPC 端口监听/禁用行为、健康检查
  - [x] 12.3 按 AGENTS.md 规范 git commit（中文 Conventional Commits）

# 追加任务（测试强化：白盒/黑盒分层与 fuzz）
- [x] Task 13: 测试分层重组与 fuzz 测试
  - [x] 13.1 根包黑盒化：`daytype_test.go`、`calendar_test.go`、`store_test.go` 迁至 `package goliday_test`，新增 `config_blackbox_test.go` 承接原 config_test.go 的 LoadYear/LoadDir/Validate 契约测试与黑盒公用 helper；`config_test.go` 精简为白盒（未导出 `parseDate`）
  - [x] 13.2 黑盒穷举/全年校验：DayType 全 256 取值不变量（Coarse ∈ {Workday,Holiday} 且幂等、IsWorkday/IsHoliday 恰一真、String 分段合法名）；Calendar 对已配置年份全年逐日结果 ∈ 6 种合法组合且粗细一致
  - [x] 13.3 fuzz 目标：`FuzzParseDate`（根包白盒）、`FuzzQueryConsistency`/`FuzzLoadYearTOML`（根包黑盒）、`FuzzDaysHandler`（server 白盒）、`FuzzGenDraft`（tool 白盒），种子内联、无新增依赖
  - [x] 13.4 同步 `docs/ARCHITECTURE.md`（目录树与测试分层说明）
  - [x] 13.5 验证：`go build ./... && go vet ./... && go test -count=1 ./... && gofmt -l .` 全绿；各 fuzz 目标逐包 `-fuzz` 冒烟通过；无 `testdata/fuzz/` 语料残留

# 追加任务（年份强校验 + 统计前缀和化：enforce-year-loading-prefix-stats）
> 变更内容：查询覆盖年份未加载即报错（`year_not_loaded`）；统计改为构造期前缀和差分（O(覆盖年数)），stats 接口取消 366 天跨度限制，days 保留；days 明细的 stats 复用前缀和路径。规格见 `spec.md`「年份加载强校验」「细粒度组合计数前缀和统计」及对应 MODIFIED Requirement。

- [x] Task 14: 根包年份强校验与前缀和实现（`calendar.go`）
  - [x] 14.1 定义导出哨兵错误 `ErrYearNotLoaded`（`errors.Is` 判别，message 含年份）与 `(c *Calendar) HasYear(year int) bool`；`Query`/`QueryCoarse`/`IsWorkday`/`IsHoliday`/`QueryRange` 增加 `error` 返回，未加载年返回包装错误，不再回退周休
  - [x] 14.2 `yearIndex` 增加按组合计数的前缀和（长度=年天数+1，`prefix[0]=0`，左闭右开语义），`NewCalendar` 一次性构建后只读；组合→计数导出：细 `ordinary=C(1)`、`compensate=C(6)`、`weekend=C(4)+C(6)+C(12)`、`festival=C(12)+C(24)`、`adjusted=C(16)+C(24)`，粗 `workday=C(1)+C(6)`、`holiday=C(4)+C(12)+C(16)+C(24)`；`detailed=false` 时 `Fine=nil`
  - [x] 14.3 新增 `StatsRange(start, end, detailed) (StatsResult, error)`：规范化日期后按年拆段差分（首年段/整年段/末年段），`Total` 为区间天数；跨年含未加载年报错；`Stats(dates, detailed)` 改返回 `(StatsResult, error)`，走前缀和路径
- [x] Task 15: 根包测试适配与新增
  - [x] 15.1 适配 `Query`/`QueryCoarse`/`QueryRange`/`Stats` 新签名；「无该年配置回退周休」契约改写为断言 `ErrYearNotLoaded`
  - [x] 15.2 新增前缀和 vs 暴力一致性测试：2025/2026 全年及随机子区间、跨年区间（2025→2026）粗/细/Total 完全一致
  - [x] 15.3 新增未加载年场景：`Query`/`StatsRange`/`Stats` 对未加载年断言 `errors.Is(err, ErrYearNotLoaded)`
  - [x] 15.4 `FuzzQueryConsistency` 不变量更新：已加载年不变量保持，未加载年断言 `ErrYearNotLoaded`；种子补未加载年样本
- [x] Task 16: 服务层年份强校验与前缀和接入（`cmd/goliday-server`）
  - [x] 16.1 新增共用错误 `errYearNotLoaded`（code `year_not_loaded`，message 列升序去重年份）；覆盖年份收集：单日 `{date.Year()}`、区间 `[start.Year(), (end-1d).Year()]` 全部整年、列表各日期年、混合并集；响应构建前统一校验，空区间（start==end）直接返回空统计
  - [x] 16.2 `resolveMultiQuery` 参数化跨度上限（days=366、stats=无上限），stats 路径不展开区间为逐日切片；统计路径改走 `StatsRange`/新 `Stats`，保证 days 与 stats 同输入统计一致；单日模式处理 `Query` 的 error 分支
  - [x] 16.3 gRPC 同步：复用覆盖年份校验与共用错误（GetDay/queryMulti 入口先校验年份，映射 `codes.InvalidArgument`）；`QueryStats` 不再校验跨度、`QueryDays` 保留 366 上限
- [x] Task 17: 服务层测试适配与新增
  - [x] 17.1 新增场景：单日/区间/离散/混合未加载年 → 400/InvalidArgument `year_not_loaded` 且 message 含年份；跨年区间含未加载中间年；空区间 200 全零
  - [x] 17.2 新增场景：stats 大跨度（2025→2026）成功且与分段统计一致；days 同区间超 366 天被拒（invalid_range）
  - [x] 17.3 适配既有用例签名与「无配置年回退」断言改写；`FuzzDaysHandler` 不变量更新（状态码仍 200/400；stats 路径无跨度 400；未加载年 → `year_not_loaded`）
- [x] Task 18: 文档同步
  - [x] 18.1 `docs/API.md`：错误表加 `year_not_loaded` 行；第 2/3 节跨度限制改为「仅 days 366 上限」；stats 章节注明前缀和实现与不限跨度
  - [x] 18.2 `docs/ARCHITECTURE.md`：Calendar 前缀和结构、构建时机、并发语义、O(覆盖年数) 统计与年份强校验说明
- [x] Task 19: 全量验证与提交
  - [x] 19.1 `go build ./... && go vet ./... && go test -count=1 ./...` 全绿；`gofmt -l .` 为空；各 fuzz 目标逐包 `-fuzz` 冒烟通过；无 `testdata/fuzz/` 残留
  - [x] 19.2 冒烟：本地起服务验证 `year_not_loaded`、stats 大跨度、days 超限 400 三类行为
  - [x] 19.3 按 AGENTS.md 先勾选 `spec/tasks.md`、`spec/checklist.md`（含「执行提交」自指项）再 git commit（中文 Conventional Commits，建议 `feat: 查询年份强校验与统计前缀和化`），提交后 `git status --short` 干净
