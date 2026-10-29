# FE-LANE-VIEW-TUNING-1：车道观感与尺寸调整——高度重设+垂直错位修复+车条高度拖拽（手测节第三轮收口前观感卡）

- 发卡：GLM 主管决策流 / 2026-10-28 深夜（依据=用户手测：编辑链全套生效✓后剩余观感三项——车条太窄/垂直映射错位[上提一半到顶·下拉一半消失·看不到底]/车条不可拉伸；用户 2026-10-28「按照 A 方案去做吧」方案通过）
- 派发确认：已确认（用户 2026-10-28 方案通过指令）
- 验收负责人：GLM 主管决策流
- 池序 214；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；P1 / 0.5 天 / 依赖=211 已合入（基线=前端 main 37dc4b6）
- 模型分级：L2 / PC 执行会话
- 并行与资源：真栈腿错峰手测节

## 已核事实（发卡侧）

1. **A 项根源**：车条高 `LANE_STRIP_HEIGHT=96`（:62，189/203 时代 overlay 小条形态定值）+车头 26（:64）+边距——画布净高≈66px；206 行内布局后行高预算已变，此值未重定。
2. **B 项映射静态无错**：`_content_rect()`（:2340-2346）=[HEADER+2+PAD, size.y−PAD]，`_point_to_local`/`_y_to_value`（:2363-2376）按 rect 标准映射——用户观感错位为运行时问题，三候选：①strip 实际渲染高度被压与映射不一致（VBox/ScrollContainer 分配）②ScrollContainer 裁剪（vertical AUTO）③值域显示映射观感（dB 域 0 值居中类）。
3. **C 项缺失坐实**：车条高为固定常量，无拖拽调高交互（206 行高拖拽=调波形区 U，非车条高）。
4. 用户可先行使用：行底分隔条拖拽=波形区高度（206 U 基面）——已在手测指引告知，全局默认行高 240 不动。

## 目标（方案定案：A 核心+B+C）

1. **A 高度重设**：96→**128** 起步（画布净高≈100），并接产品 env 覆盖（`VIT_GUI_LANE_STRIP_HEIGHT`，与行高 env 同族机制）——用户手测可试 112/128/144 定终值，env 仅调观感不落持久化。
2. **B 垂直错位定位修复**：窗口模式渲染测量探针（曲线在值 0%/50%/100% 实际像素位置 vs `_content_rect` 实测矩形）定位三候选后修复；映射一致性断言必入。
3. **C 车条高度拖拽**：车头区高度拖柄（形态从简，位置执行侧定案回执披露）——拖拽调本行车条高度（多条统一），**会话内生效不持久化**（202 缺省即态语义）；与 A 的 env/常量关系=拖拽改会话内值；超高内部滚动联动（budget 公式随实际条高）。
4. 手测闭环：用户观感验收（终值定案）+全套回归。

## 探针义务

- 窗口模式渲染测量（211 环境边界口径）+事件管线注入（拖柄交互）+§5.1 装配面；既有全家回归零回退（lane 族探针期望值随 A 高度更新属授权面，198 先例披露）。

## 文件域

`app/tracks/track_scene/track_envelope_lane_2d.gd`+高度拖柄涉及文件（勘察定，域外上交扩域裁定）+探针。

## 停止条件

- B 定位结果为引擎层裁剪特性非脚本可修→上交两案；
- C 拖柄须动行布局结构（Track_Row/行 VBox）→上交扩域裁定；
- A 与超高滚动联动出现预算冲突（budget 公式失效）→上交。

## 领取

- 领取：2026-10-29 早窗 / origin/main=bbdb9b8d / owner=GLM-5.3 ZCode PC 执行会话（本会话，L2）/ 分支=port/fe-lane-view-tuning-1 / worktree=D:/Godot/project/vit-daw-frontend-lanetune1 / 领取基线=前端 main 37dc4b6 亲核（origin/main 同点）
- 领取时前端仓基线记录：`git rev-parse HEAD`=37dc4b6da735f1e32fef3d091c8553195f5596c7；`git status --short`=零 tracked 改动（仅用户测试工程未跟踪目录 .vit_agent/.vit_derived/8.1test 等）；`git diff --stat`=空
- 回执：（2026-10-29 上午，doing→done 交主管验收）
  - **实现（单文件域内+探针）**：`app/tracks/track_scene/track_envelope_lane_2d.gd`（+137/−22）：**A** `LANE_STRIP_HEIGHT 96→128`+`VIT_GUI_LANE_STRIP_HEIGHT` 产品 env（`parse_lane_strip_height_env` 纯函数 clamp[48,256]，VIT_GUI_* 同族不落持久化）+`get_strip_height_px/set_strip_height_px/get_strip_height_source` API；**B 修复** `_ADD_BAR_HEIGHT 22→24`（＋条 themed min 实测 24；DIAG 坐实见下）；**C** 车头区右端拖柄 `Btn_Envelope_Lane_Height`（Button"⇕"22×22 CURSOR_VSIZE；竖直拖=调容器会话高，下拉增高/上拉减低；多条统一；202 即态不持久化——行池回收/重开回 env/缺省；budget 联动 wanted=24+N×实际条高）。**BaseButton 形态=overlay 网关 interactive 让路面天然认领（211 勘察先例）→零域外改动，仲裁器/mapping/drop_router/timeline_dock_adapter 全零改**。
  - **B 定案（基线 DIAG 窗口模式渲染测量，96）**：候选①**排除**（strip 渲染 rect=(315,332,2237×96)==映射基线；修复分支渲染位 vs 期望 0.1px）；候选②**坐实微型形态并修复**（＋条按钮 themed min 24>旧常量 22→stack 恒溢出 2px=滚动条常驻+条底 2px 裁剪，DIAG 实测 viewport 118 vs stack 120、离线 diag_bar 亲测 BAR min=24→常量校准 24，修复后 P0k N=1 无溢出滚动 PASS）；候选③**坐实为主症状源**（真栈 value_scale=fader 0..1 而 value_text"+0.00 dB"@0.7408（201 工件互证）→0dB 在条高 74%、可用值域带仅 54px/42% 死区=「上提一半到顶/下拉一半消失/看不到底」观感源；**A 128=主修**净高 54→86px）。**映射一致性断言必入兑现**：P2c 管线往返 err=0.0000+P2d 渲染位 0.1px+P1b 三值渲染像素位 vs 映射期望±5px 全 PASS。
  - **验收数据**：前端 commit=**34ca579**（port/fe-lane-view-tuning-1 已推，8 文件 +1066/−22：生产 1+探针 7）；真栈 run=coord/runs/FE-LANE-VIEW-TUNING-1/laneview_20261029_1（工件随 **port/fe-lane-view-tuning-1-stack** 分支入库，RESULTS.md 六节）；栈登记 828141cb/释放本批（三纪律全守：登记先于启动+推送后即时复核<60s+vsphub 手启前亲核 8787 空+拆栈三 pid 路径逐一亲核）。**正式轮 31/31 PASS×2 连续 exit 0**（窗口模式：P0 §5.1 装配[timeline_dock_adapter.tscn 生产实件+strip 渲染高==有效高+拖柄在场+N=1 无溢出滚动]/P1 渲染测量[外部写 0-50-100% 三值渲染像素位+content rect 实测]/P2 映射一致性[折线段管线建点往返 0.0000+渲染 0.1px+内核一致]/P3 拖柄管线注入[网关让路+press→+36→release 会话高 128→164+双条统一+budget clamp 316+内部滚动+零内核写]/P4 211 回归面[新高度画布建点 5 点内核一致]）+**env 覆盖轮 112 31/31 exit 0**（P0i strip=112 source=env 实锚）+探针迭代 r1-r4 留证（全探针面：r1 采样位/空集守卫、r2 零位移释放=203 语义、r3 段中点=曲率柄位[211 P5 同款]、r4 内核读竞态轮询）+**真栈族既有 3 件零回退**（edit 30/30+rebuild 23/23+layout_grow 16/16）+**离线全家终态全绿**（26 -s 件+file_reply 场景 7/7；四件初判红=输出格式误判亲证 lane_catalog 18/18+vsp_delta 30/30+vitnote_notestream 26/26[SUMMARY_JSON/checks 格式]+vitnote_input=取证件；survey×2=209 勘察件挂起无看门狗 kill 不计）。
  - **探针期望值更新披露（授权面，198 先例）**：五件随高更新——probe_autolane_layout_grow（STRIP_H 改生产面查询+N=3 语义变预算 clamp 408>316）/stack_probe_autolane_layout_grow（STRIP_H 128：want_a 392/want_b 520）/probe_autolane_rebuild（STRIP_HEIGHT 128）/probe_envelope_lane_ui（STRIP_HEIGHT 128）+全族 `_ADD_BAR 22→24` 校准；新探针 probe_lane_view_tuning（离线 26 断言）+stack_probe_lane_view_tuning（窗口模式 31 断言）。
  - **端测覆盖边界声明**：探针窗口模式=真实 GUI 派发管线+真实渲染像素采样（与用户手测同构），非用户键鼠；栈为探针最小腿（-StartKernel -RestartAgent 无 UI 全旅程）。
  - **[等待用户:手测观感终值验收]**——用户从 Godot 拉起进工程（同 shell 先设 `VIT_GUI_LANE_STRIP_HEIGHT`=112/128/144 试值定终值），验收：车条高度观感/垂直映射观感/⇕ 拖柄调高（多条统一+重开回默认=202 语义）。
  - **上交主管知悉**：ⓐB 主症状=候选③ fader 域观感非机械错位（渲染/映射/交互三面像素级一致已证）——dB 域显示增强（0dB 参考线/刻度）为独立观感小卡候选，是否开卡由主管裁定；ⓑ容器对他方 automation 写回执只刷 ●N 不重拉曲线=192 既有设计（探针显式 _request_curve 测量）——是否补外部写自动重拉由主管裁定；ⓒ＋条常量 24 依赖主题/字号（未来漂移由 P0k 拦）；ⓓN=3「全展开」语义随 128 失效（408>316 预算 clamp=超高滚动形态，206 裁定定案的按实际条高联动）。
