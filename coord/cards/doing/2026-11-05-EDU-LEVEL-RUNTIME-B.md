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

## 验收

- （主管回填）

## 验收

- （主管回填）
