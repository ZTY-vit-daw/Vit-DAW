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

- 时间 / origin/main hash / owner 模型+机器+会话 / 分支 / worktree / 领取提交：待回填

## 回执

- commit hash / 测试命令与退出码 / 截图工件路径 / 端测覆盖边界声明：待回填

## 验收

- 裁定文件 / 验收 commit：待回填
