# UI-FOUNDATION-1：v5.1 令牌 / 图标 / 组件基座 + 浏览器渲染烟测

- 发卡：主设计会话（lead）/ 2026-11-14（用户指令：新 UI 设计任务进池）
- 派发确认：已确认（用户 2026-11-14 指令；主管可调整）
- 验收负责人：用户 + 主管决策流
- 池序 344；P0/P1 / 2–3 天 / 依赖=设计交接与图标定稿；WebUI 单仓；**不需要真实栈**，可 6pm 起

## 目标

1. 把 v5.1 设计落成 WebUI 基座（不接 `App.tsx` 业务）：
   - `design-tokens-v5.json` → CSS 变量（含 v5.1 depth 令牌）；
   - `round2-sprite.svg` 23 symbol → 内联雪碧图/组件；
   - S3/S4/S5 的 primitives：按钮、输入框、上下文目标、宏卡（默认/试听/已应用/失败）、状态环（待机 3/4 虚线、进行中 3/4 实线、完成、出错）、消息/状态卡/动作卡、磁带小标志 `i-project` 与 V `i-vit`。
2. 新增组件 gallery（独立路由/页面），用真实 props 展示全部状态、16/20/24px 图标与浅/深/选中底面。
3. 建立**浏览器渲染烟测**（Playwright；Node 或复用本机 Playwright）：DOM 断言 + 1440×900 截图 + 对比度/裁切/外部资源检查，作为后续 UI 卡的验收底座（E2E-WEBUI 最小面）。
4. 不改现有 `App.tsx` 行为；只新增模块与 gallery；不硬编码示例工程冒充真实数据。

## 文件域

`agent/webui/src/design/**`（新增）、`agent/webui/src/components/**`（新增）、`agent/webui/src/styles/tokens.css`（新增）、`agent/webui/src/gallery/**`（新增）、`agent/webui/package.json`（仅 scripts/devDeps）、`agent/webui/public/**`（图标静态资产，如需要）。**不修改** `App.tsx`、现有 `styles.css` 业务规则、agent/Godot。
- **域扩展（主管 2026-11-14 晚窗授权）**：gallery 独立入口可新增 `agent/webui/gallery.html`（vite 多页入口）+ `agent/webui/vite.config.ts` 的**仅 `rollupOptions.input` 增量**（不得动其他配置项）；回执注明用了本扩展。

## 并行与资源

不需要真实栈；不占 PC 栈 owner。与 `UI-HOST-1` 文件域不相交，可并行。

## 验收标准

- `cd agent/webui && npm run test`、`npm run build` 退出 0。
- 新增 `npm run test:render`（或同模式脚本）退出 0：gallery DOM 断言 + 截图；无外部请求、无裁切/溢出、最小对比度 ≥4.5:1（正文）/3:1（图形）。
- 23 symbol 全在；16/20/24px 与 S3 矩阵一致；V 只在 OS 身份样例出现，不进入标题栏样例。
- 设计文件哈希/令牌值可回指 `design-tokens-v5.json` / `round2-sprite.svg`。

## 停止条件

- 必须改 `App.tsx` 或既有业务样式才能落地 → 停，先出最小 surface 路由方案，不擅自改。
- 需要真实 agent/工程数据 → 用明确 fixture 并标注；不得把 fixture 当真实数据。

## 领取

- 时间：2026-11-14（晚窗）
- origin/main hash：a4759235136725229d48406659dccf37ed9ed8f7（fetch 复核；卡面基线一致）
- owner：freeflash 免费车道执行子代理（GLM-5.3 Flash / ZCode）· PC · 主管会话派发（用户指令：新 UI 设计任务进池）
- 分支：port/ui-foundation-1
- worktree：D:/Vit_DAW_wt_uifnd1
- 领取提交：见本卡 doing 状态提交（coord-only，推 HEAD:main）
- 设计源哈希回执：design-tokens-v5.json = 1d985d5e7dd41bec5f7260ca16b6ed8dc3f29ca701bdc70fc2b9391e380da4ca；round2-sprite.svg = 746a49f5b7e42703ca030981a51044ef49449d838e0435f024437495ece5d981（与 CHANGELOG-V5.md v5.1 图标定稿记录一致）
- 并行核对：doing 卡（ARRLOOP-AUTHBRIDGE-1 / CLOSURE-ROUTE-ADAPT-1 / WAVESHELL-LOADING-FORENSICS-1）文件域均不涉及 agent/webui，无冲突；UI-HOST-1 仍在 todo，文件域不相交

## 回执

- 执行侧自验：PASS（2026-11-14 晚窗，freeflash 免费车道子代理）
- 实现分支：port/ui-foundation-1（已推 origin；head = 351cf26b；领取提交 5e9f2766；基线 a4759235）
  - 61bfdcf1 基座：tokens.css / 23 symbol 内联雪碧图 / S3·S4·S5 primitives / gallery 多页入口 / 渲染烟测脚手架
  - 351cf26b 烟测达标修正 + 工件入库（对比度修正、V 标断言口径、工件入库）
- 测试命令与退出码（worktree D:/Vit_DAW_wt_uifnd1/agent/webui 内按序复跑）：
  - `npm run test` → **0**（40 文件 / 481 测试全过，含新增 sprite.test.tsx 10 项：23 symbol 逐字比对 + 令牌回指 + 语义红线）
  - `npm run build` → **0**（tsc --noEmit + vite build；双入口 index.html + gallery.html 均产出）
  - `npm run test:render` → **0**（新增验收底座：vite dev 自起 + Playwright chromium headless，viewport 1440×900@1 + reduced-motion）——PASS 明细：23 symbol 全在；图标矩阵 16/20/24 × 浅底/深底/选中全齐（23×7 样张）；正文最低对比度 **5.077:1**（与设计自验值一致）；图形最低 **4.305:1**（227 项 ≥3:1 检查）；外部请求 **0**；裁切/横向溢出 **0**；最小字号 13px；reduced-motion 停转；V 标不进标题栏样例
- 截图与报告工件：`coord/runs/UI-FOUNDATION-1/render-smoke-report.json`、`coord/runs/UI-FOUNDATION-1/screens/gallery-viewport-1440x900.png`、`screens/gallery-full.png`（入库分支 = port/ui-foundation-1）
- 设计源回指：design-tokens-v5.json = 1d985d5e7dd4…；round2-sprite.svg = 746a49f5b7e4…（与 CHANGELOG-V5.md「图标定稿 v5.1」记录一致；sprite.test.tsx 对 23 个 symbol markup 做逐字包含断言；tokens.css 令牌值逐项回指 JSON）
- 域扩展使用声明：**已使用**主管 2026-11-14 晚窗授权扩展——新增 `agent/webui/gallery.html` + `vite.config.ts` 仅 `rollupOptions.input` 增量（base/plugins/server 等其余配置未动）
- 改动面核对：`App.tsx`、现有 `styles.css` 业务规则、agent/、Godot 均零改动；仅新增文件 + package.json（scripts.test:render + devDeps playwright@1.65.0、@types/node）+ package-lock（随 devDeps）+ vite.config.ts（input）
- 端测覆盖边界声明：本卡不需要真实栈（卡面明确）。**已覆盖** = 浏览器渲染面烟测（DOM 断言 + 对比度 + 裁切 + 外部请求 + 1440×900 截图，`npm run test:render` 退出 0，可作为后续 UI 卡的验收底座）。**未覆盖** = 真实三件套栈烟测（本卡不适用）、webui 主 App 渲染面与用户旅程（App.tsx 未接本基座；gallery 为独立入口 /app/gallery.html，主 App 行为零变化）
- 停止条件：未触发（未需改 App.tsx 或既有业务样式；gallery 全部工程名/消息/数据带 fixture 标注，无冒充真实数据）
- 遗留/边界：宏「试听取消」、A/B 开始方式等行为语义按 IMPLEMENTATION-HANDOFF.md 留待后续切片；i-vit 在 S3 矩阵中的样张属资产文档（渲染烟测的 V 标断言已区分「OS 身份样例」与「矩阵样张」）；Playwright 1.65.1 需 build 1255 浏览器而本机缓存为 1254，已钉 1.65.0 复用本机浏览器（环境事实，记录备查）

## 验收

- 裁定文件 / 验收 commit：待回填（验收负责人：用户 + 主管决策流）
