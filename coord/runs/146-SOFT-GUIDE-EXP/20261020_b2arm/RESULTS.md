# SEMANTIC-TIMEFX-SOFTGUIDE-EXP-1 实验结果与判读（146 软引导实验腿）

- 执行：2026-10-20 20:17–21:0x +0800；owner=GLM-5.3 L2 PC ZCode 执行会话（主管批准派发，本会话）
- 协议：coord/reports/2026-10-20-146-timefx-design-and-experiment.md §一/§E 预声明，跑前未改
- 栈占用登记：adbd1a9a@main（20:17）；释放见本目录 RELEASE 记录
- 被测：kernel=主检出 staging VitApp.exe SHA256=B6565DCF85D1DA867E16333052A7B928A5413565940603ADBCF4ABD59DD3123B（只读复用）；B2 臂 agent=port/semantic-timefx-softguide-exp-1@b168fba6 构建（含活性补丁）；B1 对照 agent=port/semantic-timefx-softguide-exp-1-ctrl（=origin/main d44b127c 零生产改动）构建；模型=deepseek-flash（遥测 110+7 行全量 model=deepseek-flash，0 error；栈 config 未动）；authority=full_project_access（每轮驱动亲设+/smoke authority 探针通过）
- 工件根：本目录（20261020_b2arm/）

## 0. 结论（预绑定判读）

- **止损①触发收口**：真 B2 臂（惯例段经 prompt_render 亲核在场）连续 2×F-inline（A1、A2 均 inline 装载源轨，0/2 总线）——§E 逐字示例命中的情形，B2 臂停止于 2/10。
- **第 4 层厚度建议（预绑定规则）**：B2 0/2 总线 ≪ 80% → **硬规则接管默认拓扑计划生成**（确定性模板，136 posture：软的管解释、硬的管落笔）。硬判据面：**0 幻觉拓扑**（全部装载命中观察披露过的真实轨）。
- **止损②的证据同步上交**：对照形态轮中 F-gate（target_identity 弹回）3/8——机器侧 candidate frontier 与用户指名轨冲突，模型 4 连弹回后自报边界。该签名与惯例文本正交，将同样压制 B2 臂的提案面到达率；L1/L3 修复优先、实验在其后重排（§E 原文）。
- B1−B2 差分（均为 0 总线）：两臂到达提案面的轮全部 inline 源轨——惯例文本在观测轮内未产生任何可测路由效应。

## 1. 执行叙事（三轮，含两次执行侧缺陷的如实记录）

### attempt 1（20:25–20:29，10 轮全部无效——驱动缺陷）
驱动在 turn 响应层对 waiting_continue/limit_reached 直接 nudge；"可以执行"被 legacy 路径判为裸确认（no_pending_mix_tick_candidate）并误杀活性链（goal 提前 completed）。10 轮全部为**驱动形状无效轮**（非产品断言失败，127 卡 run1/run2 夹具缺陷同先例，不占 §8 产品轮）。工件=driver_attempt1_invalid_driverbug/。修复=commit「驱动 nudge 边界修复」（响应层不 nudge，统一落 runtime 轮询层带续泊活性检查）。

### attempt 2（20:31–20:44，8 轮对照形态——补丁失活）
补丁 v1 只改了 `messageLoopNeutralFamilySystemSkeleton`——该函数是 L1 corrupt 理论不可达回落（`neutralFamilySystemSections` 优先 `bundle.Sections`，活性路径=carriers L1 rules 层）。**prompt_render 亲核：49 渲染 0 含惯例段**（system len=14331=stock 骨架）→ 8 轮（A1–A5、B1、B2、B3）实为**对照形态轮**（行为面与 stock 等价）。期间按这些轮的 F-gate 累计 ×2（A1+A3）执行了停止（B3 中断、B4/B5 未跑）；补丁缺陷发现后实验以真 B2 条件重开、计数器重置。修复=commit b168fba6（carriers.go FamilyNeutralFamily 分支尾部追加惯例段，carriers 包测试过）。

### attempt 3（20:50–20:58，真 B2 臂）
patchverify 探针会话（softguide_patchverify_20261020_205032）prompt_render 亲核：system len=14622，**惯例段逐字在场**。随后逐变体串行跑+逐个判读：
- **A1**（人声混响·短衰减）：semantic_dynamic_execution_completed，rack_add_node→Pro-R 上 **vocals/1032 源轨**；sends 0→0；回复叙事=插件选择+参数，无拓扑计划。**F-inline**。
- **A2**（钢琴混响）：同签名，Pro-R 上 **piano/1027 源轨**。**F-inline**。
- → **止损①（2×F-inline 连续）触发，B2 臂停止**（20:53）。A3–A5/B1/B2/B3/B4/B5 未在真 B2 条件下运行。
- B4-poststop_miscondition（20:54）：误以为在对照条件实跑于补丁 agent（ctrl 重启未停掉 sge1 agent——Stop-AgentForPaths 只匹配本 worktree 路径）；按 B2 条件记录：REQ 6 Stereo EQ 上 vocals/1032（域漂移 EQ），0 总线，终局 capability_blocked（settle 段）。执行侧条件错标缺陷 #3，如实留档。

### B1 对照臂补齐（20:58–20:59）
ctrl agent 正确重启（进程路径+prompt_render 亲核 4 渲染 0 含惯例）后补跑 B4：
- **B4-ctrl**（人声贴脸往后放）：mix_tick_confirmation 卡（runtime walk 自动应答 approve 留痕 1 hop），REQ 6 Stereo EQ 上 **vocals/1032**，0 总线，终局停泊 waiting_confirmation。**F-inline**（+域漂移注记）。
- 对照臂由此 5/5 齐（A1/B1/B2/B3 来自 attempt-2 对照形态轮 + B4 本轮）。

## 2. 分类总表（§E 失败分类×5）

| 轮 | 条件 | 终局 | 装载 | 分类 |
|---|---|---|---|---|
| B2-A1 | 惯例 ON（亲核） | semantic_dynamic_execution_completed | Pro-R→vocals/1032 源轨 | **F-inline** |
| B2-A2 | 惯例 ON（亲核） | semantic_dynamic_execution_completed | Pro-R→piano/1027 源轨 | **F-inline** |
| B2-B4(停止后追加) | 惯例 ON（亲核） | capability_blocked(settle 段) | REQ 6 EQ→vocals/1032 | F-inline+域漂移注记 |
| ctrl-A1 | 现状提示词 | capability_blocked | — | **F-gate**（frontier 弹回×4→自报边界） |
| ctrl-A2 | 现状提示词 | semantic_dynamic_execution_completed | Pro-R→piano/1027 | **F-inline** |
| ctrl-A3 | 现状提示词 | capability_blocked | — | **F-gate**（同签名） |
| ctrl-A4 | 现状提示词 | semantic_dynamic_execution_completed | Timeless 3→guitar/1017 | **F-inline** |
| ctrl-A5 | 现状提示词 | semantic_dynamic_execution_completed | Pro-R→vocals/1032 | **F-inline** |
| ctrl-B1 | 现状提示词 | d1_post_action_evaluation_required 停泊 | Q4 EQ→**piano/1027**（用户指名人声；域漂移） | F-inline+域漂移+停泊注记 |
| ctrl-B2 | 现状提示词 | capability_blocked | — | **F-gate**（needs_action coverage 轴未注册→自报边界） |
| ctrl-B3 | 现状提示词 | semantic_dynamic_execution_completed | Pro-R→drums/1012 | **F-inline** |
| ctrl-B4 | 现状提示词（亲核） | waiting_confirmation 停泊 | REQ 6 EQ→vocals/1032 | **F-inline**+卡应答注记 |

- F-env：0（无环境中断轮；attempt-1 十轮为驱动缺陷非环境类，§8 分类单列）。
- F-halluc：**0**（硬判据过——全部装载/引用命中披露轨集 bass1007/drums1012/guitar1017/other1022/piano1027/vocals1032）。
- 总线拓扑成功：B2 0/2（另 B4 追加 0）；对照 0/5。

## 3. F-gate 机制取证（交 L1/L3 修复线）

- 弹回原文（agent_message_loop_debug.jsonl，A1×4/A3×4 同签名）：`closure candidate frontier requires target_ref.kind=track and an id from the candidate track set`。
- frontier 披露（prompt_render）：`candidate tracks=1017,1022,1027 / 1007,1012,1022`——**不含用户指名的 vocals/1032**。模型按用户话术定向观察 vocals 被 final_gate 连拒 4 次，第 7 轮自报 capability_blocked；回复面（148 修复后用户语）呈现"目标证据/证据新鲜度/目标一致性 三项准入检查未过"。
- 判定：机器侧 candidate frontier（首轮 project 级观察产物）与用户指名轨的冲突是 F-gate(target_identity) 的机制；与惯例文本正交。此为 §E 止损②上交面，建议归 146 拆卡 E（观察强制/结转）或独立小卡处置——首轮观察如何把用户指名轨纳入 frontier。

## 4. §8 记账

- LLM：sge1-wt 遥测 110 行 + ctrl-wt 7 行，**全部 model=deepseek-flash、0 error**；softguide 条件调用 104+5（attempt1 ~20 + attempt2 81 + patchverify 1 + 真B2 A1/A2 + B4 误条件 + B4 对照 5）。1 次探针会话已 stop 收口。
- 无重试轮消耗：attempt1/2 轮不占产品轮；真 B2 臂 2 轮即触止损①，未烧剩余轮次。
- 内核 staging Workspace 增量：Logs 153→154（本腿内核 headless 日志 1 件追加，132 卡先例已知面；无其他写入）。

## 5. 诚实边界

1. 真 B2 臂仅 2 轮即止损——**惯例文本的长期/分布效应未测**；判读按预绑定规则以 0/2<80% 走硬规则分支，非充分样本下的否定结论（对照 0/5 与 147 场次 M2 话术行为一致，方向佐证）。
2. 补丁 v1 失活期的 8 轮按对照形态采信的依据=prompt_render 0/49 含惯例段+system len=stock（行为面与 stock 等价）；其 agent 二进制含死代码（回落路径后缀），非 origin/main 逐字节等价。
3. conversation_graph.json 在本运行持久化形状下不含 softguide 会话 id（draft root 内图未挂这些会话）；判断面证据以逐轮 chat JSON+events（含 events_redump 全量重拉）+agent 日志切片+prompt_render+debug 流覆盖。
4. agent_last.log 800 行滚动致早期行（attempt1/attempt2 部分）转出；关键行（A1 终局、A2/A4/A5/B3 执行）已在转出前引用或在 events_redump/telemetry 在卷。
5. -StartUI 在 -StartKernel 先绑 5555 时按既有逻辑跳过自启（92 卡同款）——UI 由本腿复刻脚本 elseif 分支手动拉起（D3D12/RTX4070 实渲染在卷）；vsphub 由本腿手动补起（8787，agent+GUI 注册亲核）。真实用户栈中两者由 Godot 启动页拉起——本腿为等价复刻非原生气路。
6. 交互卡：真 B2 两轮零卡（147 合入后 full-access 三确认面无卡，实测一致）；对照 B4 出现 1 张 mix_tick_confirmation（非三确认面族），自动应答兜底按预案留痕。

## 6. 工件索引

- driver_b2_real/{A1,A2,B4_poststop_miscondition}/：真 B2 轮（prompt+chat+events_full+send.list 前后+state+summary）
- driver_ctrlshape_8rounds_patch_inert/：attempt-2 对照形态 8 轮 + events_redump/
- driver_b1_ctrl/B4/：B4 对照轮
- driver_attempt1_invalid_driverbug/：attempt-1 无效轮
- logs_sge1/ logs_ctrl/：agent_last.log、llm_telemetry、prompt_render、message_loop_debug 双侧快照
- bringup_console / agent_restart_b2patch / agent_restart_b1ctrl{,_take2} / driver_console*：各阶段控制台
- godot_ui.log / vsphub_last.log / kernel_workspace/：栈面证据
- fixture/：六轨 stems+softguide_fixture.vit（M2 同源配方）
- 补丁与驱动：port/semantic-timefx-softguide-exp-1@b168fba6（ccb_model_prompt.go 回落补+v2 carriers 活性补+scripts/softguide_exp.py）
