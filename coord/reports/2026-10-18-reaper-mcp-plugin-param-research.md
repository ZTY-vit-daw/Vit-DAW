# REAPER MCP 控制第三方效果器：实现、效果与「参数名不统一」解法调研

- 日期：2026-10-18
- 调研方式：GitHub 仓库浅克隆读源码（记录 commit）+ 官方 ReaScript 文档 + web 检索；**未在真实 REAPER / Vit-DAW 栈上运行验证**
- 结论用途：判断是否值得借鉴到 Vit-DAW 的第三方插件控制层（pluginprobe / PCA / plugin_grabber）
- 仓库锚点均为调研时的 commit，链接按 commit 固定，避免上游漂移

---

## 0. 结论摘要（TL;DR）

1. **它没有把所有插件的参数名统一成一套名字。** 它统一的是**访问方式**：宿主把每个插件暴露为 `参数序号 + 参数名 + 归一化值(0–1) + 显示字符串`，所有厂商在机械层面同构；「名字歧义」交给模型在运行时读真实参数名做语义判断。
2. 真正的「统一」只发生在**自动混音引擎**里：引擎说的是抽象参数（频率 Hz、增益 dB、时间 ms、阈值 dB），再由**逐插件手写标定适配器**翻译成该插件的归一化值。这个适配器只覆盖 **8 个插件**（FabFilter Pro-Q 3 / Pro-C 2 / Pro-R / Pro-L 2 + REAPER 自带 ReaEQ / ReaComp / ReaVerbate / ReaLimit），其它插件（Waves、iZotope、Valhalla 等）引擎明确不写、只做检测与排名。
3. 视频里「肥波 + 板式混响」的观感，最可能来自 **xDarkzx/Reaper-MCP** 的 demo：`engine_mix` 自动检测 FabFilter 并按风格创建 plate/hall/room 混响总线。**但视频本身未能联网核验**（YouTube 不可达），这是中等置信推断，不是实证。
4. **可借鉴的是工程结构与数据分层，不是它的参数名技巧**：运行时参数面自省 → 语义角色（role）→ 逐插件标定/缓存 → 带回读的确定性写入。它的模糊名匹配只是兜底，且首匹配有歧义风险，不适合作为 Vit-DAW 的权威写路径。
5. **对 Vit-DAW，最有价值的 3 件事**：① 把现有 kernel `normalized_role` 从「弱本地启发式」升级为带证据/置信度的**规范参数语义注册表**；② 用插件指纹做 key，**持久化 per-plugin 参数地图**（现在 `display_probe` 每次重算、不落库）；③ 给未识别插件补一条**人类单位写入 + 回读校验 + 失败回滚**的通用写路径（typed 工具已有这套，但只覆盖已识别族）。
6. 风险提醒：REAPER MCP 的「参数扫描」会真实写一遍所有参数（有副作用/崩溃史）；扫描缓存只按插件名做 key（已因版本污染改回 gitignore）；它调用的 `TrackFX_GetParameterStepCount` 在 REAPER 7.82 官方文档中检索不到（详见 §4.3）。

---

## 1. 调研对象识别：是哪几个「REAPER MCP」？

GitHub 搜索 + 源码确认，至少有 4 类做法。按「能否控制第三方效果器」筛选后，主要参照：

| 项目 | 定位 | 架构 | 规模/许可 | commit（2026-10-18 拉取） |
|---|---|---|---|---|
| [xDarkzx/Reaper-MCP](https://github.com/xDarkzx/Reaper-MCP) | 全流程 AI 制作/混音，**唯一挂 demo 视频** | Python FastMCP ↔ 文件 IPC ↔ 常驻 Lua ReaScript | 181 tools / 26 modules，v0.8.2，Apache-2.0，100★ | `1c9863c`（2026-10-05） |
| [bonfire-systems/reaper-mcp](https://github.com/bonfire-systems/reaper-mcp) | 60 工具，混音/母带 | Python + **python-reapy**（REAPER distant API，进程内） | MIT，159★ | `4dcc8f8`（2026-10-07） |
| [mthines/reaper-mcp](https://github.com/mthines/reaper-mcp) | AI 混音工程师 + **插件知识库/记忆** | TypeScript Nx monorepo + Lua bridge + Python sidecar | 80+ tools，MIT | `4a01d1e`（2026-06-29） |
| [dschuler36/reaper-mcp-server](https://github.com/dschuler36/reaper-mcp-server) | **只读**工程解析/听感分析 | 直接解析 `.RPP` + 插件缓存 ini，无实时控制 | MIT，131★ | `451458e`（2026-10-15） |

> `dschuler` 明确「read-only by design」，不写工程、不控制插件参数，作为对照排除出主线。
> 另有 `shiehn/total-reaper-mcp`（600+ 工具 / 自然语言 DSL）、`thewhykiki` 等，路线相近但非本轮重点。

### 1.1 视频对应关系（中等置信）

- xDarkzx README 挂的唯一 demo：<https://youtu.be/F0i5njHaMXQ>。
- 该项目的卖点与视频描述高度吻合：**自动检测 FabFilter Pro-Q 3 / Pro-C 2 / Pro-R**，按 35 个风格档案自动布 EQ/压缩/混响总线，混响总线里就有 `plate` / `hall` / `room` 三类。
- bonfire / mthines / dschuler 的 README 均无视频链接。
- **未验证**：本机到 YouTube 不可达，未能播放/核对画面；因此只能按功能描述匹配，不能断言就是这条。

---

## 2. 控制链路（以 xDarkzx 为主，bonfire/mthines 对照）

### 2.1 总体架构（xDarkzx）

```
MCP Client (Claude/Cursor...)
   │ stdio / JSON-RPC
   ▼
FastMCP Python Server（工具面，180+ 个）
   │ 文件 IPC（command.json / response.json，原子替换）
   ▼
常驻 Lua ReaScript（REAPER 内，~6000 行，dispatch 表）
   │ ReaScript API
   ▼
REAPER（TrackFX_* 统一暴露所有 VST2/VST3/CLAP/JS）
```

- 桥细节：`docs/ARCHITECTURE.md`（文件锁心跳、30s/600s 超时、每个命令一个 undo 点、失败 pcall 隔离）。
- 与 Vit-DAW 的根本差异：REAPER MCP 是**宿主外部进程 + 脚本 API**；Vit-DAW 自己就是宿主，能拿到更原始的参数元数据（见 §5）。

### 2.2 读参数

- `fx_get_chain`：链上插件名、启用状态、预设、参数数量。
- `fx_get_params`：逐参数返回 `index / name / value / display`，并做 junk 过滤：
  - 跳过 `MIDI CC*`、`Internal`、`Pitch Bend`、`Channel Pressure`、空名 `-`、`Host Bypass`；
  - **FabFilter Pro-Q 专属**：先读 `Band N Used` 判断哪些 band 真被使用，跳过未使用 band 的 ~500 个参数（`reaper_mcp_server.lua:687-757`）。
- `fx_list_installed`：返回全部插件 + 每个类别「最佳可用」插件 + rack 检测（StudioRack/PatchWork/Snap Heap 内部不可见）。
- `fx_scan_params`（按需、显式）：扫一个插件的参数，学习范围/单位/曲线/步进，结果缓存。

### 2.3 写参数（当前版本唯一写路径）

**重要：`fx_add` / `fx_set_param` / `fx_set_param_by_name` 已在 0.6.9 从工具面移除**，写参数统一走 `setup_fx_chain`（CHANGELOG 2026-09-11）。原因不是省事，而是：

- 写序问题：`FabFilter Pro-Q 3 / Pro-C 2` 必须先写 band 的 `Used/Enabled`，再写 Frequency/Gain/Q，否则**参数写了没声音**；
- 更严重的是**崩溃**：0.6.3 记录 `engine_mix` 期间 Pro-Q 3/Pro-C 2 触发 `STATUS_ACCESS_VIOLATION`，minidump 定位到「`pairs()` 无序遍历参数，Frequency/Gain 先于 Used/Enabled 写入，命中插件未初始化状态」。修复是固定写序：`params_by_index` 按数字升序（Used/Enabled 自然在前），`params` 按字母序（`CHANGELOG.md:640-651`）。
- 工具实现：`setup_fx_chain(tracks)` 接受每轨 `fx_chain`，每项支持 `name`（模糊匹配添加）/ `fx_index`（已有插件）/ `params`（按名字模糊匹配）/ `params_by_index` / `preset` / `enabled`；逐项报错、不整体中断（`reaper_mcp_server.lua:4322-4428`）。

### 2.4 「肥波 + plate 混响」实际是怎么发生的

- `fx_list_installed` → `REVERB_RANKING` 里 Pro-R 排第一，其后才是 Valhalla VintageVerb / **Valhalla Plate** / Room / Abbey Road Reverb Plates 等（`mix_engine/fx_inventory.py:170-216`）。
- 但 `engine_mix` 真正能落地的只有两支族：`FABFILTER` 与 `REAPER_STOCK`；其它插件即使排名更高也**不会被写参数**，只会被报告/忽略（`mix_engine/selection.py:1-67`，instructions/00_core.md:138-151）。
- 风格档案里的 `plate` / `hall` / `room` 是**混响总线角色**；落到 Pro-R 时用 `Space` / `Brightness` 两个按名字写的参数，落到 ReaVerbate 时用 index 写 wet/room/damp/width 等（`mix_engine/plugins.py:393-408`、`:245-263`）。
- 所以视频里的「板式混响」更可能是「plate 混响总线」的听感/工作流，而不是「任意板式混响插件都被统一控制」。若视频里出现了 Valhalla Plate 等未被标定的插件，那只能是模型走通用 `setup_fx_chain` 按名字猜参数，属于**无标定、近似**路径。

---

## 3. 重点：它到底怎么处理「参数名称不统一」

先把问题拆开，否则「统一参数名」会变成一个伪命题：

| 子问题 | 例子 | xDarkzx 的解法 | 是否有保证 |
|---|---|---|---|
| A. 同一概念不同叫法 | `Mix` / `Wet` / `Blend`；`Decay` / `Time` / `Size` | 不统一名字；模型读真实参数名自行判断；自动引擎用抽象参数（Hz/dB/ms）+ 逐插件适配器 | 模型路径无保证；引擎路径有（仅 8 插件） |
| B. 同名不同量纲/方向 | Gain 显示 dB、内部 0–1；Drive 0–10；log/linear taper | `fx_scan_params` 采样显示字符串推断 linear/log/stepped + 单位；或逐插件手写公式 | 部分（扫描覆盖连续参数） |
| C. 版本/格式间 index 漂移 | Pro-C 2 / Pro-R 不同版本参数 index 不同 | 写参数优先按**名字**子串匹配，不写死 index；Pro-Q 波段用 `params_by_index`（因为波段结构可预测） | 有兜底，但首匹配可能错 |
| D. 多段/多带重复名 | `Band 1 Frequency` … `Band 24` | 名字 pattern 解析 band 号 + 过滤 unused band；EQ 写入用按 band 的 index 模板 | 有（Pro-Q 特例） |
| E. 离散/枚举 | Pro-Q 3 `Shape` 8 个值 | 扫全部步进值，把「显示字符串 → 归一化值」列出来 | 有（若扫描 API 可用，见 §4.3） |
| F. 不可自动化/被 rack 包裹 | 插件自定义 UI、StudioRack 内部 | 明确边界：不可见、不可控；rack 只检测不配置 | 无（诚实失败） |

具体机制分 6 层：

### 3.1 第一层：宿主统一暴露（机械同构）

REAPER 的 ReaScript 对所有插件给同一组 API：

- `TrackFX_GetNumParams` / `TrackFX_GetParamName` / `TrackFX_GetParam`（返回 val,min,max）
- `TrackFX_SetParam`（0–1 归一化）/ `TrackFX_GetParamNormalized`
- `TrackFX_FormatParamValue` / `GetFormattedParamValue` / `FormatParamValueNormalized`（显示字符串）
- `TrackFX_GetParameterStepSizes`（step/small/large/toggle）
- `TrackFX_GetParamIdent` / `TrackFX_GetParamFromIdent`（**稳定 ident**：`:wet`、`:bypass`、`:delta` 或插件返回的 ident 字符串）
- `TrackFX_GetParamEx`（min/mid/max）

来源：官方 ReaScript 文档 <https://www.reaper.fm/sdk/reascript/reascripthelp.html>（v7.82），以及 ReaWrap 绑定页 <https://conceptual-machines.github.io/ReaWrap/modules/track_fx.html>。

**关键点**：这一层把「不同厂商的插件」变成同一种形状（序号、名字、0–1、显示文本）。参数名不统一的问题在这里没有被消除，但被**降维**成「给模型看一串真实名字，让它自己选」。

> 一个可指出的浪费：REAPER 7 已经提供稳定 ident（`GetParamIdent`/`GetParamFromIdent`），但 xDarkzx 全仓库没有使用（grep 0 命中），仍在用名字子串匹配。对 Vit-DAW 的启示是：**能用稳定 ID 就不要用名字**。

### 3.2 第二层：运行时自省，而不是静态映射表

`fx_get_params` 每次返回**当前插件实际暴露的参数名 + 当前显示值**，模型据此选参数，例如看到 `Wet` 就知道是湿度，看到 `Mix` 同理。它没有维护「厂商 A 的 Mix = 厂商 B 的 Wet」的全局字典。

- 优点：零维护、新插件即插即用；模型对常见专业插件（FabFilter、Valhalla、Soundtoys 等）有先验知识，能直接读懂名字。
- 缺点：完全依赖模型；对命名含糊的插件（`Character`、`Drive`、`Amount`）无法保证语义正确；对同名多参数（`Gain` 出现 5 次）容易选错。
- 它的官方文档也承认这一点：**「扫描只解决参数标定，不解决插件/参数的语义；而语义是更常见、更重要的问题」**（`docs/ARCHITECTURE.md:43-50`）。

### 3.3 第三层：写入时的模糊名匹配（兜底，不是保证）

`setup_fx_chain` 的 `params: {name: value}` 路径，在 Lua 里做**大小写不敏感的子串匹配，取第一个命中**（`reaper_mcp_server.lua:4360-4389`）；旧的 `fx_set_param_by_name` 也是同一逻辑（`:1727-1749`，工具面已移除但 handler 仍被内部使用）。

```lua
for pi = 0, num_params - 1 do
  local _, pn = reaper.TrackFX_GetParamName(tr, fx_idx, pi, "")
  if pn:lower():find(pname_lower, 1, true) then found = pi; break end
end
```

- 为什么这么做：Pro-C 2 / Pro-R 的 index 随版本变化，用名字比 index 稳（`mix_engine/plugins.py:345-408` 注释明确写「fuzzy param name matching for version robustness」）。
- 风险：**首匹配**。`Freq` 会先命中 `Freq-Low Shelf`；`Gain` 会先命中 band 1；`Threshold` 若同时存在 `Threshold` 和 `Threshold (sidechain)` 则结果不可控。对高精度写参数，这不是可接受的权威路径。
- 它自己的补救：Pro-Q 波段写入改用 `params_by_index`（结构可预测）；`setup_fx_chain` 对未命中的名字逐项报 `"Parameter not found"`，不再静默丢弃（CHANGELOG 0.6.5）。

### 3.4 第四层：语义抽象 + 逐插件标定适配器（自动引擎的真正解法）

这是它「看起来什么都能控」的核心，也是唯一有数值保证的部分：

- 自动引擎的输入不是插件参数名，而是**混音语义参数**：`hp_freq`、`cut/boost {freq, gain_db, q, shape}`、`threshold_db`、`ratio`、`attack_ms`、`release_ms`、`room_size`、`dampening`、`wet_db`…
- 每个插件族有一个 adapter，把语义参数换算成该插件的归一化值：
  - **Pro-Q 3**：`freq = 10 * 3000^v`、`gain: 0.5 = 0dB，±30dB`、`Q = 0.025 * 1600^v`、`shape` 8 档按 `0/7…7/7`（`mix_engine/plugins.py:54-96`）；
  - **Pro-C 2**：threshold −60..0 dB → 0..1，ratio `log2/log2(20)`，attack/release 对数；参数用名字写（`Threshold/Ratio/Attack/Release/Knee/Output Gain`）；
  - **Pro-R**：只写 `Space` + `Brightness`（注释解释「Space 已编码 decay 行为，不再单独写 Decay」）；
  - **Pro-L 2**：`Output Level` 公式 `1.0 + true_peak_db/60`（注释记录旧公式偏了 30 dB 的 bug）；
  - **ReaEQ/ReaComp/ReaVerbate/ReaLimit**：各自硬编码 index + 曲线。
- 边界：**只覆盖 8 个插件**。Waves/iZotope/Valhalla 等即使被检测到、被排名，也不会被这套引擎写；用户偏好若不支持会显式返回 `preferences_ignored`。

> 这就是「参数名不统一」的工程化答案：**不要在名字层面统一，而是在语义层面统一，再为每个插件写一个小 adapter**。代价是 adapter 要人工标定、会随插件版本失效（Pro-L 2 公式 bug 就是证据）。

### 3.5 第五层：标定扫描 + 缓存（解决单位/taper/步进，不解决语义）

`fx_scan_params`（`reaper_mcp_server.lua:1751-1813` + `reaper_mcp_shared/plugin_cache.py`）：

1. 对每个参数采样：离散步进参数用 `TrackFX_GetParameterStepCount` 枚举全部档位；连续参数采 9 个点（0, 0.125, …, 1.0）；每个点读 `TrackFX_FormatParamValue` 的显示字符串 + 归一化值，然后**恢复原值**；
2. Python 侧从显示字符串推断 `linear / logarithmic / stepped / unknown` + 单位（`Hz/dB/%/…`）；
3. 结果按插件名缓存成 `plugin_maps/<plugin>.json`，下次直接命中，不再扫。

用途：让模型知道「这个参数是 20 Hz–20 kHz 对数」「这个下拉有 8 个档位」，从而把人类单位换算成 0–1。

> 注意它的自我定位：**「语义已由模型推理解决，扫描只在真正不确定时兜底」**（`docs/ARCHITECTURE.md:43-50`）。这句话比功能本身更有价值——它承认了扫描不是万能答案。

### 3.6 第六层：上下文裁剪与特例数据

- Pro-Q 的 ~500 参数里，未使用的 band 全部过滤；junk 参数过滤；`max_results` 硬上限 2000、默认 300，避免撑爆上下文。
- 已知的写序约束做成代码路径（Used/Enabled 先写）。
- 已知的数值坑写进注释/公式（Pro-L 2 的 30 dB 偏差、Pro-R 不写 Decay、ReaVerbate dry 不能开）。
- 这些「特例」本质是**人工领域知识**，不是统一模型。

### 3.7 对照：另外两个项目的不同答案

**mthines：把「插件知识」做成版本化 Markdown + 角色抽取**

- `reaper/mcp_bridge.lua:327-447`：
  - `analyze_eq_bands`：按名字 pattern 把参数归到 `frequency/gain/q/shape/enabled` 五个**规范角色**，再按 band 汇总；
  - `analyze_compressor`：`threshold / ratio / attack / release / makeup / knee` 的角色 → 名字 pattern 表。
- `analyze_fx` 对 EQ/压缩器返回**压缩后的语义摘要**，避免模型读 500 个参数。
- `knowledge/plugins/{vendor}/{plugin}.md`：人工/模型维护的插件档案，记录**精确参数名**、范围、推荐设置、踩坑；`fx_match` 用于把 REAPER 实际插件名匹配到知识文件；`learn-plugin` 技能可访谈用户或联网研究后写入；`memory-protocol.md` 规定用 `get_fx_parameters` 校验真实参数名后写「param map」，并在每次混音后追加「Learned notes」。
- 局限：知识是**描述性**的（物理单位），写参数仍要模型自己换算成 0–1；没有自动标定；知识库只覆盖 8 个插件（Pro-Q 3/Pro-C 2/Pro-L 2/Helix/ReaEQ 等）。

**bonfire：最小可用 + 显示值二分**

- 只提供 `get_fx_parameters`（index/name/normalized/formatted）与 `set_fx_parameter`（按 index 写 normalized）；没有语义层、没有模糊匹配。
- 但它有一个很实用的通用技巧：`_set_to_display`（`src/reaper_mcp/mastering_tools.py:95-112`）——**用 REAPER 自己的 formatter 做二分**，把「显示值 = 目标」（如 `-0.5 dB`）反解成归一化值，完全不需要知道插件公式。40 次迭代后写到目标显示精度。
- 局限：依赖 `FormatParamValueNormalized`，官方文档注明「only works with FX that support Cockos VST extensions」；且只用于母带 ReaLimit 的两个参数。

---

## 4. 效果评估：哪些是真的，哪些是 demo 光环

### 4.1 真实能力（代码可证）

- 能对**任意** VST2/VST3/CLAP/JS 插件读参数、写归一化值；能按名字/索引批量写；能撤销（每个命令一个 undo 点）。
- 批量/失败可见性做得比多数玩具项目好：逐项报错、不整体中断、preset 写入会回读校验、崩溃用心跳检测提前暴露。
- 对 8 个标定插件，自动混音/母带链路能产出**可听、可复现**的结果（风格档案固定）。
- 工程纪律强：CHANGELOG 记录根因（Pro-Q 3 崩溃、Pro-L 2 公式、preset 静默 no-op、参数名静默丢弃），并配测试/回归；这在 MCP 类玩具项目里少见。

### 4.2 关键局限（决定「能不能拿来当通用方案」）

1. **语义靠模型**：换个模型/换个插件，结果可能完全不同；没有可验证的「参数语义」保证。
2. **模糊首匹配会写错参数**：`Gain`/`Freq`/`Threshold` 这类高频词在多 band/多段插件里天然歧义；它没有「唯一命中才写」的规则。
3. **单位换算仍要模型做**：通用路径只接受 0–1；模型必须知道 taper 或先扫描。`value_text` 写入在 Vit-DAW 是明令禁止的（见 §5），REAPER MCP 也没有可靠的「按显示值写入」通用路径（bonfire 的二分是例外）。
4. **扫描有副作用**：`fx_scan_params` 会把每个参数写一遍再恢复；对正在出声的工程有风险；Pro-Q 3/Pro-C 2 的历史崩溃说明参数写入不是「纯读」操作。
5. **缓存只按插件名**：同一插件不同版本/不同格式（VST2 vs VST3）可能命中错误缓存；项目最初想「提交共享」，后来因版本污染改为 gitignore（CHANGELOG 0.6.9）——这等于承认「跨机器共享标定数据」没做成。
6. **自动引擎是固定 recipe**：35 个风格档案 + 固定 EQ/压缩曲线，不是按素材分析决策；「好听」来自预设经验，不是通用智能。
7. **不可自动化/rack 内部**：诚实不可见。
8. **硬编码公式会错**：Pro-L 2 的旧公式 `0.5 + db/60` 曾把目标上限压低了约 30 dB（limiter 几乎不工作）；公式是人工资产，需要测试与版本回归。

### 4.3 两个需要实测才能下结论的点

- **`TrackFX_GetParameterStepCount` 疑似不存在**：xDarkzx 的 Lua 直接调用 `reaper.TrackFX_GetParameterStepCount(tr, fi, i)`（`reaper_mcp_server.lua:1790`），但 REAPER 7.82 官方 ReaScript 文档（2026-10-18 抓取）检索不到该函数，只有 `TrackFX_GetParameterStepSizes`；ReaWrap 绑定页（2026-01 更新）也没有它。若 API 不存在，该调用在 Lua 里会「call nil」→ 命令失败（被 pcall 捕获），即 `fx_scan_params` 的离散步进路径可能整体不可用。**这是代码阅读发现，未实机验证**。
- **`FormatParamValue` 的适用范围**：官方文档注明 `TrackFX_FormatParamValue` / `FormatParamValueNormalized`「only works with FX that support Cockos VST extensions」。依赖显示字符串的扫描/二分（xDarkzx 扫描、bonfire 二分）对不支持该扩展的插件可能拿不到文本。`TrackFX_GetFormattedParamValue` 没有这条注释，但两者行为差异需要实测。

### 4.4 总评

- **作为「AI 控制 DAW」的产品原型：明显高于玩具水平**，尤其是写序/撤销/失败可见性这些真实工程问题。
- **作为「跨厂商参数统一」的通用答案：不成立**。它把问题拆成「宿主统一访问 + 模型语义 + 少量插件标定 + 扫描兜底」，通用性来自模型，可靠性来自那 8 个标定插件。
- **「效果很好，拿来用」的正确姿势**：借它的**分层与数据设计**；不要借它的**模糊匹配权威性**和**手写公式表**。

---

### 4.5 追问 A：参数名模糊抽象、LLM 推理不出来时，它有解吗？

**没有语义级解法；它提供的是一条显式的「外置」链路，而不是自动消歧。**

证据（xDarkzx）：

1. `fx_scan_params` 的工具说明明确划清边界（`reaper_mcp/tools/fx_tools.py:142-166`）：
   > "It does not explain what a parameter *means* or what a plugin is *for*; reasoning about a plugin's purpose and a vaguely-named control ("Character," "Drive") is something you're already equipped to do from general knowledge... Reach for fx_scan_params only when that reasoning genuinely isn't enough — e.g. an obscure/freeware plugin with no clear labeling."
   即：**抽象命名 = 模型的 general knowledge 问题，不是工具问题**；扫描只给范围/单位/曲线。
2. `docs/ARCHITECTURE.md:43-50` 再次承认：「扫描只解决参数标定，不解决语义；而语义是更常见、更重要的问题」——他们选择的答案是「模型推理已经能处理」。
3. 对未标定插件的官方话术（`instructions/00_core.md:138-151`、`30_mixing.md:17-26`）：
   > "If the user wants another plugin, set it up yourself with `setup_fx_chain` and tell them its settings are approximate."
   即：**回落到通用写路径 + 向用户声明“近似”**，没有校验、没有拒绝。
4. 可用的升级/兜底手段只有：`fx_show_ui`（打开插件 UI 让用户看）、向用户提问、undo、`script_run`（执行任意 ReaScript，需确认，且同样不解决语义）。`setup_fx_chain` 的响应里会带每个参数写后的 `display`，模型可以「写一次→看显示→再调」手工闭环，但没有自动验证目标是否达成。
5. 唯一结构化的「知识型」解法出现在 mthines：未知插件先 **web research + `get_fx_parameters` 读取真实参数名 + 写 Markdown 知识文件**（`memory-protocol.md:76-90` 的 bootstrap），交付给 `learn-plugin` 访谈 / 混音中自动记录「param map / 用户保留的设置 / gotcha」。这能记住抽象参数的真实含义，但它是**人工/模型维护的 per-plugin 知识**，不是通用推理，也不保证新版本仍成立。

结论：遇到「名字抽象到模型也猜不出」的插件，REAPER MCP 的实际行为是**照写 + 声明 approximate + 靠 undo/用户纠正**；没有 fail-closed、没有语义 admission、没有回读验证。对 Vit-DAW 的证据标准而言，这是不可接受的自信。

### 4.6 追问 B：它有没有按效果器类型「分类开发」控制脚本？还是一套设计全部响应？

**底层 FX 控制：基本就是一套通用设计全部响应；上层自动化：有分类，但分类维度不是「效果器控制脚本」。**

底层（通用面，不分类）：
- `fx_get_params` / `setup_fx_chain` / `fx_scan_params` 对任何插件同一套逻辑；没有 EQ/压缩/混响各自的控制脚本或参数语义模块。
- 只有两处窄特例：`build_fx_params` 里按插件名识别 Pro-Q 并过滤 unused band（`reaper_mcp_server.lua:695-738`）；写序规则（数字升序 / 字母序）是为 FabFilter 崩溃加的**通用排序策略**（`CHANGELOG.md:640-651`），不是 per-effect 分类。

上层（有分类，但服务于自动混音/工作流）：
- **插件族分类**：`PluginSuite = FABFILTER / REAPER_STOCK`，两个 profile 类各自实现 `eq_fx_chain_entry` / `compression_fx_chain_entry` / `reverb_fx_chain_entry`（`mix_engine/plugins.py`）。
- **处理类别分类**：`ENGINE_CATEGORIES = (eq, compressor, reverb, limiter)`，每类独立选择插件族（`mix_engine/selection.py:31-67`）；`fx_inventory.py` 另有 9 类插件排名（eq/compressor/limiter/reverb/deesser/gate/saturator/multiband/stereo）。
- **风格/轨道角色分类**：35 个 genre 档案；`profiles_v2.resolve_roles` 用 track 名 alias 解析 kick/snare/vocal/lead_synth 等角色（最长 alias 优先），再按角色下发 EQ/压缩/送/侧链（`mix_engine/profiles_v2.py:203-227`）。
- **工作流分类**：`setup_parallel_compression` / `setup_drum_bus` / `setup_vocal_chain` / `setup_sidechain` / `engine_master` / `setup_effect_bus`，每个是固定 recipe（`tools/pipeline_tools.py`、`tools/sidechain_tools.py`）。
- 代码模块化（26 modules）只是工具组织，不是参数语义分类。

因此准确说法是：**「一个通用控制面 + 一个很小的人工语义岛」**。语义岛只覆盖 2 个插件族 × 4 个类别 × 若干 recipe；岛外插件全部回落到通用面，并按「approximate」处理。mthines 在语义层分类更细（`analyze_fx` 按插件类型抽 EQ/压缩角色、知识库按 category、skills/modes 按工作流、genres 按风格），但它的写参数接口仍是通用归一化接口——分类用于「知识/流程」，不用于「控制脚本分支」。bonfire 则完全不分类。

**为什么这样设计**：上千个插件，per-plugin 控制脚本不可维护；他们用「模型推理 + 少量标定 + 扫描」换广度。代价正是：抽象命名、冷门插件、版本差异不可控。视频里「肥波 + plate 混响」恰好就是那个小语义岛，不能作为「任意第三方插件都能被统一控制」的证据。

---

## 5. 对 Vit-DAW 的对照与可借用点

### 5.1 Vit-DAW 现状（本次只读盘点，代码锚点）

**已有的（比 REAPER MCP 更强的地方）**

- kernel `get_plugin_parameters`：逐参数返回 `id / name / raw value / normalized_value / value_text / min / max / 离散/布尔 / normalized_role / display_group / display_probe（0/25/50/75/100% 文本采样）`；只列 automatable（`VitApp/Source/Core/VitPluginGrabber.cpp:57-93`、`PluginRackControlService.cpp:3213`）。
- 隔离宿主 `pluginprobe`：枚举**全部**参数（含非 automatable），额外给 `unit / default / stable_id / display_domain / 参数面指纹`（`agent/internal/pluginprobe/types.go:45-58`、`adapter.go:245-347`）；但只读（load/snapshot/render/unload）。
- 统一写入口 `set_plugin_param`：raw/normalized/value_text，写后回读 `new_normalised_value + display_text`；批写 `plugin.set_params_batch` 顺序执行、首错 partial_failure、无回滚（`agent/internal/tools/catalog.go:1169-1170`；kernel `handleSetPluginParam:1835`）。
- **typed 族工具已实现「人类单位 + 原子 + 回读 + 失败恢复」**：compressor / limiter / de-esser / reverb / delay / gate / transient / multiband / EQ，接受 dB/ratio/ms/percent/实测显示值/精确枚举标签，失败整体 restore（`catalog.go:1174-1193`）。这比 REAPER MCP 的通用写路径严格得多。
- 角色启发式已存在：`VitPluginTemplateRegistry.cpp:85-152`（threshold→comp_threshold、freq→eq_frequency、mix/dry/wet/blend→common_mix、pan/width/gain 等），但 agent 自标 `weak_local_heuristic`（`context_pack.go:87-91`）。
- 参数 guard：`agent/internal/harness/plugin_param_guard.go`（写前校验 param_id 在快照中）。

**缺的（正好是 REAPER MCP 有、而 Vit-DAW 没有的三件）**

1. **跨厂商的规范参数语义注册表**：现在三套角色并存（kernel `normalized_role`、族拓扑 role、PCA coverage axis），彼此不等价；没有统一的「role + 单位 + 方向 + taper + 枚举 + 置信度 + 证据」记录。模型仍要自己做人类单位→归一化换算，而 `set_plugin_param` 明令不要用 `value_text`（`catalog.go:1170`）。
2. **持久化 per-plugin 参数地图/指纹缓存**：`display_probe` 每次重算不落库；pluginprobe 的参数面指纹只随快照返回；profile 工具已退役；只有内存 param_id 缓存和一份手工 experiment whitelist（`experimentplugins/whitelist.go`、`plugin_profile_retirement_test.go`）。
3. **通用事务写**：单参写只校验 param_id 在快照中、不强制回读；批写无回滚；atomic+readback+restore 只在 typed 族且受 PCA/白名单限制。未识别插件没有「人类单位 + 回读 + 失败恢复」的通用路径。

### 5.2 建议落地的 3 张卡（只写建议，不擅自发卡）

**卡 A：规范参数语义注册表 v0（只读、低风险、先做）**

- 在现有 `ParameterInfo`/`SurfaceSnapshot` 之上，把三套 role 合并成一个 `ParamSemantics` 记录：`role（taxonomy 枚举） + unit + direction + taper/domain + enum_labels + confidence + evidence + source（kernel/profile/scan）`。
- 规则：只在**唯一命中 + 置信度达标**时对外提供「role→param_id」建议；歧义返回候选列表，不自动写。
- 复用现成资产：`display_probe` 文本、`pluginprobe` 的 `display_domain/stable_id`、C++ 角色启发式、`plugin_grabber` 的 EQ/comp/reverb 拓扑证据。
- 验收：对 FabFilter Pro-Q 3 / Pro-C 2 / Valhalla 某插件 / REAPER 自带插件，输出 role 绑定 + evidence；对歧义参数显式 fail-closed。
- 参考：mthines `analyze_eq_bands/analyze_compressor`（角色 pattern 表）、xDarkzx 抽象混音参数。

**卡 B：per-plugin 参数地图缓存（结构化、可失效）**

- key = `plugin identity + version + 参数面指纹`（Vit-DAW 已有 fingerprint，别学 REAPER MCP 只用插件名）；value = 参数的 `id/name/unit/min/max/step/enum labels/display samples/role bindings`。
- 生成：优先 kernel `get_plugin_parameters` 的 display_probe；必要时 pluginprobe 全参数补全；写入前不改变插件状态（只读优先）。
- 失效：指纹变化即失效；旧记录明确不兼容；缓存不写入源码/用户工程（temp 或用户级目录）。
- 验收：同一插件二次读取不再重算；版本/格式变化时明确 stale，不静默复用；旧记录可加载性测试。
- 参考：xDarkzx `plugin_maps`（但修掉「按名 key」「想提交共享后被版本污染」两个坑）、mthines knowledge 文件。

**卡 C：人类单位通用写 + 回读校验（按需写路径）**

- 给未识别插件提供 `set_param_semantic(plugin_id, param_id_or_role, target {value, unit})`：用卡 B 的域信息把人类单位反解为归一化（曲线拟合/分段插值/枚举精确标签；必要时用 display-probe 二分，类似 bonfire `_set_to_display`），写后强制回读并比对显示值。
- 失败必须 restore（照抄 typed 工具的事务语义），歧义/无域/回读不符一律拒绝。
- 验收：FabFilter Pro-Q 3 频率/增益、Pro-C 2 threshold/attack、一个 reverb 的 decay/wet，都能「给人类单位 → 回读显示值一致」；失败路径恢复原值。
- 参考：typed 族工具（已有事务实现）+ bonfire 的显示值二分 + xDarkzx 的扫描缓存（仅作兜底）。

**不建议照搬**

- 硬编码公式表（Pro-Q/Pro-L 这类）作为长期资产：应以「每插件地图 + 指纹失效 + 回归测试」替代；公式只做 seed。
- 纯模糊名匹配作为写路径：至少要求唯一命中，否则 fail-closed。
- 自动/隐式扫描：写参数有副作用/崩溃史，必须显式触发。
- 把「模型能读懂名字」当成可依赖能力：Vit-DAW 的 PCA/证据标准要求更强的确定性。

---

### 5.3 追问 C：他们的「白名单 / 版本化文档」和 Vit 的 VPS、现状是什么关系？

**先纠正一个容易混淆的点：Vit 当年做的不是「几个公式」，而是一份 344 行的可执行 Markdown 契约。**

历史文件 `docs/VPS_REFERENCE_Q10_STEREO.md`（commit `5d412663` 加入，`8aa2e439` 随通用压缩器控制基础一并删除）里明确写着：

- `schema: vps/eq/v1`，自述「本文件是统一执行接口（executor）的契约范本。任何满足此格式的 VPS 文档，执行器保证能按其描述控制对应效果器」；
- `topology_generation`：参数结构哈希指纹，插件更新导致结构变化时执行器拒绝使用并要求重新生成（防 stale 写错参数）；
- `channel_contract: shared/mirrored`、`addressing: resident/allocatable`；
- 每个 section/role 的 `param_id`、物理域（unit/min/max）、**5 点校准曲线**（normalized↔physical）、`value_law`、枚举 `normalized/label` 映射（含 `[MEASURE]` 待实测标记）、`reachable_shapes`；
- 激活语义 `activation_kind: explicit_toggle / frequency_sentinel / always_active`，且明确「activation 永远最后写，其他所有 role 先写」；
- 证据来源（参数扫描工件、生产烟测、验收证据）。

对比之下：

| 维度 | Vit 旧 VPS（已退役） | Vit 现状 | xDarkzx 白名单 | mthines 版本化 md |
|---|---|---|---|---|
| 身份/版本绑定 | `topology_generation` + 指纹，失配拒绝 | 每次 live inspect + generation-scoped `control_ref` | 插件名/家族名匹配，无失配检测 | `fx_match` 名字匹配，无版本绑定 |
| 参数契约 | 344 行 schema：role/param_id/curve/enum/activation | 运行时 topology 推断 + 角色绑定 | 8 个插件、代码内公式（如 Pro-Q 3 `10*3000^v`） | 参数表 + 推荐设置（不可执行） |
| 校验/证据 | conformance + Credential + 回读/回滚 | PCA v2 strong receipt：两次稳定 inspect、typed 可逆控制、原子回读、完整快照相等、零 LLM 调用 | 无（人工公式，错了靠 undo） | 无（模型自述 + 用户确认） |
| 失效检测 | fingerprint 变化 → stale/revoked，不可自动恢复 | 每次重新 inspect，旧 generation 拒绝 | 无 | 无（笔记可能过期） |
| 覆盖方式 | 逐插件作者化（成本高，已退役） | 11 个处理器族、identity-free，按 live surface 证明 | 8 个产品 | 任意插件，但只是知识 |
| 运行时消费 | 执行器直接读契约 | 执行器 + `plugin_grabber.explain_controls` 上下文包 | 执行器 | LLM 读 md |
| 维护成本 | 高 → 退役 | 中（识别器/证据 gate） | 低但脆 | 低但弱 |

结论：

1. **xDarkzx 的白名单 = 我们已退役的 VPS/Profile 映射层的极简版**，细节少一个数量级（没有指纹、通道契约、寻址、校准曲线、枚举标签验证、回滚 conformance，只覆盖 8 个产品）。
2. **mthines 的版本化 md = VPS 作者化流程的「文档外壳」**（per-plugin 文档、learn/recall、上下文里的用户确认），但缺了 VPS 的执行契约与证据 gate；它是 advisory memory，不是控制权威。
3. **Vit 现状不是白名单**：它不保存 per-plugin 参数映射，而是每次从 live surface 证明拓扑（身份无关），再用 typed 事务执行；PCA v2 的证据 gate 比旧 VPS 更严。就「控制契约的细致程度/可验证性」而言：**Vit 现状 ≥ Vit 旧 VPS ≫ xDarkzx > mthines**。
4. 退役原因不是「文档写得不够细」，而是控制权威从「保存的映射/凭证」换成「实时确定性拓扑」（commit `8aa2e439` 原文：*Retire SPAL, VPS, and learned profile control authority in favor of live deterministic topology.*）。ADR 明确：Profile/VPS 不得作为执行权威、参数映射来源或验证凭证，现状保留的是 live observation + typed tools + 通用归一化 fallback（`docs/SPAL_VPS_RETIREMENT_ADR.md:9-27`）。
5. 但他们的思路里有**两点 Vit 现状确实没有**，值得吸收（不是白名单本身）：
   - **运行时消费的 advisory memory loop**：mthines 的「插件档案 + 修正即记录 + seen_count/过期/合并/矛盾保留」把一个会话里学到的语义（模糊参数名含义、踩坑、用户认可的设置）变成下次可用的知识；Vit 目前只有设计文档（`COMPRESSOR_REFERENCE_PRO_C_2.md` 这类 reference exemplar，明确不是 VPS/mapping，见其开头声明），没有运行时记忆。
   - **永不空手的产品级降级策略**：未标定插件永远有一条「通用路径 + 明确 approximate」的答复；Vit 的 typed 路径在拓扑不成立时 fail-closed（正确），但用户侧缺少「能做什么/近似到什么程度」的显式说明与知识承载。

**如果要吸收，建议作为独立卡 D（advisory，非权威）**，边界照 ADR 写死：

- 内容：per-plugin（按 identity/version/参数面指纹限定）的**语义提示**（模糊参数名的可能含义）、**gotcha**（写序/前置条件/非有限端点/枚举陷阱/通道歧义）、**用户认可的设置及上下文**；不存可执行参数映射、不存 credential、不产生可调度能力。
- 信任标签沿用 VPS Forge 当年自己的模型（`observed / user-confirmed / agent-inferred / unsupported`），并明确 notes 不能升级为 authority。
- 运行时只进入 planner/LLM 上下文或只读工具；执行仍必须走 live inspect + typed executor；冲突时 live 优先；加测试断言「notes 不能新增任何可调用控制路径」（照 `plugin_profile_retirement_test.go` 的写法）。
- 记忆机制借 mthines：修正点立即写、带上下文标签、`seen_count`、约 90 天过期、定期 consolidate、矛盾保留不覆盖；最终报告说明本次是否写入。
- 一个高价值条目示例：**写序冲突**。xDarkzx 实测 Pro-Q 3/Pro-C 2 必须 `Used/Enabled` 先写（否则崩溃），而 Vit `plugin_grabber.apply_eq_edits` 的契约是「activation writes run last」（`catalog.go:1192`）。两者未必矛盾（主机/插件/版本差异），但说明写序是 per-plugin gotcha，不能定一条通用规则；这类条目正适合放在 advisory notes，由 typed executor 在规划时参考、live 证据最终裁决。

### 5.4 追问 D：记忆层与降级策略在 Vit 现有架构里的落位（会话结论）

**记忆层：属于 harness/observation 线的 D14，不应在插件控制层新开一套。**

`docs/AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md:147-162` 已规划 D14 Memory 系统，且把「三种记忆三种键」写死：

| 记忆类型 | 聚合键 | 协议 |
|---|---|---|
| 用户偏好 | 按「人」（跨设备同步） | 质询确权（D13） |
| 客观经验（**插件 bug、流程踩坑**） | 按「环境」（机器+版本指纹；`.local` 同构，不跨设备） | 环境指纹失配 → 降权待验证 → 复验通过才退役留痕 |
| 工程事实 | 按「工程」（工程账本） | append-only |

这正是我们讨论的「插件 notes/gotcha」的正确归属：**客观经验类**，键是环境指纹而不是插件名；机制形态是 append-only 原始流 + 蒸馏文档（markdown 蒸馏产物、SQLite+FTS5、consolidation loop），memory 条目已进 ref schema 对象类型轴（D2 预留），且排程上是**收尾拼装（D14 全量）**。`AUDITION_AB_AND_USER_JUDGMENT_V1.md:261-277` 也明确：raw judgment evidence 是 source of truth，未来蒸馏偏好必须可回链、主观偏好必须确权，不能把一次 A/B 选择变成永久偏好。

结论：
- **机制层（存储/检索/失效/巩固/画像）归 D14**，现在不要建并行的 plugin memory store；
- 现在可做的是**插件域的 entry schema + 写入/消费策略**（哪些算客观经验、trust 标签、环境键、过期/复验、不得成为执行权威），作为 D14 的域内规格，避免届时返工；
- 若想短期见效，只做**会话内、非持久**的 notes（或纯只读展示），不落持久层。

**降级策略：有用，但形式应从「文字拒绝」升级为「结构化、确定性的 remediation」；不是让模型自由发挥。**

现状（2026-10-18）：
- 拒绝路径只有 L0：`freeStateCapabilityBlockedReply` 输出「未通过准入门 + 没有修改/不会自动重试」+ failed gates（`goalrunner_chat.go:2104-2133`）；C2 也是类似一句话边界（`c2_dynamic_control_runtime.go:931-934`）。有 `StopReason`/`mutation_performed=false`，但没有下一步选项。
- prompt 明确禁止模型自行发明 fallback：*"Domains or action kinds outside that list are unsupported: return capability_blocked instead of inventing a fallback."*（`agentloop/ccb_model_prompt.go:153`）。这是对的方向：**边界判定必须是确定性的，模型只能解释/选择**。
- 但仓库已有可复用的零件：
  - `capabilityruntime.Readiness` 已带 `BlockedBy` + `Remediation`，且注释写死「readiness 评估确定性、绝不调用 LLM」（`readiness.go:17-70`）；
  - `plugin_recommendation.v1` 已有 recommended/alternative + tradeoff/confidence（`plugin_recommendation.go:297-302`）；
  - `semantic_treatment_strategy.v1` 已有 `decision_mode=choice_required`、`next_planner` 可取 `capability_boundary` 或 `native`（`semantic_treatment_strategy.go:695-711`）；
  - `processor_certification_entry.go` 已有产品内认证流程（临时轨、apply/readback/restore、consent token、进度阶段）；
  - PCA 拒绝有稳定 `pca_load_gate:` reason，且只拦 agent 发起的 load，用户手动加载路径不变（`PCA_PLUGIN_LOAD_GATE_V1.md:21-34`）。

建议的 **capability-boundary ladder**（由确定性 resolver 生成，模型只解释/协商）：

| 级别 | 动作 | 现状可复用件 | 风险 |
|---|---|---|---|
| L0 | 明确拒绝 + 证据原因（no mutation） | 已有 | 无 |
| L1 | 换用已准入的同族/相邻族处理器（coverage 满足） | `Readiness.Remediation` + `plugin_recommendation` | 低（须重跑 admission） |
| L2 | native/stock 处理器路径 | `next_planner: native` | 低 |
| L3 | 手动指引：打开插件 UI + live 参数/范围/预期 | `explain_controls` context pack + `open_plugin_ui` | 低（用户执行） |
| L4 | 引导走 PCA 认证/复核流程（用户确认后纳入准入） | `processor_certification_entry.go` | 中（用户确认） |
| L5 | 默认关闭的实验性近似写：preimage/readback/restore + `unverified` 标签 | 现有事务/回滚设施 | **高**，必须独立 risk class |

优先级：**先做 L0→L3（纯 agent 侧、无内核风险、可单测）**，L4 接现有流程，L5 最后且默认关闭。不要从 L5 开始——那正是 REAPER MCP「照写 + approximate」的失败模式。

为什么「模型足够聪明」仍需要它：模型不知道环境事实（装了什么、PCA 状态、coverage、risk ceiling），不同模型/会话行为不一致，纯文字回复不可测试也不可执行；把边界做成结构化对象后，模型智能是**放大器**而不是**来源**。可测试的边界对象（reason codes + evidence refs + mutation_performed=false + next actions）也才能进入任务状态、后续轮次和 D14 memory，而不是一次性的聊天话术。

---

## 6. 证据等级与未验证项

- 本报告的证据类型：外部仓库源码与文档（固定 commit）、REAPER 官方文档、web 搜索结果。**没有**真实 REAPER 运行、没有 Vit-DAW 真实栈烟测、没有播放 demo 视频。
- 未验证：视频与 xDarkzx 的对应关系；`TrackFX_GetParameterStepCount` 是否真实存在/是否版本新增；`FormatParamValue` 对哪些插件可用；xDarkzx 当前版本在真实 REAPER 上 `fx_scan_params` 是否成功；mthines 知识库覆盖度与实际写参表现。
- 若要升级为可决策证据：需要 1 次真实 REAPER（或 Vit-DAW pluginprobe）参数面复现实验，至少覆盖 Pro-Q 3 / Pro-C 2 / 一个 Valhalla reverb / REAPER 自带插件，记录「读参数→role→写→回读」全过程与失败分类。

---

## 附：主要代码锚点

**xDarkzx/Reaper-MCP @ `1c9863c`**
- Lua 桥与工具：`reaper_scripts/reaper_mcp_server.lua`（`fx_get_params:1704`、`fx_set_param_by_name:1727`、`fx_scan_params:1751`、`build_fx_params:687`、`setup_fx_chain 写参:4360`）
- 参数地图缓存：`reaper_mcp_shared/plugin_cache.py:16-125`
- 自动混音适配器：`reaper_mcp/mix_engine/plugins.py:54-96`（Pro-Q 3 标定）、`:245-408`（ReaVerbate/Pro-R/Pro-C 2）
- 族选择：`reaper_mcp/mix_engine/selection.py:1-67`
- 插件排名（含 Valhalla Plate / Abbey Road Plates）：`reaper_mcp/mix_engine/fx_inventory.py:170-216`
- 架构与「扫描不解决语义」：`docs/ARCHITECTURE.md:43-50`
- 写序/崩溃记录：`CHANGELOG.md:397-411`、`:640-651`；缓存 gitignore：`:586-590`

**mthines/reaper-mcp @ `4a01d1e`**
- 角色抽取：`reaper/mcp_bridge.lua:327-447`、`analyze_fx:727-831`
- 参数过滤/分页：`:649-725`
- 知识库与记忆：`knowledge/reference/memory-protocol.md:70-116`、`.claude/skills/learn-plugin/SKILL.md`、`knowledge/plugins/fabfilter/pro-q-3.md`
- 插件解析：`apps/reaper-mix-agent/src/plugin-resolver.ts`

**bonfire-systems/reaper-mcp @ `4dcc8f8`**
- 参数读写：`src/reaper_mcp/fx_tools.py:20-93`
- 显示值二分：`src/reaper_mcp/mastering_tools.py:95-112`
- 桥：python-reapy distant API（README）

**REAPER API**
- <https://www.reaper.fm/sdk/reascript/reascripthelp.html>（v7.82）
- <https://conceptual-machines.github.io/ReaWrap/modules/track_fx.html>

**Vit-DAW 内部锚点**
- `agent/internal/pluginprobe/types.go:45-58`、`adapter.go:245-347`
- `agent/internal/tools/catalog.go:1169-1193`
- `VitApp/Source/Core/VitPluginTemplateRegistry.cpp:85-152`
- `agent/internal/harness/plugin_param_guard.go`
- `agent/internal/workflows/plugingrabber/`（compressor/limiter/de-esser/multiband/reverb/delay/gate/eq 拓扑）
