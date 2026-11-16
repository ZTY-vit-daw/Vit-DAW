# SRC-PRESET-AGENT-1 勘察报告（勘察腿，零栈）

- 卡：SRC-PRESET-AGENT-1（池序 384）；执行：GLM-5.3 L2 实现子代理（ZCode PC）；2026-11-16 晚窗
- 基线：worktree `D:/Vit_DAW_wt_presetagent` @ `port/src-preset-agent-1`，领取基线 `d5c20765`（=当时 origin/main，全净）；领取提交（卡 todo→doing）= main `5c9f98c5`
- 性质：纯只读勘察（代码+卡卷+只读盘面文件），零栈零 LLM 零计费，不改生产代码；§8 不适用（无概率性运行）
- 结论一句话：**agent 侧「用音源」链（发现→装载→参数读写）已实证可用，但「选预设调用」的运行时通道在内核与 agent 两侧均为零——预设只能盘面枚举（Surge XT 637 个出厂 .fxp）+模板 state 固化路径落地；运行时预设切换需内核命令（在池卡 291 ARR-PRESET-1 即此事），本卡不扩域、建议以「乐器选择优先（真钢琴在库）+盘面预设索引+模板 state 换血」先行满足用户底线，运行时预设腿候 291。**

---

## ① 音源预设发现面盘点

### 现有能力（锚点）

| 面 | 能力 | 锚点 |
|---|---|---|
| pluginprobe（隔离观察宿主） | load/snapshot（参数面+总线+延迟）/render（探针音频）/unload——**观察专用，无参数写、无预设/program 面** | `agent/internal/pluginprobe/adapter.go:22-23`（"no parameter-write, profile, mapping"声明）；操作白名单 `agent/internal/vst3host/worker.go:194-201`（load/snapshot/render/unload 四枚）；native 侧 `PluginProbe/native-host/src/main.cpp:369-374`（同四枚+shutdown） |
| processorregistry | 纯数据族注册表，**全是 FX 族**（static_eq/…/reverb/delay）+generation_shell（load-only NativeSeed 先例）；无乐器族、无预设语义 | `agent/internal/processorregistry/registry.go:242-280` |
| 内核 ZMQ 插件面 | scan_plugins/plugin_list_available/plugin_search/instantiate_plugin/get_plugin_parameters/set_plugin_param/show_plugin_editor/open_plugin_ui/delete_plugin/move_plugin——**零 preset/program/state 命令**；instantiate_plugin 不收 state 入参（新实例=出厂 init 态） | `VitApp/Source/Service/CommandDispatcher.cpp:3350-3410`（注册清单亲读）；`PluginRackControlService.cpp:3217+`（handleInstantiatePlugin 全文亲读，无 state 属性消费） |
| pluginsemantics（语义索引） | **插件级**分类/检索（surge→synth 82、piano→instrument 78、drum→instrument 74；synth 查询词含「乐器/合成器」）——粒度止于插件，无预设层 | `agent/internal/pluginsemantics/index.go:303-371`（分类规则）、`:421-451`（queryTypes） |

### 硬证据：VST3 program change 不在参数面（298 取证，本腿复核）

- 298 卡（done，`coord/cards/done/2026-11-07-VSTI-PARAM-PROBE-1.md` 回执③）：两件音源 automatable 参数面 **0 个 preset/program/patch/bank 命名参数**；MIDI 通道亦不暴露；Surge XT 776 参+scene 族 158 参（agent HTTP 面披露 512/776）。
- 本腿独立复核：298 工件（`port/vsti-param-probe-1:coord/runs/VSTI-PARAM-PROBE-1/probe1_20261107_1920/surge_xt_parameters.json`，1.9MB）grep `preset|patch|bank|program` 于 param_id/param_name = **0/776 命中**。

### 盘面发现面（本腿零栈实测，只读）

- **Surge XT 出厂预设库可枚举**：`C:\ProgramData\Surge XT\patches_factory\` = **637 个 .fxp / 17 分类目录**（Keys:16[EP 1/EP 2/DX EP/Soft Suitcase/Digi Harpsi/Church/Organ 1-3/Circus/House Organ…]、Basses:59、Leads:131、Pads:65、Plucks:111、Polysynths:104、Sequences:44、Percussion:9、Winds:8、Brass:11、Chords:8、MPE:9、Splits:5、Templates:15、Tutorials:13、Vocoder:3）；另有 `patches_3rdparty\`。.fxp=VST chunk 格式（`CcnK`/`FPCh` 头，偏移 16 处 `cjs3` 魔数）。
- VST3 标准 .vstpreset 目录（`Documents\VST3 Presets\`）**仅 Plugin Alliance 条目**，无 Surge/Waves——.vstpreset 路线对本机音源无料。
- **本机在库 VSTi（语义索引 `~/.vit/plugin_semantics.json`，991 条中 17 件乐器）**：Waves shell×13——**Grand Rhapsody Piano**（真钢琴采样）、Electric88/Electric200/Electric Grand 80（电钢）、Clavinet、Bass Fingers/Bass Slapper、CR8 Sampler、CODEX、Element、Flow Motion、OVox、COSMOS、StudioVerse Instruments；FabFilter One/Twin 3（synth）；Surge XT。**「用钢琴」类诉求有专用音源可直装载（init 态即钢琴音色，无需预设切换）**。
- Waves 乐器预设（StudioVerse 体系）在插件内部，盘面不可枚举——Waves 侧预设枚举不可行（诚实边界）。

### 模板 state 固化路径的格式取证（零栈实验）

- 模板（373 产物）PLUGIN state 属性形态=`<字节数>.<自定义 base64>`：`state="67294.VMjL…"`（`coord/lesson-designs/music-creation-unit2/templates/melody-drum-v1/template.v1.xml`，三轨同 blob）。
- 本腿解码实验：`.` 为字母表替换位（串内 82 处，含连续两点，非标准 base64 填充）；将 `.` 替换为缺失字符（e/6// 三候选）后标准 base64 解码**精确命中声明长度 67294 字节**——编码确定可逆。
- 解码内容为二进制（无明文 patch XML、无 CcnK/cjs3 魔数）——**与 .fxp chunk 的格式关系未确立**；tracktion 子模块未初始化（`tracktion_engine/` 为 submodule），字母表唯一性与 state↔fxp 转换可行性=留待实现腿的小型取证项（判据：tracktion_engine 上游源 ExternalPlugin 的 state 编码函数，或任一第二已知 state 样本）。
- state 采集先例=内核亲存 .vit 提取（309 E4 fixture→`scripts/src_template_build.py:135-160` 逐字搬运+sha256）——**不经内核拿不到新预设的 state**（除非上述格式转换取证成功）。

---

## ② 乐器语义→预设匹配现状

- **无乐器→预设匹配链**：processorintent 族枚举零乐器族（`agent/internal/processorintent/intent.go:32-48`）；semanticeffect 仅压缩器语义+提示契约；pluginsemantics 是插件级分类（①表）——仓内不存在任何「乐器词→预设」规则或先例。
- **GM Program 消费点现状**：仅两处，均为**声明面**而非映射面——
  1. `agent/internal/coursetemplate/coursetemplate.go:107-114` `GMMapping{gm_program, gm_name, percussion_keys}`（kind=stand_in_declaration，卡面明言「站位声明而非 GM 兼容承诺」）；
  2. 模板 manifest `…/melody-drum-v1/template_manifest.json`：旋律轨 `gm_program:82 "Lead 2 (sawtooth)"`（Surge init 锯齿占位）+鼓轨 `percussion_keys {36:kick,38:snare,42:closed_hat}`（channel 10 记谱语义；现音色=chromatic 合成，键位/音色分离声明）。
  **仓内无 GM 1-128 标准名表**（grep `Acoustic Grand|General MIDI|program change` 全仓零命中）；TOM/staticbalance 的乐器词表是轨名分类器，与选音色无关。
- **对话触发面不认乐器语义**：装载意图门 `looksLikePluginGrabberLoadIntent`（`agent/internal/workflows/plugingrabber/commands.go`）要求装载动词（load/加载/插入）**且**插件主语（plugin/vst/eq/压缩/混响…）——「这曲用钢琴/换个钢琴音色」不触发任何工作流。模型侧**可以**经已文档化的 chat 工作流命令点名装载（`agent/internal/chat/server.go:6589`：`{"cmd":"plugin_grabber_load_and_get_params","plugin_query":"Grand Rhapsody Piano",…}`），即「点名音源装载」今日可用、「语义点音色」不可用。
- 258 勘察（`coord/reports/2026-11-05-arr-recon-1.md` 后位条目）已预见「instrument 语义化控制 PCA 家族（load_admission 同 GenerationShell 先例）」为后位卡——本卡匹配链腿即该设想的落地位。

---

## ③ 322/323 认证与 typed 写入模式摘要（384 动态调用腿复用蓝本）

### 治理链模式（322/323，均 done 已合入）

- **认证→披露链**：autosweep certify（`scripts/pca_autosweep.py`，内核 ZMQ 直连装载→收据十件落 `~/.vit/pca_certifications/`）→ vendor-scoped 收据种类导入（`agent/internal/processorauthority/receipts.go` 枚举+ReadReceipt switch，fail-closed，未知厂商拒）→ v1 store 晋升 → derive → 白名单 `~/.vit/free_state_experiment_plugins.json` → agent 披露面（free_state_plugin_candidate_disclosure.go）。
- **323 形态取舍先例**：厂商扩容采 **vendor-scoped kind**（白名单只增一枚字符串，最保守）；扩容=独立最小 commit 卡。
- **310 结论红线**：写入类必须 typed 承重（裸面 T1/T3/T4 全败）——384 的预设写入同样必须 typed 等价面。

### typed 写入链蓝本（EQ 族，`agent/internal/chat/plugin_eq_edits.go:131-200`）

读活面（readLiveEQControlSurface）→ 语义编辑计划（planEQEdit，拓扑 generation 绑定）→ 写前像（preimage）→ **原子事务**（executeEQTransaction：写+读回）→ 实际读回入回执（actual readback）。384 预设腿对应物=「预设选择写入→读回核对（参数签名/渲染特征）」。

### 装载治理（音源装载的既有门）

- 不可伪造进程内授权 token：`agent/internal/harness/harness.go:2205-2254`（AuthorizeSemanticPluginSelectionLoad / AuthorizeProcessorSelectionLoad / AuthorizeProcessorCertificationLoad——JSON/LLM 无法伪造）；eligibility 无名称回退（`agent/internal/processorattestation/eligibility.go`，identifier+指纹+族+覆盖）。
- 277（ARR-SRCLOAD-1，done）已实证音源装载全链（搜索→确认卡→装载→quick_controls）。
- 302（VSTI-WRITEGUARD-1，已合入）：agent 通用写面 `set_plugin_param` 对 `template_role=instrument` 的 unresolved 守卫码放行（族豁免单点修复，未知族维持 fail-closed）——**VSTi 参数写在 agent 面今日可用**（Surge XT 298 实证通过；Grand Rhapsody 经 302 修复后放行）。
- generation_shell 的 NativeSeed（`processorregistry/registry.go:261-277`）=「load-only 家族+出厂 bundle 指纹绑定常驻准入」先例——乐器族若进 PCA，可循此形（但本卡勘察结论是乐器装载走既有语义选择流即可，无需新族认证，除非要求预设级 typed 覆盖证明）。

---

## ④ 全链差距清单与实现腿拆分建议

### 「agent 对话说『用钢琴』→枚举/匹配→typed 写入落轨→出声」分步

| # | 步骤 | 现状 | 证据锚点 |
|---|---|---|---|
| 1 | 对话乐器语义识别（「这曲用钢琴」→乐器意图） | **缺**：装载意图门不认；无乐器语义路由 | ②意图门锚点 |
| 2 | 音源发现与候选选择 | **有**：plugin_list_available+语义索引+排序（乐器意图时偏好 IsInstrument）+确认卡 | ①表；`chat/plugin_grabber_workflow.go:95-135`（zone Z2/rankPluginLoadCandidates） |
| 3 | 预设枚举 | **运行时无**（内核/agent 双侧零命令，298 实证 program change 不在参数面）；**盘面有**（Surge 637 .fxp+分类，agent 侧无读取器）；Waves 侧盘面不可枚举 | ①全部锚点 |
| 4 | 乐器语义→预设匹配规则 | **全缺**：无 GM 名表、无匹配引擎、无降级文案 | ② |
| 5a | 音源装载落轨（typed/治理） | **有**：277 实证；harness 不可伪造 token+确认卡 | ③装载治理 |
| 5b | 预设/program 运行时写入 | **缺（内核域）**：需 `plugin_load_preset`（setStateInformation(.vstpreset 字节流/programIndex)+读回核对）——**在池卡 291 ARR-PRESET-1（todo，已确认，机动档）正是此事**，258 报告 :91-114 有完整命令面草案 | `CommandDispatcher.cpp:3350-3410`（零 preset 命令亲读）；291 卡；258 报告 |
| 5c | 创建期 state 固化写入（模板路径） | **有**：.vit XML state blob+sha256 固化（373）；**但新预设 state 采集缺通道**（现唯一来源=内核亲存 .vit；离线 fxp→state 转换=未确立的取证项①末节） | ①state 格式取证 |
| 6 | VSTi 参数微调（写后校） | **有**：set_plugin_param typed 守卫+302 豁免+298 内核面往返零 mismatch | ③302/298 |
| 7 | 出声探针非静音 | **有**：离线渲染+dBFS 度量（309/373 同 state 渲染 -12.6 dBFS 在卷）；内核 start_render/l2_render_probe | 373 卡；`CommandDispatcher.cpp:3544-3568` |
| 8 | 模板音色重做（旋律+鼓） | **结构有**（coursetemplate+manifest+确定性再生 `scripts/src_template_build.py`）；**内容缺**：旋律=锯齿占位（gm82）、鼓=chromatic 合成（36/38/42 呈音高） | ②manifest 锚点 |

### 关键结构性判断（上交主管）

1. **运行时预设枚举/切换确需内核面**——触发本卡停止条件的字面情形；但**在池卡 291（ARR-PRESET-1）已为此而设**（内核单 handler+agent 注册，258 草案在卷）。本卡不扩域：运行时预设腿建议与 291 衔接（291 前置=277 已完成，随时可领），而非 384 自行动内核。
2. **用户底线「使用音源+找合适预设调用」存在一条不依赖内核的先行满足路径**：乐器选择优先——「用钢琴」→Grand Rhapsody Piano/Electric88 直接装载（init 态即目标音色）；合成音色类→Surge XT 盘面预设索引匹配+**模板/创建期 state 固化**（5c）。运行时换预设（不重装载）才需要 291。
3. 鼓轨「GM 键位语义正确化+真打击音色」：Surge XT Percussion 分类仅 9 个 preset 且非 GM 鼓组；真 GM 打击音色本机无专用音源（Waves 无 GM 鼓）——**诚实边界**：鼓腿可先做「Percussion 类 preset 键位声学验证」或声明的 chromatic 降级，GM 鼓采样音源另议。

### 实现腿拆分建议（候主管裁定，多腿）

- **腿 A（零栈，agent 域，本卡可做）**：乐器语义面——GM 1-128 标准名表+乐器词（含中文）→候选规则（专用音源优先：钢琴→Grand Rhapsody；合成类→Surge 预设目录）；Surge 盘面预设索引读取器（.fxp 路径+分类+名→预设清单工件）；fail-honest 降级文案（匹配不到=如实说不硬造）。域：新 agent 包（或 pluginsemantics 扩展）+工件。
- **腿 B（内核域，=291 卡，不在本卡）**：`plugin_load_preset` handler+agent 注册。锚点上交：291 卡在池+258 草案+本报告①③证据。**建议主管将 291 提序**（384 动态调用腿依赖它做「不重装载换预设」）。
- **腿 C（真栈，agent 域，依赖 A+（B 或 5c 路径））**：agent 动态调用腿——乐器语义意图路由（扩装载意图门/新意图）→枚举/匹配日志在卷→typed 写入（经治理确认卡；运行时走 B 命令+读回核对（typed 等价面，310 红线）；模板路径走 5c state 固化）→出声探针非静音→用户听到音色变化。
- **腿 D（内容腿）**：模板重做——旋律轨按谱例乐器语义装配（若 380 未完则现旋律只换音色）；鼓轨 groove 重写+打击音色（含诚实边界 3）；manifest 固化 blob 叠加语义音色声明；`src_template_build.py` 确定性再生+新 state 采集（优先内核亲存路径；若①末节格式取证成功可离线合成）。
- **腿 E（可选小型取证，零栈）**：state 编码字母表唯一性+fxp↔tracktion state 转换可行性（判据已在①末节列明）——成功则腿 D 的 state 采集免栈免内核。

### 与池内其他卡的边界

- 291 ARR-PRESET-1（todo）：内核预设命令——本卡 5b 的正主，建议衔接不重复。
- 381（YuE 音色）、380（转写）：卡面已排除/衔接，无碰撞。
- 后位「inspect_synth 族」（258 报告）：乐器族 PCA 化的远期位，本卡不动 PCA 库（无新族认证写入）。

---

## 工件与合规

- 本报告路径：`coord/runs/SRC-PRESET-AGENT-1/RECON.md`（port/src-preset-agent-1 分支）
- 零栈声明：未登记栈位、未启动任何 VitApp/Godot/agent 进程；只读盘面文件（ProgramData/语义索引/298 分支工件）
- 零 LLM/零计费；无 run ID
- 生产代码零改动（diff=本报告+卡面回执）
