# EDU-CAP-RECON-1：乐局能力面勘察（符号分析归宿 / 参照物清单 / 数据流通 / UX-AB 复用线）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=用户裁定 [2026-11-05-EDU-DEMO-SCOPE-RULINGS-1.md](../../rulings/2026-11-05-EDU-DEMO-SCOPE-RULINGS-1.md) §五 全部 + 本日口头"勘察卡可出"；取代已撤的原 EDU-RECON-1 能力面各点）
- 派发确认：已确认（用户 2026-11-05 口头开工；执行转交由用户触发）
- 验收负责人：GLM 主管决策流
- 池序 256；目标=只读勘察，报告落 coord/reports/；P1 / 0.5 天 / 依赖=无（与 257 并行）；L1 / PC 执行会话；**不占真实栈、零 LLM 调用、零生产代码改动**

## 背景

开发对象=一局《小星星》乐局（模型+用户共创工程，12 月底预答辩展演其录屏轨迹）。本卡为能力面（构建块 1 符号分析 / 4 问询协议 / 5 拍谱管线）前置勘察。工具面已亲核项见裁定记录 §二，勿重复。

## 勘察点（每点产出 = 代码锚点 + 结论）

1. **符号分析归宿验证**：验证"agent 工具目录命令"路径——`tools/catalog.go` 的命令注册模式、返回结构惯例、evidence refs 写法；与 EPM 投影模式（`internal/epm/types.go` 的 LLMContext/Confidence/Limitations）对照，给定位建议（工具命令 vs 投影，或命令+投影分层）。若结论=命令，给**注册面草案**（新增命令名/参数/返回 schema 草案，不写代码）。
2. **参照物清单盘点（坐标系规则落地面）**：marker / 自设 A-B 段 / 循环区 / 轨道与 clip 名单 / transport 时间 / **tempo-小节换算**——逐项查三件事：模型从哪个命令/投影拿到（可得性）、精度如何、现有模型引导（capability_pack）已覆盖哪些。**重点回答：小节能否成为可靠的可问坐标**（模型能否拿到确定的小节↔时间换算）。产出=参照物矩阵（参照物 × 可得性 × 精度 × 引导覆盖）+ 缺口清单。
3. **MIDI/符号数据流通路径**：工程内读取（`get_midi_clip_notes` 数据面是否够符号分析用：音高/时值/小节定位精度）vs 文件导入（`import_midi_to_track`）两条路的数据形态；**拍谱接线面**：Audiveris/music21（用户部署、离线）输出→进工程的最短路径（中间格式落点：MusicXML→MIDI→导入？）。
4. **UX-AB 试听入口复用线**：`coord/runs/UX-AB-AUDITION-IMPL-1/` 与 `coord/reports/2026-10-22-ab-audition-entry-design.md` 的试听入口能否承载乐局内的对比播放/问询 A/B（"重复 vs 模进"两段对比）；不能则给最简承载路径（对话内 A/B 描述+play 循环段）。

## 验收标准

- 报告 `coord/reports/2026-11-05-edu-cap-recon-1.md`（执行日顺延则用执行日）：每点 `file:line` 锚点 + 结论；
- 点 1 含命令注册面草案（或投影定位建议）；点 2 含参照物矩阵与小节坐标结论；点 3 含拍谱最短接线路径图；
- 对构建块 1（符号分析引擎，变换族：重复/模进/倒影/逆行）的实现面给修订估时意见。

## 停止条件

- 任一点发现结构性障碍（如 clip 数据面不足以支撑小节级定位且无替代数据源）→ 停在该点记录证据上交决策侧，不自行改生产代码绕过；
- 全程只读；临时分析脚本落专用临时目录。

## 领取

- 会话：ZCode 执行流（GLM-5.3）/ 2026-11-05
- worktree：主检出 `D:\Vit_DAW`（只读勘察，不建 worktree、不占真实栈）；分支 main
- 领取基线：HEAD `b46a9096406fcde6fcfd29013bccd724ae938fb5`
- status --short：8 个已跟踪文件改动（VitApp/Workspace 两 xml、M3 卡迁移、DEFENSE-SHOW-1 卡、UX-AB run 工件 4 件）+ 大量 untracked（coord/runs/ 工件、11-05 新卡与裁定、docs 新稿）——均为他卡/决策侧已有改动，本卡只读不碰
- diff --stat：9 files changed, 4551 insertions(+), 4479 deletions(-)（大头为 VitApp/Workspace/default_project.xml 运行时状态）

## 回执

- 报告：`coord/reports/2026-11-05-edu-cap-recon-1.md`（2026-11-05 执行当日）
- ① 符号分析归宿：**工具命令为主 + 返回结构借 EPM LLMContext/Confidence 惯例**（命令+投影字段分层，首版不做 peer 投影）——catalog.go spec 注册模式（:965/:1148）与 epm/types.go（:19-32/:148-155）对照成立；注册面草案见报告 §点①（命令名 analyze_midi_transformations / midi.analyze_transformations，RiskDirect 只读，返回带 bar 锚定 findings + music_context_echo + llm_context 字段族）。
- ② 参照物矩阵：marker/A-B段(range marker)/轨道clip名单/transport 秒 全可用且多数有引导；**tempo/拍号可读**（get_project_state 的 music_context 块，CommandDispatcher.cpp:2235-2252/3817，tempo_bpm+numerator+denominator）但零引导；**循环区内核有 transport_set_loop（:3224/TransportAudioService.cpp:553）agent 未注册**；**小节=可靠可问坐标，三条件**（恒定 tempo 守门 tempo_setting_count==1 / 换算放命令层返回 bar 字段 / 参照物清单进引导）；缺口清单 4 项（含拍号写命令缺失，非阻塞上交）。
- ③ 数据流通：get_midi_clip_notes 返回 clip 内 beat 域完整音符（id/start/length/pitch/velocity/channel，MidiService.cpp:310-320）——**数据面够符号分析**；import_midi_to_track=PPQ SMF 多轨合并单 clip、不读 tempo meta（导入后需显式 set_tempo 对齐）；**拍谱最短接线=MusicXML→(music21 离线)→MIDI→既有 import 命令，工程侧零改动**。
- ④ UX-AB 复用：audition 机制链生产就绪但候选形态=渲染 wav 对照；乐局两区间对照**可承载**——主路径=start_render 区间渲染(range 参数，TransportAudioService.cpp:1195)×2+复刻 mix_tick mount（webui 零改动，估 1-1.5 天），兜底=注册 transport_set_loop+seek+play 循环段播放（三行级）；推荐兜底先行、主路径随问询 A/B 节点卡。
- 构建块 1 估时修订：**2.5-4 天**（Go 单包 agent/internal/edu/，变换族统一检测核+命令面+判定器同核复用+注册引导；零内核/零侧车/零 Python），较原侧车路线（4-6 天）省 1-2 天。
- 勘测量：agent/internal/tools/catalog.go（注册表+usage+bindings）、capability_pack.go（9 pack 全读）、epm/types.go（全读）、policy/policy.go、queryengine、agentloop/message_loop.go（EPM 消费段）、chat/audition_events.go（锚点）；内核 CommandDispatcher.cpp（music_context/markers/audition/loop/tempo 段）、MidiService.cpp（get_midi_clip_notes/get_midi_clip_data/import_midi_to_track 全读）、TransportAudioService.cpp（set_loop/start_render）、ProjectMarkerService.cpp（list/upsert）；报告 2026-10-22-ab-audition-entry-design.md 全读。
- 边界：只读零改动零真栈零 LLM；工作树他卡改动未触碰；上交决策侧 3 项（拍号写命令缺失 / transport_set_loop policy 分级 / 参照物清单引导落点）见报告尾节。

## 验收

- **结论：pass**（GLM 主管决策流 / 2026-11-05）
- 亲核记录：transport_set_loop 内核注册（CommandDispatcher.cpp:3224→handleSetLoop:553）+ agent 侧零注册（rg 零命中）双半均证；music_context 披露块字段族逐字吻合（:2235-2252，含 tempo_setting_count/time_sig_setting_count）；midiNoteToVar 六字段（MidiService.cpp:310-320）；import_midi_to_track 1784-1935 段零 tempo 命中（不读 meta 属实）；start_render 区间渲染 handler 在位；epm LLMContext 惯例早前已亲核。只读纪律：git status 跟踪面与领取基线一致，零生产改动。
- 验收标准四点齐备：①注册面草案（命令/输入/返回 schema+三处配套）②参照物矩阵+小节坐标三条件结论 ③拍谱最短路径（工程侧零改动）④UX-AB 兜底先行/主路径分层建议；构建块 1 估时修订（2.5-4 天 Go 单包）成立。
- **上交三项处置**：①拍号写命令缺失=挂账（小星星 4/4 不阻塞；非常规拍号教材曲时开内核小卡）；②transport_set_loop 分级=**裁定 RiskDirect**（传输态不改工程内容，与 play/stop 同级），随问询 A/B 兜底注册执行；③参照物清单引导落点=随裁定 §五-15（问询通用化）联动，倾向通用引导。
