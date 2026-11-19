# TUTOR-SIM-2：工程打开自动开场与 UI 展示

- 池序 436；P0；PC；发卡=lead（2026-11-19 22:35）；派发确认=用户「一路执行」；验收负责人=lead。
- 前置：435 TUTOR-SIM-1 done（/agent/tutor/start|status、/agent/chat tutor 分支、真实 flash smoke PASS）。
- 目标：用户打开含 .vit_agent/tutor_sim.yaml 的工程后，ProjectSurface **自动**调用 /agent/tutor/start（无按钮/无命令词），把 LLM 开场显示在对话流；输入框后续消息用返回的 conversation_id 走 /agent/chat tutor 分支；普通工程 start 返回 unavailable 时不显示、不改变现有行为。
- 范围：仅 UI 接线 + 测试；不做状态/进度卡、问题/选择控件、谱面/音频、生产 YAML 节点 brief。
- 文件域：agent/webui/src/project/ProjectSurface.tsx、projectService.ts、projectData.ts（如需）、lib/api.ts、对应 test；不改 agent/server/实验 YAML。
- 验收：vitest 覆盖——active 工程且 start=ok：start 恰好一次、开场消息渲染、composer 发送用 tutor conversation_id；start=unavailable：不显示、不报错；非 active/普通工程零请求或静默；无命令词；npm run test + npm run build exit 0。
- 停止：若 ProjectSurface 需要大改聊天状态机/新增服务协议，停手上报；不把自动开场硬塞进普通 chat。
