# UI-SPRITE-MOUNT-1：DesignSprite 全局挂载——修复全应用图标不渲染

- 发卡：GLM 主管决策流 / 2026-11-15（用户手测观察：「发送和等待输出都没有 icon」——359 目检后续深查触发）
- 派发确认：待用户触发领取
- 验收负责人：主管决策流 + 用户目检（发送键 i-send/回复中 i-stop 可见）
- 池序 361；P1 / 0.25 天 / 依赖=无；PC；零栈（浏览器渲染面）

## 缺陷（已定位）

- `agent/webui/src/design/sprite.tsx:91` 导出 `DesignSprite()`（symbol 定义挂载组件），**全代码库无任何入口挂载它**——`<use href="#i-send">` 等 sprite 引用全部落空，**全应用所有 v5 sprite 图标渲染为空**（发送键=黑方块无图标；busy 态 i-stop 同空）。
- 连带缺陷：渲染烟测的图标对比度断言（data-graphic ≥3:1）空转——图标为零时无从检查=空过，未拦截本缺陷。
- 附带记录：应用内存在**双图标体系**——v5 组件用 sprite（TextField/MacroCard/gallery），App.tsx 工程面 composer 仍用 lucide 内联图标（`<Send>`/`<Square>`，App.tsx:7096-7106，能渲染）。本卡不强制统一，但须在回执中记录两体系清单供后续裁定。

## 目标

1. 在应用入口挂载 `DesignSprite()`（main.tsx/App 根/LauncherSurface 根——领取时锚定唯一正确位置，一次挂载全树可用）。
2. 渲染烟测图标断言去空转化：断言页面 sprite 图标元素**计数 ≥1** 且对比度检查非空集合，防回归。
3. `npm run test`（vitest）+ `npm run build` + `npm run test:render` 全绿；逐面截图（launcher+project）留工件。

## 非目标

- 图标体系统一（sprite vs lucide）——记录清单，另裁定。
- 磁带 icon 三态动效（SPEC-V5 §7.4）——后续卡候选。

## 文件域

`agent/webui/src/`（挂载点+渲染烟测断言）+ coord 工件；领取时回填精确清单。

## 验收标准

三门全绿+图标计数断言在卷+用户目检发送/停止键图标可见。

## 停止条件

挂载后出现样式冲突（sprite.css 全局泄漏）且非显式选择器可收敛→取证上交。

## 领取/回执/验收

### 领取（2026-11-15）

- owner：GLM-5.3 Flash 执行流（ZCode 免费车道量产子代理，PC / Git Bash）
- 领取时 origin/main：5957e5a2132ff7fc2b0be21b8c9af3ac73f8604b
- 主仓本地 main：5672fc39（=origin/main + 4 个 coord-only 未推提交 358/360/361/362 发卡与裁定，逐提交 show --stat 核对仅含 coord/ 变更；领取提交随其一同 fast-forward 推送）
- 主仓工作树领取时状态（非本卡文件域，不触碰不整理）：
  - tracked 改动：`CURRENT-STATE.md`、`VitApp/Workspace/Settings/Settings.xml`、`VitApp/Workspace/default_project.xml`、`coord/decisions/2026-11-11-legame-demo-experience-first.md`、`tracktion_engine`（submodule 指针，diff --stat 5 files +4464/−4421）
  - untracked：`.vit_derived/`、`.zcode/`、`agent/webui/design/*` 设计稿等
- 执行分支：port/ui-sprite-1（基线=origin/main 5957e5a2）
- worktree：D:/Vit_DAW_wt_uisprite1
- 文件域：`agent/webui/src/main.tsx`（挂载+sprite.css 导入）、`agent/webui/src/design/sprite.tsx`（如需微调，预期不动）、`agent/webui/src/gallery/render-smoke.mjs`（图标计数/非空断言+本卡工件）、coord 工件；禁入：LauncherSurface.tsx、App.tsx、launcher.css、agent/internal/**、scripts/**
- 勘察补充事实（同缺陷链，回执详述）：sprite.css 全仓仅 gallery/main.tsx 导入——真实入口缺 CSS 导入，只挂载不导入会致 .v5-sprite 裸 svg 以默认 300×150 显示；修复=挂载+导入均在 main.tsx。渲染烟测只测 gallery.html fixture 面（各面自挂 sprite），从不加载真实入口 index.html——挂载缺失因此逃逸。

### 回执：待回填（实现 commit / 三门退出码 / 截图工件 / 双图标体系清单 / 覆盖边界）

### 验收：待主管决策流
