# G0-SCORE-SPIKE-1 报告：谱面库选型 spike——真实编辑回读出声证据去风险

- 卡：G0-SCORE-SPIKE-1（池序 369，P0）；执行 owner：GLM-5.3 执行子代理（ZCode L2 会话）/ PC
- 领取提交：`d7d84c94a9e4a87e375b4d242f54cae3d56e0460`（基线 origin/main `9cc3048c`）；worktree `D:/Vit_DAW_wt_369coord`
- 日期：2026-11-15 深夜 ~ 2026-11-16 凌晨（Asia/Shanghai）
- 上游依据：[展演审计](../../reports/2026-11-12-demo-execution-audit/README.md) §4「谱面工作台=主要新开发」+ H4；课程硬需求 [design.v1.yaml](../../lesson-designs/music-creation-unit2/design.v1.yaml)（2/4 工程、四类节奏含附点/切分、休止、原题 4/4 计量、材料含 3/4 演示）

## 0. 运行口径（AGENTS §8 成文）

- **零栈**：未启动 VitApp/Godot/agent 真栈，未占 `coord/resources/PC-RUNTIME-STACK.md`，无 run ID。内核回读链路只做静态盘点（§1）。
- **LLM 参与：无**。两份 spike 均为确定性 node 脚本（无随机、无网络请求于运行时）。npm install 走 npmmirror（仅装包，不属 LLM 概率运行）。
- **运行次数**：最终版各 1 次通过（Spike A exit 0 / Spike B exit 0）。调试过程如实记录：Spike A 共 6 轮=5 轮修正+1 轮通过（1 轮代码缺陷=空档段未按切分点切；1 轮=VexFlow 5 StaveTie 参数名 camelCase 迁移；2 轮=断言期望与规则输出不一致〔e2 休止窗、e4 弱起附点跨小节线〕；1 轮=游标表未按小节分组）；Spike B 共 5 轮=4 轮修正+1 轮通过（类名 `dots`、timemap 返回类型为对象数组、`measureOn` 键名、MIDI note-off gap）——修正轮均为「脚本方期望错误/文档勘误」，非候选库缺陷，无环境中断。
- **失败分类**：环境中断 0；候选库缺陷 0（发现的行为差异均已记录为选型发现，见 §3）；断言期望错误 8（全部修正后单次通过）。
- **止损线**：npm 安装连续失败或 wasm 无法加载 → 记环境中断、该候选降级为文档级打分。未触发。

## 1. 内核回读命令面静态盘点（接口假设的代码锚点）

全部为**只读勘察**（未运行），锚点基于领取基线 `9cc3048c`：

| 面 | 命令/字段 | 锚点 | 契约形状 |
|---|---|---|---|
| 音符回读 | 内核命令 `get_midi_clip_notes` | `agent/internal/edu/command.go:293-315`（parseClipNotes） | `notes[]: {id, pitch, start, length}`——**start/length 单位=beats（相对 clip 起点），pitch 0-127，id 为内核稳定音符 ID**；行内缺字段 fail-closed |
| clip 起点 | `get_midi_clip_data` | `agent/internal/edu/command.go:197-207` | `start_seconds`（秒；×tempo/60 得全局 beat） |
| 拍号/速度 | `get_project_state` → `music_context` | `agent/internal/edu/command.go:171, 245-263` | `tempo_bpm / tempo_setting_count / time_signature_numerator / time_signature_denominator / time_sig_setting_count`；**多段 tempo/拍号时 bar 换算已有 fail-honest 先例门**（edu 判定器同款） |
| 音符写入 | `apply_midi_note_patch`（别名 `midi.write_clip_notes`） | `agent/internal/tools/catalog.go:1238,146`；`agent/internal/harness/harness.go:11142-11148` | `operations[]: insert_note/delete_note/move_note/resize_note/transpose_note/set_velocity/quantize_region/replace_region`，`time_unit:"beats"` 默认；**RiskConfirm（治理确认链）** |
| clip 创建 | `insert_midi_clip` / `create_midi_clip` | `agent/internal/tools/catalog.go:1232-1233` | 同上确认链 |
| 速度/定位 | `transport.set_tempo` / `seek` | `catalog.go:1151`；`harness.go:9860` | — |
| 拍号写入 | **无 `set_time_signature` 生产命令** | 全仓 grep 仅 set_tempo（与审计 H4 一致） | 展演「创建工程设 2/4 并回读一致」是**独立阻塞点，须内核侧补 handler（另卡）**，不在谱面库选型内解决 |
| 游标时钟 | `current_selection.playhead_seconds / transport_position_seconds / is_playing` | `agent/internal/chat/server.go:2277,2577-2579,6802` | GET `/agent/state`（只读白名单 `scripts/g_runtime_readonly_smoke.ps1:16-17`）；秒→beat 用恒定 tempo 换算 |
| 通用调用面 | HTTP `/agent/invoke`（POST） | `agent/internal/chat/server.go:751`（handleInvoke `:1747`） | webui/烟测执行工具命令的现成通道 |
| webui 现状 | 无任何谱库依赖 | `agent/webui/package.json`（deps 仅 react 19/react-dom/lucide-react） | 谱面库为全新引入 |

**映射结论（接口假设）**：内核 note id 是回读一等公民，谱面模型可直接携带 kernel note id 实现 ID↔符号双向映射；**附点/延音/休止/切分在内核无原语**（内核=平 onsets/durations），它们是 beat-grid 之上的**派生记谱视图**——因此无论选哪家库，都必须自建一层「内核平音符 → 记谱符号（含延音链/休止填充/附点归并）」的确定性转换（= W18 的核心），库的分工只是**渲染与命中检测**。这一结论直接决定 §2 打分权重：谁强制我们接受它的模型，谁就在最重的轴上制造阻抗。

## 2. 候选盘点（六轴打分）

六轴=卡面硬需求展开：A 拍号（2/4 工程/4/4 原题/3/4 材料播放）；B 附点/切分/休止/延音**真实编辑**；C 音符 ID↔内核 clip/note 映射；D 随谱播放游标时钟；E MIDI↔记谱双向（W18 承接）；F 工程集成与维护（TS/Vite/React19、许可证、体积、与治理链/内核播放的关系）。符号：✓ 原生满足 / △ 需自研或受限 / ✗ 不满足。

| 候选 | A 拍号 | B 真实编辑 | C ID 映射 | D 游标 | E MIDI 双向 | F 集成维护 | 结论 |
|---|---|---|---|---|---|---|---|
| **VexFlow 5.0.0**（MIT，TS 原生，纯渲染引擎，模型自持） | ✓（spike 亲证 2/4） | △ 编辑由**自研模型**承担（spike 亲证四要素全链） | ✓ 模型自持，kernel id 直挂符号（spike 亲证） | △ 无内置时钟；tick→x 自建（spike 亲证可行） | △ 转换层自持（spike 亲证 SMF 往返**精确全等**） | ✓ 无 wasm、MIT；node 无头测试需 jsdom | **推荐主路线** |
| **Verovio 6.4.1**（LGPL-3.0-only，wasm 27.9MB unpacked） | ✓（spike 亲证 2/4+measureOn 锚） | △ `edit()` 官方自标 *experimental not to rely on*；spike 走 MEI 字符串手术（可行、无对象模型、人体工学差） | ✓ xml:id→SVG id→timemap/MIDI 全链可寻址（spike 亲证） | ✓ `renderToTimemap` qstamp/tstamp 开箱即用（spike 亲证） | △ 出向 `renderToMIDI` 亲证（含 note-off gap 弊，见 §3-B）；**入向无 MIDI→MEI** | △ LGPL 弱 copyleft（Web 嵌入需守边界）；wasm 冷启 93ms | 材料只读显示的强候选；不进首版编辑链 |
| abcjs 6.7.2（MIT，单维护者） | ✓（ABC `M:2/4`） | ✗ 编辑=改 ABC 文本串，非对象级 WYSIWYG | △ 元素可寻址（getElement），但音符模型在文本串里，与内核模型距离最远 | ✓ TimingCallbacks 官方游标 | △ MIDI 下载导出有、导入无 | △ 自带 WebAudio 播放与内核播放重复 | 排除（B/C/E 三重阻抗） |
| OSMD 2.3.1（BSD-3） | ✓（MusicXML） | ✗ 官方定位 viewer，编辑仅视觉属性 | △ 模型归 MusicXML 文档 | ✓ osmd-cursor | △ 读 MusicXML，MIDI 出向第三方 | ✓/△ 底层即 VexFlow——用它仍绕不开 VexFlow 系 | 排除（若需要 VexFlow 系，直接用 VexFlow 少一层） |
| alphaTab 1.9.0（MPL-2.0） | ✓ | △ API 可改模型，但 tab 中心，无拍内切分/延音编辑原语 | △ 自有模型 id 体系 | ✓ 播放引擎+游标原生（与内核播放重复） | △ 读 MusicXML/GP/alphaTex；MIDI 出有 | △ MPL 文件级 copyleft（不改源可集成）；体量大 | 不推荐（播放面重复+模型不由我） |
| 自研 SVG+SMuFL | ✓ | ✓ 全自写 | ✓ | ✓ | ✓ | ✗ 工作量最大（版式/字形/谱线细节全自担），展演窗口内风险最高 | 保留为兜底，不首发 |

事实来源：npm registry（`npm view`：vexflow 5.0.0 MIT、verovio 6.4.1 LGPL-3.0-only、abcjs 6.7.2 MIT、opensheetmusicdisplay 2.3.1 BSD-3、@coderline/alphatab 1.9.0 MPL-2.0）；[Verovio toolkit methods](https://book.verovio.org/toolkit-reference/toolkit-methods.html)（edit 实验性、RenderToTimemap/RenderToMIDI）；[abcjs docs](https://docs.abcjs.net/animation/timing-callbacks)（TimingCallbacks）；[OSMD](https://github.com/opensheetmusicdisplay/opensheetmusicdisplay)（viewer 定位）；[alphaTab](https://github.com/coderline/alphaTab)（MPL-2.0、播放引擎）。

## 3. Spike 证据（≤2 候选，scratch 隔离，未碰生产 src/ 与主仓 package.json）

同一份 3 小节 2/4 测试内容（覆盖六硬需求要素）：小节1=附点八分+十六分（教材「前长后短」）+八分休止+八分；小节2=八分休止+**弱起切分延音对**（跨拍）+跨小节线延音首半；小节3=延音尾半+八分休止+四分休止。

### 3A. Spike A：VexFlow 5 + 自研 beat-grid 模型 —— exit 0

- 目录：`scratch/vexflow-spike/`（deps：vexflow@5.0.0 + jsdom@24.1.3，仅 scratch；node_modules 已 gitignore）
- 命令：`node spike.mjs` → **EXIT=0**，27 断言全过；`artifacts/results.json` + 5 份状态 SVG + `final.mid`
- 腿与结果：
  - **L1 确定性推导**：附点×1、休止窗×2、延音组×2（其一跨小节线）——规则 v0：R1 每跨度在小节线/拍线处切分；R2 段内允许符号贪心；R3 同 kernel 音符相邻符号互为延音。
  - **L2 编辑→序列化→重解析→重推导→重绘往返**：insert（休止窗被音符替换）/ delete（延音对消失、空档按拍线落合法休止组合）/ resize（跨小节线延音消失）/ dot-toggle（弱起附点跨小节线→八分+十六分延音链，**正确记谱且确定性**）四操作全过。
  - **L3 游标**：tick→x 表按小节分组行内单调，音符起点命中精确、拍间插值正确；渲染 3 小节 36ms。
  - **L4 MIDI 往返**：模型→SMF（手写 writer，PPQ=48，全整数无浮点漂移）→解析回读，pitch/start/length **集合全等**（5 音符）。
- 选型发现（进主开发卡风险表）：
  1. VexFlow 5 渲染需真实 DOM——node 无头须 jsdom（jsdom 无 canvas 仅产生文本测量告警，渲染正常）；**vitest 测试环境需配 jsdom**；生产浏览器无此问题。
  2. VF5 对 VF4 的 API 漂移实证：`StaveTie` 参数 `firstNote/lastNote`（camelCase）。锁版本+适配层可控。
  3. 推导规则 v0 已可用但需按教材语汇精化（如「拍内 3+1 优先归并附点符号、弱起 3 跨线必须延音链」等约定要成文进 W18 契约）。

### 3B. Spike B：Verovio 6.4.1 / MEI —— exit 0

- 目录：`scratch/verovio-spike/`（deps：verovio@6.4.1，仅 scratch）
- 命令：`node spike.mjs` → **EXIT=0**，32 断言全过；`artifacts/results.json` + 4 份 SVG + `verovio.mid`
- 腿与结果：
  - **B1 渲染+ID 寻址**：MEI `xml:id` 7 音符全部进 SVG `id="…"`；拍号/附点（`class="dots"`）/延音×2/休止×4 在场。
  - **B2 timemap**：`renderToTimemap` 直接返回对象数组（**非文档暗示的 JSON 串**）；节点含 `qstamp`（quarter 坐标，直接=beat 坐标）+`tstamp`+`measureOn` 锚+noteId 披露——**游标时钟开箱即用，这是 Verovio 最强的一面**。
  - **B3 renderToMIDI**：延音链自动合并为单音符（=内核形状，pitch/start 全等）；**但每音符固定 note-off gap≈0.00833q（division=120 下 1 tick）——时值不能以 MIDI 为真值，必须回读 MEI**（对照：Spike A 自写 SMF 时值精确全等）。
  - **B4 编辑**：MEI 字符串手术 3 例（删延音对/改时值/去附点）重载重渲染差异正确；`toolkit.edit()` 官方自标 experimental，**不可依赖**。
  - 性能：wasm 冷启 93ms、渲染 32ms；包 unpacked 27.9MB；LGPL-3.0-only。
- 选型发现：Verovio 适合**只读材料显示**（教材 MusicXML 原谱、3/4 演示），其 id 寻址+timemap 对展示面是净收益；但编辑轴（B）必须绕开其实验性 edit() 走 XML 手术，与「学生逐符号编辑」的产品需求不匹配。

## 4. 选型结论

**主开发路线：VexFlow 5（渲染）+ 自研 beat-grid 记谱模型（编辑/映射/转换/游标全自持）。**
理由：六轴中最重的 B/C/E 三轴本质上都必须由我们自己的模型承担（§1 映射结论：附点/延音/休止/切分是内核平音符之上的派生视图）——VexFlow 是唯一「不强迫接受外来模型、MIT、TS 原生、无 wasm」的成熟渲染引擎；spike 已亲证四要素编辑→数据→重绘往返、ID 携带、游标查找、SMF 精确往返全部可行。**Verovio 不进首版编辑链**，登记为材料只读显示（MusicXML 教材谱例/3/4 演示）的后续可选件，若加装须过 LGPL 边界裁定；abcjs/OSMD/alphaTab 排除（理由见表）。自研 SVG 保留为兜底不首发。

## 5. 实施卡草案（谱面工作台主开发卡，供主管发卡参考）

**SCORE-WORKBENCH-1（草案）——谱面工作台：节奏/旋律模式编辑 + 内核真写回读 + 随谱游标**

- 前置依赖：①拍号写入（内核 `set_time_signature` handler + 创建工程设 2/4 保存重开回读一致——审计阻塞点，**须独立内核卡先行或并行**）；②交互卡宿主（compose-ui 装配面，367 线）。
- 文件域（草案，主管核对后定）：`agent/webui/src/scoreworkbench/**`（新：model/derive/render/cursor/edit + `*.test.ts(x)`）＋ `agent/webui/package.json`（+vexflow、+jsdom devDep）＋ composer 装配接线点 ＋ `scripts/` 烟测扩展（`dev_agent_smoke.ps1` 加参数或同模式新脚本；只读腿参照 `g_runtime_readonly_smoke.ps1` 白名单）。不碰内核/VitApp（出声依赖 H5 固定模板线，另行）。
- 里程碑：
  - M1 模型+推导+渲染：kernel 形状模型（携带内核 note id）、推导规则成文（R1-R3 + 教材语汇精化：拍内附点归并优先级/弱起延音链/休止填充表）、VexFlow 渲染与命中检测；vitest（jsdom env）全绿。
  - M2 编辑交互：点击放符/时值切换/附点开关/休止插入/延音建立与解除/删除/拖动（16 分网格），全部经自研模型走推导重绘；**确认粒度与治理链对齐**（apply_midi_note_patch=RiskConfirm——用户直接 UI 编辑的确认粒度/合并批量 patch 策略需主管裁定，见风险 R2）。
  - M3 内核往返：`/agent/invoke` apply_note_patch 真写 → `get_midi_clip_notes` 回读 → 模型重建回显；保存重开工程回读一致。
  - M4 随谱游标：`/agent/state` playhead_seconds→beat→x（恒定 tempo 假设+多段 tempo fail-honest 门复用 edu 先例）；与内核播放对齐的视觉反馈。
  - M5 端侧验收：渲染面烟测（浏览器级 DOM 断言，E2E-WEBUI-1 面）+ 真栈烟测腿（三件套、脚本 exit 0，AGENTS §5 门槛）。
- 风险表：
  - R1 拍号写入缺失（阻塞 M3 验收里的 2/4 工程语义）→ 前置内核卡；
  - R2 治理确认粒度：逐编辑步确认将打断节奏输入——需主管裁定 UI 直操与确认卡的边界（治理链红线不动）；
  - R3 单 tempo/单拍号假设（多段时 bar/beat 换算失效）→ fail-honest 降级而非静默错位；
  - R4 VF5 版本 API 漂移（StaveTie camelCase 实证）→ 锁版本 + 薄适配层；
  - R5 无头测试依赖 jsdom（canvas 告警无害但须在 vitest 环境成文）；
  - R6 游标平滑度受 `/agent/state` 轮询粒度限制（必要时评估 events 流，不改内核）；
  - R7 教材语汇精化规则集是 W18 契约的一部分（附点/切分/休止/延音/拍内归并约定要可审查）；
  - R8 材料显示若引入 Verovio → LGPL 边界+27.9MB wasm，须单独裁定，不捆绑本卡。
- 验收标准（草案）：vitest 新测试全绿；渲染面烟测 PASS；真栈烟测腿 exit 0（写入→回读→游标断言）；回执声明端测覆盖边界。

## 6. 工件清单

```
coord/runs/G0-SCORE-SPIKE-1/
  REPORT.md                          ← 本报告
  scratch/vexflow-spike/
    spike.mjs  package.json  package-lock.json  .gitignore
    stdout.log  stderr-summary.log
    artifacts/ results.json state0-initial.svg state-{insert,delete,resize,dottoggle,final}.svg final.mid
  scratch/verovio-spike/
    spike.mjs  package.json  package-lock.json  .gitignore
    stdout.log  stderr.log(空)
    artifacts/ results.json state0-initial.svg state-{del-tiepair,resize-n3,undot-n1}.svg verovio.mid
```

## 7. 未做与边界

- 未启动真实栈（内核回读命令面为静态盘点）；真实出声验证留主开发卡。
- 未修改任何生产文件（主仓 `agent/webui` 与 `package.json` 未动）；scratch npm 装包不涉生产依赖树。
- W18 确定性转换只验证了「内核平音符↔记谱符号」与「模型↔SMF」两段确定性；YuE2 ABC 方言段不在本卡。
- 主开发卡为**草案**，发卡权在主管；本卡不创建 todo 新卡。
