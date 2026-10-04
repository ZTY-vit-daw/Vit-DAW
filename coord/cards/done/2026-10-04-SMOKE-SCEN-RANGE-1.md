# SMOKE-SCEN-RANGE-1：范围线两卡烟测场景——note 载荷时间维度注入+意图层框选拆分提案（dev_agent_smoke 扩展）

- 池序 3（REGION-TIME-1/INTENT-WIRE-1 转正判据卡）；目标仓库=D:\Vit_DAW（agent+scripts）；依据=AGENTS §5 端侧烟测门槛（新场景=现有冒测脚本加参数）+2026-10-04 晚用户质询处置（手测兜底→脚本化）
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无（两卡实现已合 main：4c98c643/2f5e57d1/c2edb0d6）
- 模型分级：L1 / flash 可接（先例=dev_agent_smoke.ps1 场景模式+g_runtime_readonly_smoke 白名单 GET 模式）
- 目标：
  1. **场景 A（REGION-TIME-1 转正判据）**：`dev_agent_smoke.ps1` 新增参数场景——起真实三件套后，POST `/agent/chat` 携带 note 载荷（faces 含 timeline 面 domain.range_time_span+条目 range_clip_start/end+顶层 range_time_span，fixture 构造合法 v3.1 形态）；断言打**组装/投影面**（非 LLM 文本——AGENTS §8 确定性口径）：note 会话落库+遥测/状态面可见 time_digest 注入（领取时实锚断言点：note_sessions 投影/section_stats/等效确定性面）；旧载荷（v3 无时间键）fail-open 零报错同场景断言。
  2. **场景 B（INTENT-WIRE-1 转正判据）**：同栈 POST `/agent/chat` 携带 requestContext.selected_clip_ranges（合法结构 fixture：单 clip 单 range，界在 clip 内）+userText「把这段拆出来」；断言响应提案=**两条 clip.split（先 end 后 start）**；反例组：无 ranges 同话术→零提案（回落既有行为）。
  3. 脚本以 **exit 0** 收口（两场景各成组，-Scenario 参数选择）；run 工件落 coord/runs/SMOKE-SCEN-RANGE-1/。
- 文件域：scripts/dev_agent_smoke.ps1（加参数/场景）+必要 fixture 文件（scripts/fixtures/ 下）；agent 代码零改动（发现断言点缺失属停止条件）。
- 约束：真栈泊位（E2E 7897 即起即拆模式，不占用户栈）；不碰 webui/Godot。
- 验收标准：脚本 exit 0+两场景断言全过+run 工件可回指；决策侧复跑一轮 exit 0。
- 停止条件：断言点实锚发现组装面无确定性暴露（time_digest 只活在 LLM prompt 内部）→ 锚点清单上交，决策侧定断言面或降级口径。
- 领取：2026-10-04 20:30（PC 执行侧，ZCode 会话）/ origin/main=b4af7c60 / 主工作树直做（scripts 单文件域+coord 卡片，PC 惯例直提 main；领取时工作树已有他人改动：VitApp/Workspace/Settings/Settings.xml、default_project.xml，未触碰）
- 回执：
  - **改动**：scripts/dev_agent_smoke.ps1 单文件——新增 `-Scenario note_time|range_split|all` + `-RunArtifactsDir`；场景模式=隔离泊位（7897，拒绝复用已监听栈即 throw，起栈后 finally 拆除 kernel+agent）；默认流程零改动（Scenario 空值时全路径原样）。
  - **场景 A（note_time）= 完成，真栈 exit 0**（run=coord/runs/SMOKE-SCEN-RANGE-1/20261004_210017）：
    - 断言面（全确定性，不碰 LLM 文本）：①`/agent/runtime/status` 的 note_sessions 投影含 v31/legacy 两会话行（messages=2，落库面）；②v3.1 会话 store 文件（.vit_derived/<uuid>/note_sessions.json）faces 携带 range_clip_start/end（v3.1 时间键持久化往返）；③LLM 遥测 JSONL（VIT_AGENT_LLM_TELEMETRY_PATH 注入泊位 agent）两条 source=vitnote_chat 记录 char_count 差值：v31=3297 vs legacy=3067，**delta=230 且跨进程稳定复现**（v3.1 快照段含 time_digest 时间维度、legacy fail-open 只剩身份/全长行）。
    - 泊位就绪门：起栈后轮询 /agent/state 至 shadow.project_uuid 稳定（两次连读一致）才跑场景——note 持久化与投影必须解析同一工程身份（工程 identity=进程内 P0 默认草稿+uuid，进程生命周期稳定）。
    - 旧载荷 fail-open：legacy 轮零报错+正常落库 ✓。
  - **场景 B（range_split）= 脚本建成、判据被证据推翻 → 触发停止条件，本卡转 blocked**：
    - 驱动方式：`disable_agent_loop=true` 走 legacy 包络路径（意图层唯一可达处；agentloop 内无 split 确定性快速意图，生产 Godot 主对话不带 legacy 标志）。
    - 正例断言（形态无关版）：恰两条切分提案、先终点(3.5)后起点(2.0)、clip/track 正确、needs_confirmation+plan_id；tool-form（意图层产物）与 cmd-form（模型包络产物）双形态都解析并记录来源层。
    - **证据链（同二进制同配置三轮）**：
      1. run 20261004_210105：模型纯文本回复→意图层接管→tool-form clip.split 3.5→2.0+needs_confirmation+plan_id，反例组（无 ranges）零提案 ✓——**实现本身端到端工作正常**；
      2. run 20261004_210955（-Scenario all）：模型包络抢先 3/3，cmd-form **先起点(2.0)后终点(3.5)**——裁定书论证过的内核错误序（先切起点后原 clip_id 保留在左半，第二刀 3.5 界外被拒）；
      3. run 20261004_211446：模型抢先 3/3 且同 run 内顺序/形态漂移——attempt1 混合形态（cmd+tool 双键）先终点，attempt2/3 先起点。
    - **核心发现（超出脚本面的产品缺陷）**：意图层坐在模型包络**之后**（`if len(env.Commands)==0` 才兜底，chat/server.go:2630），模型对「把这段拆出来」多数回合会自吐切分方案且顺序随机（先终点/先起点/单刀都出现过）——真实栈上用户确认卡看到的方案约半数是内核会失败的错误序。这直接动摇 INTENT-WIRE-1 的转正手测复验点（手测同样会撞模型随机序）；agent loop 生产路径不经过意图层，问题面更大（模型逐刀调用，第一刀起点后第二刀是否自纠未测）。
    - 反例组：run 210105 反例通过（无 ranges→零提案，模型读命令不构成提案）。
  - **泊位纪律**：每 run 独立工件目录；每轮起拆即净（log 证实 agent/kernel 双停）；未触碰用户栈（预检 throw 拒绝复用）；VitApp/Workspace 内核默认工程零写入（提案面未 approve，note 会话落进程本地草稿 .vit_derived）。
  - **端测覆盖边界声明**：场景 A 全过且确定性；场景 B 断言面无确定性锚（模型包络顺序随机），exit 0 判据在现状下不可交付——上交决策侧定断言面。
  - **提交给决策侧的选项**：
    a.（建议）开 agent 修复卡：把范围切点合成提到 LLM 调用**之前**（ranges+话术匹配即确定性接管，模型包络不再能抢先）——同时根治确认卡随机序缺陷，本卡场景 B 断言即可转严格 tool-form 判据；
    b. 服务端加 `/smoke range_split` 确定性 dev 命令直探意图层（agent 代码小改）；
    c. 接受概率口径（模型先终点=通过）——不建议，违反 AGENTS §8 确定性口径；
    d. INTENT-WIRE-1 转正判据回退手测——手测同样撞随机序，且掩盖产品缺陷，不建议。
  - 断言点实锚清单：A=note_sessions 投影+store 文件时间键+遥测 char_count 差值（三面皆确定性）；B=意图层 tool-form 形态（仅在模型让路时出现，非确定性）。
- 验收：pass——A 判据达成（执行侧两轮+决策侧复跑断言①②过，③证明目标由真栈真 LLM 回复直接证据替代，env 差异记录）；B 停止条件正确行使移交 INTENT-WIRE-FIX-1（[rulings/2026-10-04-SMOKE-SCEN-RANGE-1-pass.md](../../rulings/2026-10-04-SMOKE-SCEN-RANGE-1-pass.md)）
