# 2026-09-30：便签池 P1 拍板（用户裁定）+ M1 复验中断记录

## P1 拍板：便签池 v1 = A+B 都要

- 用户原话锚："是的A+B都是需要的"（2026-09-30 晚窗，对设计 §9 的答复）。
- 落地口径：v1 = 档 A 锚点小标签（空间语义本体）+ 档 B 侧边总列表**最简版**（分组列表+点击重开；搜索/批量管理推 v1.x）——即设计 §9 建议方案。
- 影响面：VITNOTE-IMPL-6（视觉卡）按此执行；docs/VITNOTE_V1_DESIGN.md §9/§13 同步更新。

## M1 复验中断记录（2026-09-30 20:0x 手测）

- 用户手测中报告："audition 轨迹还是存在而且发生错误，而且执行出现了报错"；活栈只读取证（现场未关）结论：
  1. **主缺陷（新）**：该轮（goal_4e4c14a93473a1e2 / run_bea18cde3304d937，草稿工程 Unsaved.vit，用户问"检查当前工程有什么问题"）status=failed——`durable continuation failed: minimal audio closure controller failed: illegal phase transition fs7_improvement_proposal -> fs9_terminal`（d1 欠账结算尾段）。前序：内核已渲染 audition A/B 预览（candidate-a/b）；20:09:40 free_state final gate 拒绝 LLM 的二次 needs_experiment 准入（要求先出 settle report），其后调试流静默、轮失败。
  2. **代码锚**：internal/audioclosure/phase.go:93——fs7 合法后继={fs8_experiment_verification, fs4_diagnostic_round}，fs7→fs9 被校验器（:148）正确拒绝；**缺陷在 d1 欠账结算续跑路径请求了非法直达终态**。取证卡 FS-SETTLE-TERMINAL-1 已开。
  3. **audition 轨迹判定**：FIX-AUDITION-TRAIL-1 验收条件（处理中 lane 按族身份排除）**无回归证据**；"轨迹存在+报错"是轮失败的下游表现（轨迹行随失败轮停留在错误态），根因归 FS-SETTLE-TERMINAL-1，不立轨迹返工卡。
  4. **次要发现**：authority restore INFO 日志每秒 3 条连刷（agent_last.log 全 500 行、2 分钟窗口）——轮询面每次 GET 都打 restore 日志。开卡 AUTH-RESTORE-LOGSPAM-1（P3）。
  5. **M1 复验完成度**：audition 轨迹项=被新缺陷打断未有效复验；确认卡两项（full access 直执/过期收卡）本轮未见用户申报结论——复验顺延，与 FS-SETTLE-TERMINAL-1 修复后同场。
- 证据固化：coord/runs/M1-RETEST-20260930/evidence/（ui-state-failed.json / final-gate-error.jsonl / kernel-audition-sessions.txt / authority-logspam-count.txt）。
