# FE-ENVELOPE-DETAIL-HOME-1(200) 真栈腿回执——envelope_detail_home_20261027_1245

- owner=GLM-5.3 PC 执行流（ZCode 会话，200 卡会话）/ 2026-10-27 午窗（F1=A 用户裁定实施）
- 被测=前端 worktree D:/Godot/vit-daw-frontend-envdetail1 @ port/fe-envelope-detail-home-1：**R1 时=61f2135（交付面 commit）；R2/R3=61f2135+探针悬空捕获修复工作树=即 0cc771a 内容**（探针即源零构建差；生产代码两轮间零改动）
- kernel=**190 自建二进制只读复用** `D:/Vit_DAW_wt_envp1/VitApp/build_release/VitApp_artefacts/Release/VitApp.exe` sha256 85B4E8DF…（196 同款只读复用；本卡零内核改动亲核：origin/main 自 72a4f0ea(190) 后无 VitApp/ 提交）pid 7416 本腿 ps1 -StartKernel 自启
- agent=stack worktree D:/Vit_DAW_wt_envdetail1_stack（origin/main=7ada4fe1）构建 VitAgent pid 36008（ps1 自启）
- ps1=最小腿 `dev_agent_smoke.ps1 -StartKernel -RestartAgent -NoChatSmoke -NoStripSilenceSmoke -KernelExe <190 二进制> -GodotProjectRoot <envdetail1 worktree> -RepoRoot <stack worktree>` **exit 0**（ps1_min_leg.log；含 Range context 冒测绿+内核 ZMQ 5555/5556 监听核对+project.state ok；ps1 不自带 teardown，owner 末腿手拆）
- VIT_PROJECT_XML=run 目录 git 干净版隔离副本（基线 4E899FB6←stack worktree HEAD=7ada4fe1 的 git blob，经 git show 字节直出）显式透传

## 探针结果（stack_probe_envelope_detail_home.tscn，场景模式生产 GUI 传输链 UDP 4445→agent→内核 ZMQ）

- **R1 12:40**：exit 1（24 PASS+看门狗超时）——**探针缺陷非生产缺陷**：W4 回 detail 后 ⤢ 转发直达=activate_view 重建 adapter，探针沿用 W2 捕获的旧实例（已 queue_free）→ lambda 悬空捕获 → 套件挂起 → 看门狗 420s；核心面（setup/W0/W1/W2/W3/W4 前 3 面）首轮全绿。修正=转发后重取新实例断言（栈探针 W4+-s 探针 D18 同隐患同步修），commit 0cc771a 零生产改动。
- **R2 12:4x**：**exit 0，30/30 PASS**
- **R3 12:49**：**exit 0，30/30 PASS**（连续两轮收口；-s 探针修复后复跑 52/52）

## 断言面（卡面目标①②③+验收③真栈腿全覆盖）

- setup：clear_project→add_track（track:1035）→plugin_search ValhallaSupermassive（**真发现非回退**）→rack_add_node 装载外部 VST→描述符预热（真参数行）→目标参数 dry level（真值域锚定）
- W0：VitDockRoot.tscn 全树 headless 实例化+settle（shrink phase A 同款）+**repo 从内核同步 rack node 到位（生产刷新链）**
- W1 方块按钮链：vit_open_track_detail→特写窗开窗（复用键 track:1035）+view_type=detail+detail adapter 真实装配树在场
- W2 包络主场直达：vit_open_envelope_detail（ctx 预选）→**同窗复用（frame 同一实例）**+切 envelope+envelope adapter 真实装配树在场+**窗内视图切换面含 Detail/Envelope 两项**
- W3 呈现读回：目录走 repo 生产链（repo_synced，无注入回退）→**ctx 预选落地（真选中 plugin+param）**→外部 set_points 三点（c=0.25/-0.5 非零曲率）→bus 他方写回执自动重拉→**canvas 呈现逐点（t,v,c）=写点+内核 get_curve 对照一致**
- W4 车道 ⤢ 带参直达：先回 detail（模拟用户在 3D 波形页）→timeline_dock_adapter 真树转发口直调（⤢ 链）→**同窗直达 envelope+新装配实例预选（plugin+param 真选中）**
- W5 启动器撤列（真栈面）：stage_state_snapshot spawnable 不含 envelope（timeline/detail 完整性并行）+**全树 EmptyViewportLauncher 按钮标签不含 Envelope**
- W6 旧口降级：vit_open_envelope_in_stage_slot 直调→**台前 stage 视图集不变零 spawn+特写窗表仍单窗**
- cleanup：窗口全关+dock root 释放+clear_project ok

## 三纪律（§2.2 增补条款全执行）

- a) 登记 db0bbf8e 推送后 **12:39:28 即时复核**（距 ps1 启动 <60s）进程/端口全零（VitApp/VitAgent/VspHub=0；TCP 5555/5556/7878/8787/18787 listen=0；UDP 4444/4445=0）
- b) 无 already listening（kernel 7416/agent 36008 本腿 ps1 自启）
- c) 登记先于启动（12:39 登记→12:39:3x ps1）

## 错峰在案

- 登记前 fetch 重读记录：196 已收口拆栈在案（12:28 登记+拆栈复核，无在飞占用）
- 非本腿 Godot 探针进程注记：登记前核对时见 lane_auto/lane_ui -s 探针进程陆续出现（198 执行侧在飞零回退验证，不占栈端口，留档不动）；本腿全程未与其互染

## VIT_PROJECT_XML 隔离亲证

- 隔离副本被内核写入：4E899FB6→**52E3C5E5**（隔离生效）
- stack worktree VitApp/ 运行前后 git 干净（default_project.xml git blob=HEAD 287956e7 不变；工作文件 mtime=12:37 建树时刻=运行期零触碰；PowerShell 原文件哈希 947E4D00 与 git show 口径差异=行尾过滤器，非写入）
- 主检出 default_project.xml=13A155AA（会话开局 git status 即有 M=既有改动，本腿零触碰）

## 拆栈

- owner 手拆 VitAgent 36008/VitApp 7416 → **12:50:04 复核**：VitApp/VitAgent/VspHub=0、TCP 5555/5556/7878/8787/18787 listen=0、UDP 4444/4445=0

## -s 探针面（零栈，交付时点全绿）

- probe_envelope_detail_home（新独立探针）**52/52**（A 注册撤列/B 源码接线/C frame 单件切换面/D dock_root 全树——同窗复用/ctx 管线读回/⤢ 真树转发/旧口零 spawn）
- probe_envelope_closeup（194 R/O 段迁移，撤列翻转+清单完整性+新入口/零 spawn/存根三面，**79→83 不弱化**）**83/83**
- 既有零回退：curve 55 / lane_ui 39 / lane_auto 41 / badge 33 / datachan 50 全绿

## 工件清单

- ps1_min_leg.log（ps1 最小腿 console，exit 0）
- probe_r1.log（探针悬空捕获缺陷轮留证，核心面首轮全绿）/ probe_r2.log / probe_r3.log（连续两轮 exit 0，SUMMARY_JSON 各含 180 横幅 stack_warning）
- default_project_isolated.xml（隔离副本，运行后被内核写入=生效证据）
- 端测边界声明：headless 断言面=装配树/状态机/数据链；**窗口视觉呈现/菜单手感/⤢ 真实点击手感不在探针面，留用户手测**（渲染面与用户旅程门槛两层级建设卡 E2E-WEBUI-1/JOURNEY-1 前按 AGENTS §5 显式声明）
