# M3-C4-AUTO-DISCLOSE-1 真栈工件与结果（RESULTS）

- 卡：coord/cards/done/2026-10-29-M3-C4-AUTO-DISCLOSE-1.md（池序 217）
- 执行：ZCode GLM-5.3 PC 执行会话（217 卡会话）；worktree D:/Vit_DAW_wt_m3c4ad1 @ 分支 port/m3-c4-auto-disclose-1
- 领取基线：origin/main=635c0592（领取提交 d50a65a0）；占用登记 aab65181（rebase 后）

## 一、冒测执行形态与两轮判定

- 命令（两轮同形，worktree 根执行）：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_m3c4ad1/scripts/dev_agent_smoke.ps1 `
  -StartKernel -RestartAgent -AutomationAgentChainSmoke -NoChatSmoke -NoStripSilenceSmoke `
  -KernelExe D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe `
  -RunArtifactsDir <run 目录>
```

- **R1（automation_agent_chain_20261029_r1）：exit 0 PASS**——腿本体 21:20:01–21:20:03（整轮含 agent 构建+栈启动约 21:16 起）
- **R2（automation_agent_chain_20261029_r2）：exit 0 PASS**——腿本体 21:20:52–21:20:54（独立轮：R1 拆栈后重启内核+agent）
- 被测栈：kernel=主检出 `D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe`（sha256 c1e32dfe5338a9e1…，2026-10-28 21:26 构建，209 卡登记「190/201 面在场」同源只读复用；本卡 C++ 零改动）；agent=worktree 自建（dev_agent_smoke.ps1 构建部署 `agent/bin/VitAgent.exe`，含本卡全部 agent 面改动）；无 UI。
- 两轮装载插件同为 ValhallaSupermassive（plugin_search 首选命中）；两轮 LEG A 目标参数同为 `dry level`（raw 值域 [0.0, 1.0]）。

## 二、五腿断言明细（两轮同形全绿）

| 腿 | 断言点 | 结果 |
|---|---|---|
| setup | 隔离工程（new_project+save_as_project）+双轨+Supermassive 装载（内核 ZMQ rack_add_node，pgtdc 探针先例不碰 PCA 门控）+轨级 Volume&Pan 插件 id 解析（agent project.state 无 plugin 行→内核 get_project_state fallback） | R1/R2 ✓ |
| LEG A 外部插件参数链（agent /agent/invoke 面） | get_plugin_parameters（include_parameters:true）行可见（agent 投影 param_id 集含目标）→ **未确认 set_points → status=needs_confirmation+requires_confirmation=true+preview 非空+get_curve point_count=0（确认前零写入=RiskConfirm 确认面实证）** → confirmed=true 三点域内写 → result.written=3 → get_curve point_count=3+逐点 t/v/c 一致（容差 1e-6）→ 内核描述符 envelope_point_count 跟随=3 → clear_region removed_points=3 → 曲线与描述符回 0 | R1/R2 ✓（7 检×2） |
| LEG B 轨级 volume ride 链（agent 面） | agent 投影恰 volume/pan 两 param_id → 内核描述符面恰 volume/pan 两行+volume raw 值域 [0,1]+pan [-1,1]（201 镜像）→ confirmed set_points 三点 ride（0.2→0.5→0.8）→ written=3 → get_curve 逐点读回一致 → 描述符跟随=3 → clear_region → 全零 | R1/R2 ✓（6 检×2） |
| LEG C undo 腿（agent 面） | set_points 三点 → get_curve=3 → **project.undo → get_curve=0（SupportsUndo 回滚+读回归零）** | R1/R2 ✓（3 检×2） |
| LEG D 负面例（agent 面） | 两点载荷（一点域内 v=0.5+一点域外 v=2.0>max 1.0）confirmed=true → **整令拒绝**（status=kernel_error，拒绝回执原文在 summary artifact：`automation.set_points point #1 has v outside the parameter value range [0, 1]`）→ get_curve point_count=0（拒绝后零写入） | R1/R2 ✓（3 检×2） |

### agent 面与内核面 schema 边界（如实记录）

- agent /agent/invoke 的 get_plugin_parameters（include_parameters:true）回参行=display-probe 增强投影：行标识键=`param_id`，值域在 `display_domain_candidate`（display 域，如轨级 volume -100~6 dB），**无顶层 min/max、无 envelope_point_count 列**。
- raw 值域（set_points 校验域）与 envelope_point_count（201 描述符列）在**内核 ZMQ 面**行（顶层 id/min/max/envelope_point_count）。断言体双面分工：写入/读回/清除/undo 全走 agent invoke（本卡断言面）；raw 值域与描述符计数读内核面；agent 投影参数行可见性单独断言。此为投影面边界非缺陷，201 的描述符面验证本就在内核面。

## 三、迭代留痕（脚本面缺陷三轮，被测面零失败）

1. R1 首轮失败=断言体未带 `include_parameters:true`（agent 面回压缩摘要无 parameters 数组）——脚本修复。
2. R1 次轮失败=断言体在 agent 投影行上找顶层 min/max（投影无此键）→ 改为内核面读 raw 值域+双面记录；同时发现并修复工程隔离缺失（见 §四）。
3. R1 三轮失败=LEG D 断言期待 status=error，实际 agent 面内核拒绝回执 status=kernel_error（fail-closed 语义本身正确，point #1 越界拒绝消息在卷）——断言接受两种拒绝形态并加"outside the parameter value range"文本断言。
4. 被测面（agent automation 链）三轮迭代中零失败；最终 R1/R2 两轮 exit 0。

## 四、主检出 default_project.xml 写入事故（如实上交）

- 事实：R1 无隔离轮（21:03–21:06 窗口）内核执行断言体 `new_project` 时覆写了**主检出** `VitApp/Workspace/default_project.xml`（mtime=21:06；git diff vs HEAD=+11/-142，新增 "Input 1"/"Input 2"=new_project 默认轨特征，与本腿命令逐项吻合）。
- 机制：本腿复用主检出 build 内核 exe（209 先例）——内核 Workspace 锚自 exe 路径回溯至主检出 `VitApp/Workspace`；201 先例无此问题因其内核为 worktree 自建（写 worktree Workspace）。183/185（同款先例事故）后 ps1 已有 GUI 腿隔离段，但本腿为 agent 面腿未在条件列表内。
- 损失评估：覆写前 mtime 12:12=用户手测遗留态（vs HEAD 已为 M 态）——**该态内容已被覆盖，无法从 git 恢复**（HEAD 非该态）。处置（恢复/保留/找其他备份）交主管裁定；执行侧按 AGENTS §12 不擅自 reset/checkout。
- 修复（本卡内完成）：`-AutomationAgentChainSmoke` 加入 dev_agent_smoke.ps1 共享工程隔离段条件列表（VIT_PROJECT_XML=run 目录 git 干净副本，任何进程启动前设置）；R1 最终轮与 R2 隔离亲证生效（console 留 `project isolation: VIT_PROJECT_XML=…` 行；**主检出 mtime 保持 21:06 不再变化**，隔离副本承接了全部写入）。

## 五、回归与单测

- `go build ./...` exit 0（收口复跑）。
- `go test ./internal/capabilitycontext/... -count=1` ok（含本卡新增锁定测试 `TestAutomationNotExcludedAndDisclosedInStaticMixManifests`：四 manifest Excluded 不含 automation+Guidance 披露 automation.* 工具面+轨级 Volume & Pan 寻址）。
- `go test ./internal/chat/ ./rules/... -count=1` ok（chat 全家 117s 含 `TestChatSystemMatchesRulesetEmbedByteForByte`+`TestChatSystemFallbackMatchesEmbedMinusPostMigration` parity 锁定——legacy 模板与 embed 源双插入后字节一致）。
- ContextManifest 消费面盘点：全部在 capabilitycontext 包内（context_builder/context_pack/gain_staging/static_balance/pan_layout/low_end_relation/frequency_cleanup），无跨包消费——全家测试即全覆盖。

## 六、端测边界声明

- 覆盖=agent /agent/invoke HTTP 面真栈全链（确认面/写/读回/清除/undo/负面例，两轮独立栈）+内核 ZMQ setup/值域/描述符面；内核复用主检出二进制（190/201 面在场，209 同源）。
- 不覆盖=webui 渲染面与用户旅程（E2E-WEBUI-1/JOURNEY-1 既有边界）；LLM 话术→工具选择面（本腿为确定性工具链驱动，模型路由指引已入 system prompt 但模型行为未测——归第二段卡 M3-C4-AUTO-INTENT-1 烟测面）；Godot 车道呈现（agent ZMQ 写不入 kernel_command_signal_bus 的感知延迟=197 已记既有边界）。
