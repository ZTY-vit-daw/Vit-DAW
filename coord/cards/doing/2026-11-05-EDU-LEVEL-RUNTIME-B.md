# EDU-LEVEL-RUNTIME-B：关卡运行时·挂载——确定性拦截面 + 插点 + 持久化 + 真栈烟测（乐局 3b）★全链通电

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[257 报告](../../reports/2026-11-05-edu-runtime-recon-1.md) 卡 B 原案+挂载方案）
- 派发确认：已确认（用户 2026-11-05 批准入池）
- 验收负责人：GLM 主管决策流
- 池序 281；前置=280；P1 / 1 天；L1-L2 / PC；**真栈烟测腿一次（窗口登记）**

## 目标

1. **`chat/edu_jam_chat.go`（新建，~100 行）**：确定性拦截面——enter（"开始乐局"→装载 279 乐局卡建 jam state，幂等拒绝重复 enter）、状态查询（"现在第几关"）、节点推进检查（学生说"写好了"→读 clip→判定器→回话，**形态 i 拦截面内闭环**，零侵入主循环）；挂载点=goalrunner_chat.go:180 段与 handleMixWorkflowQueueChat 并排（同 durableInvocationID=="" 门，~5 行插点）。
2. **拉回机制**：多轮未推进当前节点（279 卡 pullback_after_turns 值）→运行时把当前节点目标重新注入上下文。
3. **终局判定接线**：闭合判定（全节点 done_when+整曲可播检查）→ agent 问询语义模板（人格渲染层占位）→ 确认后命名+导出+沉淀钩子（rulings §五-18）。
4. **持久化**：workspace 快照（freeStateLoop 同型 storeXxx+persistCurrentProjectWorkspace 先例）；restore fail-closed。
5. **exit**：局终置终态+投影一条终局面（trajectory 事件零新增协议）；**零路由改动红线**（不碰三处白名单/owner 注册表）。
6. **真栈烟测腿**（§5）：HTTP chat 驱动一局最小流程（enter→节点 1 推进→判定→exit），扩现有冒测脚本，exit 0。

## 约束

- 文件域=chat/edu_jam_chat.go（新）+goalrunner_chat.go ~5 行插点+edu 包接线+测试+烟测脚本；**不触 semantic_entry/automation/M3 域**；不动 catalog/policy/capability_pack（非注册链）。

## 验收标准

- 烟测 exit 0（run ID+工件）；`go test ./internal/chat ./internal/edu -count=1` 全绿零回退；
- 回执附：插点 diff、最小局烟测对话摘录、拉回触发用例、commit hash。

## 停止条件

- 插点与主循环假设不符（257 勘察后新变更）→锚点取证上交；栈占用冲突按窗口登记协调。

## 领取

- 2026-11-07 / origin/main `0401a6d8` / owner=GLM 主会话 L2-worker 子代理（ZCode，PC）/ 分支 `port/edu-level-runtime-b` / worktree `D:/Vit_DAW_wt_281impl`
- 领取基线：HEAD=`0401a6d8`；`git status --short` 空；`git diff --stat` 空（净树建枝）
- 段划分：**段 1=挂载实现+单测+build 零栈先行**（本次领取即开工）；真栈烟测腿候栈——挂载完成先停下回报，等主管调度栈窗口后再派烟测腿指令（266 verifyrun2 占栈中，本卡不自行占栈）

## 回执

### 段 1：挂载实现（2026-11-07，commit `87ea0b29` @ `port/edu-level-runtime-b` 已推送）

- **diff 清单**（7 文件，+2070/−2）：
  - `agent/internal/chat/edu_jam_chat.go`（新，核心挂载面）：确定性拦截面（与 handleMixWorkflowQueueChat 并排，同 `durableInvocationID==""` 门）——enter（装载 279 卡建 jam state+会话/工程双绑定，活跃局幂等拒绝重复 enter）/状态查询/推进检查（**形态 i 闭环**：「写好了」→观察组装（get_project_state+get_midi_clip_notes 经 runtime_slots track_index 寻址）→EvaluateNodeConditions 全叶反馈→AdvanceJam→回话，零侵入主循环）/事件记录（`乐局事件 <name> [key]`，闭集 fail-closed）/闭合判定+确认终局/exit；**拉回**（pullback_after_turns 阈值→当前节点目标注入 chatContext[`edu_jam_pullback`]，非命令轮不偷、放行既有路由）；**终局**（structure_complete 三件判定→closure_inquiry 问询→确认后置终态+trajectory `EventSettled` 零新增协议+沉淀钩子位 memory_writes/objectives_check/trajectory_archive 进 WorkflowData）；**持久化** storeEduJamSession+persistCurrentProjectWorkspace（freeStateLoop 同型）+restore 逐会话 fail-closed（未知 schema/卡不可加载/绑定不符→丢弃计数 Warn 留痕）
  - `agent/internal/chat/goalrunner_chat.go`：+9 行插点（180 段 if 块内，注释 4+代码 5）
  - `agent/internal/chat/server.go`：+15 行快照接线（import+Server 双字段+projectAgentRuntimeState 字段+persist 行+restore 行）——**文件域说明**：卡面目标 4 点名「freeStateLoop 同型 storeXxx+persistCurrentProjectWorkspace 先例」，该先例落点在 server.go，此为最小接线（无逻辑分叉），请验收时裁定
  - `agent/internal/edu/command.go`：+7 行（ParseClipNotes 导出，与 276 命令面同解析器零漂移）
  - `agent/internal/chat/edu_jam_chat_test.go`（新，9 测例）/ `scripts/edu_jam_smoke.py`（新，烟测腿驱动）/ `scripts/dev_agent_smoke.ps1`（+82 行：-EduJamSmoke 开关+隔离清单+env 块+runner 块）
- **测试命令与退出码**（全 0）：`go test ./internal/chat -run 'TestEduJam' -count=1`（9 例 PASS）；`go test ./internal/chat -count=1`（全包 128.7s ok 零回退）；`go test ./internal/edu -count=1`（ok）；§6 五条健康检查（history/harness/capabilitycontext/mom/chat 生命周期）全部 ok；`go build ./...` exit 0
- **拉回触发用例**：TestEduJamJudgeFailAttemptFeedbackAndPullback——n1 判定失败（attempt=1，计 1 次未推进）后第 3 条普通轮（阈值 4）到点：消息放行（handled=false）但 chatContext 注入 `edu_jam_pullback`（node_id=n1_read_and_fix+goal 文本），计数归零后 3 条普通轮不再注入；「开始乐局吧」类自然语不入局不吞轮（TestEduJamCheckAndConfirmWithoutJamNotHandled）
- **零路由红线核验**：未触 assessFreeStateCapacity/validCapacityAssessment/restoreCapabilityRoutes 三处白名单、controllerOwners/orchestrationcontroller 枚举、catalog/policy/capability_pack；注册面第四披露面检查=乐局读面走 s.kernel.SendCommand 直读（automation_choreo.go 同型先例），不经工具目录披露，无需白名单同步
- **端测边界声明（AGENTS §5）**：段 1 仅单测+build（假内核注入缝 eduJamKernelSendOverride）；真栈烟测腿（-EduJamSmoke：fixture 42 音主题→HTTP chat 驱动 enter→n0→n1 judge→状态→exit，全确定性**零 LLM** 并断言 telemetry 零行）**已备妥未执行——候栈**（277 ARR-SRCLOAD-1 smokerun1 占栈中，卡面让位记录 7b6469b0 已见）；人格渲染层=占位（问询/enter 文本直出，rulings §五-19 终审前）
- **运行口径（§8，烟测腿跑前成文）**：单 run 无重跑；exit 0=PASS / 2=环境（栈/传输/fixture）/ 4=确定性面缺陷（取证上交）；n1 judge 过兼作 fixture↔卡 notes.theme 对账

### 段 2：真栈烟测腿 smokerun1（2026-11-07，**py exit 4=§8 预写确定性面缺陷类**，单 run 无重跑兑现；取证上交，卡留 doing/）

- **run**：`smokerun1_20261107_0924`（09:24:02 启动）；完整命令/退出码/二进制哈希/工件清单见 `coord/runs/EDU-LEVEL-RUNTIME-B/smokerun1_20261107_0924/evidence.md`（工件随 `port/edu-level-runtime-b` `bfaf6955` 已推）；py exit 4 → PS1 exit 1（dev_agent_smoke.ps1:9019 throw，预期映射）；agent=现场构建 SHA256 `B73212AC…95C`；kernel=wt_228 `F9E3DA0A…D67A` 只读复用
- **过程**：Phase A fixture（ZMQ new_project+save_as_project 落 run 目录+3 轨+42 音回读计数）PASS → B1 enter PASS（workflow=edu_jam/stop=edu_jam_entered/level=first-run-twinkle/n0_photo_score_open，回复含「乐局开始」）→ **B2 n0「写好了」未被乐局面拦截**（workflow=semantic_entry/stop=semantic_entry_unresolved conf 0.20）→ B3-B5 与 Phase C 零 LLM 断言未及；n1 judge 的 fixture↔卡 notes.theme 对账未到达
- **根因**（agent_last.log 时序+代码锚点，全在 turn 1 的 623ms 请求窗口内）：fixture 经内核落盘后 agent 工作区尚未激活（此前=脚本临时草稿）→ turn 1 enter 经挂载点正常开局、`storeEduJamSession` 写内存会话表（edu_jam_chat.go:262；projectUUID 走 kernel 回退 :945-964）→ **同请求内**工作区管理器激活 fixture 工作区（log 09:24:15 `[workspace] activated … conversations=0`）→ 新工程盘上无 runtime state → `restoreProjectAgentRuntimeStateLocked` 执行 `s.eduJamSessions = s.restoreEduJamSessionsLocked(空)`（server.go:7899）**整表空覆盖在途会话** → turn 2 查无会话按设计放行（edu_jam_chat.go:401-404）→ semantic_entry LLM 分类 unresolved。**定性=enter 允许 pre-activation 开局 × 激活空态 restore 整表覆盖的组合竞态，281 文件域内确定性接线缺陷**；非环境非模型随机（分类是放行下游后果）。9 单测全在单一预置工作区状态下跑，未建模请求内激活切换
- **计费（如实）**：telemetry 2 行 deepseek-flash（默认 strip silence **chat** 子腿 1 行——**申报更正**：登记文「-NoChatSmoke 保全 run 零 LLM」不完整，该开关只关 9391 行 chat 路由腿；+烟测会话 semantic_entry 分类 1 行）≪¥0.01；烟测会话域若跑达 Phase C 也会以 1≠0 失败（缺陷下游如实记）
- **栈纪律**：占用登记 `24a8247b`（09:23，净空 09:17+09:22:47 两轮亲核+登记后 60s 即时复核 09:23:44 全零）；owner 亲拆 VitAgent 1704/VitApp 20740；**TEARDOWN_ALL_ZERO 亲核 09:24:55**（六口 TCP listen=0+UDP 4444/4445=0+VitApp/VitAgent/vsphub/Godot/python 全零）；释放登记随本笔
- **处置（交主管）**：修复候选 A=激活路径对 EduJamSessions merge 不整表覆盖（需定义同 conversation 冲突规则）/B=enter fail-closed 要求工作区已激活且绑定=内核当前工程（冒测腿需先触发激活再 enter）/C=查无会话回退盘上快照/内核绑定再判；均应配「enter→激活→推进」竞态建模单测。段 1 验收（挂载设计/词形/拉回/持久化面）不被本腿推翻——缺陷在激活交互面
- **边界声明（AGENTS §5）**：本腿为真栈烟测（真实内核+agent HTTP/ZMQ 面）；渲染面/用户旅程面不在覆盖内（不变）

### 段 3：修复腿（2026-11-07，裁定 B 形态零栈先行，commit `b81449de` @ `port/edu-level-runtime-b`，基线同步 merge `5232093b`；**真栈烟测重发腿候主管调度，本腿未占栈**）

- **修复实现**（3 文件，+269/−18；commit `b81449de`）：
  - `agent/internal/chat/edu_jam_chat.go`：`eduJamEnsureWorkspaceActivation` 两步门替代原 `eduJamProjectUUID`——①同步触发 `activateCurrentProjectWorkspace`（幂等，同身份 fast-path）；②双源核对：`activeWorkspaceUUID` 必须 == 内核 `get_project_state` 直读 uuid（不经 shadow 惯性，与 observeEduJam 同读面）；无法确保 → fail-closed 拒绝开局，回话「工程还没打开好，稍等再开始乐局（…）」，stop_reason=`edu_jam_workspace_activation_incomplete`+WorkflowData.gate_reason 四细分（workspace_not_activated / kernel_read_face_unavailable / kernel_identity_unavailable / workspace_identity_lagging_kernel）。**删除 kernel 回退绑定路径（pre-activation 开局入口）**；server.go 恢复面零改动（restore 整表覆盖语义保持——按裁定不启用候选 A）
  - `agent/internal/chat/edu_jam_chat_test.go`：竞态建模单测 `TestEduJamEnterActivationRacePreClosedPostSurvives`（复刻 smokerun1 时序，两方向）；`TestEduJamEnterFailClosedWithoutProjectBinding` 契约随门演进（stop_reason 更新+gate_reason 断言）
  - `scripts/edu_jam_smoke.py`：A2 激活前置步（GET /agent/ui/state 轮询 nudge 至工程身份=fixture，上限 20s）+ B1 enter 有界重试（≤6 次，门拒绝=by-design 重试类；耗尽=**exit 2 环境类非 face defect**）；ps1 无需透传改动（零新参数）
- **竞态单测断言摘录**（两方向）：①pre-activation enter（内核=fixture、工作区仍绑旧草稿）→ handled=true+stop=`edu_jam_workspace_activation_incomplete`+回话含「工程还没打开好/稍等」+gate_reason=`workspace_identity_lagging_kernel`+会话表断言为空——随后空态整表覆盖（`s.eduJamSessions = s.restoreEduJamSessionsLocked(nil)`，server.go:7899 同型覆盖行）发生**无会话可丢**；②激活完成（绑定追上内核）→ enter 成功且 session.ProjectUUID==激活工作区 uuid → 边界 restore 重放（projectAgentRuntimeStateLocked 快照模拟盘上态）后 turn 2「写好了」仍查得会话并在拦截面闭环（n0 `edu_jam_node_done`，不重演放行→semantic_entry 路径）
- **测试命令与退出码**（全 0）：`go test ./internal/chat -run 'TestEduJam' -count=1`（10 例 PASS：9 既有+1 新竞态）；`go test ./internal/chat ./internal/edu -count=1`（chat 150.6s ok+edu ok 全包零回退）；`go build ./...` exit 0
- **边界声明（AGENTS §5）**：本腿**零栈**——仅单测+build（假内核注入缝），真栈烟测重发腿（-EduJamSmoke 重跑 smokerun2）候主管在 277 段 3a 释放后另派或并入本腿候指令；修复边界如实声明：门通过后内核再切换工程=B5 真实换工程语义（settle+新工程快照 restore），不属本竞态类；内核 uuid 仅作核对真值不作恢复/落盘基址（与 FIX-START-STATE-RECOVERY-1 用户裁定不冲突——不扫描草稿、不绕激活身份源）；pre-existing vet 警告（semantic_treatment_strategy.go unreachable，273/274 域）未触碰
- **烟测重发腿预期变化**（供派腿参考）：B1 enter 可能先收 1-N 次门拒绝回话（by-design，chat_responses.json 留痕+summary.phases.enter_attempts），激活轮询/重试耗尽才 exit 2；其余 Phase B2-B5/Phase C 断言面不变

## 验收

- （主管回填）

## 验收

- （主管回填）
- **段 1 验收（2026-11-07 凌晨，主管）：pass（轻验，终验待烟测腿）**——挂载设计合格（横切状态机：确定性命令词形前置于语义入口=入口识别薄层定位一致[与 C4 归类同族]；非命令轮零侵入拉回；restore fail-closed）；server.go 15 行快照接线=卡面目标 4 点名先例落点**追认**；零路由红线亲证（白名单/owner/catalog/policy 零触+kernel.SendCommand 直读先例同型）；9 测例+全包+§6 健康检查全绿+主管临时检出交叉复跑 ok；烟测腿（零 LLM 确定性 fixture）候栈（277 段 2 占用中 7b6469b0）。
