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

- （执行侧回填）

## 验收

- （主管回填）
- **段 1 验收（2026-11-07 凌晨，主管）：pass（轻验，终验待烟测腿）**——挂载设计合格（横切状态机：确定性命令词形前置于语义入口=入口识别薄层定位一致[与 C4 归类同族]；非命令轮零侵入拉回；restore fail-closed）；server.go 15 行快照接线=卡面目标 4 点名先例落点**追认**；零路由红线亲证（白名单/owner/catalog/policy 零触+kernel.SendCommand 直读先例同型）；9 测例+全包+§6 健康检查全绿+主管临时检出交叉复跑 ok；烟测腿（零 LLM 确定性 fixture）候栈（277 段 2 占用中 7b6469b0）。
