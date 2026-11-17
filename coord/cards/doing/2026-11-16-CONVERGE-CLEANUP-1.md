# CONVERGE-CLEANUP-1：旧 LauncherSurface 退役+烟测迁移（收敛阶段 1 / 卡 5，阶段收口）

- 发卡：主管 2026-11-16 晚（权威=[368 施工图](../../runs/UI-CONVERGE-RECON-1/REPORT.md) 阶段 1 表+节 7）
- 池序 389；P1；依赖=**388 合入后**；真栈烟测腿
- 目标：旧 LauncherSurface.tsx 归档（git 历史可回指）+render-smoke launcher fixture 段改指新壳+styles.css 死段清理启动。
- 文件域：`src/launcher/LauncherSurface.tsx`+render-smoke.mjs 段+styles.css。
- 验收：test:render 8 段（launcher 段断新壳）全过+CONVERGE-SHELL 旧组件零引用+`vitshell_full_smoke.ps1` 指向复核对齐。
- **阶段 1 出口判据（368 §阶段 1）**：用户从 Godot 拉起→新启动页完成 recents/新建/打开/进工程全旅程（用户手测节点；磁带卡目检随 387 已提前）。
- **387 上交对齐项（2026-11-16 晚补入）**：①`ui_entry_smoke.ps1` 腿1 前提失效（UI-ENTRY-2 已退役 unavailable→restore 回退链，脚本断言仍在期望旧行为）——本卡一并处置（脚本腿退役/改写对齐 ui_entry2 语义，主管裁量）；②§12 guard 的 `data-tape-badge` 残留断言换壳后平凡通过（新壳锚点=data-tape-card）——改为对齐新锚点或随旧组件退役移除。

## 领取/回执/验收：待回填

## 2026-11-17 只读施工核对与范围修订

- /root/retry396（Luna/high）在388分支cd8ed4c1只读核对：生产App已挂ConvergedLauncher；旧LauncherSurface仍被gallery多种fixture与旧单测导入。报告锚点：gallery/main.tsx:19/512-603/938-959，render-smoke.mjs:591-846/1406-1413/1680-1782；ui_entry_smoke.ps1:296-341旧回退断言与ui_entry2:311-357冲突。
- 允许实施范围（≤4生产/脚本文件）：旧LauncherSurface.tsx退役、gallery/main.tsx迁新壳fixture、gallery/render-smoke.mjs迁新壳语义/磁带锚点与guard、scripts/ui_entry_smoke.ps1对齐现行去回退。必要launcherSurface.test.tsx迁移保留新壳既有用例及旧门/错误/空态等价覆盖。文档纯历史评论不当运行时引用，不为rg全零乱删记录。
- styles.css无旧壳专属selector证据，共享reset/标题/focus/reduced-motion与MainApp仍在用；本轮不清这些段。launcher专属共享css仍由新壳使用，不按猜测删。此范围缩减依据证据，不是降低验收。
- 切换guard须先亲证新壳和data-tape-card在场，再断project时根/卡消失与ack/loading gate，不能只断旧data-tape-badge=0空过。所有现行渲染段（包括388管理段）必须过；不沿用过时“8段”计数、不删除旅程。
- 仍依388真实栈验收合入后领取；当前只准备独立worktree，不授实现权或栈权。

## 2026-11-17 正式领取
- 主管/验收=/root；388/401已主管PASS并合入be43a2a2。
- owner=/root/cleanup389（Sol/high，PC）；branch=codex/converge-cleanup-389；worktree=C:/Users/timoz/.codex/worktrees/converge-cleanup-389/Vit_DAW。
- 本领取提交同步main并重读owner后开始；先将干净checkout ff到本协调main，记录HEAD/status/diff。真栈未放行，271验证先排；只零栈实现/测试，不启动Godot/agent/kernel/VitShell。

