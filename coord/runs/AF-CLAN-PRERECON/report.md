# A-F 族 ablation 预勘察报告：B2/B3/C1 入口要素预扫（264 方法学应用）

- 编制：免费车道量产执行子代理 / 2026-11-07 / 纯读档零 LLM 零栈零代码改动
- 被测基线：HEAD `0401a6d8`（工作树已有改动均为他卡工件，本卡仅新增本目录两个文件；领取时 `git status --short` 顶部项=VitApp Workspace 运行态+UX-AB 工件，与本卡文件域不相交）
- 依据：264 B4 裁定 §三（coord/rulings/2026-11-06-B4-ABLATION-1-pass.md——「入口命运由话术要素明确度决定，不由钩决定」）+ 257 拆减矩阵 §三/§五/§六（coord/reports/2026-11-05-af-family-health.md）+ 263 判定（coord/reports/2026-11-06-c4-ablation-1-ablation-verdict.md §二 + coord/rulings/2026-11-05-M3-GROUP3-DISCRIMINATE-1-pass.md）
- 性质：**静态预勘察，不是运行结论**。所有「预判」均标注待 ablation run 验证；本卡全程零 LLM 调用（含仲裁器面）。

---

## 一、入口链公共结构（三族同构，锚点级）

裸话术（不含词表词）从 chat 入口到执行链要过**两级门**，锚点如下（行号基于 HEAD 0401a6d8）：

```
chat POST /agent/chat
  └─ server.go:2747  handleCapabilityRuntimeCanary        ← 第一级：词表层（确定性零 LLM）
       └─ capability_runtime_canary.go:37 resolveCapabilityOwner
            └─ capability_owner.go:49 → :225 inferProjectAwareCapability
                 词表命中 → B2/B3/C1 各族 canary runtime（第二级固定面）
                 词表未中 → return "" → :42-43 返回 false → 穿透
  └─ goalrunner_chat.go:261  planSemanticEntry             ← 第二级：语义入口仲裁器（小 LLM 分类面）
       ├─ semantic_entry.go:46   semanticEntryMinimumRouteConfidence = 0.50
       ├─ semantic_entry.go:267  conf < 0.50 强制 route=unresolved（validate 层）
       ├─ semantic_entry.go:389  semanticEntryUnresolvedReply 确定性模板拒绝
       └─ conf ≥ 0.50 → route ∈ {discussion, observation, explicit_control,
            open_semantic, other} → 按 route 进 message loop / free-state 等链
```

要点：

1. **词表先于仲裁器**。词表命中的话术根本不进仲裁器（canary 直接 handled）；词表拆除（ablation 钩）后话术全部改道仲裁器——264 钩的两件套（词表短路+dispatch 分支禁用，capability_owner.go:219/221-223 + capability_runtime_canary.go:116-118）即此结构，B2/B3/C1 复用时注入点对应 capability_owner.go:227-235/236-240/248-252 三段词表 + capability_runtime_canary.go:109/:119/:136 三个分支。
2. **守卫在词表层之内**（capability_owner.go:53-58）：B4/C1 反门只在 `inferred` 已命中该族时才触发。**裸话术词表未中 → 守卫根本不执行**——C1 的「全工程/整首」守卫词对裸话术不构成额外门槛。
3. **仲裁器判据面**（semantic_entry.go:72-103 system prompt）：只分类交互意图，明确禁止推断处理族/插件/观察视图/控制器。route 六值 + target_scope 三值（none/current_selection/project_context，可选值由 context 选择面供给 :105-122）+ control_mode + user_authorization + confidence。
4. **仲裁器是小 LLM 面**（planSemanticEntry 真实调 LLM，budget 12s :131，fail-open 有容量事实短路 :272-278）——同一话术的 conf 天然有 run 间波动（263 组 3 实测三样本 0.18-0.35 浮动）。

## 二、conf 判据要素（263/264 对照归纳）

| 对照 | 话术 | conf | 仲裁器拒绝/放行理由（工件逐字） | 要素读法 |
|---|---|---|---|---|
| 263 组 3 r1 | 裸时间窗族 | **0.22** | "Insufficient context to determine the requested DAW interaction or **target**." | 缺交互意图+目标 |
| 263 组 3 r2 | 裸时间窗族 | **0.22** | "Too ambiguous to determine whether this is playback, automation, acoustic change…**no target is identified**." | 路由歧义+无目标 |
| 263 组 3 r3 | 30 秒之后动起来 | **0.32** | "lacks a clear DAW **action**, **target**, or intended **outcome**." | 缺动作+目标+预期结果 |
| 264 放行 | 分析一下当前工程的低频关系 | ≥0.50 | 目标（低频关系）/动作（分析）/域（当前工程）三要素明确 → message loop → mix.observe → done（5/5） | 三要素齐 → 放行 |

**判据逻辑（264 方法学，本报告预判框架）**：conf≥0.50 需要——
- **动作要素**：请求的交互类型可被安全归入六 route 之一（观察/控制/开放声学目标/讨论）；
- **目标要素**：作用对象可识别（轨/关系/频段/声场等，r3 逐字要求 "target"）；
- **域要素**：作用范围与 available_target_scopes 匹配（全工程/选中面，semantic_entry.go:98）；
- **预期结果要素**（r3 逐字 "intended outcome"）：泛化动词+无目标=「动起来」形态必拦。

## 三、B2 静态平衡（static_mix.static_balance.v0）

- **① 固定路由词表**（capability_owner.go:227-235，12+ 词）：`static_mix.static_balance` / `static_mix.focus_position` / `b2` / `b5` / 「静态平衡」「静态音量」「推子平衡」「静态主次」「建立主次」「主次关系」「核心元素定位」「突出主唱」/ `static balance` / `focus position`。（257 报告行号 :212 系 15ef1050 基线，偏移=98fc4ca4 插入 B4 钩注释块。）
- **② canary dispatch 入口**：capability_runtime_canary.go:33 `handleCapabilityRuntimeCanary` → :136 `capabilityID != staticBalanceCapabilityID` 分流后 B2 主路径（:139 起要求 kernel 在线 → VSPStateSnapshot → acquireCapabilityCanaryB2Context :278 → CCB → Frozen ActionSet → Execution Coordinator → VSP readback，257 §三 B2 行锚点一致）。
- **③ 裸话术判据面**：B2 语义核心=电平/主次关系——该族裸说法天然携带「关系型目标」（谁压谁/谁突出），与 264 放行话术同构度高。

### B2 裸话术预判表（均不含上述任何词表词，逐词核对）

| # | 裸话术 | 要素（动作/目标/域） | 预判 | 预判理由（**预判，待 ablation run 验证**） |
|---|---|---|---|---|
| B2-1 | 帮我听一下各轨音量合不合理 | 听/音量合理性/全工程 | **放行**（observation） | 与 264 同构：观察动词明确+目标可识别+域 project_context |
| B2-2 | 人声被伴奏盖住了，帮我把人声提出来一点 | 提出/人声-伴奏电平关系/全工程 | **放行**（open_semantic） | 关系型目标+开放动作，264 同型 |
| B2-3 | 把整体音量平衡一下 | 平衡/整体音量/整体 | **放行·边缘** | 动作明确但剂量开放——放行后 harness 自主剂量，257 §五 B2 行缺口（主次判据指引）在此层显形 |
| B2-4 | 感觉混音有点乱，你看着办 | 看着办/「乱」感受/整体 | **边缘·波动区** | 「乱」无声学维度映射+「看着办」开放——discussion vs open_semantic 之间 conf 预计 0.4-0.6 浮动；不过=251 形态（unresolved 拦截），**不构成 harness 缺口证据** |
| B2-5 | 鼓和贝斯谁该大一点 | 咨询/两者电平/两者 | **放行但 route=discussion** | discussion 高置信；**不进执行链**——ablation 判据若期待 tool_chain，此样本=设计内「假阴性」，verdict 归属须预先定义 |

## 四、B3 声像布局（static_mix.pan_layout.v0）

- **① 固定路由词表**（capability_owner.go:236-240，仅 5 词）：`static_mix.pan_layout` / `b3` / 「声像布局」/ `pan layout` / `panning layout`。——三族中最薄（词法后继退化梯度 257 §三-2：B2 12+ > B3/B4/C1 4-8 > C2/C3 仅显式标记）。
- **② canary dispatch 入口**：capability_runtime_canary.go:109-110 → `handlePanLayoutRuntimeCanary`（pan_layout_runtime_canary.go:21：kernel snapshot → session/proposal 边界 → acquirePanLayoutCanaryContext :66 CCB 只读上下文 → projectcut → Frozen plan → 授权链）。
- **③ 裸话术判据面**：声像语义天然含「位置/方向」目标词（左中右/宽窄/前后），动作+域易自明。

### B3 裸话术预判表

| # | 裸话术 | 要素 | 预判 | 理由（**预判，待 ablation run 验证**） |
|---|---|---|---|---|
| B3-1 | 吉他太靠边了，把它往中间挪一点 | 挪/吉他声像/具名轨 | **放行**（explicit_control） | 动作+目标+域三要素全明确，explicit_control 判据直配 |
| B3-2 | 整体声场再宽一点 | 加宽/声场宽度/整体 | **放行·边缘** | 「宽」为明确声学目标词；域=整体明确（不触 C3 入口——C3 仅显式标记） |
| B3-3 | 帮我把各个乐器的位置摆得更清楚 | 摆/位置清晰度/各乐器 | **放行·边缘** | 动作+域明确，「清楚」感受性——open_semantic 归类应可达 |
| B3-4 | 声音都挤在中间了 | （无动作请求） | **边缘·偏拦或 observation** | 纯陈述无请求词——分类面在 observation/discussion/other 间摇摆；即便放行也不进执行链（同 B2-5 假阴性逻辑） |
| B3-5 | 左右声道感觉不平衡，帮我检查一下 | 检查/左右平衡/整体 | **放行**（observation） | 观察动词+关系目标+域明确，264 同型 |

## 五、C1 频段清理（fine_mix.frequency_cleanup.v1）

- **① 固定路由词表**（capability_owner.go:248-252，8 词）+ **双门槛守卫**：词表=`fine_mix.frequency_cleanup` / `c1` / `frequency cleanup` / `frequency clean-up` / 「频段清理」「频率清理」「频谱清理」「全工程eq清理」；守卫=capability_owner.go:56-58（inferred 命中后，若无显式标记且选中轨上下文 → 反门退回）+ :112-115 `explicitC1InvocationMarker`（`fine_mix.frequency_cleanup` / `c1` / `full project` / `whole project` / 「全工程」「整首」）。**守卫只作用于词表已命中的话术；裸话术穿透后守卫不执行**（见 §一-2）。
- **② canary dispatch 入口**：capability_runtime_canary.go:119-120 → `handleFrequencyCleanupRuntime`（c1_frequency_cleanup_runtime.go:57，613 行固定面：kernel snapshot → acquireFrequencyCleanupContext :89 → projectcut → ShadowC1FromVSP :106 → envelope → planC1Treatment（**固定面内部 LLM decode**，c1_frequency_cleanup_planner.go:23，treatment_plan JSON 一次修复重试）→ EQ batch → proposal/授权/verifier :471 链）。
- **③ 裸话术判据面**：「清理」类动作词明确；但**入口放行 ≠ 能力复现**——裸话术放行后进 message loop 的 semantic_effect EQ 闭环（选中/单轨导向）+ full-access 指引，「全工程频段清理计划」语义（defer non-static 判据现硬编码在 planner decode，257 §五 C1 行）无 harness 等价物。

### C1 裸话术预判表

| # | 裸话术 | 要素 | 预判 | 理由（**预判，待 ablation run 验证**） |
|---|---|---|---|---|
| C1-1 | 整首歌听起来浑浊，帮我清理一下 | 清理/浑浊/整首歌 | **放行**（explicit_control 或 open_semantic） | 含「整首」但不触守卫（词表未中守卫不执行，§一-2）；三要素齐 |
| C1-2 | 低频有点糊，帮我减一点 | 减/低频糊/未指明 | **边缘** | 动作+目标明确；域悬空——有选中面→current_selection 明确→放行；无选中→target_scope=none（semantic_entry.go:98）→conf 预计 0.4-0.6 波动。「低频糊」不含 B4 词表词，不进 B4 |
| C1-3 | 把没必要的中低频能量清掉，让混音更干净 | 清掉/中低频能量/混音整体 | **放行** | 动作+频段目标+域全明确 |
| C1-4 | 帮我看看哪些频率堆太多了 | 看/频率堆积/全工程 | **放行**（observation） | 观察动词+目标+域明确；harness 侧等价=已暴露观察工具（frequency_relationship/ccb 面，257 §四.1） |
| C1-5 | 做一下频率管理 | 做（泛化）/频率管理（抽象）/未指明 | **预判拦**（unresolved，conf 预计 0.3-0.45） | 251「动起来」同型：泛化动词+抽象目标+域悬空——三要素齐缺 |

## 六、汇总表：三族 × 四维

| 族 | 词表依赖度 | 入口要素预判（裸话术面） | ablation 风险 | 拆减建议优先级 |
|---|---|---|---|---|
| **B2** 静态平衡 | **高**（12+ 词，三族最重词法后继；拆钩后改道话术量最大） | 裸说法天然携带关系型目标 → 放行率预判高；5 条中 1 条波动区（B2-4）、1 条 discussion 假阴性形态（B2-5） | **中**：入口面风险低；主风险=复现判据面（主次关系 foreground/anchors/support 判据指引缺，257 §五 B2 行）+ 剂量开放样本的验证断言缺 | **中后**（B4 之后、C1 之前） |
| **B3** 声像布局 | **低**（5 词，三族最薄——日常话术本就大量走仲裁器，词表拆除的改道面最小） | 声像说法天然含位置/方向/域 → 放行率预判三族最高；5 条中 1 条无动作请求陈述（B3-4） | **中低**：入口面最干净；缺口与 B2 同族（声像布局判据指引缺）但面更小；canary 分支独立 handler 注入点清晰 | **高**（下一拆候选） |
| **C1** 频段清理 | **高+双门槛**（词表+守卫；守卫词「全工程/整首」本身是高明确度域词） | 「清理」动作词明确 → 裸话术放行率预判中高；但**放行后进的是 EQ 闭环/观察链，非全工程清理计划语义**（C1-1 至 C1-4 均此形态）；C1-5 泛化形态预判拦 | **高**：①入口放行≠能力复现（defer non-static 判据硬编码在 planner decode+cleanup 验证器无 harness 等价物，257 §五 C1 行）→判据面最易写出假阳/假阴；②固定面 613 行+面内 LLM decode 双重依赖 | **低**（最后拆；与 C2 同批或其后；先补判据指引+验证器） |

**对称观察**：三族词表路径话术（如「建立主次关系」「声像布局」「全工程频段清理」）本身三要素明确度都高——按 264 方法学，**钩开启后词表话术改道仲裁器的放行率预判普遍高**（264 B4 canonical 话术 5/5 放行先例）；族间差异集中在**裸话术变体面**与**放行后的能力复现面**。B4 残差②（canonical 同款×5、变体面未扫）在三族同样适用：本报告的裸话术表即为变体面的预扫输入。

## 七、建议：下一张 ablation 卡候选族与 run 设计注意

1. **候选族排序建议：B3 → B2 → C1**（结合 257 §七 原排序 C4✓→B4✓ 之后的剩余序列）。B3 首拆的预勘察理由：词表最薄（改道面最小）+入口要素预判最干净+canary 分支独立（pan_layout_runtime_canary.go:21 单文件 handler，钩注入点=B3 词表段 capability_owner.go:236-240 + dispatch 分支 capability_runtime_canary.go:109）。**此为静态预判排序，最终由主管裁定**。
2. **钩形态**：复用 264 两件套模式（词表短路+dispatch 分支禁用+配对三态单测），264 ruling §四-1 已确立该模式供 B2/B3/C1 复用。
3. **verdict 设计注意（263/264 方法学直接应用）**：
   - 每族样本面=裸话术×N+词表话术×N 两腿（覆盖 B4 残差②变体面缺口）；
   - **discussion/observation 放行 ≠ 执行复现**：B2-5/B3-4 形态样本若被判据期待 tool_chain 会产生假阴性——verdict 词表须按 route 语义分列（264 判据面 route-aware 化）；
   - 波动区样本（B2-4/C1-2/C1-5 形态）conf 预计 0.3-0.6：run 中同一话术跨轮 verdict 分叉属预期内，须**跑前**成文预判归属规则——unresolved 拦截=诚实边界/入口判据事实，不记 harness 缺口（263 §二-3 判别事实口径+264 §三推论）；
   - C1 专项：判据须区分「入口放行」与「全工程清理计划语义复现」两级，防假阳。
4. **预判验证顺序建议**：若采纳 B3 首拆，可用本报告 B3 表 5 条作 RUNBOOK 话术基样板（跑前把预判列转成预期 verdict 列，分歧样本单列 probe）。

## 八、诚实边界

- 本报告全部「预判」为**静态预判**：基于源码判据面（semantic_entry.go prompt+阈值、词表子串匹配）+263/264 已归档运行证据的逻辑外推。仲裁器是 LLM 分类小面，conf 有 run 间波动（263 实测同族 0.18-0.35），预判给方向与要素归因，**不给 conf 点估计，不冒充运行结论**。
- 前提假设：预判期间（HEAD 0401a6d8）读到的词表/阈值/prompt 与 ablation run 时一致；若中间合入改动语义入口面，须重扫。
- 263/264 工件中的 conf/rejection 逐字引用未在本卡重新运行核对，以原工件为准（工件路径见 §二表）。
- 本卡零 LLM 调用、零栈占用、零代码/脚本改动；仅写本目录 PROGRESS.md 与 report.md。

## 附：关键文件锚点索引

| 面 | 文件:行（HEAD 0401a6d8） |
|---|---|
| B2 词表 | agent/internal/chat/capability_owner.go:227-235 |
| B3 词表 | agent/internal/chat/capability_owner.go:236-240 |
| C1 词表+守卫 | agent/internal/chat/capability_owner.go:248-252、:56-58、:112-115 |
| B4 钩（复用模板） | agent/internal/chat/capability_owner.go:211-223、capability_runtime_canary.go:112-118 |
| canary 总入口 | agent/internal/chat/capability_runtime_canary.go:33；调用点 server.go:2747 |
| B3/B2/C1 dispatch 分支 | capability_runtime_canary.go:109 / :136 / :119 |
| B3 handler | agent/internal/chat/pan_layout_runtime_canary.go:21 |
| C1 handler+planner | agent/internal/chat/c1_frequency_cleanup_runtime.go:57；c1_frequency_cleanup_planner.go:23 |
| 仲裁器阈值/validate/模板 | agent/internal/chat/semantic_entry.go:46 / :267 / :389 |
| 仲裁器 prompt 判据面 | agent/internal/chat/semantic_entry.go:72-103 |
| 仲裁器调用点 | agent/internal/chat/goalrunner_chat.go:261 |
| 263 证据 | coord/reports/2026-11-06-c4-ablation-1-ablation-verdict.md §二；coord/rulings/2026-11-05-M3-GROUP3-DISCRIMINATE-1-pass.md §一/§二 |
| 264 证据 | coord/rulings/2026-11-06-B4-ABLATION-1-pass.md §三；coord/reports/2026-11-06-b4-ablation-1-ablation-verdict.md §二 |
| 257 矩阵 | coord/reports/2026-11-05-af-family-health.md §三/§四/§五/§六 |
