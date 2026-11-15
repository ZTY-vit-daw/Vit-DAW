# UI-SMOKE-AUTOASSERT-1：渲染烟测断言对齐 SHELL-5 自动执行行为（微修）

- 发卡：GLM 主管决策流 / 2026-11-16 凌晨（根因=主管验收缺口记录在案：SHEE-5/367 验收未跑 test:render+栅栏禁入 render-smoke.mjs 致断言漂移；进度驱动无工期）
- 派发确认：已确认（主管发卡，**紧急**——374 M5 与后续所有 webui 卡的三门被此红阻塞）
- 验收负责人：主管决策流
- 池序 376；P0（微卡）；依赖=无；PC；零栈

## 缺陷

main 的 `npm run test:render` 两面 FAIL（LAUNCHER+SHADOWBOX）：fixture 断言期待 directive 旧态「待执行占位（准备打开…，不虚报已打开）+手动执行按钮」，SHELL-5 已改为**自动执行**（`state=auto`、headline「正在打开《X》…」、note「已自动经 Vit 桥发出…」）。产品行为=SHELL-5 用户裁定口径（正确），断言过时。

## 目标

1. `agent/webui/src/gallery/render-smoke.mjs` 中 LAUNCHER 面与 SHADOWBOX 面的 directive 断言更新为自动执行态期望（state=auto/执行语态 headline/note；无「让 Vit 打开」手动按钮的负断言可加）。
2. 三门全绿：npm run test（598 基线）+build+**test:render 全段 PASS（两面转绿）**。

## 非目标

- 其它任何行为/样式改动；374 的 scoreworkbench 面。

## 文件域

仅 `agent/webui/src/gallery/render-smoke.mjs`+coord 工件。

## 验收标准

test:render 全段 PASS 亲证（主管复跑）+零行为改动（diff 仅断言）。

## 领取

- 领取（2026-11-16，GLM-5.3 flash 执行子代理 / PC 主检出 / 零栈微卡直做）：owner=本执行子代理，基线 HEAD 456b96effd6b8e5a09a36ec1015afa3aa1c0cb2d；工作树已有他卡改动（CURRENT-STATE.md、coord/runs/*、VitApp/Workspace/* 等）零触碰，本卡文件域=render-smoke.mjs 断言+coord 工件。领取时 render-smoke.mjs 工作树干净（无未提交改动）。工件目录=coord/runs/UI-SMOKE-AUTOASSERT-1/（PROGRESS.md 断点续跑账）。

## 回执/验收：待回填
