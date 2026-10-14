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

- 领取：2026-10-14 10:52 +0800 / origin/main=dad17d85 / GLM-5.3 L1 执行流（PC，ZCode 会话，用户 2026-10-14 早窗指派「领取卡池卡 91」）/ 前端仓分支 port/fe-gui-shrink-b-1（自 Phase A 基线 c56e38f 建）/ worktree=D:/Godot/project/vit-daw-frontend-fe-gui-shrink-b（独立 worktree）/ 领取时前端仓主检出 HEAD=c56e38f@port/fe-gui-shrink-1、status 净（仅运行时工程 untracked 目录，与本卡无关）/ doing 两卡核对=109 FIX-FS-SETTLE-DEADCHAIN-1、110 FIX-CARRIES-REPORT-TIGHTEN-1 均为 agent 仓域，与前端仓 vit_dock/+tools/diagnostics/ 域不相交 / 领取提交=b2e8f678
- 回执：**Phase B 交付（规格断言②③④⑥+⑤前半 chip 拖拽）**
  - 前端仓实现 commit=**1c61482**（port/fe-gui-shrink-b-1 已推 origin；16 文件净 +2041/-39：新 stage_slot_engine.gd+widgets/{stage_switch_panel,view_drawer_bar,stage_pointer_drag}.gd+probe_fe_gui_shrink_phase_b.gd；改 vit_dock_root/viewport_frame(+tscn)/timeline_dock_adapter/三诊断探针）
  - 落地面：①四位引擎=StageSlotEngine 纯逻辑（1=全屏/2=左右 0.58/0.42/3=左整 1.4+右上下/4=田字；满四挤出 LRU 队首；指定位顶替原住客回台后；拉下自动重排；poke 触碰序）+root 引擎驱动重建（帧按 view_type 缓存于 StageBackstageRack 隐藏停摆，重推复用）②F2 台前调度面板（左列积木陈列+台前中黄标+推上台前+前台铺排镜像小卡；推后收面板=V11 语义）③视图抽屉坞（顶部柄+积木条 chip+拖入即装载+落主区槽自动收起）④指针式自绘拖拽（幽灵卡+阈值+落点高亮；生产=轮询释放守卫）⑤槽位头=位号+视图名+✕拉下 ⑥双击 MIDI=hint_clip_id 非空分流→vit_push_piano_roll_into_stage_slot（校验层，无内核诚实不上台）+vit_open_piano_roll_in_stage_slot（装配核心）；轨头按钮（hint 空）保持独立窗语义 ⑦越域修复=三诊断探针 _apply_workspace_layout 增 stage_apply_probe_layout 分支（legacy 分割树载荷→视图序列上台，>4 截断如实上报；manager 路径保留回退态）
  - 探针=**tools/probe_fe_gui_shrink_phase_b.gd（119 钉）×2 连跑 PASS exit 0**：静态契约 26 钉+引擎纯逻辑 21 钉+场景装配/注入 72 钉（F2 键级注入 parse_input_event headless 原生生效；面板按钮 pressed 信号注入；chip 拖拽经控制器公共 notify_*=生产同代码路径；LRU 挤出=推前队首语义钉；双击=真实轨行信号诚实分支+核心直调双钉）
  - ps1 包装=**scripts/run_fe_gui_shrink_phase_b_probe.ps1（主仓随本回执入库）**：红线 git 门（9 文件 vs 基线 924318d 零触碰）+九探针电池（Phase B 119+Phase A 69+vitnote 六+FE-RACK 76）**exit 0**；工件 [coord/runs/FE-GUI-B-FOURSLOT/](../../runs/FE-GUI-B-FOURSLOT/)（20261014_112758 电池全量日志+dev_agent_smoke_20261014 真栈件，索引 README.md）
  - 真栈三件套=**dev_agent_smoke.ps1 -StartKernel -StartUI -MixSmoke -GodotProjectRoot <本卡 worktree> exit 0**（build→kernel 19876→UI 端口门→agent 285 工具→strip-silence/range-context/chat-route/no-pending-confirm 全绿→**mix.observe status=ok obs_20261014T034250_80770a0c23d5**=红线#8 链路真栈活证）+godot 窗口启动补充证据（D3D12 RTX4070 真渲染，start_page VSP Hub/Agent tools 就绪，零 SCRIPT ERROR，真栈在线态启动）；占用/释放已登记（180ca228+释放行随本批）；11:52 拆栈核净（四端口监听=0、四类进程=0）
  - 端测覆盖边界声明（AGENTS §5，FE-RACK 先例口径）：headless probe=端侧门——覆盖=引擎全语义+全树装配+注入钉；**未覆盖**=真实视觉观感/指针手感/焦点次序/动效（归用户手测）；三诊断探针端到端运行需真栈 VSP hub（headless 实测 VSP hello 失败=环境边界），slot-grid 等价物以静态钉+API 三形态钉收口，诊断面真实运行归其既有使用场景验证
  - 偏离登记（4 项，待主管裁定）：①「⊞ 视窗 F2」按钮位——规格布局表置于走带栏右区，但红线 9 文件含 Top_Transport_Bar.tscn/.gd 零触碰（Phase A git 门沿用）→入口改设主区顶部抽屉柄簇（▦ 视图 ▾+⊞ 视窗 F2 并列）+F2 全局键；若须回归走带栏位需红线豁免另卡 ②台后卡数——规格原型 5 视图口径「5-台前数」，实现按 viewport_registry spawnable=6（detail 亦可上台）→默认两位台后=4 卡，探针按 registry 派生断言 ③chip 点击=浮窗（规格⑤后半）与面板「⊞ 浮窗」按钮=Phase C 面，本阶段不接（chip 子标签「拖入视窗装载」） ④底部状态栏（台前位况行）不在断言②③④⑥面，未实施（布局表项归后续）
  - 手测清单（Phase B 范围，手测入口=用户从 Godot 拉起前端进工程）[等待用户:手测排期]：①F2 面板观感与动效（左列陈列/黄标/前台镜像/出入场）②推上台前→田字密度与可读性（音轨+机架+卷帘+混音全开）；3 位=左整+右上下 ③满四再推→LRU 挤出直觉性（被挤出者回台后）④槽位头 ✕拉下→重排（至 1 位=独占全宽；至 0 位=空态提示）⑤抽屉坞开合手感、chip 拖拽幽灵跟手、落位高亮、装载+自动收起 ⑥双击 MIDI 片段→卷帘入音轨位、音轨回台后；轨头钢琴卷帘按钮仍开独立窗 ⑦走带栏/电平表/🤖AI 助手(C) 外置窗与 Phase A 后行为一致 ⑧对话内 mixboard 状态卡正常出（遥测链冒烟）
  - 复验 worktree 保留：D:/Godot/project/vit-daw-frontend-fe-gui-shrink-b（验收后由决策侧清理，§3 worktree 纪律）
- 验收：**pass（4 项偏离全部采信）** / 2026-10-14 批量一（[rulings/2026-10-14-BATCH1-rulings.md](../../rulings/2026-10-14-BATCH1-rulings.md)②；红线 9 文件独立 diff 复核零命中+探针/真栈工件亲核（SMOKE_EXIT=0+mix.observe 活证）+vitnote input 探针 verdict 与 91 前逐字一致=回归中立；偏离①若需走带栏位另立红线豁免卡、④登记 Phase C 候选输入；手测清单 8 条[等待用户]；复验 worktree 决策侧拆除；Phase C(92) 解锁，基线=port/fe-gui-shrink-b-1@1c61482）
