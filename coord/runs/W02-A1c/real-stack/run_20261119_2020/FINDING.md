# W02-A1c attempt1 发现（2026-11-19）

- 正式轮 run_20261119_2020：S0/S1/S1.5/S2/S3 PASS（选中 clip 1014 生效）；S4 失败。
- 现象：S4 真实提问（同会话 w02-a1c-20261119_202005）stop_reason=done、reply 非空，但 agent_prompt_render 最后一行（55KB）含 free_state=True、selected_clip，**不含 lesson_context/edu_jam**。
- 上下文快照文件 agent_context_snapshots.jsonl 未新增（stale 2026-10-06）；prompt render 才是本轮模型输入证据。
- 归因：S4 走 free-state active 的 message loop；agentloop.buildModelContextSnapshot 的 neutral 分支（helpers.go:65-95）用 neutralContext 重建 snapshot，未携带 full.LessonContext，因此 A1b 注入在真实模型输入中被丢弃。A1b 单测走 chat.buildAssembly，未覆盖 agentloop neutral 路径。
- 修复方向：neutral 重建后保留 full.LessonContext（并测试 free-state neutral 模型快照含 lesson_context，同时仍剥离 plugin 身份）；文件域扩展 agent/internal/agentloop/helpers.go + test。
- 栈：teardown 全零；recents/authority 还原一致；不原样重跑。
