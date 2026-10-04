# Ruling：WEBUI-SESSION-SEMANTICS-1 —— conditional pass（2026-10-04，PC 决策侧）

## 裁定

**conditional pass**。机制面三件套全过；转正挂**用户真栈目检复验**（启动侧边栏出现「主对话流 · <工程名>」；note 输入后 webui 新建会话输入框上方不再残留旧轨迹）。

## 验收依据

1. **diff 直读**（`45840390`，8 文件 +548/-10）：
   - 命名协议落地：身份键 `ask_vit_session_main.v1:<scope-slug>` 每 scope 持久（fail-open：损坏/无 window 读空、空 id 不吞旧值、写失败 best-effort）+`mainConversationDefaultTitle` 裁定 1 确定性模板（未保存退化「主对话流」）+App scope 物化三分支（initial/evolution 采纳/迁移）首见即钉（幂等，新建/切换不触碰主身份）。
   - upsert 语义核实：`sessionFlow.ts:129` `input.title !== undefined && entry.title === ""` ——**只填未命名行**，用户改名最高且持久属实。
   - 轨迹绑定：`reduceTaskTrajectoryForConversation` 按 `task.conversation_id` 归属裁决（异会话即时清空/载荷缺位保留现状/monotonic 单一出处）+App `refreshState` 换用（依赖补 conversationID，切换后下一拍即裁决）+PlanBar `planBarOwnLive` 门控（异会话任务在跑兜底置空、栏随快照清空立即卸载）。
   - 文件域守住 webui：Go 侧零改动（投影本就携带 conversation_id）。
2. **我方独立复跑**（[verify_pc](../runs/WEBUI-SESSION-SEMANTICS-1/verify_pc/)）：vitest **435/435**+`npm run build` exit 0（worktree）；**E2E wrapper 复轮 `WEBUI_RENDERED_DOM_VERDICT pass`**（我方 20261004_113457，SS1-SS3 全 True，隔离 7897 即起即拆 port_released）。
3. **工件亲读**：执行侧连续两轮 verdict=pass（111745/112210）report 内 `session-semantics-SS1/SS2/SS3` pass 且附实锚（SS1 主流行「主对话流 · Unsaved.vit」、SS2 切 note 清场、SS3 切回重绑）。
4. **事故披露核实**：主树 `scripts/webui_rendered_dom_smoke.mjs` 误写已还原——主树 scripts/ diff 为零，属实。

## 边界声明确认（如实，不阻塞转正）

①全局 runtime 投影同一时刻只报一个 goal：并发双任务时属主快照可能被顶掉，主流 PlanBar 诚实清空（按会话查询需 agent 端点扩展，越域未做）；②注册表跨栈遗留未命名行不回填命名（fail-open 零回退）；③E2E note 行为网络层注入，note 真链路归 NOTESTREAM 面。

## 归档与配套

合 main=**2f5e57d1**（--no-ff，实现 commit 45840390 执行侧已推 port 分支）；主树 webui dist 11:39:53 收口重建（用户目检直接可用）。

## 转正条件

用户真栈目检（Godot 拉起）两点过 → 转正 pass；不过 → 取证卡。
