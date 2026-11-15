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

## 领取/回执/验收：待回填
