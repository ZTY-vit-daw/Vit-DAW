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
- 领取：
- 回执：
- 验收：
