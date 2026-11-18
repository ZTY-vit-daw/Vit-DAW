# SHELL-ENTRY-RAIL-1（B1）验收：PASS（零栈结构面）

- 卡：`2026-11-18-SHELL-ENTRY-RAIL-1`（池序 417）；主管=lead。
- 执行：`lead/railb1`（DeepSeek-V4.1-Flash，PC）；分支 `codex/shell-entry-rail-417`，commit `d231d67e85ef3a5cac5a41c7bdd130d28988d9e7`（父 4826e5ea）；已合入主仓 main `f529e3f25ae84148e755ebe844a0d7c262249475` 并推送。
- 交付：App 入口壳层 `EntryShell`（280px 常驻左栏 + 右主区）；启动页工程列表经 portal 进入左栏；工程面左栏占位；收展复用 WindowPresentation.sidebarOpen；fixture/gallery 路径不变。

## 主管复核（亲跑）

| 项 | 结果 |
|---|---|
| 文件域 | 9 文件（App/ConvergedLauncher/ProjectSurface/entry 三件/render-smoke/RWP 断言；≤10） |
| `npm run test` | exit 0；60 files / 722 tests |
| `npm run build` | exit 0 |
| `node tests/render-window-presentation.mjs` | exit 0；**50 checks / 0 failed**（既有 35 + 新增 15：rail 挂载/280px/工程列表 portal/收起释放宽度/同节点跨 surface/工程占位/状态保持/恢复） |
| `npm run test:render` | exit 0；APPENTRY/ENTRY388/RESULT/LAUNCHER/PROJECT/LOADING/FIDELITY/SHADOWBOX/REC-TAPE/WIDGETS/CONVERGED 全 PASS |
| 目检 | 截图（launcher-pending 1440×900）：左栏 280px、Vit 品牌+工程列表标题、recents 磁带卡、底部设置按钮、右主区正常；布局成立 |

## 已知后续（转 B2a/418 处理）

1. 启动页主标题栏仍显示「Vit」品牌，与左栏品牌重复 —— B2a 隐藏 rail 模式下主标题栏品牌（或改为当前工程/状态）。
2. 设置按钮目前仍在 drawer 内容底部；工程面/收起态无法常驻。用户要求“常驻左栏底部、不随列表切换变化”，B2a 做 rail footer（settings + 未来头像槽）。
3. 收展状态仍在内存（WindowPresentationProvider）；用户要求按 DSH 习惯持久化，B2a 加 localStorage。
4. 工程面左栏仍是占位；B2a 接对话流树/只读预览/继续动作，B2b 接跨工作树。

## 边界

零真栈（playwright + vite mock HTTP/host）；真实端侧与手测在 B2 完成后统一验证。
