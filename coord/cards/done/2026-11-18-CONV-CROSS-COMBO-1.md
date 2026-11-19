# CONV-CROSS-COMBO-1（422）：B2b 跨工作树真栈组合验证

- 池序 422；P1；真栈卡；发卡=本会话 PC 主管（lead）；**done（2026-11-19 真栈 PASS）**（421/419 已合入 main，UI 零栈验收已 PASS）。
- 目标：在真实栈（kernel+agent，必要时 Godot DAW）验证跨工作树完整旅程：
  1. 真实工程创建/保存后创建第二工作树副本；
  2. 跨越工作树只读浏览（`version.status` 按 project_path，不触发 kernel/DAW 切换）；
  3. 点【打开此工作树并继续此对话】→ `version.conversation_open` → 断言内核工程路径切到目标工作树、目标对话节点/分支 active、单次 `open_project` 重载、DAW 状态一致；
  4. 失败语义：未知/失效工作树、reload 失败、target_materialized 披露；recents 快照还原与 teardown 全零。
- 前置：栈位登记+书面授权；隔离 fixture 工程与工作树副本（真实栈正常入口/历史工具创建）；runbook 与工件契约先写后跑。
- 验收：真实栈脚本 exit 0 + 断言工件（目标工程路径/节点/分支、单次 reload 计数、失败分类）；recents 还原哈希亲证；零 LLM。
- 边界：不重开 421 UI 卡；不改产品代码，除非缺陷取证后另开修复卡。

## 领取（2026-11-19 18:42）

- owner=`combo422`（DeepSeek-V4.1-Flash，PC，独立 worktree）；分支 `codex/conv-cross-combo-422`；worktree `C:/Users/timoz/.codex/worktrees/pc422-combo/Vit_DAW`；基线 main `b9c9d943`。
- 本轮范围：runbook + 脚本 + 静态/干跑证据；**不启真栈、不改产品代码**；真栈轮候 lead 登记。
- 文件域：`coord/runs/CONV-CROSS-COMBO-1/`、`scripts/`（仅新脚本）；回执≤15行。

## 验收（2026-11-19 19:13）

- 正式轮：`run_20261119_1928`，`summary.status=passed`，166 断言 0 失败，S0–S6 全 true，`single_reload=1`，`target_materialized=true`；recents/authority 还原；零 LLM；teardown 全零。
- 装置：分支 commits `80c03f12`/`b971b1cd`/`218dff3c`；合入 main `99b21d1b`/`d6ee2742`/`f6d33a98`。
- 轮次：1905 S1 transient workspace warning（装置）→ `b971b1cd`；1920 S4 `worktree_is_root` 字段（装置）→ `218dff3c`；1928 PASS。
- 契约更正：`version.status` 参数 `project_path`；`conversation_open` 根判据顶层 `worktree_is_root`。
- 边界：reload 失败 / `target_materialized=unknown` / GUI 像素 / 真人 chat 节点未覆盖。
- 裁定：[pass](../../rulings/2026-11-19-CONV-CROSS-COMBO-1-pass.md)；工件：[summary](../../runs/CONV-CROSS-COMBO-1/real-stack/run_20261119_1928/summary.json)。


