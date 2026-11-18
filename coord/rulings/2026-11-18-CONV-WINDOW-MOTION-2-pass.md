# CONV-WINDOW-MOTION-2（430）验收：PASS（零栈）

- 卡：`2026-11-18-CONV-WINDOW-MOTION-2`（池序 430）；主管=lead；执行=lead/convrail418。
- commit `e2ce71ef`（4 文件 +90/−45，纯 CSS 时长/缓动 + token）；已合入 main `c7195e18` 并推送。

## 动效（computed style 断言）

| 令牌 | 值 | 用途 |
|---|---|---|
| `--v5-conv-motion-layer` | 480ms | 右侧 surface 层 active 入场（400–500ms 上沿） |
| `--v5-conv-motion-rail` | 320ms | 左栏槽内容（工程列表↔对话流） |
| `--v5-conv-motion-content` | 420ms | 工程 thread 内容体/launcher 内容体 |
| `--v5-conv-motion-layer-delay` | 40ms | 右侧比左栏晚 40ms 错峰 |
| `--v5-conv-motion-ease` | cubic-bezier(0.05,0.7,0.1,1) | emphasized-decelerate |

- 右侧层 opacity 0.15→1 + translateX(12px)，`animation-fill-mode: backwards`；inactive 立即 hidden（无双影）；左栏/内容体只动各自内容；reduced-motion 全关。

## 主管复核（亲跑）

| 项 | 结果 |
|---|---|
| `npm run test` | exit 0；63 files / **755 tests** |
| `npm run build` | exit 0 |
| `node tests/render-window-presentation.mjs` | exit 0；**126 checks / 0 failed** |
| `npm run test:render` | exit 0；全段 PASS（fixture/gallery 不回归） |
| 主检出 dist | 重建 SHA256=`1F517BCC3A0608D93BCA60F80DFAAFD75337948A4DB61D21E8DBB91AA1CE2C29` |

## 边界

- 仍是 A 方案结构（未统一 titlebar/composer）；本卡在业界 400–500ms 上沿内放慢。若用户仍觉得快，下一步按人测选择：~550ms 层级或“动效速度”偏好（标准/慢速），不再全局无限加长。
- 零真栈；候用户手测。
