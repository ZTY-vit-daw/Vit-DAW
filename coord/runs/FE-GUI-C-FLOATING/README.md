# FE-GUI-C-FLOATING（卡 92）工件索引

- 领取：2026-10-18 12:0x +0800 / 主仓 origin/main=a71b54bb / owner=DSH PC 执行流 exec-fe-gui-c（本机 DSH 团队会话）/ 前端仓分支 **port/fe-gui-shrink-c-floating**（自 1c61482 建）/ 前端仓 worktree **D:/Godot/project/vit-daw-frontend-fe-gui-c** / 主仓探针分支 **port/fe-gui-shrink-c-probe-1**（自 origin/main a71b54bb 建）/ 主仓探针 worktree **D:/Vit_DAW_wt_feguic_probe**
- 前端仓实现 commit：**4b8bb9c**（Phase C 首版）→ **f6abe20**（92-R1 手测返工；9 文件 +746/−43：ui_motion.gd 新增 + 抽屉/面板/浮卡/真窗/管理器/root/C 探针；port/fe-gui-shrink-c-floating 均已推 origin；vit_dock/ 域 + tools/，红线 9 文件零触碰）
- 主仓脚本：`scripts/run_fe_gui_shrink_phase_c_probe.ps1`（Phase B wrapper 模式：Godot import 预热 + 红线 git 门 924318d..worktree 9 文件零命中 + 十探针电池）

## 92-R1 手测返工（2026-10-18 午窗用户实机反馈 3 项）

1. **抽屉被台前视窗遮挡**（根因）：台前视图内部元素带 z_index（实测轨行 10/20、MIDI 层 3/88、卷帘 lasso 50/100、机架菜单 4096），CanvasItem 按有效 z 全局排序——z=0 的抽屉被这些元素越过。修复=`view_drawer_bar.gd` OVERLAY_Z=300（root 装配时 set_overlay_z 显式设置），抽屉浮于全部台前窗格之上；`StageSlotGrid`/帧结构零改动（不删窗格、不降级）。
2. **F2 面板贴屏幕上缘/被走带栏遮挡**：修复=`stage_switch_panel.gd` 顶层 overlay——z_index=4096（=APP_CHROME_Z_INDEX；同 z 时按树序 root 面板在 TopLevelShell 之后=盖住走带栏与台前内容）、row 顶部 offset=header+transport 实际高度+12px（root `_sync_stage_overlay_metrics` 在布局后/窗口 resize 时按真实 chrome 高度同步）、左列包 ScrollContainer（小窗口可滚动不越屏）。
3. **无 UI 动效**：新增 `vit_dock/core/ui_motion.gd`（时长 0.12/0.16/0.20s∈[120,220]ms、cubic ease-in/out、headless 禁用=探针确定性）；抽屉 slide+fade、F2 面板 backdrop+内容淡入+上移、浮卡出现/吸回/关闭/浮出 scale+fade、真窗口内容淡入、台前位推上（新帧淡入+0.97→1）/拉下（存活帧回稳）。全部可中断幂等（先 kill 旧 tween 再重建；连点只留终态）。

- 92-R1 探针/电池：**Phase C 167 钉**（原 129 钉全保留 + 新增 38：层级 7 / 顶部定位 5 / 动效策略与幂等 22 / 浮卡落位等）；Phase B 119 / Phase A 69 / vitnote 六 / FE-RACK 76 全 exit 0；红线 9 文件零命中。工件=`20261018_141711/`。
- 92-R1 视觉证据（本机窗口态离屏渲染诊断，非真栈；未占栈端口、VIT_SKIP_DEV_AUTOSTART=1、音频 Dummy）：`20261018_141711/visual_evidence/` 三图=抽屉完整浮于台前窗格之上 / F2 面板让出 chrome 且盖住台前内容 / 浮卡落于台前主区。窗口态实测动效在场（drawer mid alpha=0.899→1.0、panel mid row_alpha=0.16/offset 257.6→终态 266/1.0）。

## 落地面（Phase C）

- **浮卡**：抽屉 chip 点击 / F2 面板台后卡「⊞ 浮窗」→ Dock 内浮卡实例化（可拖拽 Control 卡、卡体=实时视图实例 `vp_float_<n>_<view>`）；动作后抽屉/面板自动收起；同视图重复打开=聚焦既有项（多开=不同视图各一份，V11 原型 openFloats 语义）。
- **真窗口双档**：浮卡 ⚠/↗ → 真 Godot Window 实例（单视图内容重挂、独立窗口策略、非 transient/exclusive；不复用/不继承 detached_viewport_window.gd）；「⇤ 吸回」=内容回浮卡+窗口释放；系统关闭=关闭该多开项；浮卡 ✕=关闭。
- **锚保留**：window_manager 现役三活调用面（轨头钢琴卷帘 / track·audio detail / master output 独立窗）零改道，Phase C 管理器独立窗口表；探针静态钉+装配级开窗锚（detail 路径仍经 window_manager 开 legacy detached 窗后清理）。
- **终态收口**：底部状态栏（vit_dock/ 域新增 `stage_status_bar.gd`，挂 TopLevelShell 末位=主区底部）三要素=台前位况（N 位 · 视图名，随 1-4 位/空态实时刷新）+ 快捷键提示（F2/▦ 抽屉/双击 MIDI/Ctrl+S）+「agent 对话 = DAW 窗外独立窗口（webui）」注记；抽屉提示与 chip 子标签含「点击=浮卡」。

## 工件目录

| 目录/文件 | 内容 |
|---|---|
| `20261018_120623/` | Phase C 首版探针电池一次性全量日志（godot_import 预热 + **Phase C 129 钉** + Phase B 119 钉 + Phase A 69 钉 + vitnote 六探针 + FE-RACK 76 钉，全部 exit 0；红线门通过行在电池 stdout） |
| `20261018_141711/` | 92-R1 返工探针电池全量日志（**Phase C 167 钉** + Phase B 119 + Phase A 69 + vitnote 六 + FE-RACK 76，全 exit 0）+ `visual_evidence/` 三张返工视觉证据（抽屉/F2 面板/浮卡窗口态离屏渲染） |
| `dev_agent_smoke_20261018_123056/` | 真栈三件套腿（见下节）：`console_stdout.log`（SMOKE_EXIT=0 + mix.observe 活证）+ `agent_last.log` + `agent_llm_telemetry.jsonl`（6 调用 0 错误）+ `MaterializeMetrics.log` + `godot_windowed_boot.log`（补充窗口态启动：start_page VSP Hub/Agent tools ready）+ `VitHeadlessServer2026-10-18_12-30-58.log`（内核日志，504KB）+ `run_meta.txt`（命令/头/SHA256/拆栈证据） |

## 真栈三件套腿（2026-10-18 12:30–12:35 +0800）

- 占用：Lead 登记 PC-RUNTIME-STACK 协调提交 f946e271（owner=exec-fe-gui-c，窗口 12:30–15:30）；启栈前亲核（12:30:40）四端口 7878/5555/5556/8787/4445/19876 监听=0、四类进程（VitApp/VitAgent/vsphub/Godot 含变体名）=0。
- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW\scripts\dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_feguic_probe -KernelExe D:\Vit_DAW\Export\staging\runtime\VitApp.exe -StartKernel -StartUI -MixSmoke -GodotProjectRoot D:\Godot\project\vit-daw-frontend-fe-gui-c`（console 全量入工件；新工件目录，不覆盖旧 run）
- 结果：**SMOKE_EXIT=0**；`mix.observe status=ok` **observation_id=obs_20261018T043129_0819166048df**（红线#8 链路真栈活证）；agent 构建 passed；内核 pid=15952（工作目录=主检出 staging runtime）；agent pid=17064。
- 被测二进制：内核=`D:\Vit_DAW\Export\staging\runtime\VitApp.exe` SHA256 **B6565DCF85D1DA867E16333052A7B928A5413565940603ADBCF4ABD59DD3123B**（主检出 staging，C++ 本卡零改动，未在探针 worktree 重建）；agent=探针 worktree Go 源构建 `agent\bin\VitAgent.exe` SHA256 **136137E55F9EE586F1848FAEE0F82B1E04C66A48472FAA8B855250C7FCAE80FC**（Go 本卡零改动，探针 worktree 分支 origin/main a71b54bb 谱系）。
- **Godot UI 说明（如实）**：脚本 `-StartUI` 在本 run 未实际拉起重启 Godot——日志明示 `kernel command port already listening before UI start: 5555 pid=15952`（-StartKernel 的内核先绑定 5555，脚本按既有逻辑跳过 UI 启动；91 Phase B run 同款行为：其 console 亦有此行）。故按 91 先例补**窗口态启动证据**：`Godot_v4.6.1-stable_win64_console.exe --path <本卡前端 worktree> --quit-after 1200`（退出码 0），日志=D3D12 RTX 4070 真渲染 + `VitIpcClient: registered autostart child hub pid=15088` + `start_page: VSP Hub ready.` + `start_page: Agent v0.5 tools ready.`，无 SCRIPT ERROR；退出时 start_page 自动停 hub（pid15088），无残留。唯一环境缺陷=worktree 缺 `addons/godot_cef` 构建产物 dll（.gitignore 构建输出，未复制；仅玻璃浏览器面受影响，start page/dock 面无影响，Phase B 同源边界）。
- LLM 记账（§8 如实）：`agent_llm_telemetry.jsonl` 共 **6 次**调用（deepseek-flash；MixSmoke 的 strip-silence chat/mix chat/no-pending confirm 腿，脚本既有行为，非本卡驱动），errors=0，总耗时 3.86s/1.71s/3.15s/5.58s/7.82s/5.56s。Lead 预期 0 次，如实登记为 6 次。
- 脚本告警（非致命，exit 0）：`warn: Strip Silence chat confirmation did not complete`（模型分支走 waiting_confirmation）与 `warn: Strip Silence suggest did not produce expected pending apply regions`（pending=2）——chat 语义腿模型行为差异；91 Phase B run 同一模型分支为 completed。不改变 exit 0/mix.observe 门。
- 拆栈（12:34–12:35）：停本腿自起 VitApp pid15952 + VitAgent pid17064（仅本腿 pid，未杀用户进程/编辑器）；Godot 补充启动进程按 --quit-after 自行退出、其 autostart hub pid15088 由 start_page 停；复核**四端口 7878/5555/5556/8787/4445/19876 监听=0、四类进程（VitApp/VitAgent/vsphub/Godot 含变体名）=0**。真栈已释放（Lead 侧登记释放）。

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
