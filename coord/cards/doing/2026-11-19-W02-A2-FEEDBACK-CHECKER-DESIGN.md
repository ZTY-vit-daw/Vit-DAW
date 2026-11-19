# W02-A2：作品反馈与结构检查（设计/契约侦察）

- 池序 433；P1；PC；发卡=lead（2026-11-19 21:05）；派发确认=**待主管/用户确认**；验收负责人=lead。
- 前置：W02-A1 已 done（A1a/A1b/A1c 全 PASS）；374/375 已合入；W18 未建；契约 R1–R6 **v0-frozen**。
- 目标：把 W02 剩余的“对当前作品反馈”和“基本结构检查”切到可执行范围，避免在冻结契约上直接加 schema。

## 审计（基于 A1c 实跑）

1. A1c S4 的问题是「第几关/目标」，LLM 回复引用了当前节点，但**没有针对学生选中 clip 的实际音符做反馈**；lesson_context 只含 selected clip id/track，不含 notes。
2. 编译卡当前节点的 `done_when` 只有 `clip_exists`（n2/n3/n4/n5 同型），没有 notes judge 差分；A1c S3 只能证明“读到了选中 clip”，不能证明“对作品内容判定”。
3. 结构检查（小节范围/时值/休止合法性）需要新的 check 类型或独立 advisory 层；前者触碰冻结 R1–R6，后者可先做披露不做 pass gate。
4. 会话 event log/拉回/恢复已有基础（A1c S5/S6 证了 selected/unmatched/重开），检查记录与“返回修改”尚未形成用户旅程。

## 候选切片

### A2a 作品反馈（零契约变更，优先）
- 在 A1c 场景上把 S4 问题换成**针对选中 clip**的作品问题（如“我选中这段旋律的走向如何？音高还是节奏更值得先改？”）。
- 要求：模型必须通过 `get_midi_clip_notes` 或等价证据读到该 clip 的实际 notes；回复不得在未读 notes 时声称“听/看过”；记录实际工具调用与回复；单次正式 LLM 回合。
- 验收：选中 clip 的 notes 被读取（工具回执/上下文证据）；回复引用至少一个实际音符特征（音高/时值/走向）或诚实说需要更多信息；无假反馈；`lesson_context` 仍在。
- 文件域：device script（扩 A1c 脚本的 question mode）/RUNBOOK；若需产品注入 notes 才另开产品卡。

### A2b 结构检查（需契约裁定，后置）
- 选项 1：advisory 披露（不改变 pass gate），在 edu/编译侧输出小节/时值/休止检查结果；不改 R1–R6。
- 选项 2：新增 check 类型（触碰冻结契约，需用户/主管裁定 + 版本化）。
- 依赖 W18（记谱↔MIDI 纯函数）与 W08 计量；不得用“首个 clip”或模板判过。
- 先写设计/裁定单，不直接实现。

## 建议

先派 **A2a**（真实作品反馈，零契约变更），用 A1c 的栈/工程装置；A2b 单独立裁定卡，不把结构检查硬塞进当前卡。

## 验收/停止

- A2a 真栈 exit 0 + 选中 clip notes 读取证据 + 回复不伪；A2b 只在用户/主管对契约选项裁定后开工。
- 若模型在未读 notes 时自宣“看过/听过”，立即停手，不上交为 PASS。

## A2a 领取（2026-11-19 21:08）

- owner=`w02fb`（DeepSeek-V4.1-Flash，PC，独立 worktree）；分支 `codex/w02-a2a-feedback-433`；worktree `C:/Users/timoz/.codex/worktrees/pc433-feedback/Vit_DAW`；基线 main `2794d135`。
- 本轮只扩 A1c 真栈脚本的 work-question 模式 + RUNBOOK；不启栈、不改产品代码。
- 场景：复用 A1c S0–S3（导入/开局/双 clip/选中 second），S4W 发送针对选中 clip 的作品问题；要求模型通过 `get_midi_clip_notes` 或等价证据读到实际 notes，回复引用至少一个实际音符特征，或诚实说需要更多信息；未读 notes 不得声称看过。默认 status 模式保持 A1c 原语义。
- 文件域：`scripts/run_w02_a1c_smoke.ps1`、`coord/runs/W02-A1c/RUNBOOK.md`、`coord/runs/W02-A2a/`；不改产品/设计源/其他脚本。
- 验收：PowerShell 解析、无授权 exit 3、dry-run exit 0；不启栈；真实 LLM 单次正式回合留给 lead 跑；回执≤15行。

