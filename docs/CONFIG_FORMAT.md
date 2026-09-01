# 节假日配置文件格式（CONFIG_FORMAT）

本文档描述 goliday 年度节假日配置文件的**格式选型、稀疏表原则、字段定义、校验规则与判定算法**。

- 带注释的完整示例：[holiday_config_example.toml](./holiday_config_example.toml)
- HTTP 接口用法：[API.md](./API.md)
- 新年度配置的生成流程：[generate_prompt.md](./generate_prompt.md)

---

## 1. 格式选型：为什么是 TOML

| 维度 | TOML（采用） | YAML | JSON |
|---|---|---|---|
| 注释支持 | 原生支持 `#` | 支持 `#` | **不支持**（JSON5/JSONC 为非标准方言） |
| 手写易错度（缩进 / 隐式类型） | 低：无缩进语义；日期一律写成带引号字符串，语义唯一 | 高：缩进敏感，缩进错误会静默改变结构；`2026-01-01` 被隐式解析为日期对象且各解析器行为不一；YAML 1.1 中 `01-01` 按六十进制隐式解析为数字 61；`no`/`on`/`off` 隐式转布尔 | 中：无缩进陷阱，但无注释导致信息只能外置；对尾逗号、引号要求严格 |
| Go 生态 | `github.com/BurntSushi/toml`（事实标准，本项目唯一三方依赖） | `gopkg.in/yaml.v3` 可用，但存在 v2/v3 版本分叉与行为差异 | 标准库 `encoding/json` |
| 结论 | ✅ **采用** | ❌ 排除 | ❌ 排除 |

展开说明：

- **JSON 被排除——无注释**。稀疏表必须内嵌解释性注释（为何某个周五是 `off`、为何周末一律不写入、公告文号出处），无注释的配置无法自解释、难以人工审计。
- **YAML 被排除——隐式解析与缩进风险**。`01-01` 在 YAML 1.1 中被解析为六十进制数字 `61`；`2026-01-01` 被隐式解析为日期对象且各解析器行为不一；`no`/`off` 被静默转为布尔。再叠加缩进敏感，本配置以「LLM 生成 + 人工校对」为主要生产方式，任何隐式转换都是潜在事故源。
- **TOML 胜出**。语法扁平，日期显式写成 `"YYYY-MM-DD"` 字符串、由代码严格解析，语义唯一；原生注释；扁平字符串数组最贴合稀疏表形态；Go（BurntSushi/toml）与 Python 3.11+（内置 tomllib）生态成熟，便于脚本交叉校验。

---

## 2. 稀疏表原则

配置**只记录节假日办「调整过」的日期**；凡可由标准库按星期推导的信息（周末、普通工作日）一律不写入。

| 字段 | 只允许写入 | 不允许写入 | 语义 |
|---|---|---|---|
| `[adjust].off` | 周一~周五（工作日变休息） | 周六/周日 | 放假日：含节日当天（若为工作日）与为拼长假而调休的工作日 |
| `[adjust].work` | 周六/周日（周末变上班） | 周一~周五 | 补班日：被「征用」来补班的周末 |
| `[[festival]]` | 仅 `name` + `date` 两个字段 | — | 节日名称与节日当天 |

要点：

1. **off 仅周一~周五**：工作日被调整为休息才写入；节日当天若为工作日，也出现在 off 中。
2. **work 仅周六/周日**：周末被调整为上班才写入。
3. **假期中的自然周末不写入**：周六/周日本来就是休息日，不属于「调整」，由程序按星期自然推导为休息日。
4. **普通工作日不写入**：未被动过的周一~周五按星期推导为工作日。
5. **festival 仅 name + date**：只记节日当天（如春节只记正月初一那天，不记除夕、不记整个假期区间）；节日当天恰逢周末时只写 festival、不写 off/work，为周一~周五时则必须出现在 off 中（见校验规则 10）。

这样每个年份文件通常只有 20~30 个日期，可以被人在几分钟内逐条审计。

---

## 3. 文件组织与字段

每年一个文件，放在配置目录（服务 `-config-dir` 指定，默认 `./configs`）下，命名为 `<year>.toml`（如 `2026.toml`）。文件名必须是**纯四位数字年份**，其他文件（README、子目录等）在加载时被忽略。

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `year` | int | 是 | 年份，必须与文件名一致 |
| `name` | string | 否 | 备注名（如公告文号），不参与判定 |
| `[[festival]].name` | string | 是 | 节日名称，如 `"春节"` |
| `[[festival]].date` | string | 是 | 节日当天，`YYYY-MM-DD` |
| `[adjust].off` | string[] | 否 | 放假日列表，仅周一~周五，建议升序；可为空数组或缺省 |
| `[adjust].work` | string[] | 否 | 补班日列表，仅周六/周日，建议升序；可为空数组或缺省 |

所有日期一律写成**带引号的字符串**，格式严格为 `YYYY-MM-DD`，且必须是真实存在的日期（`2026-02-30` 会被拒绝，闰年 2 月 29 日合法）。

---

## 4. 完整示例

摘自 [holiday_config_example.toml](./holiday_config_example.toml)（2026 年假设方案，非官方）：

```toml
year = 2026

# 节日定义：仅名称与节日当天日期
[[festival]]
name = "元旦"
date = "2026-01-01"

[[festival]]
name = "春节"
date = "2026-02-17"

[[festival]]
name = "清明节"
date = "2026-04-05"

[adjust]
# off = 工作日变休息（仅周一~周五）
off = [
  "2026-01-01", "2026-01-02",           # 元旦（01-03/04 为自然周末，不写入）
  "2026-02-16", "2026-02-17", "2026-02-18",
  "2026-02-19", "2026-02-20",           # 春节（02-21/22 为自然周末，不写入）
  "2026-04-06",                         # 清明调休（04-05 为周日，只写 festival）
]
# work = 周末变上班（仅周六/周日）
work = [ "2026-02-14", "2026-02-28" ]   # 春节补班
```

完整示例文件含全部七个节日、逐条注释与「不写入示例」，可直接作为新年度文件的模板。

---

## 5. 校验规则

服务启动加载与 `goliday-tool validate` 执行**同一套校验**，违规即报错并指明文件路径与具体原因：

| # | 规则 | 对应非法样例（`testdata/invalid/`） |
|---|---|---|
| 1 | `year` 与文件名一致（文件名须为纯四位数字 `NNNN.toml`） | `year_mismatch.toml` |
| 2 | 所有日期合法：`YYYY-MM-DD` 格式且真实存在（含闰年） | `invalid_date.toml`（`2026-02-30`） |
| 3 | 所有日期均落在 `year` 年内 | — |
| 4 | `off` 仅含周一~周五（写入周末即违反稀疏原则） | `off_weekend.toml` |
| 5 | `work` 仅含周六/周日 | `work_weekday.toml` |
| 6 | `off`、`work` 各自内部无重复日期 | `dup.toml` |
| 7 | `off` 与 `work` 互斥（同一日期不得同时出现） | — |
| 8 | `work` 不含任何 `festival.date`（节日当天不得补班） | — |
| 9 | `festival.date` 之间无重复 | — |

校验失败行为：`LoadYear` 返回以文件路径开头的错误；服务启动失败；`goliday-tool validate` 打印错误并以非 0 退出码结束。

---

## 6. 判定算法与 DayType 掩码

### 6.1 单日期判定（优先级从高到低）

1. 命中该年 `work` → `Compensate|Weekend`（补班日必为周末）；
2. 命中该年 `off` → `Adjusted`（放假日必为工作日，覆盖周休基线）；
3. 为某 `festival.date`（节日当天）→ 在上述结果上**追加** `Festival` 位（`t |= Festival`）；
4. 否则周休回退：周六/周日 → `Weekend`，周一~周五 → `Ordinary`；
5. 该年无配置文件 → 不回退，返回 `year_not_loaded`（见 [API.md](./API.md) 第 5 节）：避免「未配置」被误读为「真实周末」。

等价表达：`t = 周末 ? Weekend : Ordinary`；`if off 命中 { t = Adjusted }`；`if work 命中 { t = Compensate|Weekend }`；`if 节日当天 { t |= Festival }`。

### 6.2 DayType 掩码值表

`DayType` 为 `uint8` 位掩码：

| 常量 | 数值 | 二进制 | 含义 | 粗粒度归属 |
|---|---|---|---|---|
| `DayTypeOrdinary` | 1 | `0b00001` | 普通工作日（细） | 上班日 |
| `DayTypeCompensate` | 2 | `0b00010` | 补班（细） | 上班日 |
| `DayTypeWeekend` | 4 | `0b00100` | 周末（细） | 休息日 |
| `DayTypeFestival` | 8 | `0b01000` | 节日（细） | 休息日 |
| `DayTypeAdjusted` | 16 | `0b10000` | 调休（细） | 休息日 |
| `DayTypeWorkday` | 3 | — | 粗粒度：上班日 = `Ordinary\|Compensate` | — |
| `DayTypeHoliday` | 28 | — | 粗粒度：休息日 = `Weekend\|Festival\|Adjusted` | — |

细→粗映射（`Coarse()`）：含 `Compensate` 位 → `Workday`（**补班优先归上班日**，即使当天是周末）；否则含 `Weekend/Festival/Adjusted` 任一位 → `Holiday`；否则 → `Workday`。

### 6.3 六种细粒度合法组合

由真实方案数据与周休推导产生的组合全集：

| 数值 | 组合 | `String()` | 典型场景（2026 假设方案） | 粗粒度 |
|---|---|---|---|---|
| 1 | `Ordinary` | `ordinary` | 普通工作日：2026-03-03（周二） | workday |
| 4 | `Weekend` | `weekend` | 自然周末：2026-02-15（周日） | holiday |
| 6 | `Compensate\|Weekend` | `compensate\|weekend` | 周末补班：2026-02-28（周六） | workday |
| 12 | `Festival\|Weekend` | `weekend\|festival` | 节日恰逢周末：2026-04-05（周日，清明） | holiday |
| 16 | `Adjusted` | `adjusted` | 工作日调休：2026-02-20（周五） | holiday |
| 24 | `Festival\|Adjusted` | `festival\|adjusted` | 节日当天且为工作日：2026-02-17（周二，春节） | holiday |

> 注 1：该全集的前提是配置符合真实方案——节日当天要么是工作日（必在 `off`，得 24）、要么恰逢自然周末（得 12）；校验规则 8（work 不含节日当天）保证了不会出现「补班 × 节日」的组合，校验规则 10（工作日节日须在 off）保证了不会出现「工作日 × 节日」的组合。
>
> 注 2：`String()` 组合名按细粒度位**从低到高**连接，故 `Festival|Weekend` 输出 `weekend|festival`（weekend=4 低于 festival=8，低位在前），而 `Festival|Adjusted` 输出 `festival|adjusted`（festival=8 低于 adjusted=16）。

---

## 7. 年度更新流程

每年 11 月左右国务院办公厅公布次年放假安排后：

1. **获取官方公告**：以《国务院办公厅关于 YYYY 年部分节假日安排的通知》原文（国办发明电〔YYYY〕XX 号）为准，不要使用自媒体转述版本。
2. **生成草稿**（二选一，可互为交叉校验）：
   - 工具：`goliday-tool gen -year 2027 -out /tmp/2027.toml -file 公告.txt`（`-file` 缺省或为 `-` 时从 stdin 读取公告文本；`-out` 为 `-` 时输出到 stdout）。工具会剔除假期中的自然周末、把补班日写入 `work`、按规则推断节日当天；无法推断时该条目 `date` 置占位值 `"TODO"` 并打印告警，此类草稿在人工补全前无法通过 validate（属预期），补全后即可通过。
   - 提示词：将公告原文粘贴进 [generate_prompt.md](./generate_prompt.md) 的模板，交给 LLM 生成。
3. **机器校验**：`goliday-tool validate /tmp/2027.toml`，通过时打印 `OK` 并退出 0；失败打印 `FAIL <路径>: <原因>` 并退出 1（用法错误退出 2）。
4. **人工抽查**：至少核对每个节日当天（春节须为正月初一）、每个假期的边界工作日（是否在 `off`）、每个补班日（是否为周六/周日）。
5. **入库**：将文件放入 `configs/2027.toml`。
6. **重启服务**：配置仅在启动时加载、无热加载。重启后确认日志与 `/healthz` 返回的 `years` 已包含新年份。
