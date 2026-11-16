# UI-FOUNDATION-1 PROGRESS（freeflash 免费车道子代理 · PC · ZCode）

- 卡：coord/cards/done/2026-11-14-UI-FOUNDATION-1.md（池序 344，P0/P1）——**终态：DONE（执行侧自验 PASS，待决策验收）**
- 基线：origin/main = a4759235（领取时）；分支 port/ui-foundation-1（远端 head=351cf26b）；worktree=D:/Vit_DAW_wt_uifnd1
- 卡片状态提交：领取 5e9f2766 → 收口 6a677935（均推 main，coord-only）

## 步骤

- [x] S0 领取：fetch 复核 / worktree / 领取提交推送（5e9f2766，远端 owner 已复核）
- [x] S1 令牌与雪碧图：tokens.css（--v5- 前缀全量令牌含 v5.1 depth + depthComputed）+ 23 symbol 逐字内联组件（sprite.tsx/sprite.css）
- [x] S2 primitives：按钮/输入框/上下文目标/宏卡四态/状态环四态/状态卡执行轨迹/动作卡三后果/消息/快照回执/i-project/i-vit（src/components/ 共 12 文件）
- [x] S3 gallery 页 + vite 多页入口（gallery.html + vite.config.ts 仅 rollupOptions.input，主管域扩展已用并声明）
- [x] S4 浏览器渲染烟测：npm run test:render 退出 0（DOM 断言+1440×900 截图+对比度/裁切/外部请求/reduced-motion 检查）
- [x] S5 自验回执：三条 npm 命令按序复跑全 0 → 卡面回填 → doing→done（6a677935 推 main）

## 最终自验（worktree 内按序复跑）

| 命令 | 退出码 | 结果 |
|---|---|---|
| npm run test | 0 | 40 文件 / 481 测试全过（含新增 sprite.test.tsx 10 项） |
| npm run build | 0 | tsc + vite build，index.html + gallery.html 双入口 |
| npm run test:render | 0 | PASS：23 symbol、23×7 矩阵、textMin 5.077:1、graphicMin 4.305:1（227 项）、外部请求 0、裁切 0、字号 ≥13px、reduced-motion 停转、V 标不进标题栏 |

- 工件（入库于 port/ui-foundation-1）：coord/runs/UI-FOUNDATION-1/render-smoke-report.json + screens/gallery-viewport-1440x900.png + screens/gallery-full.png
- 设计源哈希：design-tokens-v5.json=1d985d5e…；round2-sprite.svg=746a49f5…（与 CHANGELOG v5.1 定稿一致）

## 关键决策记录

1. CSS 变量统一 `--v5-` 前缀，避免与现行 styles.css 变量名冲突；冻结 markup 内联 var(--icon-bg) 经每元素别名映射解析。
2. 雪碧图 23 symbol markup 逐字冻结（sprite.test.tsx 逐字包含断言）；class 契约 CSS 语义逐字来自 v5-system.html，作用域 .v5-design。
3. 深底状态环样张 = 默认纸色（S3 矩阵口径）；语义色只走实心状态点（红/蓝细线深底不达 3:1，SPEC §4.4）。
4. 黄底 chip/旗标内图标用墨色（SPEC §4.4：黄只做底色）。
5. V 标断言口径：S3 矩阵为资产文档允许 i-vit 样张；组件样例中 i-vit 仅 OS 身份样例；标题栏禁止。
6. Playwright 钉 1.65.0 复用本机 chromium_headless_shell-1254（1.65.1 需 1255，207MB 下载网络过慢中止，按卡面"优先复用本机"处理）。
7. 烟测脚本 vite dev server 显式 host=127.0.0.1 + port 5391（port 0 被 vite falsy 回落默认端口导致拒连）。

## 失败/环境中断记录

1. `npx playwright install chromium`（1.65.1）网络下载 207MB 跑 25 分钟仅到 50%，中止转方案：钉 1.65.0 复用本机缓存（环境中断，不改验收标准）。
2. test:render 首跑 ERR_CONNECTION_REFUSED：vite 对 port:0 的 falsy 处理 + localhost 解析问题，显式 host/port 后解决。
3. 三处对比度失败（黄底图标/失败旗标/深底环）+ 两处烟测脚本参照 bug（border 模式误含自底色）——均为本卡文件域内修正，修正后全过。
4. 收口 push 被拒（main 前进：UI-HOST-1 done + 342 收口）——按 §2.1.3 fetch 重读，确认无同卡竞争、agent/webui 零重叠后 cherry-pick coord 提交重推成功（6a677935）。

## 断点续跑信息（若重派）

- 本文件为终态 DONE。全部产物已入库：实现分支 port/ui-foundation-1（远端 351cf26b），卡片 done/（main 6a677935）。
- 无未完成步骤。剩余事项归决策侧：最终验收（ruling）与实现合入。
