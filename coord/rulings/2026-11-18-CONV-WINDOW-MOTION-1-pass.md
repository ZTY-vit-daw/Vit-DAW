# CONV-WINDOW-MOTION-1（429）验收：PASS（零栈）

- 卡：`2026-11-18-CONV-WINDOW-MOTION-1`（池序 429）；主管=lead；执行=lead/convrail418。
- commit `03f8debd`（5 文件 +253/−12，纯 CSS 动效 + 内容模式属性）；已合入 main `ae832888` 并推送。

## 动效清单（computed style 断言）

| 面 | 时长 | 缓动 | 属性 |
|---|---|---|---|
| 右侧 surface 流层（仅 active 入场） | 340ms | cubic-bezier(0.22,0.61,0.36,1) | opacity 0→1 + translateX(10px) |
| 左栏槽内容（工程列表↔对话流） | 240ms | 同曲线 | opacity + translateY(4px) |
| 工程面 thread 内容体（预览/返回/继续） | 300ms | 同曲线 | opacity + translateY(7px) |
| launcher 内容体/线程新条目 | 300ms | 同曲线 | opacity + translateY(6px) |
| reduced-motion | 全部 none | — | — |

- inactive 层立即 hidden（无双影/同时切换）；只动 opacity/transform（无布局跳动）；不与 180ms 收展混淆；纯 CSS 无 JS 定时器。

## 主管复核（亲跑）

| 项 | 结果 |
|---|---|
| `npm run test` | exit 0；63 files / **755 tests** |
| `npm run build` | exit 0 |
| `node tests/render-window-presentation.mjs` | exit 0；**124 checks / 0 failed**（114+10） |
| `npm run test:render` | exit 0；全段 PASS（fixture/gallery 不回归） |
| 主检出 dist | 重建 SHA256=`345417A7EB2C4C38D0EA98BD6F0EF1C7C4A5C7C001A08E960C16085E898CFDD5` |

## 边界

- 未改结构：right surface 仍沿用各自 frame（titlebar/composer），本卡只解决“过快/不自然”；若用户仍觉得像“两个窗口在换”，需 A+ 共享 frame（另卡裁定）。
- 零真栈；候用户手测观感。
