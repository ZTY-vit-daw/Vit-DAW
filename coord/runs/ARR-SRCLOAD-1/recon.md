# ARR-SRCLOAD-1 段 1 勘察：编曲语境音源装载链（读档零 LLM）

- 卡：`coord/cards/doing/2026-11-05-ARR-SRCLOAD-1.md`（池序 277，注册链②，段 1/N）
- 执行：GLM 主会话 L2-worker 子代理（GLM-5.3 / PC）/ 2026-11-07
- 基线：worktree `D:/Vit_DAW_wt_277impl` @ `port/arr-srcload-1`，HEAD=`9e5af73c`（origin/main，领取提交 `6b897640` 之上）
- 口径：全部结论来自代码阅读 + grep 锚点，零真栈占用、零 LLM 调用、零内核改动；258 报告（`coord/reports/2026-11-05-arr-recon-1.md`）锚点逐条在当前 HEAD 复核
- 停止条件核验：**未触发**——装载链内核/agent 注册面全部在场（见 §1/§2），无推翻卡面前提的证据

---

## 1. 内核面复核（258 报告锚点全部仍然有效）

| 锚点 | 当前 HEAD 位置 | 结论 |
|---|---|---|
| `is_instrument` 索引字段 | `VitApp/Source/Service/PluginRackControlService.cpp:1372` | plugin_list_available/search 返回 instrument 标记 ✓ |
| synth 默认 Z2 | `VitApp/Source/Service/CommandDispatcher.cpp:1317-1324`（`plugin->isSynth() → "Z2"`） | rack_add_node 装载乐器落 Z2 ✓ |
| MIDI 轨挂 instrument 可渲染 | `VitApp/Source/Service/VitProductionCoordinator.cpp:73`（`takesMidiInput() \|\| producesAudioWhenNoAudioInput()`）+ :823-836（错误信息明示 "load an instrument plugin on the MIDI track"） | 内核引擎层明文支持 MIDI 轨+VSTi 渲染 ✓ |
| 装载命令不排除 VSTi | `instantiate_plugin` / `rack_add_node`（PluginRackControlService.cpp） | 两条装载通道均 instrument 无差别 ✓ |
| 离线渲染命令 | `start_render`（catalog.go:1220，工具名 `render.start`，RiskConfirm） | 渲染非静音断言的命令面在场 ✓ |
| preset 机制 | Service 目录 grep "preset" 零命令命中（258 报告后无变化） | 结构性缺口维持，ARR-PRESET-1 机动位 ✓ |

## 2. agent 面复核与增量发现

### 2.1 注册面（catalog/policy，全部现成）

- catalog.go:1163-1165：plugin_search / plugin_semantic_build_index / plugin_semantic_search 已注册（RiskDirect）；:1169-1170 get_plugin_parameters / set_plugin_param；rack_add_node 族在 :1194-1197 段（RiskConfirm）。
- policy.go directCommands（:27-52）：plugin_list_available / plugin_search / get_plugin_parameters / plugin_grabber_explain_controls 在场；plugin_semantic_search 经 catalog spec 分类（Classify spec 优先于 directCommands 兜底，policy.go:76-84），无需兜底行。
- **本卡段 1 无新命令 → policy.go 无注册行**（276 三处中的 policy 腿在本段自然为空）。

### 2.2 能力轮披露面（capability gating，本段核心增量发现）

披露链：`agentLoopCapabilityNames`（消息→能力名）→ `toolNamesForAgentLoopCapabilities`（能力名→agentLoop*Tools 白名单求 allowed）→ `ModelCatalogSummaryForCapabilityPacks`（pack.Tools ∩ allowed 渲染进 prompt）。**pack.Tools 与 agentLoop 白名单求交，白名单缺行则 catalog/policy/pack 三处注册全部被遮蔽**（276 第四披露面教训，goalrunner_chat.go:4939-4948 注释在卷）。

- `agentLoopPluginTools`（goalrunner_chat.go:4962-4990）**已含完整装载链**：plugin.list_available / search / semantic_search / semantic_get / semantic_build_index / scan / **plugin.load_to_rack / rack.add_node** / get_parameters / open / show_editor / **set_parameter** / plugin_grabber.explain_controls——plugin 能力轮装载工具可达。
- `plugin.instantiate` 排除维持（toolNamesForAgentLoop :4404，2026-06-03 起无注释）→ 装载走 rack_add_node 工作流，与 258 报告倾向及卡面约束一致。
- `agentLoopTransportTools`（:5029-5035）：play/stop/return_to_zero/seek/toggle_click/set_tempo/record——试听工具在 transport 能力轮可达。
- **render 工具无任何能力 case**：switch（:4441-4479）无 "render" 能力，`render.start` 不在任何 agentLoop 白名单——能力轮（任何门控轮）都无法调用离线渲染；只有非能力轮（全工具面）可达。**新缺口 #7**。
- **"plugin" 能力触发词表不含音源词**（:4616-4618 插件/效果器/plugin/vst/eq/压缩/混响/延迟/grabber/参数——无 音源/合成器/乐器/synth/vsti/instrument/piano）：
  - 例句「给这条 MIDI 轨加个钢琴音源」→ capabilities={midi, track}，plugin 工具不在 allowed → **agent-loop 路径 rack_add_node 不可披露不可调用**；
  - 这就是 258 报告「编曲语境路由未经真栈验证」的词表级根因锚定。**新缺口 #6，本段修复**。

### 2.3 legacy chat 装载工作流（对照面，非本卡文件域）

- 入口词表 `hasInstrumentPluginSubject`（`agent/internal/workflows/plugingrabber/commands.go:55-67`）：synth/synthesizer/instrument/vsti/sampler/合成器/乐器/音源/MIDI乐器——「加钢琴音源」因含「音源」可命中；intent kind=instrument → `include_instruments:true`+`plugin_type:"synth"`（:128-134）。
- 工作流装载命令=rack_add_node，候选 IsInstrument → zone="Z2"、pluginKind="Instrument"（`agent/internal/chat/plugin_grabber_workflow.go:113-129`）。
- 词表缺口：钢琴/piano/organ 等具体乐器名单独词不在词表（「加个钢琴」仅当已知插件名别名命中才可路由）——**域外（workflows/plugingrabber），留段 2 处置或上交**。

### 2.4 语义索引 instrument 面（usage 披露补丁的依据）

- `pluginsemantics/index.go`：:63 IncludeInstruments 选项；**:215 默认搜索排除 instrument**（显式 type 含 instrument/synth 或 include_instruments 才放行）；:303-305/:368-370 别名（piano/drum/sampler→instrument）；:440 中文别名（合成器/乐器）。
- handler `searchPluginSemanticIndex`（harness.go:9533-9612）：**type/include_instruments 形参已在实现面接受并转发**（:9594-9602），但 catalog usage（catalog.go:508 `query:string optional type:string manufacturer:string format:string limit:number`）**未披露 type 值域与 include_instruments**——模型不知道要用 type=instrument 才搜得到乐器。这就是卡面目标 3 的 usage 披露补丁点。
- 直查 `plugin_search` 无 type 形参（内核缺口 #4 维持，非阻塞）。

## 3. 命令缺口清单刷新（对照 258 报告 §1.4）

| # | 缺口 | 258 定级 | 本次刷新 | 处置 |
|---|---|---|---|---|
| 1 | preset 装载命令（内核零命令） | 结构性 | 不变 | ARR-PRESET-1 机动位 |
| 2 | 编曲语境装载引导（pack 缺失） | 引导级 | **本段修复**（arrangement pack 骨架） | 本卡段 1 |
| 3 | plugin.instantiate 排除 | 一行级 | 维持排除（258 倾向+卡面约束） | 不动 |
| 4 | plugin_search 无 type 结构化过滤 | 内核小改 | 不变（semantic_search type 已达） | 后位 |
| 5 | instrument 语义化控制 PCA 家族 | 后位 | 不变 | 11 月不做 |
| **6** | **能力触发词表不含音源词 → 编曲轮装载工具被遮蔽**（2.2 新发现） | — | **本段修复**（arrangement 触发词表+case+白名单） | 本卡段 1 |
| **7** | **render.start 不在任何能力白名单 → 门控轮无法离线渲染**（2.2 新发现） | — | **本段顺带修复**（纳入 arrangement 白名单；不动其他能力面） | 本卡段 1 |

## 4. 择一决策存档：新增 arrangement pack（不扩 midi pack）

决策：**新增独立 arrangement pack**。依据：

1. **卡面验收标准硬约束**：「既有九 pack 行为零变化（对照单测）」——扩 midi pack 必然改其 Tools 面与能力轮披露，直接违反验收线；
2. **词表/披露分离**：midi 触发词（音符/旋律/量化…）的轮次不该连带披露 plugin+transport+render 工具（过度披露扩大攻击面与 prompt 噪声）；编曲词（音源/合成器/编曲/钢琴）的语义远超 midi pack 的"note editing"域描述；
3. **taxonomy 对齐**（2026-11-05 三分法）：编曲域 = MIDI 编辑 + 音源控制面——独立 pack 镜像域边界；编曲消息同时命中 midi+arrangement 两能力时两 pack 并披（allowed 并集），写音符引导（midi pack）与装载引导（arrangement pack）互补而非互替；
4. **回归面最小**：midi pack 刚在 276 完成扩域（analyze_transformations），再叠加装载面扩大回归面；新 pack 对既有九 pack 零 diff。

## 5. pack 骨架设计（段 1 落地面）

- 触发词表（agentLoopCapabilityNames 新增 `add("arrangement")`）：编曲/配器/音源/合成器/乐器/钢琴 + synth/synthesizer/instrument/vsti/sampler/piano（与既有 mix/plugin 词表无子串冲突；未收 arrangement/compose 英文词以防误触）。
- `case "arrangement"` + `agentLoopArrangementTools()` 白名单（第四披露面，276 同款注释）：track.add/list + plugin 装载子面（list_available/search/semantic_search/semantic_get/load_to_rack/rack.add_node/get_parameters/set_parameter/explain_controls）+ midi 子面（create_clip/apply_note_patch/write_clip_notes/read_notes/read_clip_notes/read_clip_data/quantize/transpose/set_velocity/analyze_transformations[衔接占位]/clip.select）+ transport 试听面 + **render.start**（缺口 #7 顺带修复）。
- pack 内容：装载编排次序 Preconditions（semantic_search type=instrument|synth → rack.add_node zone Z2 → get_plugin_parameters → 写 clip → 试听/离线渲染）+ Z2 语义 + preset 缺口引导（用 set_parameter 参数级控制）+ 符号分析衔接占位（analyze_transformations 进 Tools，详细引导归 ARR-SYMLINK-1=283）。
- usage 披露补丁：`plugin_semantic_search` argHint 补 `type:string(effect|instrument|synth)` 值域 + `include_instruments:boolean`（实现面已支持，纯披露补丁，卡面目标 3）。
- policy.go：无新命令，无注册行。

## 6. 段 2/3 留题（勘察结论交主管过目项）

1. **真栈腿前置**：本机 VSTi 索引状态未验证（`plugin_list_available` is_instrument 计数>0 预检，卡面目标 4）；
2. **装载路由双通道**：编曲消息在 legacy chat 层可能先被 plugin_grabber 装载工作流截获（含「音源」即命中），agent-loop 能力轮路径由本 pack 补齐——两条通道在真栈上的实际分流未经验证（段 2 烟测腿验证目标）；
3. **plugingrabber 词表缺钢琴单独词**（2.3）：域外小补丁（workflows/plugingrabber/commands.go），建议随段 2 一并处置或另开微卡；
4. **render 静音断言**：渲染产物非静音判定脚本面（烟测脚本扩展，段 2/3）。
