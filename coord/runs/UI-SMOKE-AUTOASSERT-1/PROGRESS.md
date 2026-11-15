# UI-SMOKE-AUTOASSERT-1 PROGRESS

卡：coord/cards/todo/2026-11-16-UI-SMOKE-AUTOASSERT-1.md（池序 376，P0 微卡）
工件目录：D:/Vit_DAW/coord/runs/UI-SMOKE-AUTOASSERT-1/

## 领取基线（2026-11-16）

- HEAD: 456b96effd6b8e5a09a36ec1015afa3aa1c0cb2d
- 工作树已有他卡/他腿改动（CURRENT-STATE.md、coord/runs/*、VitApp/Workspace/*、tracktion_engine 等），本卡零触碰；本卡文件域=agent/webui/src/gallery/render-smoke.mjs + coord 工件（卡/池索引/本 run 目录）。
- render-smoke.mjs 领取时工作树状态：待核对（下方步骤 1）

## 步骤

- [x] 1. 读 render-smoke.mjs，定位断言：LAUNCHER directive 断言=727-733 行（旧态 pending/准备打开）；SHADOWBOX 报告 failures=launcherFailures+projectFailures 聚合（1157 行）→修 LAUNCHER 即两面转绿。产品 ground truth（launcherData.ts+LauncherSurface.tsx 778 行+失败报告实测 DOM）：state=auto、headline=正在打开《示例工程甲》…、note 含「已自动经 Vit 桥」；旧手动按钮选择器 data-launcher-directive-execute 已被 SHELL-5（988ab5e4）移除→负断言用之
- [ ] 2. 领取协议：卡 todo→doing 已 mv + POOL-INDEX 376 行已加 + 领取节已回填；提交推 main（待做）
- [ ] 3. 更新断言为 SHELL-5 自动执行态（仅断言，零行为改动）
- [ ] 4. 三门：npm run test（598 基线）→ npm run build → npm run test:render（全段 PASS）
- [ ] 5. 卡转 done + POOL-INDEX + 提交推 main（pull --no-rebase 先同步）

## 长命令记账

- （预留）cd agent/webui && npm run test / npm run build / npm run test:render
