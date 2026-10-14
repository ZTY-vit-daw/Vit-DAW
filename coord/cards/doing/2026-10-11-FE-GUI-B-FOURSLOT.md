# FE-GUI-B-FOURSLOT：前端收缩 Phase B（四位引擎+LRU+F2+抽屉坞+指针拖拽）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚夜（依据=FE-GUI-SHRINK-IMPL-1 Phase A 验收收口+其设计基线 [FE-GUI-SHRINK-ROUND1-SCOPED-RECON](../../runs/FE-GUI-SHRINK-ROUND1-SCOPED-RECON/)）
- 派发确认：已确认（Phase A 裁定随卡：B/C 各拆新卡）
- 验收负责人：GLM 主管决策流
- 池序 91；目标仓库=前端仓 D:/Godot/project/vit-daw-frontend（+主仓工件）
- 晚窗参考序②（2026-10-14 早窗校准：前端仓域与 109 腿 D 文件域不相交可并行；涉栈验收腿避让 109 收口占用时段）
- 优先级 / 预估 / 依赖：P1 / 预估依规格 / **Phase A（前端仓 port/fe-gui-shrink-1 c56e38f）为基线**
- 模型分级：L1 / PC 执行流（前端仓域）

## 目标（Phase A 回执划入 B 面的断言②③④⑥）

四位引擎（主区双台前位扩展）+LRU+F2 面板/拉下+抽屉 chip 拖拽+双击 MIDI 入位+指针拖拽。**含越域修复**：三诊断探针（tools/diagnostics/{gui_architecture_audit_probe,gui_health_playback_probe,bulk_track_gui_probe}.gd）随新装配面提供 slot-grid 等价物（Phase A 越域上报项）。

## 文件域与红线

前端仓 vit_dock/ 域；红线 9 文件零触碰（Phase A git 门沿用 scripts/run_fe_gui_shrink_probe.ps1 模式）；渲染面按 AGENTS §5（headless probe=端侧门，FE-RACK 先例口径）。

## 验收标准

规格断言②③④⑥ 逐条探针钉死+全回归（vitnote 五探针+FE-RACK+Phase A 69 钉）+真栈三件套 exit 0+手测清单更新[等待用户]。

- 领取：2026-10-14 10:52 +0800 / origin/main=dad17d85 / GLM-5.3 L1 执行流（PC，ZCode 会话，用户 2026-10-14 早窗指派「领取卡池卡 91」）/ 前端仓分支 port/fe-gui-shrink-b-1（自 Phase A 基线 c56e38f 建）/ worktree=D:/Godot/project/vit-daw-frontend-fe-gui-shrink-b（独立 worktree）/ 领取时前端仓主检出 HEAD=c56e38f@port/fe-gui-shrink-1、status 净（仅运行时工程 untracked 目录，与本卡无关）/ doing 两卡核对=109 FIX-FS-SETTLE-DEADCHAIN-1、110 FIX-CARRIES-REPORT-TIGHTEN-1 均为 agent 仓域，与前端仓 vit_dock/+tools/diagnostics/ 域不相交 / 领取提交=本提交
- 回执：（前端仓 commit / 探针结果 / 端测边界声明）
- 验收：（裁定文件）
