# CONVERGE-CLEANUP-1：旧 LauncherSurface 退役+烟测迁移（收敛阶段 1 / 卡 5，阶段收口）

- 发卡：主管 2026-11-16 晚（权威=[368 施工图](../../runs/UI-CONVERGE-RECON-1/REPORT.md) 阶段 1 表+节 7）
- 池序 389；P1；依赖=**388 合入后**；真栈烟测腿
- 目标：旧 LauncherSurface.tsx 归档（git 历史可回指）+render-smoke launcher fixture 段改指新壳+styles.css 死段清理启动。
- 文件域：`src/launcher/LauncherSurface.tsx`+render-smoke.mjs 段+styles.css。
- 验收：test:render 8 段（launcher 段断新壳）全过+CONVERGE-SHELL 旧组件零引用+`vitshell_full_smoke.ps1` 指向复核对齐。
- **阶段 1 出口判据（368 §阶段 1）**：用户从 Godot 拉起→新启动页完成 recents/新建/打开/进工程全旅程（用户手测节点；磁带卡目检随 387 已提前）。
- **387 上交对齐项（2026-11-16 晚补入）**：①`ui_entry_smoke.ps1` 腿1 前提失效（UI-ENTRY-2 已退役 unavailable→restore 回退链，脚本断言仍在期望旧行为）——本卡一并处置（脚本腿退役/改写对齐 ui_entry2 语义，主管裁量）；②§12 guard 的 `data-tape-badge` 残留断言换壳后平凡通过（新壳锚点=data-tape-card）——改为对齐新锚点或随旧组件退役移除。

## 领取/回执/验收：待回填
