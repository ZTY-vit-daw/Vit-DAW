# CONV-CROSS-COMBO-1（422）：B2b 跨工作树真栈组合验证

- 池序 422；P1；真栈卡；发卡=本会话 PC 主管（lead）；**todo 待栈窗口**（421/419 已合入 main，UI 零栈验收已 PASS）。
- 目标：在真实栈（kernel+agent，必要时 Godot DAW）验证跨工作树完整旅程：
  1. 真实工程创建/保存后创建第二工作树副本；
  2. 跨越工作树只读浏览（`version.status` 按 project_file_path，不触发 kernel/DAW 切换）；
  3. 点【打开此工作树并继续此对话】→ `version.conversation_open` → 断言内核工程路径切到目标工作树、目标对话节点/分支 active、单次 `open_project` 重载、DAW 状态一致；
  4. 失败语义：未知/失效工作树、reload 失败、target_materialized 披露；recents 快照还原与 teardown 全零。
- 前置：栈位登记+书面授权；隔离 fixture 工程与工作树副本（真实栈正常入口/历史工具创建）；runbook 与工件契约先写后跑。
- 验收：真实栈脚本 exit 0 + 断言工件（目标工程路径/节点/分支、单次 reload 计数、失败分类）；recents 还原哈希亲证；零 LLM。
- 边界：不重开 421 UI 卡；不改产品代码，除非缺陷取证后另开修复卡。
