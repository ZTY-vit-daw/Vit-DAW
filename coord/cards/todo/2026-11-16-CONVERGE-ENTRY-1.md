# CONVERGE-ENTRY-1：工程管理层挂载+进出语义（收敛阶段 1 / 卡 4）

- 发卡：主管 2026-11-16 晚（权威=[368 施工图](../../runs/UI-CONVERGE-RECON-1/REPORT.md) 阶段 1 表+节 7 双轨）
- 池序 388；P1；依赖=**387 合入后**；真栈烟测腿
- 目标：recents 抽屉/新建/打开进新壳（387 未含全量则本卡收尾）+「回到启动页」交互（setSurface 内部切换）+宿主 vit_surface 双向对齐登记。
- **补充（2026-11-17 凌晨裁定档 §三）**：+settings 按钮**左下角**（启动台+工程面两态都放；入口即可，设置面板本体如无既有面则占位跳转/最小面板——用户裁定「主流 agent 窗布局」）。
- 文件域：`src/launcher/`+App.tsx（分发逻辑不改，必要时白名单）；Godot 侧收编点 A（`_AGENT_WEBUI_URL`→`_AGENT_WEBUI_URL_LAUNCHER` 1 行）**另微改卡**（前端仓域，本卡只登记）。
- 验收：桥协议五 type 零破坏断言（launcherBridge.test.ts 平移不删）+`ui_entry2_smoke.ps1` exit 0+vit_surface→project→launcher 往返烟测。
- 备注：383（列表可点击加载）若已在本卡前合入，其断言随迁新壳（领取时核对 383 状态防双做）。

## 领取/回执/验收：待回填
