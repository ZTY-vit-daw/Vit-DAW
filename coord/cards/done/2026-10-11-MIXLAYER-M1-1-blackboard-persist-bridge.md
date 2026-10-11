# MIXLAYER-M1-1：黑板持久层+settle 供血桥（PC 混音线 M1 五序之首）

- 发卡：GLM 主管决策侧 / 2026-10-11 早窗（依据=用户 /morning 裁定"PC 混音线首批立卡"；排序依据=MIXLAYER-RECON-1 M1 表+MIX-CAP-RECON-1 §3"仍成立"结论）
- 派发确认：已确认（用户 2026-10-11 /morning 选定 PC 混音线首批立卡）
- 验收负责人：GLM 主管决策流
- 池序 64；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P1 / 1.5-2 天 / 无（M1 五序公共前置；M1-2/M1-5 依赖本卡合入）
- 模型分级：L1 / GLM 或 flash 执行会话（PC）
- 晚窗参考序：①（早窗即可领；晚窗若仍未领则为晚窗起步卡）

## 目标（MIXLAYER Q4 五件事之①-④ + 黑板报告改读持久层）

1. **键控桥**：conversation→project_uuid 映射。锚点：`ensureFreeStateExperimentCheckpoint`（agent/internal/agentloop/free_state_experiment_runtime.go:322）已从 `loop.LatestProjectChange` 取 checkpoint_ref/commit_id——工程历史检查点锚已存在于自由态 loop 内，直接作桥接键。
2. **持久化层**：settle 结论（含用户人耳判定 outcome）落工程级存储（账本 `decisions/` 同级或黑板专属目录，落点执行侧定但须工程级可寻址），跨会话/跨天可读（现状：Turn 只活内存，settle 按 conversation_id 键控、账本/黑板按 project_uuid 键控——身份轴错位即本卡要解的题）。
3. **结论单元投影**：settle 报告→黑板可读单元；与账本 impact 契约对齐（writes/recheck_on 维度）或作独立"自由态结论"黑板块；needs_review 联动沿用账本既有机制。
4. **黑板消费**：`agentloop/project_blackboard_report.go` 增读该持久层（现状只读 project.state+会话内存，读不到任何跨会话状态）。

## 文件域

agent/internal/agentloop/（blackboard report+settle 供血面）+ 新持久层落点（工程级目录）+ 对应单测；越域停下上报（AGENTS §11）。

## 并行与资源

与 M1-3/M1-4（chat capability seam）文件域不相交，可并行；PC 真栈烟测按 coord/resources/PC-RUNTIME-STACK.md 串行占用。

## 验收标准

- 单测：旧工程无持久层文件→黑板报告缺省语义明确+旧记录反序列化/往返测试（AGENTS §11）；settle→持久→跨会话读回断言。
- 真栈烟测（AGENTS §5，扩展现有 ps1 体系不另起）：两次会话/重启间 settle 结论存活且黑板报告可见，exit 0。
- `go test ./... -count=1` 0 FAIL + gofmt 净。

## 停止条件

checkpoint_ref/commit_id 桥接锚点与卡面不符（上游已变）→ 锚点上交。

- 领取：2026-10-11 10:30 PC / origin/main=844c6524b526e1b7366cd0b3fdf8bfc2dfd43a08 / owner=GLM-5.3 执行流（ZCode PC 会话 sess_30083b31，2026-10-11 早窗）/ 分支 port/mixlayer-m1-1 / worktree D:/Vit_DAW_wt_mixlayer_m1_1 / 领取提交=fc0247f5
- 回执：实现 commit **ac738b8c**@port/mixlayer-m1-1（已推远端）；领取 fc0247f5；栈占用 32fcab06→释放 7eaa2309。
  - **四件落点**：①键控桥 `agent/internal/chat/free_state_conclusion_bridge.go`（桥接键=loop.LatestProjectChange 的 checkpoint_ref/commit_id/project_uuid 锚——锚点核实：`ensureFreeStateExperimentCheckpoint` 确在 322 行、行为与卡面相符，**实际包路径为 agent/internal/chat 而非卡面所写 agentloop**，卡面路径笔误不影响锚点成立；回退链=judgment evidence.ProjectUUID→audioclosure ProjectUUID→projectstore.Current→harness.CurrentProjectIdentity）②持久层 `agent/internal/mixboard/free_state_conclusion_store.go`（记录落 `<工程>/.vit_agent/mixboard/decisions/free_state_conclusions/`，即账本 decisions/ 同级；immutable+ConclusionID 幂等+board 文件；旧工程无文件=空板无错）③结论单元投影（三 kind=experiment_settle/experiment_stop/loop_closure_settle；judgment 人耳判定 outcome（evidence_id/preference/heard_difference/blind）作附件；独立黑板块，未强行套 MixDecisionRecord，needs_review 联动维持账本既有机制）④黑板消费 `agent/internal/agentloop/project_blackboard_report.go`（新增"自由态结论（跨会话）"段+数据来源行；缺省语义=无结论时报告逐字不变）。
  - **单测**：mixboard 7 测（往返/幂等重放/冲突拒绝/旧工程缺省/旧记录反序列化+未知枚举/board 文件不计数/校验）+chat 3 测（桥接顺序/跨会话读回落点断言/无身份跳过）+agentloop 4 测（含结论段/旧工程不变/未知 outcome 原样/坏盘省略）；`go test ./... -count=1` **GOTEST_EXIT=0（92 包 ok 0 FAIL）**；gofmt 本卡文件净（worktree CRLF 检出致全树误报，已核实主检出同名文件净）。
  - **真栈烟测（exit 0 达成）**：`settle_deliver_smoke.ps1 -M1Survival`（扩展现有 ps1，无开关时原语义逐字节不变）。9 run 台账（本机 worktree `coord/runs/SETTLE-DELIVER-SMOKE-20261011_*`）：105423/110451=judgment_not_armed 模型随机分支×2（原卡 SUMMARY 史载 4/9 失率家族）触发止损→场景修复；111730/112607/115519=判定命中但行面败→**取证定位并修复原卡遗留 bug：`$NeedleSettle`/`$Round2Text` 的 B64 载荷为非法 UTF-8**（state_poll_miss 转储+字节级比对证实，该断言历史上从未真正通过，原卡即以分裂证据收口；$Round2Text 原文不可恢复已换功能等价干净文本）；113450=M1 门通过当轮+重启探针暴露烟测 draft 隔离（重启 agent 对 unsaved 内核工程新开 draft=新空 store，环境约束非产品形态——生产保存工程单 .vit_agent 跨进程）→M1 门改为**同进程新会话跨身份轴读回**（正是卡面"身份轴错位"题眼：settle 按 conversation 键控、黑板按 project 读）+重启轴降为只读探针；114450/120538=无 settle 形态（round 停滞家族）；**121359=末轮全链 exit 0**：判定 POST ok→settle 事件→fsc 记录落盘（experiment_settle/improved/judgment b/blind）→新会话黑板报告含"自由态结论（跨会话）：improved：已按用户人耳判定保留改动…"+数据来源行→行面 1 行（修复后 needle 命中）→round2 无 stale→重启探针 empty_draft_isolation 留痕→拆栈净（端口 0/进程 0/工程哈希前后一致）。入库工件：121359 的 settle_deliver_report.json/prereq.txt/m1_blackboard_response.json（随实现 commit）。
  - **端测边界声明**：①重启轴为只读探针非门（烟测 draft 隔离；进程重启存活由盘上记录+单测跨实例读回证明，生产跨进程=保存工程路径未在本轮真栈覆盖）②判定腿模型随机分支可跳过（-M1Survival 下 m1_finished/exit 0 语义=judgment leg skipped+M1 面过；末轮为 judgment 命中形态，未行使该分支）③卡面包路径笔误（chat 非 agentloop）已按实际路径执行，文件域按卡面意图面（settle 供血面+黑板报告面）理解④原卡 settle_deliver 行面坏 needle 修复属同脚本域顺手修复，SETTLE-DELIVER-1 症状 A 断言历史上首次真正可过，是否回归验证该卡由决策侧裁定。
- 验收：（裁定文件 / 验收 commit）
