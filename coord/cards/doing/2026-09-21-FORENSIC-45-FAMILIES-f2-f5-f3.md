# FORENSIC-45-FAMILIES：⑤ 已知族三联取证（F2 settle 中间态 / F5 声学桥竞态 / F3 强制终局不可解析）

- 优先级 / 预估 / 依赖：P1 / 0.5 天 / 素材已齐（FIX-GATE-FRESHNESS-1 三轮工件 + agent 全量 JSONL 日志在位）
- 模型分级：L1-2 / GLM-5.3 可接（只读取证；三族根因机制已初步定位，本卡定分而非修复）
- 背景：FIX-GATE-FRESHNESS-1 真栈验证中 ⑤ 三轮红全部落在修复域外（G 门读路径），三族各自独立；F3 已首次还原完整失败链（2026-09-21 20:18-20:19）。本卡逐族取证定责，修复另开卡。
- 目标：
  1. **F2（settle 中间态）定分**：区分"goal 状态翻转与事件落盘竞态（问句在 but 晚到）"vs"合成取行错误（抓了 item.completed 标题而非 turn 级 reply）"。取证手段：对 conv product_path_vocal_clarify_20260921_201033 与 mix_single_tick_vocal_clarify_20260921_101205 拉全量 events，查是否存在含澄清问句文本的 turn.completed 行；对照脚本 Wait-ChatTurnSettled 的取行规则（scripts/run_vit_product_path_smoke.ps1 settle 合成段）
  2. **F5（声学桥 request_id 竞态）定分**：定位 mixboard 戳记与 kernel_prepared 两条写路径的代码点与更新顺序；用两轮工件（product_path_20260921_104355 / 201633 的 authoritative_mixboard_feature_snapshot.json）比对分叉窗口形态；给出原子化修复建议（单写者或版本围栏），修复另卡
  3. **F3（强制终局不可解析）三问**：a) G4 拒绝是否合法——conv product_path_vocal_clarify_20260921_201633 的 loop 上下文/事件里 diagnostic_rounds 实况（真没闭合 vs 又一例滞后读）；b) 7955 token 最终输出的原始形态（散文/多次 JSON/重复）——从 agent_prompt_render/telemetry/对话存储提取；c) clarify ask 为何三连触发观测循环不闭合诊断轮（提示面检查：歧义 ask 的协议教学是否引导不足）
- §8 预声明：纯只读取证，零代码改动；每族产出"定责结论+证据指针+修复建议单行"；不扩大到修复
- 文件域：coord/reports/（取证报告落此处）；零代码零脚本改动
- 验收标准：①三族各一份定责结论（含证据指针可回溯）；②F3 三问各有答案或明确的不可及边界；③修复建议单行入档供决策开卡
- 停止条件：events/对话存储已被轮转覆盖导致证据不可及 → 如实记录证据边界上交
- 领取：2026-09-21 21:10 / PC 执行侧（GLM-5.3） / origin/main=36531d1d（+开卡 569a9bcf 本地待补推）/ 素材在位（三轮工件 + agent JSONL 全量日志）
- 回执：（阶段稿 2026-09-21 21:40——F3 三问已答，F2 半答，F5 概念定分待代码定位）
  - **F3-Q1（7955 token 原文）——已答，修正此前"作文"误判**：debug 日志 20:19:53 条目的 raw 字段含完整解析后决策——**格式完全合规的优质提案**（needs_experiment + improvement_proposal.v1：目标 Track 1007、+1.0dB track_gain、真实 obs 引用、confidence 0.45、limitations 自述"频段占用与频率分离维度尚未闭合，因此本次不提出任何频段类调整"）。7955 token 含大量推理 token（可见决策仅约 1200 字符）。fallback 的 "unparseable" 措辞失实——决策可解析，是 G4 拒绝后"无可采纳"。
  - **F3-Q2（偶然还是边界设计）——已答：协议结构性陷阱，非模型偶然**。同日同阶段对照：绿轮 103431 同样吃到 final-turn 指令（10:38:14）后模型选 **capability_blocked 诚实边界**（"观测轮次已用尽…下次需补取频率占用与动态观测"）→ 终态族免门 → 采纳 → 绿；红轮 201633 模型选**真提案** → TIMING-1 下锁死轮提案仍走全 G 门 → G4 拒 → 死。**指令给的两个选项中"提案"在 G4 未闭时结构性不可达——只有边界出口真能走通；选择更有用的出口（真提案）反而被罚**。反向激励确认。
  - **F3-Q3（轮次太少）——已答：不是思考时间问题**。调用次数：绿 103431=10 次、红 103919=8、红 201633=8、绿 103115=4——调用数不分红绿；201633 最终调用 51s/7955token 产出了完整答案，缺的不是思考空间而是"让答案被接受的合法路径"（锁死轮禁工具→G4 永不可补）。103431 自述"观测轮次已用尽"仍绿——同样预算下出口选择决定成败。
  - **F2（半答）**：conv 201033 的 debug 显示 20:12:40 G 门拒（G4 族）→ 20:13:35 final-turn 指令 → **之后无任何门/fallback 条目**（模型终输出未形成可解析决策），waiting_clarification 疑来自确定性澄清回退路径（messageLoopFinalFocusTrackClarification 类）；脚本捕获 reply=工具步标题。问句原文是否存在于 turn 级事件→需 durable 对话存储查询（agent 停机后 events API 不可及，工件未存该文件）——**证据边界如实登记**，可用下次真栈运行时补查或读对话持久化文件。
  - **F5（概念定分，代码定位待做）**：用户假设"按合理次序更新"方向正确但不充分——任何固定次序都留分叉窗（先换表头或先换行章各有一个不一致窗口），需要**原子发布**（copy-on-write 快照整体替换或单写者）或读侧一致性围栏。
- 验收：
