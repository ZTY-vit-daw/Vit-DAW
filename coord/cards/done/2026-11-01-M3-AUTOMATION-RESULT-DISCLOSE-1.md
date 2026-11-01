# M3-AUTOMATION-RESULT-DISCLOSE-1：automation 面工具结果披露（agentloop summary 面+harness 供数面双层修复）+重验门重跑

- 发卡：GLM 主管决策流 / 2026-11-01 晚（依据=236 验收裁定+主管载荷/代码取证：候选恢复已通，断点钉死在结果披露面；236 锚点修正=主断点在 agentloop 层）
- 派发确认：待确认（236 验收处置连带首排，用户转交时视为确认）
- 验收负责人：GLM 主管决策流
- 池序 237；目标仓库=**主仓 agent**；P1 / 0.5 天 / 依赖=236 已在 main（merge+2d1c12a8）✓；L1 / PC 执行会话（两层锚点已定死；重验判读承 235/236 测量面）

## 标的（236 验收主管亲核，两层，不重考）

- **层 1（主断点，agentloop）**：`recentObservationForTool`（`agent/internal/agentloop/execution_memory.go:745`）对非 CCB 观察工具 summary=`mixObservationPromptSummary(result)`（`agent/internal/agentloop/message_loop.go:6492-6500`）——非 Mix 观察名且结果无 `acoustic_digest`/`observation`/`context_pack` 三键 → **早退 nil** → `plugin.get_parameters` 成功回执的模型可见面只剩 `{tool,command_name,status}`，**out 的任何键都到不了会话**（236 载荷亲证：95 行全量请求 `parameter_count` 零出现=基础元数据也未到达）。
- **层 2（供数面，harness）**：`publicPluginParametersResult`（`agent/internal/harness/harness.go:12879`）参数行仅 `include_parameters`/`include_full_parameters`/`include_parameter_snapshot`=true 时组装（:12910 门控）；教学段未教该键；且内建插件的 quick_controls 是否有数未证。
- **结论（发卡即定案）**：只修层 2 无效（层 1 早退吞掉一切）；只修层 1 需要层 2 供数。**两层必须同卡修。**

## 目标（四面）

1. **披露面（agentloop，层 1）**：`recentObservationForTool` 补 automation 面分支——`plugin.get_parameters` 结果 → summary 带紧凑参数面（plugin_id+参数行 `{param_id,label,min,max,default…}`，有界 ≤32 行）；`automation.get_curve` 结果 → summary 带 `{plugin_id,param_id,point_count,bypass,points 首N 或有界采样}`。纯披露：零判定逻辑改动，不引入分页语义。
2. **供数面（harness，层 2）**：`plugin.get_parameters` 的 out 对 automation 消费面带最小参数面——勘察定案形态：新 **ungated 紧凑键**（如 `automation_param_surface` 有界行，既有调用方零扰动）vs `include_parameters` 缺省 true（改默认语义）；倾向前者，`include_parameters` 全量语义不动。
3. **单测**：harness out 含新键断言（含内建 Volume&Pan 双插件 fixture 有参数行）；agentloop summary 面含参数行+有界断言；零回退（既有 mix/CCB summary 披露路径字节形态不变——`mixObservationPromptSummary` 既有键行为不动，新分支只增不改）。
4. **重验腿（组 1 门重判）**：同码（唯一 diff=本卡目标 1/2/3）同 fixture 同引擎（deepseek-flash）重跑组 1，载荷捕获+needle 判读继续开。预期链=歧义→候选→plugin_id→**ok+参数面可见**→set_points 点值在范围内→治理卡→写入→get_curve 回读。判据同 235/236：组 1 no_route 消失+`tool_chain_ok`。门过且预算余量时组 2/3 跑满（非验收门）。

## 预期与诚实边界

本卡=结果披露面收口，非模型/指引面动作。参数面亲证可见后模型仍不写 set_points → **指引+工具面双全下的纯模型行为残差**——那才是换引擎实验的干净判别点（届时证据包上交，不再折披露卡）。预期不承诺通过率。

## 停止条件

- 参数面可见（载荷亲证模型实收含参数行）仍三连同型止损→纯模型残差证据包上交（载荷+收敛面+形态分型），转主管换引擎裁定，**不再折第三张披露卡**；
- 披露后出现新确定性断点（如 set_points 治理卡形态异常/写入键不匹配）→锚点上交不连修；
- 既有 mix/CCB summary 披露回归（既有测试红）→回退两案上交；
- ps1 无关腿超时（236 已记录一次环境时序类）若在本卡复现→记录为第二次，随回执开 flaky 腿修复卡建议；
- LLM 面栈不可用→环境中断如实记录。

## 文件域

`agent/internal/agentloop/execution_memory.go`+`agent/internal/agentloop/message_loop.go`（层 1 分支；若勘察定案挂点在单文件则另一文件不动，锚点已定为 recentObservationForTool 链）+`agent/internal/harness/harness.go`（层 2 供数键）+各自测试+`scripts/automation_llm_route_smoke.py`（如判读需微扩）。越域即停上交。

## 验收标准

- 单测：三层断言（harness 供数/agentloop 披露/零回退）+`go test ./internal/...` 相关包全绿。
- 真栈腿（§5 门槛）：ps1 体系 AutomationLLMRouteSmoke 组 1 场景腿 COMPLETED exit 0+载荷捕获工件在 run 目录；栈登记/释放全流程。若 ps1 整体 exit 1 仅因无关腿超时：如实分列+按 236 先例环境时序分类记录（第二次则附 flaky 卡建议）。
- 回执分列**链路进度**（参数面披露在场+模型实收亲证）与**最终验收**（组 1 三件：no_route/set_points 包络形态/参数面可见亲证）；§8 计费披露。

## 领取

- 2026-11-01 晚 / origin/main=ed2e811c / owner=GLM-5.3 PC 执行会话（ZCode，L1）/ 分支=port/m3-automation-result-disclose-1 / worktree=D:\Vit_DAW_wt_237impl
- 领取基线三件：HEAD=ed2e811c；impl worktree `git status --short` 空、`git diff --stat` 空（干净基线，无领取前 diff）
- 主工作树 D:\Vit_DAW 既有未提交改动（VitApp Settings/coord runs 工件等）与本卡文件域（agentloop/harness/scripts）不相交，不动
- 真栈腿：领取后按 §2.2 登记 PC-RUNTIME-STACK.md（见回执）

## 回执

- 实现：port/m3-automation-result-disclose-1 @ **978d5dc9**（+204/-0 纯增，5 文件全在卡面文件域：execution_memory.go 层1 分支+harness.go 层2 供数键+两个测试文件+smoke py 判读微扩）；工件 65b67cf4。
- **链路进度（目标 1/2/3）：全达**——层2 harness out ungated `automation_param_surface`（≤32 行 param_id/label/min/max/unit/normalized_role/supports_automation，include_* 全量分页语义不动）；层1 `recentObservationForTool` 补 automation 读工具分支（get_parameters 带参数面+元数据，get_curve 带 points ≤16 采样+point_count，mutation 不披露，Mix/CCB 路径只增不改）；单测三层断言全绿（内建 Volume&Pan 双行值域/披露面+别名+有界/非 automation 工具仍 nil/Mix+CCB 零回退）+**全量 `go test ./internal/... -count=1` exit 0**。
- **真实栈亲证（alignrun1）**：**row54 模型 request 实收 `automation_param_surface` 完整参数行**（volume `{min:0,max:1,supports_automation:true}`/pan `{min:-1,max:1}`）——236 钉死的"ok 回执 summary:{} 参数面零披露"断点**修通**；歧义→候选→plugin_id=1011 恢复链延续在场；新测量面 `payload_param_surface rows_with=1/86`（与 ok 回执 1:1）。
- **最终验收（组1 门三件）：一件达成两件未过**——参数面可见亲证 ✓；no_route 消失 ✗（no_route×3 三连同型止损）；set_points 包络形态 ✗（三轮零调用）。ps1 整体 exit 0（场景腿 COMPLETED exit=0；**236 无关腿超时未复现**——不触发 flaky 卡建议）。
- **停止条件第一条触发：纯模型残差证据包上交**（EVIDENCE.md 三件套：row54 实收载荷+三轮收敛数据+r3 逐调用链形态分型）——模型拿参数面后 response 判定"缺少权威披露"转 CCB 视图兜圈 ~30 次不收敛（指引 81/86 在场+工具面参数面实收=双全），断点后移至模型对在野证据采信/收敛行为，**转主管换引擎裁定，不再折披露卡**。
- 栈登记/释放：占用 c90e64ec（登记前复核全零+push 后 ≤60s 即时复核全零）→释放 c90e64ec 后续提交（19:34–19:52，TEARDOWN_ALL_ZERO：VitAgent 4792/VitApp 26928 手停+PROCS=NONE+TCP 5555/5556/7878/8787/18787/5178=0+UDP 4444/4445=0）。
- §8 计费披露：deepseek-flash 86 调用，1,291,342 in（含 cache）/116,112 out（真栈腿 LLM 面）。
- run：`coord/runs/M3-AUTOMATION-RESULT-DISCLOSE-1/alignrun1/`（EVIDENCE.md+payload/telemetry/render 三 log+rounds g1_r1-r3+summary+双 console+ps1_exit=0；clip.wav 现有 *.wav 规则排除）。

## 验收

- （主管回填）
