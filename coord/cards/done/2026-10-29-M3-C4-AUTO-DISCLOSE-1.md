# M3-C4-AUTO-DISCLOSE-1：automation 披露解锁+路由指引+agent 全链真栈首证（A-F C4 直达底座=形态案三第一段）

- 发卡：GLM 主管决策流 / 2026-10-29 晚（依据=用户指示按 agent_action_workflow §6.1 C4+§6.3 要求实现包络自动化；197 勘察案三推荐（两段混排）用户拍板；锚点全取 197 RECON §三）
- 派发确认：已确认（用户 2026-10-29 晚指示"按 A-F 混音能力环节要求实现包络线自动化控制"+索手测方案）
- 验收负责人：GLM 主管决策流
- 池序 217；目标仓库=**主仓 D:\Vit_DAW agent/ 域**；**P1** / 0.5 天 / 依赖=无（内核面 201/190 已合入真栈验证 ✓；catalog 三工具在位 ✓）；L2 / PC 执行会话
- 并行与资源：真栈腿 §2.2 登记（190/196 模式扩展）；与 216 卡文件域不相交可并行
- 领取：2026-10-29 20:40 / origin/main=635c0592（领取前即时复核，初查 82dc1566 后远端推进至 216 卡确认提交 635c0592，worktree 自最新基线建立）/ owner=ZCode GLM-5.3 PC 执行会话（217 卡会话）/ 分支 port/m3-c4-auto-disclose-1 / worktree D:/Vit_DAW_wt_m3c4ad1（干净新建）/ 领取提交=coord/m3-c4-auto-disclose-1-claim（本提交）。领取核对：卡在 todo+派发已确认 ✓；依赖无（201/190 已合入 ✓）；doing 唯一卡=GEN-SUNO-WEB-DESIGN-1（Mac 侧设计卡，文件域不相交 ✓）；216 已确认未领取（域不相交+216 离线零真栈 ✓）；真栈记录最新=214 卡 2026-10-29 19:1x 释放（待用前即时复核）。领取基线记录：主检出 HEAD=82dc1566、status 脏文件均与本卡域无关（VitApp/Workspace 运行时状态+coord/runs 工件+webui design 未跟踪件，摘要入 run 工件）；实现 worktree 自 origin/main=635c0592 新建零改动零漂移。

## 背景（A-F 权威要求，docs/agent_action_workflow_v1_master_plan.md）

- §6.1 C4=段落自动化（verse/chorus/bridge 音量 ride/能量/空间变化）；A-F 非线性调用（直达请求合法形态）。
- §6.3=线性包络线/automation 读写是 DAW 缺口清单项，补齐原则：最小可用 capability+typed executor/capability contract（禁 raw command 旁路——catalog 工具面已合规）+补齐后重跑 workflow smoke。
- 本卡=直达底座（显式包络/音量 ride 请求可达）；段落编舞（时变话术→范围 goal 单轮终态+mix_decision_record+workflow 节点）为第二段卡，**本卡合入后另拆**。

## 已核事实（发卡侧，全取 197 勘察+亲核）

1. catalog 三工具在位：automation.get_curve（:862-882）/set_points（:884-924，RiskUndoable+MutatesProject+SupportsUndo+RequiresConfirmation+RefreshAfter 全旗，fail-closed 整令拒绝）/clear_region（:926-955）；harness pre-journal validateAutomationWrite（harness.go:1079-1089）。
2. 披露锁两处：context_manifest.go 四处 Excluded 含 automation（:215/:257/:301/:336，197 核对零漂移）；server 路由指引零 automation 行（server.go:6198-6212，对位 :6203 clip.fade 行式样）。
3. 普通回合 AllowedTools 无 automation 过滤（goalrunner_chat.go:4171-4192）——解锁即可达；自由态 loop 活跃时收窄（:4137-4145）维持不动。
4. 轨级寻址已具备：每轨恒挂 Volume & Pan 内置插件（type=volume），201 已放行 volume/pan 参数行进 get_plugin_parameters（envelope_point_count 同源）——**音量 ride=轨级 volume 参数 set_points，201 真栈已验内核链（三点写→count=3→clear→回 0 双轨隔离）**。
5. agent 侧全链真栈零证据（M3-KERNEL-1 回执自认 authority 面不在其腿内）——本卡补。

## 目标

1. **披露解锁**：context_manifest 四处摘 automation 词+各视图 Guidance 补 automation 指引行（披露契约变更=显式行为；manifest 既有单测随改）。
2. **路由指引**：server 路由段补 automation 行——含轨级 volume/pan 寻址指引（音量 ride 形态）+与 clip.fade 的淡出分界一行；对位既有行式样。
3. **真栈首证腿**（190/196 模式扩展冒测脚本，两轮口径）：话术驱动 /agent/invoke → get_plugin_parameters 读值域 → set_points 写（RiskConfirm 确认面）→ get_curve 读回点数/值一致 → clear_region 清除 → count 归零；**必含轨级 volume ride 腿**（外部插件参数腿+轨级腿各一）；undo 腿（SupportsUndo 回滚+读回）。
4. **负面例腿**：值域外点（fail-closed 整令拒绝+拒绝回执）。
5. 既有全家回归零回退（manifest 消费面+capabilitycontext 测试）。

## 手测面归属（2026-10-29 晚补注）

本卡交付后用户手测为**可选冒烟**（指令式话术一轮即可）；正式手测面=自然语义（「副歌让这条轨存在感强一点」类），归属第二段卡 M3-C4-AUTO-INTENT-1（池序 218）——指令式话术烟测能过则手测必过，手测增量价值在语义理解与自主决策面（用户裁定 2026-10-29 晚）。

## 探针义务

§5.1 装配面（生产装配形态探针）；时序面按适用；真栈腿=完整命令+退出码+run 工件入 coord/runs/M3-C4-AUTO-DISCLOSE-1/。

## 文件域

`agent/internal/capabilitycontext/context_manifest.go`+路由指引所在文件（勘察锚 server.go :6198 段，执行侧核对包位）+`scripts/` 冒测扩展+探针。**内核/前端仓零改**。越域即停。

## 停止条件

- 四处 Excluded 或路由段锚点与勘察记录大面积不符（结构漂移）→锚点清单上交；
- /agent/invoke authority 面发现 catalog 外必经环节（如确认面缺装配）→证据上交两案；
- 轨级 volume ride 腿遇 201 描述符面行为不符（寻址失败/值域缺失）→如实记录上交（内核域外不修）。

**三条停止条件均未触发**：四处 Excluded :215/:257/:301/:336 与 server :6203 clip.fade 行零漂移；authority 面 confirmed 语义直达（needs_confirmation 挂起/确认执行，无 catalog 外环节）；轨级寻址与值域 [0,1] 与 201 记录一致。

## 回执（执行侧自验完成，待主管验收）

- 实现 commit：**port/m3-c4-auto-disclose-1 @ 8906d209**（7 文件：manifest+新锁定测试+server legacy 行+embed 源行+ps1 开关腿与隔离条件+新断言体+run 工件；diff 亲读无越域；分支已推远端）。
- 改动一：context_manifest.go 四处摘 automation 词（B2/B3/B4/C1 Excluded）+四视图 Guidance 各补 automation 披露行（automation.set_points/get_curve/clear_region 三工具名+B2/B3 轨级 Volume & Pan 寻址+各能力"本域不写"边界）；新测试 TestAutomationNotExcludedAndDisclosedInStaticMixManifests 锁定（Excluded 零 automation 回退+Guidance 披露在+轨级寻址在）。
- 改动二：server 路由段补 automation 行——**发现权威面=ruleset embed**（legacy 模板仅 fail-open 回落且 parity 测试字节锁定），故 legacy（server.go）+embed 源（chat.discipline.plugins_mixing.txt）**双插入同文**；行含包络请求路由三工具+外部插件/轨级 Volume&Pan 双寻址+值域先读+写后读回+**clip.fade 淡出分界**（对位 :6203 行式样）。parity 两测试全绿。
- 真栈首证：**R1/R2 两轮 exit 0**（automation_agent_chain_20261029_r{1,2}，独立栈轮）——五腿全绿×2：A 外部插件链（ValhallaSupermassive dry level [0,1]：agent invoke 行可见→**未确认 needs_confirmation+requires_confirmation+preview+曲线零写入**→确认写三 written=3→get_curve 逐点 t/v/c 一致→描述符 count 跟随 3→clear 回零）+B 轨级 volume ride 链（恰 volume/pan 两行+值域 [0,1] 镜像 201+同链写读清）+C undo 腿（project.undo→读回归零）+D 负面例（域内+域外两点载荷→**整令拒绝** status=kernel_error+拒绝回执原文在 summary artifact+曲线零写入）。迭代三轮均为脚本面缺陷（include_parameters/raw 值域在内核面/kernel_error 形态），被测面零失败。
- **agent/内核面 schema 边界（如实）**：agent invoke 的 get_plugin_parameters 回 display-probe 投影（param_id+display_domain_candidate，无 envelope_point_count 列）；raw 值域与 201 描述符列在内核 ZMQ 面——断言体双面分工（写读清 undo 全 agent 面=本卡断言面；值域/计数读内核面+agent 投影可见性单独断言）。非缺陷，201 验证本在内核面。
- **上交事项（主管裁定）**：R1 无隔离轮（21:03-21:06）内核 new_project 覆写**主检出 VitApp/Workspace/default_project.xml**（mtime 21:06，+11/-142；覆写前 12:12 用户手测遗留态不可从 git 恢复）——机制=主检出 build 内核 exe 的 Workspace 锚回溯（201 先例因 worktree 自建内核天然隔离无此问题）；修复=AutomationAgentChainSmoke 入 ps1 共享工程隔离段（VIT_PROJECT_XML=git 干净副本），最终轮起主检出零再写亲证；主检出该文件现态处置交主管（§12 不擅动）。详见 RESULTS.md §四。
- 回归：go build exit 0；capabilitycontext 全家（含新测试）+chat 全家（117s 含 parity 两测试）+rules/ruleset 全绿；ContextManifest 消费面盘点=全在 capabilitycontext 包内无跨包消费。
- 工件：coord/runs/M3-C4-AUTO-DISCLOSE-1/{RESULTS.md,automation_agent_chain_20261029_r1/,automation_agent_chain_20261029_r2/}（§9 契约：完整命令/退出码/时间戳/迭代留痕/事故记档/端测边界；console+summary+隔离 marker 入库，运行时工程文件剔除）@ port/m3-c4-auto-disclose-1。
- 真栈登记/释放：aab65181 登记（三纪律：登记先启动+推送后 21:01 即时复核净空<60s；vsphub 不需要）→释放记录同批本提交；拆栈复核 VitApp/VitAgent/VspHub/Godot=0、TCP 5555/5556/7878/8787 listen=0。
- 端测边界声明：覆盖=agent /agent/invoke 真栈全链两轮+内核 setup/值域/描述符面；不覆盖=webui 渲染/用户旅程（E2E-WEBUI-1/JOURNEY-1 既有边界）+LLM 话术→工具选择面（归 218 烟测）+Godot 即时感知（197 已记既有边界）。
