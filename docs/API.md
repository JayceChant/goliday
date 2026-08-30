# goliday HTTP API

goliday-server 提供中国法定节假日日期类型查询服务：单日、区间（左闭右开）、离散列表与混合查询，粗/细两级粒度，以及统计接口。服务仅使用 Go 标准库，配置为纯内存、启动时一次性加载。

相关文档：[CONFIG_FORMAT.md](./CONFIG_FORMAT.md)（配置格式与 DayType 掩码详解）、[ARCHITECTURE.md](./ARCHITECTURE.md)。

---

## 1. 启动服务

```bash
go run ./cmd/goliday-server -addr :8080 -config-dir ./configs
```

| flag | 默认值 | 说明 |
|---|---|---|
| `-addr` | `":8080"` | HTTP 监听地址 |
| `-config-dir` | `"./configs"` | 年度配置目录（`<year>.toml`，格式见 [CONFIG_FORMAT.md](./CONFIG_FORMAT.md)） |
| `-v` | — | 输出 `goliday-server version 0.1.0` 后退出 |

启动时全量加载配置目录；任一文件解析或校验失败则启动失败并打印带文件路径的错误。加载成功后输出已加载年份列表日志。

路由一览（Go 1.22+ `ServeMux`「方法 + 路径」模式）：

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/days` | 日期类型查询，含逐日明细 `days` |
| GET | `/api/v1/stats` | 与 `/days` 统计口径一致，不含 `days` 明细 |
| GET | `/healthz` | 健康检查，返回已加载年份 |

所有日期参数格式均为 `YYYY-MM-DD`。下文示例使用 2026 年假设配置数据（非官方方案）。

---

## 2. GET /api/v1/days

### 2.1 参数

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `date` | string | 三选一 | 单日查询 |
| `start` | string | 与 `end` 成对 | 区间起点（**含**） |
| `end` | string | 与 `start` 成对 | 区间终点（**不含**，左闭右开 `[start, end)`） |
| `dates` | string | 三选一 | 逗号分隔的离散日期列表；可与 `start+end` 同时提供（并集、去重、升序） |
| `detailed` | bool | 否，默认 `false` | `true` 时单日查询的 `type` 为细粒度掩码、`stats` 为细粒度交叉计数；多日明细 `days[].type` 恒为细粒度数值，本参数仅切换 `stats` 口径 |

模式判定：仅 `date` → 单日；`start+end`（可叠加 `dates`）→ 区间 / 混合；仅 `dates` → 离散列表。区间与 `dates` 同时提供时取并集去重升序，`mode` 为 `"list"`。`date` 与 `start+end`/`dates` 并存时以 `date` 为准（单日模式），其余参数被忽略；单日响应不含 `mode` 与 `days` 字段。

限制：`end` 不得早于 `start`；区间跨度不得超过 366 天；违反返回 400（见第 5 节）。

### 2.2 单日查询

默认粗粒度：

```bash
curl "http://localhost:8080/api/v1/days?date=2026-02-20"
```

```json
{"date": "2026-02-20", "type": 28, "type_label": "holiday", "total_days": 1, "stats": {"holiday": 1, "workday": 0}}
```

2026-02-20 为周五春节调休（`off`），粗粒度为休息日 `holiday`（28）。

细粒度：

```bash
curl "http://localhost:8080/api/v1/days?date=2026-02-20&detailed=true"
```

```json
{"date": "2026-02-20", "type": 16, "type_label": "adjusted", "total_days": 1, "stats": {"adjusted": 1, "compensate": 0, "festival": 0, "ordinary": 0, "weekend": 0}}
```

### 2.3 区间查询

```bash
curl "http://localhost:8080/api/v1/days?start=2026-02-14&end=2026-02-18"
```

左闭右开：覆盖 2026-02-14 ~ 2026-02-17 共 4 天（不含 `end`）。

```json
{
  "mode": "range",
  "start": "2026-02-14",
  "end": "2026-02-18",
  "total_days": 4,
  "days": [
    {"date": "2026-02-14", "type": 6,  "type_label": "compensate|weekend"},
    {"date": "2026-02-15", "type": 4,  "type_label": "weekend"},
    {"date": "2026-02-16", "type": 16, "type_label": "adjusted"},
    {"date": "2026-02-17", "type": 24, "type_label": "festival|adjusted"}
  ],
  "stats": {"holiday": 3, "workday": 1}
}
```

逐日说明：02-14（周六）补班 → 粗粒度 workday（细粒度 `compensate|weekend`）；02-15（周日）自然周末 → holiday；02-16（周一）春节调休 → holiday；02-17（周二）春节当天 → holiday（`festival|adjusted`）。注意 `days` 明细的 `type` **恒为细粒度数值**（与 `detailed` 无关），`detailed` 仅切换 `stats` 统计口径。

细粒度版本（`&detailed=true`，`days` 明细不变，仅 `stats` 换为交叉计数）：

```json
{
  "mode": "range",
  "start": "2026-02-14",
  "end": "2026-02-18",
  "total_days": 4,
  "days": [
    {"date": "2026-02-14", "type": 6,  "type_label": "compensate|weekend"},
    {"date": "2026-02-15", "type": 4,  "type_label": "weekend"},
    {"date": "2026-02-16", "type": 16, "type_label": "adjusted"},
    {"date": "2026-02-17", "type": 24, "type_label": "festival|adjusted"}
  ],
  "stats": {"ordinary": 0, "compensate": 1, "weekend": 2, "festival": 1, "adjusted": 2}
}
```

### 2.4 离散列表查询

```bash
curl "http://localhost:8080/api/v1/days?dates=2026-02-16,2026-02-28,2026-02-17"
```

输入允许乱序、允许重复，返回去重后升序排列：

```json
{
  "mode": "list",
  "total_days": 3,
  "days": [
    {"date": "2026-02-16", "type": 16, "type_label": "adjusted"},
    {"date": "2026-02-17", "type": 24, "type_label": "festival|adjusted"},
    {"date": "2026-02-28", "type": 6,  "type_label": "compensate|weekend"}
  ],
  "stats": {"holiday": 2, "workday": 1}
}
```

2026-02-28 为周六补班（`work`），故粗粒度统计归入 workday（细粒度 `compensate|weekend`）。细粒度版本：

```bash
curl "http://localhost:8080/api/v1/days?date=2026-02-28&detailed=true"
```

```json
{"date": "2026-02-28", "type": 6, "type_label": "compensate|weekend", "total_days": 1, "stats": {"adjusted": 0, "compensate": 1, "festival": 0, "ordinary": 0, "weekend": 1}}
```

### 2.5 混合查询（区间 + 离散）

```bash
curl "http://localhost:8080/api/v1/days?start=2026-02-01&end=2026-02-03&dates=2026-03-08"
```

区间 [02-01, 02-03) 与 03-08 取并集，`mode` 为 `"list"`：

```json
{
  "mode": "list",
  "total_days": 3,
  "days": [
    {"date": "2026-02-01", "type": 4,  "type_label": "weekend"},
    {"date": "2026-02-02", "type": 1,  "type_label": "ordinary"},
    {"date": "2026-03-08", "type": 4,  "type_label": "weekend"}
  ],
  "stats": {"holiday": 2, "workday": 1}
}
```

02-01 与 03-08 均为周日；02-02 为周一普通工作日。

### 2.6 细粒度统计的交叉计数

`detailed=true` 时，`stats` 的五个键（`ordinary` / `compensate` / `weekend` / `festival` / `adjusted`）是**单标志位交叉计数**：组合日对其含有的**每个标志位各计 1**。以 2.3 的区间为例：

- 02-17 为 `festival|adjusted` → `festival` +1 **且** `adjusted` +1；
- 02-14 为 `compensate|weekend` → `compensate` +1 **且** `weekend` +1。

因此**各键之和（0+1+2+1+2 = 6）可以大于 `total_days`（4）**。使用建议：

- 「总休息/上班天数」→ 用粗粒度 `stats.holiday` / `stats.workday`；
- 「含某标志的天数」（如整个春节假期含 festival 的天数）→ 用细粒度对应键。

---

## 3. GET /api/v1/stats

参数与 `/api/v1/days` **完全相同**（`date`、`start`、`end`、`dates`、`detailed`），统计口径一致，但**不返回 `days` 明细**。两接口复用同一处理器逻辑，同一输入的 `total_days` 与 `stats` 保证一致。

区间、粗粒度：

```bash
curl "http://localhost:8080/api/v1/stats?start=2026-02-14&end=2026-02-18"
```

```json
{
  "mode": "range",
  "start": "2026-02-14",
  "end": "2026-02-18",
  "total_days": 4,
  "stats": {"holiday": 3, "workday": 1}
}
```

离散、细粒度：

```bash
curl "http://localhost:8080/api/v1/stats?dates=2026-02-17,2026-02-28&detailed=true"
```

```json
{
  "mode": "list",
  "total_days": 2,
  "stats": {"ordinary": 0, "compensate": 1, "weekend": 1, "festival": 1, "adjusted": 1}
}
```

（02-17 为 `festival|adjusted`，02-28 为 `compensate|weekend`，各标志位分别计数。）

---

## 4. GET /healthz

```bash
curl "http://localhost:8080/healthz"
```

```json
{"status": "ok", "years": [2025, 2026]}
```

`years` 为已加载的年份（升序），可用于部署后确认新年度配置已被加载。

---

## 5. 错误格式

所有非 2xx 响应统一为：

```json
{"error": {"code": "invalid_date", "message": "非法日期 \"2026-02-30\"：须为 YYYY-MM-DD 格式的有效日期"}}
```

| HTTP | code | 触发条件示例 |
|---|---|---|
| 400 | `missing_query` | `date`、`start+end`、`dates` 均缺省 |
| 400 | `invalid_date` | `date=2026-02-30`（不存在的日期）或 `date=20260230`（格式非法） |
| 400 | `invalid_range` | `start`/`end` 只出现其一；`end` 早于 `start`；区间跨度超过 366 天 |
| 400 | `invalid_detailed` | `detailed` 不是合法布尔值（如 `detailed=abc`） |
| 404 | `not_found` | 访问未注册路径，如 `GET /api/v1/unknown` |
| 405 | `method_not_allowed` | 对 `/api/v1/days` 使用 POST 等非 GET 方法 |

示例：

```json
{"error": {"code": "method_not_allowed", "message": "方法不被允许: POST"}}
```

---

## 6. DayType 掩码数值表与 type_label 对照

`type` 字段为整数掩码（`DayType`，`uint8`）。位定义：

| 常量 | 数值 | 含义 |
|---|---|---|
| `DayTypeOrdinary` | 1 | 普通工作日 |
| `DayTypeCompensate` | 2 | 补班 |
| `DayTypeWeekend` | 4 | 周末 |
| `DayTypeFestival` | 8 | 节日 |
| `DayTypeAdjusted` | 16 | 调休 |
| `DayTypeWorkday` | 3 | 粗粒度：上班日 = `Ordinary\|Compensate` |
| `DayTypeHoliday` | 28 | 粗粒度：休息日 = `Weekend\|Festival\|Adjusted` |

响应中 `type` 的全部取值及 `type_label` 对照（`type_label` 即 `DayType.String()`）：

| type | 组合 | 粗粒度归属 | `detailed=false` 时 | `detailed=true` 时 |
|---|---|---|---|---|
| 1 | `Ordinary` | Workday(3) | — | `ordinary` |
| 4 | `Weekend` | Holiday(28) | — | `weekend` |
| 6 | `Compensate\|Weekend` | Workday(3) | — | `compensate\|weekend` |
| 12 | `Festival\|Weekend` | Holiday(28) | — | `weekend\|festival` |
| 16 | `Adjusted` | Holiday(28) | — | `adjusted` |
| 24 | `Festival\|Adjusted` | Holiday(28) | — | `festival\|adjusted` |
| 3 | `Workday`（粗粒度值本身） | — | `workday` | — |
| 28 | `Holiday`（粗粒度值本身） | — | `holiday` | — |

细→粗映射：含 `Compensate` 位 → `Workday`（补班优先归上班日，即使当天是周末）；否则含 `Weekend/Festival/Adjusted` 任一位 → `Holiday`。

> `type_label` 由 `DayType.String()` 生成：组合值按细粒度位**从低到高**以 `|` 连接小写名。故 `Festival|Weekend`（8|4 = 12）输出 `weekend|festival`（`weekend` 位更低排在前），而 `Festival|Adjusted`（8|16 = 24）输出 `festival|adjusted`。
