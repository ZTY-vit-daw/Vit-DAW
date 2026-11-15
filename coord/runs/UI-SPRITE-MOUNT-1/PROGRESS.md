# PROGRESS — UI-SPRITE-MOUNT-1（执行流，PC，GLM-5.3 Flash）

- 卡：coord/cards/todo/2026-11-15-UI-SPRITE-MOUNT-1.md（池序 361）
- worktree：D:/Vit_DAW_wt_uisprite1（分支 port/ui-sprite-1，基线 origin/main=5957e5a2）
- 工件目录：本目录（coord/runs/UI-SPRITE-MOUNT-1/）

## 步骤日志

- [S0] 2026-11-15 开工勘察完成：读了任务卡+PROTOCOL §0/§2；main.tsx 入口确认（D:/Vit_DAW/agent/webui/src/main.tsx，createRoot 渲染 App）；DesignSprite 在 design/sprite.tsx:91；sprite.css 已存在且只含 v5 前缀作用域类（泄漏面低）；渲染烟测=src/gallery/render-smoke.mjs（1180 行）。下一步：领取提交+worktree。
- [S0b] 勘察关键发现（超出卡面定位的补充事实，同缺陷链）：
  1. sprite.css 全仓只被 gallery/main.tsx 导入——真实入口 main.tsx 不导入。只挂载不导入 CSS 会导致 .v5-sprite 裸 svg 以默认 300x150 显示（样式泄漏级故障）。修复=挂载+导入两行都在 main.tsx。
  2. 真实面 LauncherSurface.tsx:487 / ProjectSurface.tsx:346 根节点自带 v5-design 类，sprite.css 的 .v5-icon 契约（作用域 .v5-design .v5-icon）可直接生效。
  3. 渲染烟测只测 gallery.html 五个 fixture 面（各面 gallery/main.tsx 自挂 DesignSprite），从不加载真实入口 index.html——挂载缺失因此逃逸。fixture 面图标计数基线（graphicChecks）：gallery=234、launcher=10、project=56、loading=9；四面 v5-icon 元素均 ≥1（launcher 面经 ProjectBadge→Icon 渲染）。
  4. 各 fixture 面断言已有 sprite mounted/23 symbols（gallery 面）；launcher/project/loading 面有图形对比度扫描但无图标计数断言；fidelity live 面无图形计数断言。
  5. 真实入口离线渲染 .app-shell（默认 surface=full，同步渲染），可在烟测中挂载断言（.v5-sprite 在场+23 symbols+css 生效计算宽度=0px）。
- [S1] 领取完成：卡→doing（领取节回填）+POOL-INDEX 361→doing；提交 b0ecfec5 推 main 成功（5957e5a2..b0ecfec5 fast-forward，含 4 个 coord-only 发卡提交）。worktree D:/Vit_DAW_wt_uisprite1 建立（port/ui-sprite-1 @ 5957e5a2）。下一步：main.tsx 挂载+烟测断言，然后三门。
- [S2 计划] 将要执行的长命令（断线重派时判断是否需重跑）：
  - npm install（worktree agent/webui/ 下——新 worktree 无 node_modules；2026-11-15 首跑发现 vitest 缺失）
  - node --check src/gallery/render-smoke.mjs（已过，SYNTAX_OK）
  - 代码改动已落盘：main.tsx（DesignSprite 挂载+sprite.css 导入）+ render-smoke.mjs（图标计数/非空断言×5 面+真实入口 §11 段+本卡截图）
- [S3] 三门全过：
  - Gate1 npm run test → GATE1_EXIT=0（47 files/570 tests 全过=基线不回退）
  - Gate2 npm run build → GATE2_EXIT=0
  - Gate3 npm run test:render → GATE3_EXIT=0（6 报告全 PASS，含新增 APPENTRY 真实入口段）
- [S4] 负对照实证：临时摘除挂载行复跑 test:render → exit 1，APPENTRY 三断言拦截（spriteCount=0/symbols=0/css=null）；恢复后复跑 GATE3_FINAL_EXIT=0。证明修复前不可拦截的缺陷现已可拦截。
- [S5] 断言实质计数（去空转后首跑）：gallery icons=221/graphics=234；launcher icons=10/graphics=10；project icons=44/graphics=56；loading icons=8/graphics=9；fidelity live 每视口 icons=7-8/graphics=7-11；APPENTRY spriteCount=1/symbols=23/css absolute+0x0/uses=0(主面 lucide,预期)/external=0。
- [S6] 实现提交 59842aa8 推 port/ui-sprite-1 成功（2 files +226/−1，仅 main.tsx+render-smoke.mjs；worktree 内冒测重写的他卡 tracked 工件不随本卡提交）。本卡工件已拷贝主仓 coord/runs/UI-SPRITE-MOUNT-1/（report+4 PNG+PROGRESS）。下一步：回执回填+卡 done+POOL-INDEX+coord 提交推 main。
  - npm run test（worktree agent/webui/ 下；vitest 基线 570）
  - npm run build（worktree agent/webui/ 下）
  - npm run test:render（worktree agent/webui/ 下；自起 vite dev server 127.0.0.1 随机端口+headless chromium；工件写 worktree coord/runs/ 各卡目录，完成后拷贝本卡目录到主仓）
