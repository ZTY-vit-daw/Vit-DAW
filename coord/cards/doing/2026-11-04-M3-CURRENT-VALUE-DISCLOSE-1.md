# M3-CURRENT-VALUE-DISCLOSE-1：参数当前值披露（harness 行 add-only +value/value_text+册行携带+重跑组 1）

- 发卡：GLM 主管决策流 / 2026-11-04 晨（依据=241 验收新真断点定性（4ea33461 合入+验收段）+内核源码主管亲核）
- 派发确认：待确认（241 验收后链内续卡，用户在场时口头确认即可开工）
- 验收负责人：GLM 主管决策流
- 池序 242；目标仓库=**主仓 agent**；P1 / 0.5 天 / 依赖=241 已在 main（4ea33461）✓；L1 / PC 执行会话

## 标的（241 验收+主管亲核，不重考）

- 模型消费册行亲证：L68 回复原文「我先读一下这条轨道音量参数的可写范围和当前值，好把『越来越突出』写成一段从当前音量平滑上行的音量曲线，而不是一次跳变」；终局申报只剩「还缺当前值」（240 的缺标识/缺区间消失）。
- 模型走向工具链亲证：r1/r3 调 automation.get_curve；一次早期 denial 后即转向（237 式 ~30 次循环消失）。
- **断点钉死**：内核参数行携带 value/normalized_value/value_text（PluginRackControlService.cpp:741-743 主管亲核）但 harness `compactAutomationParamSurfaceRows` 投影键集只出 param_id/label/min/max/unit/normalized_role/supports_automation——**当前值在 harness 投影被丢**；include_* 门控快照有但骨架未教且体积大。模型要当前值是合法写前基线需求（平滑上行起点，其原文自证）。

## 目标（四面）

1. **harness 行 add-only**：`compactAutomationParamSurfaceRows` 行键集新增 `value`/`value_text`（内核行携带则透传；未携带时键缺省不硬造）。红线：**既有键字节不变，仅新增键**（237 键集契约的 add-only 延伸——既有消费者忽略未知键零回退）。
2. **册行携带**：`automation_surfaces` 册行 params 新增 `value`/`value_text`（持久性同 241 论证：当前值与范围同寿命——读时在场、决策时也要在场；原地刷新语义已覆盖过期——mutation 后重读即刷新，revision 前进出册防线不变）。
3. **单测**：harness 行键集断言（新增键在场+既有键不变+内核缺键时缺省）+册行携带断言+零回退（240/241 全部既有测试原语义绿）。
4. **重跑腿**：同码（唯一 diff=本卡目标 1-3）同 fixture 同引擎（**deepseek-flash**）重跑组 1；判据同链口径：组 1 no_route 消失+`tool_chain_ok`（set_points 包络形态）。门过且预算余量时组 2/3 跑满（非验收门）。

## 预期与诚实边界

本卡=241 同模式第三块拼图（模型已入工具链，plugin_id/param_id/min/max/时间范围全部可及，唯当前值缺格）。**判定成功=组 1 绿或新真断点**（同 241 口径：载荷亲证 value 被消费后若仍止损，取证下一个真断点，模式成立续行）。预算 1-2 轮 ≈¥1.5-3。

## 停止条件

- 新增键破坏既有消费者或 240/241 测试红→回退两案上交；
- 组 1 绿→链收口（组 2/3+统一手测转正），不开新卡；
- 仍止损且载荷亲证 value 在场被消费→下一真断点取证包上交（同模式），不自修不开盲卡；
- LLM 面栈不可用→环境中断如实记录。

## 文件域

`agent/internal/harness/harness.go`（compactAutomationParamSurfaceRows 行键集）+`agent/internal/agentloop/`（册行构造+测试）+`scripts/automation_llm_route_smoke.py`（仅 needle 同步微扩）。越域即停上交。

## 验收标准

- 单测全组+相关包全绿；
- 真栈重跑腿：场景腿 COMPLETED+载荷亲证（compact 行与册行 value 在场[实测值]+value 被模型消费或下一断点取证包）+组 1 判据；栈登记/释放全流程；§8 计费披露。

## 领取

- 2026-11-04 00:08 +0800 / origin/main=6b63fd90 / owner=GLM-5.3 PC 执行会话（ZCode，L1，242 卡会话，用户开场指定领取）/ 分支=port/m3-current-value-disclose-1 / worktree=D:\Vit_DAW_wt_242impl（协调 worktree=D:\Vit_DAW_wt_242claim，分支 coord/242-claim）
- 领取基线三件：HEAD=6b63fd90；impl worktree `git status --short` 空、`git diff --stat` 空（干净基线，无领取前 diff）
- 领取核对：卡在 todo（本地+远端 origin/main 一致）、派发已确认（用户在本会话开场指定卡 ID 与角色=口头确认开工，241 链内续卡）、依赖 241 已合入 main（4ea33461 ✓）；doing 唯一卡 GEN-SUNO-WEB-DESIGN-1 文件域=coord/runs 报告零生产代码，零冲突
- 主工作树 D:\Vit_DAW 既有未提交改动（366 项，VitApp Workspace/coord runs 工件/webui design 等）与本卡文件域（harness.go/agentloop/automation_llm_route_smoke.py）grep 亲核零匹配，不动
- 真栈腿：重跑前按 §2.2 登记 PC-RUNTIME-STACK.md（fetch 重读 main+进程/端口亲核全零后登记，push 后 ≤60s 即时复核）

## 回执

- （执行侧回填）

## 验收

- （主管回填）
