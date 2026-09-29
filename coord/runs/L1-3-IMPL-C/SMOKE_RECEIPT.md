# L1-3-IMPL-C 真栈烟测回执（run R6 = 交付 run）

## 交付 run（R6）

- **命令**：`powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev_agent_smoke.ps1 -SkipBuild -NoChatSmoke -NoStripSilenceSmoke -RefQuerySmoke`（worktree `D:\Vit_DAW_worktrees\l1-3-impl-c`，分支 port/l1-3-impl-c）
- **退出码**：**0**（场景全绿——8 条断言全部 ok，见 smoke_console_R6.log）
- **测试时 HEAD**：6a94eadd（领取提交）；被测工作树含本卡未提交 diff（catalog.go/harness.go/ref_query.go/ref_query_test.go/dev_agent_smoke.ps1，与实现 commit 一致）
- **被测二进制**：VitAgent.exe 由 R4 现场构建（sha256 `fd233bb6b41c1115c467f9386cdaee19c0cea55565a46e9b68705ecb120c08b4`，agent/bin/VitAgent.exe）；R6 用 `-SkipBuild` 复用 R4 构建+R4 重启的 agent（脚本层改动无需重建）。内核=主仓 staging 既有产物 `D:\Vit_DAW\Export\staging\runtime\VitApp.exe`（本卡零内核改动）；Godot 前端=`D:\Godot\project\vit-daw-frontend`（Godot_v4.6.1）。三件套由 R1 以 `-StartKernel -StartUI -RestartAgent -KernelExe <staging>` 拉起
- **场景结果**（全确定性断言，无 LLM 参与）：
  - ref.query/ref.diff 在 /agent/tools 广告 ✓
  - ref.query kinds=[dom,fxm,com,acp] limit=50 → **50 真实行**（draft 工程物化观察票），cost_class=index，五元数据齐 ✓
  - 降级路径（payload_conditions）→ cost_class=compile + degraded=payload_index_unavailable（T11 真栈）✓
  - T10 三反例：空谓词 / limit=501 / 未知字段 → 全部 fail-closed 拒绝（错误文案匹配）✓
  - ref.diff base_revision=current → ok，cost_class=index，unchanged_count=23 ✓
  - ref.diff depth=content → 显式 not implemented 拒绝（IMPL-D 诚实边界）✓
- **原始工件**：smoke_console_R6.log（控制台）、agent_last_R6.log（agent 日志）、.run_note（各 run 退出码与二进制哈希）

## 迭代经过（R1→R6，按 §9 如实记录）

| run | 结果 | 原因 |
|---|---|---|
| R1 | exit 0 但**场景未跑** | 内核插件表冷载拖慢 ZMQ 绑定（>60s），脚本端口检查时 5555 未监听 → RefQuerySmoke 块被跳过；exit 0 不采信，未当交付 |
| R2 | exit 0 但场景未跑 | 端口已监听、project.state ok，但 RefQuerySmoke 块嵌错层（落在 `-NoStripSilenceSmoke` 跳过的 if 内）——嵌套修正 |
| R3 | exit 1 | 场景首跑，抓到实现缺陷：Invoke 管线注入 agent_action_id/tool_call_id 触发未知字段拒绝 → 修复（refHarnessManagedKeys 剥离）+ 补单测 TestRefQueryToleratesHarnessManagedKeys |
| R4 | exit 1 | agent 重建后场景过半（广告/50 行/T11 ✓）；挂点在脚本层：/agent/invoke 对拒绝回非 2xx，Invoke-WebRequest 抛异常未取响应体 → 容错读取 |
| R5 | exit 1 | 容错 catch 读 GetResponseStream 为空（PS 5.1 流已被错误格式化消费）→ 改读 `$_.ErrorDetails.Message` |
| R6 | **exit 0 场景全绿** | 交付 run |

## 泊位声明

R6 后已拆除全部本卡启动的进程：VitAgent（7878）、VitApp 内核（5555/5556/5557，pid 34008）、Godot（内核关闭后自行退出）；复核 5555/5556/7878 端口全空、Godot*/VitApp/VitAgent 进程全清——**零常驻进程**。烟测产生的 ProjectHistory draft（AppData 下 draft_20260929T*）为脚本既有基线步骤（range_context smoke）产物，非本卡场景新增（本卡场景纯只读 invoke）。
