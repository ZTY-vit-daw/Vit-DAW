# CONV-WINDOW-MOTION-3（431）验收：PASS（零栈，试验值）

- 卡：`2026-11-18-CONV-WINDOW-MOTION-3`（池序 431）；主管=lead；执行=lead/convrail418。
- commit `4d9190d2`（2 文件 +11/−9，只调左栏 token 与断言）；已合入 main `4dee0d62` 并推送。
- `--v5-conv-motion-rail` 320ms → **800ms**（仅 EntryShell 左栏槽内容：工程列表↔对话流）；其余保持 430（右侧 layer 480ms+40ms 错峰、内容体 420ms、ease、reduced-motion）。

## 主管复核（亲跑）

| 项 | 结果 |
|---|---|
| `npm run test` | exit 0；63 files / 755 tests |
| `npm run build` | exit 0 |
| `node tests/render-window-presentation.mjs` | exit 0；126 checks / 0 failed（rail 断言更新为 800ms） |
| 主检出 dist | 重建 SHA256=`FA293DB3B8713F05F27D531F82B5FF7063244F94681CDC9769B402C01397304E` |

## 说明

- 800ms 为超出常规 200–500ms 区间的试验值，用户要求先试；后续按手测单点调 token 即可。
- 零真栈；候用户手测。
