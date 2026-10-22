# SOFTGUIDE-VERIFY-ROUND2-1 真栈腿结果（B2 臂 10 变体全量+B1 对照臂 5——15/15 有效轮完成面）

- 执行：2026-10-22 22:07–23:0x +0800；owner=GLM-5.3-flash L1 PC ZCode 执行会话（175 卡会话，领取 e2620998）
- 协议：[PREDECLARATION.md](../PREDECLARATION.md)（跑前锁定，跑后未改）+ [146 §E 原协议](../../../runs/146-SOFT-GUIDE-EXP/20261020_b2arm/RESULTS.md) + [159 复用面](../../../runs/SOFTGUIDE-RESCHEDULE-1/run_20261022_1846/RESULTS.md)
- 栈占用：登记 33791860（22:07，本卡分支 push 面，通道偏差已在登记行声明）；释放=本 RESULTS 同提交（端口/进程复核 0 在卷）
- run=**run_20261022_2210**（本目录）

## 0. 被测与环境（§9 契约）

- agent=**主检出 `D:/Vit_DAW/agent/bin/VitAgent.exe`** SHA256 前 16=**6CE34E541E77270F**（2026-10-22 21:40 构建；含 162 域漂移修复+169 别名扩展+170 披露确定性供应——与 170 补证腿同一二进制；本卡生产代码零改动）；vsphub 前 16=**B10CC41295066C48**（22:04 亲核）
- kernel=主检出 K3 Release SHA256=**722A79D7**A3F53797…（22:05 哈希亲核+bringup 硬校验，只读复用；VIT_PROJECT_XML 指 run 目录 kernel_workspace 副本，主检出/staging workspace 零触碰）
- 手动起栈三进程（bringup_sv2.ps1，170 脚本面复用）：kernel 12800 / vsphub 32920 / agent 22600；23:0x 拆栈，复核 tcp 5555/5556/7878/8787 全 free、VitApp/VitAgent/vsphub/VitHeadless/Godot 五类进程=0
- 模型=deepseek-flash：**114 次调用全 deepseek-flash、0 error**（遥测在卷）；≤135 预算内
- git：分支 port/softguide-verify-round2-1（领取基线 origin/main=9b79ceb3=本地 main，生产码零 diff）；工作树实现面=驱动/工件非生产码

## 1. 结论（机械计数，判读交主管）

- **15/15 有效轮全部捕获**（B2 臂 10/10[159 中断面 6/10 补全]+B1 对照臂 5/5[159 未跑面]）；三批驱动缺陷轮 0、环境中断 0、产品轮原样重跑 0（批 2 authority 409 死于批首变体前=操作事件非产品轮，见 §7）。
- **主判据如实计数**：
  - **F-gate=1**（B5，no_candidate 子形态；**0 target_identity 签名**——159 同签名两轮形态本轮零再现；`free_state_final_gate` 弹回行全程 **0 行**）。止损线（×2）未触发；对照臂 0 F-gate=新变体签名复核未重现。
  - **提案面到达率=14/15 完成面**（唯一未到达=B5）。B2 臂 9/10、对照臂 5/5。
  - **0 幻觉拓扑**（硬判据）：全腿 10 条 send 边（B2 臂 6+对照 4）源轨全部 ∈ 披露六轨集 {1007,1012,1017,1022,1027,1032}，目标=本提案新建 FX 轨 1037（逐变体 project.open 复位后新建）；send_list_after 逐轮读回亲核。
  - **域漂移收敛实证**：159 形态 A3 源轨 other/1022→本轮 **drums/1012 ✓**、A4 源轨 piano/1027→本轮 **guitar/1017 ✓**（169 别名扩展+162 域漂移修复后按预期归零，6/6 总线执行轮源轨全对）。
- **总线拓扑执行：B2 臂 6/10、对照臂 4/5**（@-6dB 默认发送量全量在卷）；插件装载=Reverb Bus×8+Pro-R、Delay Bus×1+Timeless 3（A4）；惯例命名（A4=delay 族→Delay Bus，其余=Reverb Bus）。
- 其余分类：F-inline 1（B4[B2 臂]：mix_tick 直应用 F6 Stereo EQ 落人声/1032，**源轨正确**、域轴漂移注记=纵深话术→EQ 轴，146 ctrl-B4 同族生产形态）；F-notopo 0、F-halluc 0、F-env 0。

## 2. F-gate 唯一实例签名面（B5，如实上交判读）

- **B5（让这条吉他有点远近层次）**：诊断完成（semantic_state=diagnostic_complete，诊断文本在卷：相关性近满值/包络平/峰均比约 3dB），终答=「当前冻结语义轴（envelope_emphasis）下，本机没有同时满足 族准入、冻结轴覆盖与实验白名单形态的 Transient Shaper 候选；没有回退到不可执行候选，也没有选择、加载或修改工程。」零变更收口。
- 机制面：**插件准入/白名单层无候选诚实停机**（no_candidate_found 形态），非 146/159 的 target_identity 门拒/边界自报（无弹回、无指名轨身份缺失话术）；按 §E「未达提案」计入 F-gate，子形态如实注记。归属（生产库覆盖缺口 vs 准入设计内 fail-closed）交主管。

## 3. 分类总表（RUNBOOK §5 机械口径）

| 轮 | unt 解析 | 终局 | 装载/拓扑 | 分类 |
|---|---|---|---|---|
| B2-A1 | 1032 ✓（渲染披露在场） | improvement_proposal 卡 resolved→插件选择卡停泊（操作员批间收口） | 总线**计划文本**（Reverb Bus+send 1032，未执行零装载） | **到达+停泊注记** |
| B2-A2 | 1027 ✓（169 修复生效） | needs_experiment 准入+总线计划文本，completed（批间操作员收口） | 计划面（零装载） | **到达** |
| B2-A3 | 1012 ✓ | completed | Reverb Bus/1037+Pro-R；send **drums/1012**→bus @-6 | **到达+总线执行（源轨正确）** |
| B2-A4 | 1017 ✓ | completed | Delay Bus/1037+Timeless 3；send **guitar/1017**→bus @-6 | **到达+总线执行（源轨正确）** |
| B2-A5 | 1032 ✓（渲染披露 9 行在场=170 修复面） | completed | Reverb Bus/1037+Pro-R；send **vocals/1032**→bus @-6 | **到达+总线执行**（159 F-gate #2 形态翻转） |
| B2-B1 | 1032 ✓ | completed（全访问自主装载无卡） | Reverb Bus/1037+Pro-R；send vocals/1032→bus @-6 | **到达+总线执行（源轨正确）** |
| B2-B2 | 1027 ✓ | completed | Reverb Bus/1037+Pro-R；send **piano/1027**→bus @-6 | **到达+总线执行（模糊话术）** |
| B2-B3 | 1012 ✓ | completed | Reverb Bus/1037+Pro-R；send **drums/1012**→bus @-6 | **到达+总线执行（模糊话术）** |
| B2-B4 | 1032 ✓ | completed（mix_tick 完全访问直应用） | F6 Stereo EQ **inline** 落 1032（track 态变更在卷）；0 总线 | **到达+执行**；F-inline+域轴漂移注记 |
| B2-B5 | 1017 ✓（诊断面） | completed（no_candidate 诚实停机） | 零变更 | **F-gate #1**（no_candidate 子形态） |
| ctrl-A1 | 1032 ✓ | completed | Reverb Bus/1037+Pro-R；send vocals/1032→bus @-6 | **到达+总线执行** |
| ctrl-B1 | 1032 ✓ | completed | Reverb Bus/1037+Pro-R；send vocals/1032→bus @-6 | **到达+总线执行** |
| ctrl-B2 | 1027 ✓ | needs_experiment 准入 completed | Reverb Bus+Pro-R；send piano/1027→bus @-6 | **到达+总线执行** |
| ctrl-B3 | 1012 ✓ | needs_experiment 准入 completed | Reverb Bus+Pro-R；send drums/1012→bus @-6 | **到达+总线执行** |
| ctrl-B4 | 1032 ✓ | needs_experiment 准入+A/B 判定请求×3 | mix_tick 静态 EQ 参数面落 1032（track_state_revision before/after 变更在卷，无 rack 节点名）；0 总线 | **到达+执行**；F-inline 同族注记 |

- 惯例段在场：逐变体 prompt_render_snapshot 亲核，**15/15 轮全部渲染行含「总线拓扑」惯例段**（对照臂=同生产 agent，惯例段生产在场，仅作计数对照不做差分判读——159 预声明 §1 职能收窄原样）。
- 域漂移观察补充：A2/ctrl-B2 渲染面出现 `user_named:1032` 行残留（本变体话术=钢琴、unt 解析仍正确落 1027）——疑似 158 seed/carry 跨会话残留面，不影响本轮计数，判读交主管。

## 4. §8 记账

- LLM=114 次调用（3 authority 探针+15 变体链）全 deepseek-flash、0 error、0 超时；fixture 构建零 LLM（确定性 stems+import+DAD ready 轮询）；≤135 预算内。
- 轮次：15 产品轮全部有效捕获；0 驱动缺陷轮、0 环境中断、0 产品轮原样重跑（批 2 take1 死于变体前 authority 409=操作事件，take2 为批次重入非变体重跑）。
- 内核增量：run 目录 kernel_workspace 副本内（Logs 类），主检出/staging workspace 零触碰。

## 5. 诚实边界

1. A1/A2 终局含**操作员批间收口**（/agent/turn/stop，reason=operator_batch_boundary_settle）：两变体到达面与计划文本在收口前已捕获落盘（variant_summary/chat/events），操作员停止仅收口停泊链未改变已捕获分类面；agent 语义态历史诚实记录 cancelled（stop 原因在卷）。
2. B2-B4 与 ctrl-B4 的 EQ 形态为**完全访问 mix_tick 直应用**（无确认卡，147 合入后形态），到达判定以 mix_tick.pending+语义态准入+工程态变更为据；B4 为 rack 节点（F6 Stereo）、ctrl-B4 为参数面（track_state_revision 变更、无节点名）——两者装载形态差异如实注记。
3. agent_last.log 为 800 行滚动日志；判定面证据以逐轮 chat/events/runtime_status/prompt_render 快照+遥测为准（146/159 §5 同界）。
4. authority_mode 在批 1 结束后、操作员收口前（22:14 读数）显示回落 manual_confirmation——批 1 绑定证据=authority_switch.json+probe `authority_smoke_ok bound=full_project_access`+行为面（mix_tick 直应用/零能力墙）；驱动每批重绑（批 2–5+对照三批全部 200）。回落机制归属交主管，本腿计数不受影响（各批捕获均在重绑后）。
5. 提案面到达的「终答计划文本」形态：A1/A2 为计划文本+卡面复合形态，其余到达轮为执行/停泊形态；到达口径按 §5 机械信号计数，未细分权重。
6. 对照臂与 B2 臂同生产 agent（惯例段同源），到达率/总线率两臂数字**不构成惯例效应差分**（146 §0 硬规则分支已锁，159 预声明职能收窄原样）。
7. 端测边界（预声明 §7）：本腿=agent HTTP/ZMQ 面 chat 链真栈（真实内核+真实 deepseek 链+真实工程副本隔离）；**不含** Godot GUI 渲染面与 webui 渲染面断言；提案面到达判定以 chat 响应/events/内核回执工件为据。

## 6. 工件索引

- `driver/arm_B2/arm/{A1,A2,A3,A4,A5,B1,B2,B3,B4,B5}/`、`driver/arm_B1/arm/{A1,B1,B2,B3,B4}/`：prompt.txt / chat_1.json / hop_*.json / walkhop_*.json / events.json / send_list_{before,after}.json / project_state_after.json / runtime_status_final.json / variant_summary.json / prompt_render_snapshot.jsonl
- `driver/authority_switch.json`+`authority_probe.json`（双臂各一，批 1/对照批 1 在卷）；`driver/batch1_A1A2_console.txt`、`batch2_A3A4_console.txt`（take1 authority 409 留证）、`batch2_A3A4_console_take2.txt`、`batch3_A5B1_console.txt`、`batch4_B2B3_console.txt`、`batch5_B4B5_console.txt`、`batchc{1,2,3}_*_console.txt`
- `driver/fixture/`：六轨 stems+softguide_fixture.vit（M2 同源配方；`fixture_MANIFEST.sha256.json` 160 件哈希清单在卷）
- `logs/`：agent_last.log、agent_llm_telemetry.jsonl（114 行）、agent_prompt_render.jsonl、vsphub_last.log、MaterializeMetrics.log、pid 三件、kernel_sha256.txt
- **留盘不入库（重件面，159/170 先例）**：`driver/fixture/fixture_stems/`（wav 配方可再生）、`driver/fixture/.vit_history/`、`driver/fixture/.vit_derived/`、`kernel_workspace/`（运行时工程态）——均在本 run 目录原样保留可回指
- prep 件：PREDECLARATION.md / RUNBOOK.md / bringup_sv2.ps1 / driver_softguide_exp_snapshot.py（e2620998，跑前锁定）

## 7. 执行侧缺陷与偏差如实记录

1. **批 2 take1 authority 409**：批 1 两变体停泊链持有 running turn，agent authority 切换守卫拒绝（409 authority_mode_change_blocked，goal_25715cdfbcd30b35 在卷）；驱动单发无重试 EnvAbort。处置=/agent/turn/stop 收口 A1+A2 停泊链（操作事件，非产品轮），take2 整批重入 exit 0。死因=批间操作契约缺口（驱动无批间收口步），非产品缺陷；已作为观察面上交（停泊链与 authority 守卫的交互归属交主管）。
2. RUNBOOK 批次示例的 out-dir 笔误未犯（159 教训采纳：B2=arm_B2、对照=arm_B1 全程一致）。
3. 栈登记通道偏差：按 159/170 先例以本卡分支 push 为登记可见面（登记行已声明；本轮 main 与 origin 已合齐，登记面仍走分支保持先例一致）。
4. 错峰序：登记时点 173（VSP-HUB-PUBLISH-RESILIENCE-1）未领取在飞、栈空闲（170 已 21:49 释放），本腿按用户直派口令错峰序 173 之后登记执行。
