# CONV-RAIL-CROSS-1（421，B2b）验收：PASS（零栈）

- 卡：`2026-11-18-CONV-RAIL-CROSS-1`（池序 421）；主管=lead；执行=lead/convrail418。
- commit `3c2e7775`（6 文件 +1464/−165；含 `conversationCross.test.ts` 9 纯函数测试）；已合入 main `7cea611f` 并推送。

## 交付

- 左栏工作区→工作树→分支：其他工作树折叠组（名称/路径/head），展开懒加载 `version.status`（一次只取一次，手动刷新显式重取）；失败态 + 重试。
- 分支归属：active_branch 所有者 > head==worktree head > 图证据 > 当前工作树连续性 > 工作区「其他分支」只读组；每分支恰出现一次。
- 只读预览 + 蒙版；当前工作树=【继续此对话】(version.checkout)；其它工作树=【打开此工作树并继续此对话】→ confirm → `version.conversation_open {project_file_path, branch, node_id}`；loading/失败保留预览并披露 target_materialized/branch_warning；成功刷新 uiState、左栏切到新 active 并清预览。

## 主管复核（亲跑）

| 项 | 结果 |
|---|---|
| `npm run test` | exit 0；63 files / **755 tests**（746 + 9 cross 纯函数） |
| `npm run build` | exit 0 |
| `node tests/render-window-presentation.mjs` | exit 0；**101 checks / 0 failed**（既有 75 + 19 cross + 7 hotfix） |
| `npm run test:render` | exit 0；全段 PASS，fixture/gallery 零改 |
| mock invoke | version.status（展开/刷新/失败重试）与 version.conversation_open（worktree+branch+head node）实参已断言；浏览零写调用 |

## 边界/后续

- 零真栈（Playwright+mock）；**422 CONV-CROSS-COMBO-1 真栈组合验证待做**：真实栈创建/打开工作树副本→跨树浏览→【打开此工作树并继续此对话】→断言内核工程路径/节点切换、单次 reload、失败语义。
- 用户手测：dist 已重建 `8D4D25F0…`，可刷新验证 427 的两个 bug 与 421 跨工作树 UI。
- 分支归属在异常历史（head 多命中/图缺失）按测试规则 fail-honest，不猜。
