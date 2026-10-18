# FE-GUI-C-FLOATING（卡 92）工件索引

- 领取：2026-10-18 12:0x +0800 / 主仓 origin/main=a71b54bb / owner=DSH PC 执行流 exec-fe-gui-c（本机 DSH 团队会话）/ 前端仓分支 **port/fe-gui-shrink-c-floating**（自 1c61482 建）/ 前端仓 worktree **D:/Godot/project/vit-daw-frontend-fe-gui-c** / 主仓探针分支 **port/fe-gui-shrink-c-probe-1**（自 origin/main a71b54bb 建）/ 主仓探针 worktree **D:/Vit_DAW_wt_feguic_probe**
- 前端仓实现 commit：**4b8bb9c**（port/fe-gui-shrink-c-floating 已推 origin；4 改 + 4 新增产品脚本 + 4 .uid + C 探针，vit_dock/ 域 + tools/，红线 9 文件零触碰）
- 主仓脚本：`scripts/run_fe_gui_shrink_phase_c_probe.ps1`（Phase B wrapper 模式：Godot import 预热 + 红线 git 门 924318d..worktree 9 文件零命中 + 十探针电池）

## 落地面（Phase C）

- **浮卡**：抽屉 chip 点击 / F2 面板台后卡「⊞ 浮窗」→ Dock 内浮卡实例化（可拖拽 Control 卡、卡体=实时视图实例 `vp_float_<n>_<view>`）；动作后抽屉/面板自动收起；同视图重复打开=聚焦既有项（多开=不同视图各一份，V11 原型 openFloats 语义）。
- **真窗口双档**：浮卡 ⚠/↗ → 真 Godot Window 实例（单视图内容重挂、独立窗口策略、非 transient/exclusive；不复用/不继承 detached_viewport_window.gd）；「⇤ 吸回」=内容回浮卡+窗口释放；系统关闭=关闭该多开项；浮卡 ✕=关闭。
- **锚保留**：window_manager 现役三活调用面（轨头钢琴卷帘 / track·audio detail / master output 独立窗）零改道，Phase C 管理器独立窗口表；探针静态钉+装配级开窗锚（detail 路径仍经 window_manager 开 legacy detached 窗后清理）。
- **终态收口**：底部状态栏（vit_dock/ 域新增 `stage_status_bar.gd`，挂 TopLevelShell 末位=主区底部）三要素=台前位况（N 位 · 视图名，随 1-4 位/空态实时刷新）+ 快捷键提示（F2/▦ 抽屉/双击 MIDI/Ctrl+S）+「agent 对话 = DAW 窗外独立窗口（webui）」注记；抽屉提示与 chip 子标签含「点击=浮卡」。

## 工件目录

| 目录/文件 | 内容 |
|---|---|
| `20261018_120623/` | 探针电池一次性全量日志（godot_import 预热 + **Phase C 129 钉** + Phase B 119 钉 + Phase A 69 钉 + vitnote 六探针 + FE-RACK 76 钉，全部 exit 0；红线门通过行在电池 stdout） |
| （真栈工件） | 真栈三件套腿工件目录（dev_agent_smoke_<stamp>）在栈窗口获批后补入本索引 |

## 端测覆盖边界（AGENTS §5，FE-RACK/91 先例口径）

- **覆盖**（headless probe=端侧门）：浮卡/真窗口双档状态机全语义（开卡/拖拽/⚠/↗ 提升/吸回/系统关闭/关闭/重复打开聚焦/多视图多开）；全树装配（浮卡层先于 F2 模态面板、状态栏挂载位与三要素、无残留窗口/卡）；注入钉走生产同代码路径（抽屉 chip 走 chip `gui_input` press → 拖拽控制器 click 语义；卡拖拽走卡头 `_gui_input`；⚠/↗/✕/⇤ 走按钮 `pressed` 信号；F2 走 headless parse_input_event 键注入+直调生产 `_input` 兜底）；window_manager 三活调用面装配级锚。
- **未覆盖**：真实系统窗口外观/位置/焦点/原生平台行为（headless 无原生 DisplayServer，真窗口以「Window 节点实例+内容重挂+窗口策略+开关语义」装配层断言，不伪造系统窗口在场）；指针手感、动效、真实视觉观感、多显示器行为——归用户手测（入口=用户从 Godot 拉起 DAW 前端再从前端启动页进工程）与真栈烟测（dev_agent_smoke exit 0 只证明链路/栈面健康，不证明视觉与交互手感）。

## 终态收口偏差登记（如实）

1. **走带栏新增按钮位未做**（规格布局表：走带栏右区新增 `⊞ 视窗 F2`、`保存`）：`Top_Transport_Bar.tscn`/`top_transport_bar.gd` 属红线 9 文件零触碰；沿用 91 偏离①裁定——F2 入口在抽屉柄簇（▦ 视图 ▾ + ⊞ 视窗 F2），保存沿用既有菜单/保存路径。若需回归走带栏位，须红线豁免另卡。
2. **F2 面板「⊞ 浮窗」只挂台后卡**（与 V11 原型一致：台前中卡只显示黄标、无多开入口）；若需台前卡也可多开，另卡扩展。
3. **⚠/↗ 双入口同语义**：规格断言⑤后半文字为「⚠ ↗ 弹真系统窗口」，V11 原型仅 ↗；实现为双按钮同语义（均提升到真 Window），探针两条入口各自钉住；若 ⚠ 另有语义裁定，按裁定调整。
4. 91 偏离④「底部状态栏=Phase C 候选输入（不强制）」已按主管 2026-10-18 裁定**落地**（第二优先；仅 vit_dock/ 域新增节点，未触碰走带红线文件），探针钉三要素与刷新。

## 手测清单（Phase C 范围，手测入口=用户从 Godot 拉起前端进工程）[等待用户]

- [ ] 抽屉「▦ 视图」落下 → 点击任一 chip → 浮卡出现（380 宽卡、含视图实时内容）；抽屉自动收起
- [ ] 浮卡卡头拖拽跟手；浮卡 ⚠ 与 ↗ 均弹出独立系统窗口（窗口标题/位置跟随卡）、系统窗口可移动缩放
- [ ] 真窗口「⇤ 吸回浮卡」→ 内容回浮卡；关系统窗口 → 该多开项关闭；浮卡 ✕ 关闭正常
- [ ] F2 面板台后卡「⊞ 浮窗」→ 浮卡出现且面板收起；同一视图重复打开=聚焦既有卡/窗口（不重复多开）
- [ ] 不同视图同时多开数张浮卡/真窗，Dock 主区与浮卡互不遮挡错乱（F2 面板盖在浮卡之上）
- [ ] 轨头钢琴卷帘按钮 / track·audio detail / master output 独立窗行为与 Phase B 后一致（锚未改道）
- [ ] 底部状态栏观感：台前 2/3/4 位与空态时「N 位 · 视图名」实时正确；快捷键提示与 agent 外置窗注记可读
- [ ] 走带栏/电平表/🤖AI 助手(C) 外置窗与 Phase B 后行为一致（红线面零触碰的目检）
- [ ] 对话内 mixboard 状态卡正常出（遥测链冒烟，真栈腿覆盖）
