# Checklist

## 工程与依赖
- [x] Go module `goliday` 已初始化，go 指令为 1.27
- [x] 核心逻辑位于根包 `goliday`，服务位于 `cmd/goliday-server`，工具位于 `cmd/goliday-tool`
- [x] 全项目仅一个第三方依赖 `github.com/BurntSushi/toml`（`go list -m all` 验证）；HTTP 服务仅标准库（无路由框架）
- [x] `go build ./...`、`go vet ./...`、`go test ./...` 全部通过

## DayType 枚举
- [x] `DayType` 为 uint8 位掩码：`Ordinary=1`、`Compensate=2`、`Weekend=4`、`Festival=8`、`Adjusted=16`、`Workday=3`、`Holiday=28`
- [x] `Coarse()` 按补班优先规则映射（`Compensate|Weekend` → `Workday`）；`IsWorkday()`/`IsHoliday()` 与之一致
- [x] 细粒度合法组合全集：`1`、`6`、`4`、`12`、`16`、`24`
- [x] `String()`：组合按位序 `|` 连接小写名；粗值返回 `workday`/`holiday`
- [x] 枚举单元测试覆盖全部 spec Scenario

## 稀疏配置
- [x] 配置为 TOML 按年文件 `<year>.toml`，启动时经 `-config-dir` 加载，纯内存无数据库
- [x] 稀疏表原则：仅记录调整过的日期——`off` 仅周一~周五（工作日变休息）、`work` 仅周六/周日（周末变上班）；周末/普通工作日等标准库可判定信息未写入
- [x] `[[festival]]` 仅含名称与节日当天日期
- [x] 判断算法：work→`Compensate|Weekend`；off→`Adjusted`；节日当天附加 `Festival`；否则周休回退；无该年文件整年回退
- [x] 校验：year 与文件名一致、off/work 稀疏合法、无重复且互斥、work 不含 festival.date、日期合法且在年内；违规启动失败并指明文件与原因
- [x] `LoadDir` 忽略非 `.toml` 文件与子目录
- [x] `docs/CONFIG_FORMAT.md` 含 TOML/YAML/JSON 选型对比（结论 TOML）与稀疏表原则说明

## 查询与统计
- [x] 单日期查询默认粗粒度（`workday`/`holiday`），`detailed` 开启细粒度
- [x] 区间查询左闭右开 `[start, end)`，`days` 不含 end，跨度上限 366 天
- [x] 离散列表去重、升序；与区间可混合（并集，`mode=list`）
- [x] 统计粗粒度 `workday/holiday`；细粒度 `ordinary/compensate/weekend/festival/adjusted`，组合日交叉计数（各标志各计 1 天）
- [x] `/api/v1/stats` 与 `/api/v1/days` 同区间下 `stats`、`total_days` 一致
- [x] 参数校验：`end<start`、跨度超限、输入均缺省、日期非法 → 400 统一错误格式

## HTTP 服务
- [x] flag 参数：`-addr`（默认 `:8080`）、`-config-dir`（默认 `./configs`）、`-v`
- [x] 路由（Go 1.22 ServeMux）：`GET /api/v1/days`、`GET /api/v1/stats`、`GET /healthz`；中间件：日志、Panic 恢复
- [x] 统一错误响应 `{"error":{"code","message"}}`；日期解析 `2006-01-02`
- [x] handlers 测试覆盖全部 API Scenario（httptest）

## 生成工具与提示词
- [x] `goliday-tool validate`：合法样例退出 0，invalid 样例报错并退出非 0
- [x] `goliday-tool gen`：由年份 + 公告原文生成稀疏 TOML 草稿（剔除周末、补班入 work、节日推断，无法推断留占位注释），产物可通过 validate
- [x] `docs/generate_prompt.md` 提供公告 → 年度 TOML 的 LLM 提示词模板
- [x] `docs/holiday_config_example.toml` 完整注释示例；`docs/API.md`、`docs/ARCHITECTURE.md` 与实现一致（17 处偏差已按代码修正）

## TODO 项登记（本次不实现，仅记录）
- [x] ~~登记：后续实现 gRPC 接口（语义与 HTTP API 一致）~~（已由追加任务 Task 9~12 落地）
- [x] ~~登记：后续提供 `proto/goliday/v1/goliday.proto` proto 文档及生成说明（`DayType` 以 `uint32` 表达）~~（已落地）

## 追加验收（gRPC TODO 落地）
- [x] `proto/goliday/v1/goliday.proto` 存在：package `goliday.v1`、`go_package` 正确、DayType 以 uint32 位注释表达、GolidayService（GetDay/QueryDays/QueryStats）
- [x] 生成代码 `proto/goliday/v1/{goliday.pb.go,goliday_grpc.pb.go}` 入库，无需 protoc 即可编译
- [x] `-grpc-addr`（默认 `:50051`，空串禁用）生效；gRPC 与 HTTP 同进程；注册标准健康检查；优雅关闭覆盖双协议
- [x] gRPC 查询语义与 HTTP 一致（单日/区间/离散/混合、detailed、跨度 ≤366、明细恒细粒度、stats 交叉计数）
- [x] 参数错误 → `codes.InvalidArgument`，message 文案与 HTTP 一致
- [x] grpc_test.go（bufconn）覆盖语义一致性与错误场景
- [x] 根包依赖审计：根包导入无 gRPC/protobuf 模块；`go.mod` 直接依赖仅 BurntSushi/toml + grpc/protobuf（genproto/rpc、x/* 等均为 gRPC 传递依赖）
- [x] `docs/API.md` 新增 gRPC 章节、`docs/ARCHITECTURE.md` 依赖分级与目录树更新
- [x] 全量验证通过 + 中文 Conventional Commits 提交
