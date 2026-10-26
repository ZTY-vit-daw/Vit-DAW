# M3-ENVELOPE-CLOSEUP-VIEW-1(194) 真栈腿回执——envelope_closeup_20261026_2131

- owner=ZCode GLM-5.3 PC 执行会话（ZCode，194 卡会话）/ 2026-10-26 晚窗
- 被测=前端 worktree D:/Godot/project/vit-daw-frontend-envcloseup1 @ port/m3-envelope-closeup-view-1 **@2f232a0**（包络特写视窗交付面：registry envelope 条目+vit_open_envelope_in_stage_slot+envelope_dock_adapter+envelope_closeup_canvas+车道 ⤢+timeline 转发）
- kernel=**192 自建二进制只读复用** `D:/Vit_DAW_wt_envauto1/VitApp/build/VitApp_artefacts/Release/VitApp.exe`（拆栈前路径亲核=pid 14164 即该文件；本卡零内核改动亲核：`git log e70bafc7..origin/main -- VitApp/` 为空，与 192 回执 sha 5D31056A 系同源）——复用依据=189 先例「零内核改动只读复用」
- agent=stack worktree D:/Vit_DAW_wt_envcloseup1_stack（origin/main=499239bb）构建 VitAgent.exe pid 7188（路径亲核）
- ps1=最小腿 `dev_agent_smoke.ps1 -StartKernel -RestartAgent -NoChatSmoke -NoStripSilenceSmoke -KernelExe <192 二进制> -GodotProjectRoot <envcloseup1> -RepoRoot <stack worktree>` **exit 0**（ps1_min_leg.log）
- VIT_PROJECT_XML=run 目录 git 干净版隔离副本（947E4D00 基线，源=stack worktree）显式透传

## 探针结果（stack_probe_envelope_closeup.tscn，场景模式生产 GUI 传输链 UDP 4445→agent→内核 ZMQ）

- **R1 21:3x**：exit 1（28 断言 4 败）——**探针期望缺陷非生产缺陷**：V6/V7 场景设计中 region=[前锚 0, 后锚 3+ε] 覆盖外部写全部三点，「本地区域覆盖」=正确合并语义，但未构造 region 外保留面；V1-V5 核心面首轮全绿。修正=探针 V6 外部写加 t=5 点（区域内本地覆盖+region 外外部保留双面）+V7 改四点删至唯一点（commit 2f232a0）。
- **R2 21:37**：**exit 0，28/28 PASS**
- **R3 21:38**：**exit 0，28/28 PASS**（连续两轮收口）

## 断言面（卡面验收③全覆盖）

- setup：clear_project→add_track→plugin_search ValhallaSupermassive（真发现）→rack_add_node 装载外部 VST→描述符预热→目标参数 dry level
- V1 开视窗：adapter 场景实例化+setup_view（ctx 预选 track/plugin/param）→目录注入→**真描述符请求经生产链回执**→页签构建（envelope_point_count 全零=灰显无标记）→真曲线拉取→空曲线披露
- V2 呈现读回：外部 set_points 三点（c=0.25/-0.5 非零曲率透传）→bus 他方写回执→adapter 自动重拉→**canvas display 逐点（t,v,c）=写点+内核 get_curve 对照一致**
- V3 视窗拖点：局部手势机 press/motion/release→释放恰一笔 set_points region→回显门→display=新值+**内核对照逐点一致**
- V4 折线加点：按下即建+拖+释放→4 点+内核对照一致
- V5 Delete 删点：区域内剩余=set_points region→3 点+内核对照一致
- V6 拖拽态外部写：拖拽中外部写（含 region 外 t=5 点）到达内核→**canvas 挂起忽略（working 不跳变）**→释放写回→**合并=区域内本地覆盖+region 外外部保留**+内核对照一致
- V7 删至唯一点→clear_region 分支→空披露+内核回空
- cleanup clear_project ok

## 三纪律（§2.2 增补条款全执行）

- a) 登记 d3bcd919 推送后 **21:31:58 即时复核**（距登记 <60s）进程/端口全零（VitApp/VitAgent/VspHub=0；TCP 5555/5556/7878/8787/18787 listen=0；UDP 4444/4445=0）
- b) 无 already listening（kernel 14164=本腿 ps1 自启）
- c) 登记先于启动（21:31 登记→21:31:5x ps1）

## VIT_PROJECT_XML 隔离亲证

- 隔离副本被写：sha 947E4D00→338365E6（末次写 21:37=R3 探针 cleanup 前内核自动保存）=隔离生效
- stack worktree `VitApp/Workspace/default_project.xml` 运行前后同哈希 947E4D00+git status 干净=零触碰
- 主检出 default_project.xml=1fef9ce9（=189 回执基线，本腿零触碰）

## 拆栈

- owner 手拆 VitApp 14164/VitAgent 7188（路径亲核归属本腿）→ 21:38:22 复核：VitApp/VitAgent/VspHub=0、TCP 5555/5556/7878/8787/18787 listen=0、UDP 4444/4445=0
- 注记（不处置）：登记前见两个非本腿 Godot 进程（33164 7MB/12836 137MB，不占栈端口，未动——189 同款纪律）

## 工件清单

- ps1_min_leg.log（ps1 最小腿 console，exit 0）
- probe_r1.log（探针期望缺陷轮留证）/ probe_r2.log / probe_r3.log（连续两轮 exit 0，SUMMARY_JSON 各含 180 横幅 stack_warning）
- default_project_isolated.xml（隔离副本，运行后被写=生效证据）
