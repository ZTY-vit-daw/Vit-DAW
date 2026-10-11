# FE-GUI-SHRINK-IMPL-1：PC 端四位台前调度布局改造（按 HTML 设计稿实施）

- 发卡：GUI 设计会话（GLM）/ 2026-10-10
- 派发确认：**已确认**（用户 2026-10-10 授权立项：「按照 html 设计稿把现有前端修改好」+设计稿存档为未来依据）
- 验收负责人：GLM 主管决策流
- 目标仓库：**D:\Godot\project\vit-daw-frontend**（仓库外；本卡勘察期只读——**领取前须获写授权**，并在 coord/resources/PC-RUNTIME-STACK.md 登记真栈占用）
- 优先级 / 预估 / 依赖：P1 / Phase A 1-2d，Phase B 2-3d，Phase C 1-2d / 无硬依赖（与 FE-RACK-CTX-PLUGIN-LOAD-1 文件域部分相邻，领取时核对不相交）
- 模型分级：L2（涉及交互引擎重写与机制退役，下游失败面广）

## 设计契约（权威输入，实施以此为准）

- 规格文档：`coord/runs/FE-GUI-SHRINK-ROUND1-SCOPED-RECON/实施卡规格-第一轮.md`（含布局表/交互清单/退役清单/红线/probe 断言/手测清单/分期）
- 交互稿与终图：`coord/runs/FE-GUI-SHRINK-ROUND1-SCOPED-RECON/prototypes/`（**V11-PC端.html=本卡蓝本**；V5/V7=移动端设计基线，本卡不实施）
- 该目录已定为**设计基线存档**（README-BASELINE.md），是后续前端迭代与未来移动端设计的依据，实施不得改动其中文件

## 目标

PC 端主工作区由「自由分割树多视口」改为「**四位台前自动铺排**」（1=全屏/2=左右/3=左整+右上下/4=田字；默认=音轨+机架；满员挤出 LRU；Master 置顶第一轨），配套 F2 台前调度面板（左列陈列+前台镜像小卡）、视图抽屉坞（拖拽装载/点击浮窗/平时收起）、浮卡+真窗口双档多开；agent 对话维持外置窗现状。分期交付：**Phase A**=固定双位+退役清单+Master 置顶（删除性工程，最小可交付）→ **Phase B**=四位引擎+LRU+F2 面板+抽屉坞+指针式拖拽 → **Phase C**=浮卡/真窗口双档。

## 文件域（前端仓）

- `vit_dock/scenes/vit_dock_root.gd`、`vit_dock/core/viewport_split_tree_builder.gd`（退役）、`workspace_layout_manager.gd` / `workspace_preset_repository.gd`（降级为数据层，不驱动 UI）
- `vit_dock/scenes/AppHeaderShell.tscn/app_header_shell.gd`（收敛为菜单栏；走带栏 Top_Transport_Bar **仅加按钮位**）
- `vit_dock/scenes/viewport_title_bar.*`（视口标题栏退役）、`views/*_dock_adapter.gd`（换宿主，内容不动）
- **红线零触碰**：`app/kernel/autoloads/telemetry_manager.gd:1963`、`app/kernel/clients/mix_client.gd:39-42/:65/:71`、`app/shell/LLM_Chat_Controller.gd` 数据面、`app/kernel/autoloads/vsp_asset_adapter.gd`、`app/browser/left_library_dock.gd`、机架/轨行右键菜单、插件拖拽双落点、走带/电平数据链

## 验收标准

1. 规格文档中 **headless probe 断言 8 条**全过（默认两位/田字/独占全宽/LRU 挤出/拖拽注入装载/双击上台/Master 首轨/mixboard id 红线回归），按仓库 ps1 冒测体系扩展（复用 dev_agent_smoke.ps1 模式，脚本 exit 0）
2. 真实三件套烟测（内核+Godot 前端+agent）exit 0（AGENTS.md §5 门槛）
3. 手测清单 7 条交付用户（手测入口=用户从 Godot 拉起）
4. 退役清单落地核验：预设 tabs/分割按钮/视口切换菜单/New Window/右缘坞不在 UI 面

## 停止条件

- 发现前端仓结构性异常（如分割树与遥测链隐藏耦合）→ 停止并上交证据
- 红线面任何回归（mixboard id 形态/波形包络物化/聊天观察卡）→ 立即停卡上报
- 连续两次同断言失败→按 §8 补锚点或拆卡，不得原样重跑

## 并行与资源

真栈占用（烟测腿）按 PC-RUNTIME-STACK 登记，与其他在池烟测卡串行；纯代码阶段可并行。

- 领取：2026-10-11 22:20 +0800 / 主仓 origin/main=643dff54fa6daffb94ce7cca5006f23b95c47c92（本地已 ff 一致）/ 前端仓 HEAD=924318dea1f55f5c9591d573b174a1a2307398e6（活线 port/vitnote-region-time-1 tip，含 FE-RACK-CTX 装载成果=本卡红线面；前端仓 main 落后活线 13+ 笔为已登记卫生项，集成落点按 PROTOCOL §2=port 分支）/ owner=GLM-5.3-Flash · PC · ZCode flash 执行流会话（用户 2026-10-11 晚口令指派「执行前端收缩 Phase A」）/ 写授权凭证=同口令——规格 §验收负责人与流程建议「实施卡领取即授权写入前端仓」/ 分支=前端仓 port/fe-gui-shrink-1（自 924318d 建）/ worktree=前端仓主检出 D:/Godot/project/vit-daw-frontend（本卡独占；偏离 §2.1 独立 worktree 常规的理由：当前无并行前端流，真栈烟测 -StartUI 默认 GodotProjectRoot 即主检出，独立 worktree 会引入 --import 缓存与烟测路径偏差；与全部 7 张在池 doing 卡文件域不相交，纯代码阶段不占真栈）/ 领取提交=本提交（仅 coord/ 卡片变更）/doing 六卡核对=GEN-API-ADAPTER-1、GEN-CTX-DISCLOSE-1、LOUDNESS-WATCH-DESIGN-1、MIDI-ARGSPEC-ALIGN-1、MIXLAYER-M1-4、PULL-PROBE-TIER-EXT-1、VIS-FILM-P0-ROUTE-ENABLE 均为 agent 侧/webui/Mac 线，与前端仓 vit_dock/ 域不相交
- 回执：**Phase A 交付（用户 2026-10-11 晚指派范围；Phase B/C 未在本卡执行——建议主管拆卡，见尾注）**
  - 实现 commit=vit-daw-frontend **c56e38f**（port/fe-gui-shrink-1，已推前端仓 origin；任何 main 未触碰）。产品代码净 -125 行（524 删/399 增，含探针 ~280 行）
  - Phase A 落地面：①主区固定双台前位——vit_dock_root `_rebuild_fixed_stage_slots()`（timeline 0.58 + graph_rack 0.42，viewport id 沿用 vp_timeline_main/vp_rack_main，视口状态初值同 default_arrange 预设）②退役清单落地——viewport_split_tree_builder.gd 删除、viewport_title_bar.gd/.tscn 删除（7 视图切换菜单/水平垂直分割/弹出 detached/关闭按钮随体退役）、AppHeaderShell PresetTabs+BtnNewWindow 删除+File 菜单 New Window 项移除、viewport_frame split/popout/close 信号+DragFeedbackOverlay+_get_drag_data 删除（零消费实证后删）③workspace_layout_manager/workspace_preset_repository 降级数据层（文件原样保留，零引用）④Master 置顶=双路径既有实现钉住（非虚拟 `_track_list.move_child(master_row, 0)` + 虚拟 `_ensure_virtual_master_row` base_index 钉位），零代码改动
  - 保留面核验：window_manager 留（轨头钢琴卷帘/track·audio detail/master output 独立窗三活调用者实证，Phase C 重塑面）；EmptyViewportLauncher 留（满位隐藏）；chat_toggle_requested→外置 webui 窗语义不变；VitNoteLayer 挂载不变量不变；左库/走带/底栏抽屉零触碰
  - 探针=**tools/probe_fe_gui_shrink_phase_a.gd（三段 69 钉）×2 连跑全 PASS exit 0**：①静态退役契约（退役 4 文件不存在/数据层 2 文件保留/6 文件无退役标识符且新装配入口在位/Master 双锚+mixboard_ id 构造静态在位）②全树装配烟测（VitDockRoot.tscn 真场景 headless 实例化 settle：StageSlotGrid 恰两位含视图/比例/适配器绑定断言；全树零 ViewportTitleBar/DragFeedbackOverlay；AppHeaderShell 零 PresetTabs/BtnNewWindow+File 菜单无 New Window 项 6 项留 Save；launcher 满位隐藏；chat 接线在位；VitNoteLayer 挂载）③功能钉（Master 虚拟路径真实路由器=index=插入基位+内容层紧随；MixClient 空上下文→`mixboard_<msec>` 请求 id 构造+缺 clip 源诚实 blocked 分支+自定义前缀族同构造）
  - ps1 包装=**scripts/run_fe_gui_shrink_probe.ps1**（主仓 port/fe-gui-shrink-1 **e70e817d**）：红线 git 门（9 红线文件 vs 基线 924318d 零触碰）+探针腿，**exit 0 ×2**；工件 coord/runs/FE-GUI-SHRINK-IMPL-1/20261011_{221328,221505,221520}
  - 真栈三件套=**dev_agent_smoke.ps1 -StartKernel -StartUI -MixSmoke exit 0**（工件 dev_agent_smoke_rerun_stdout.log：build→kernel 2548→UI 端口门→agent 265 工具→strip-silence/range-context/chat-route/no-pending-confirm 全绿→mix.observe **status=ok obs_20261011T142837_56c255d1b6ff**（mixboard 链真栈走线=红线#8 活证））+godot 窗口启动补充证据（D3D12 RTX4070 真实渲染环境，start_page/VSP Hub/Agent tools 就绪，启动日志零 SCRIPT ERROR，godot_windowed_boot.log）
  - 回归面：vitnote 五探针（circle_state 52/container 141/face_resolve 143/input/notestream 26/panel_summary 16）全 PASS exit 0 + FE-RACK 机架右键探针 76 钉 PASS（红线回归）；红线 git 门 9 文件零触碰
  - 渲染面端测覆盖边界声明（AGENTS §5）：Godot 前端无浏览器 DOM 断言面，headless probe 即端侧门（FE-RACK 先例口径）；三件套烟测 UI 腿=脚本既有设计（内核被脚本占用时 -StartUI 按端口门验证，Godot 不由脚本拉起），窗口环境启动以补充证据留档；真实视觉/指针/焦点交互归用户手测
  - 手测清单（Phase A 范围，手测入口=用户从 Godot 拉起前端进工程）[等待用户:手测排期]：①默认双位呈现=音轨左/机架右、无视口标题栏、资料库 ⋮ 柄开合正常 ②菜单栏 File/Edit 无 New Window（按钮与菜单项）、无预设 tabs ③走带栏/电平表/🤖AI 助手(C) 外置窗行为与改前一致 ④机架右键装载插件（FE-RACK 红线面回归目检）+左库插件拖拽双落点 ⑤音轨视图 Master 行显示在最上 ⑥对话内 mixboard 状态卡正常出（遥测链冒烟）⑦窗口缩放下双位比例保持 58/42
  - 越域上报（AGENTS §11，未动手）：tools/diagnostics/{gui_architecture_audit_probe,gui_health_playback_probe,bulk_track_gui_probe}.gd 依赖主检出 workspace_layout_manager 应用自定义布局——降级为其既有 missing_workspace_layout_manager 诚实失败路径（锚点 .gd:438/:898/:314）；建议 Phase B 拆卡时为诊断面提供 slot-grid 等价物
  - 脚本卫生修复（主仓 e70e817d）：dev_agent_smoke.ps1 -MixSmoke 腿 mix.observe ok 响应无 error/result 键，裸 $mix.error 在 StrictMode 下中断腿（49a332dc 起既有缺陷，首轮实测中断+残留进程，证据 20261011_2240_first_run_aborted/）→Get-OptionalProperty 严格安全读取，复跑全绿
  - 资源：占用登记 65cf890c（首登 push 冲突遇 MIXLAYER-M1-4 已占，按 §2.2 撤销等待释放后重登）；末轮 teardown 后实测端口 7878/5555/5556 监听=0（仅 TIME_WAIT 残迹）、VitApp/VitAgent/Godot/VspHub 进程=0
- 验收：（裁定文件）待主管——Phase A 范围裁定+拆卡建议：规格探针断言 8 条中 **①（默认两位+退役元素不在 UI 面）⑦（Master 首轨）⑧（chat 接线+mixboard_ id 红线）** 已在本回执覆盖（①含右缘坞=现役装配树本无右缘坞（勘察简报 §1：左+底 rail 结构），退役面以静态+装配双钉实证）；**②③④⑥ 属 Phase B 面（F2 面板/拉下/LRU/抽屉 chip 拖拽/双击 MIDI 入位）、⑤后两档属 Phase C 面（浮卡/真窗口）**；建议本卡按 Phase A 收口、Phase B（四位引擎+LRU+F2+抽屉坞+指针拖拽）与 Phase C（浮卡/真窗口双档）各拆新卡
