# M3-LOOP-UNAVAILABLE-VIEW-GUARD-1：不可用视图重试机械执法+重定向披露（237 残差的执法面收口）+flash 重跑组 1

- 发卡：GLM 主管决策流 / 2026-11-01 深夜（依据=237 残差终局定性的执法面缺口主管亲核+**用户裁定 2026-11-01**：模型排序修正 flash(1017)>v4-pro→pro 判别撤销，方向=执法护栏+重跑；flash=可用最强引擎，残差指向架构/执法面）
- 派发确认：已确认（用户 2026-11-01 深夜裁定「执法护栏+重跑」）
- 验收负责人：GLM 主管决策流
- 池序 240；目标仓库=**主仓 agent**；P1 / 0.5 天 / 依赖=237 已在 main（79443422）✓（238 工件无生产代码依赖）；L1 / PC 执行会话（锚点已定死；重跑判读承链测量面）

## 标的（237 验收+主管亲核，不重考）

- 237 残差形态：模型对 `processor.identity_and_controls`（对内建轨级插件返回 `unsupported_projection`/`source evidence is missing`）**反复请求 ~30 次**（237 载荷 50 行上下文反复携带 unavailable 回执，主管亲核），骨架规则明文「If a CCB observation returns rejected, unavailable, or deferred evidence for a requested view set, do not retry the same view set」**但循环层零机械执法**——规则只存在于 prompt 自觉层。允许自家规则被违反 30 次无后果的循环，对任何模型都不是公平测试。
- 同时：unavailable 回执的 omission_reason 不带重定向——模型不知道 processor.* 投影不覆盖内建轨级插件，也不知道上下文里已有的 automation 面结果（plugin.get_parameters 参数面）即权威参数面。

## 目标（四面）

1. **机械执法护栏**：同一 exact view set+同 target 在此前已返回 unavailable/unsupported/rejected 后**第二次请求起，循环机械拒绝执行**（返回机器可读拒绝：该视图集本轮已判定不可用+引用规则出处+不再消耗观察轮）。勘察定案拦截位（agentloop 观察请求执行面 vs CCB 服务面，选最小侵入+对全部消费路径生效者）。红线：只拦「完全同视图集+同 target+无 revision 变化」的重试——不同视图集/不同 target/project revision 前进**不拦**（规则原文 exact_view_set 语义）；拒绝不消耗观察预算（拒绝不是观察）。
2. **重定向披露**：unavailable 回执的 omission_reason 补 closed-template 机制说明：processor.* 投影不覆盖内建轨级插件→其权威参数面=automation 面（plugin.get_parameters 结果，若 recent_observation 已携带则指明在场）。承 237 验收「次要事实」条。
3. **单测**：拦截三件（同集+同 target 第二次拒/不同视图集放行/revision 前进放行）+拒绝回执形态断言（机器可读+规则引用）+重定向文本断言+零回退（正常观察路径与既有 rejection/declared_contradiction 语义字节不变）。
4. **重跑腿（flash 回归最强引擎）**：同码（唯一 diff=本卡目标 1/2/3）同 fixture 同引擎（**deepseek-flash**，撤 v4-pro）重跑组 1；载荷捕获+needle+param_surface 全开。判据同链口径：组 1 no_route 消失+`tool_chain_ok`（set_points 包络形态）。门过且预算余量时组 2/3 跑满（非验收门）。

## 预期与诚实边界

本卡=执法面收口（把已有明文规则从自觉层升到机械层）+披露补全，非新增判断逻辑。**终局分叉预埋**：护栏+重定向在场（载荷亲证）flash 仍三连同型止损 → flash=可用最强+执法+披露三全下仍不收敛=**架构面终局证据包**上呈用户裁定（接更强引擎 vs 架构重构 vs 收线接受边界申报形态），不再折卡。预算 ≈237 量级（¥1.5 档）。单 run 存在性判别（§8）。

## 停止条件

- 护栏拦截正常合法观察（异集/target/revision 放行路径误伤，既有观察测试红）→回退两案上交；
- 重跑仍三连同型止损→架构面终局证据包上交（载荷+收敛面+护栏触发计数三件套），转用户裁定不开新卡；
- LLM 面栈不可用→环境中断如实记录。

## 文件域

`agent/internal/agentloop/`（观察请求执行面拦截+omission 披露）或 CCB 服务面（勘察定案单边）+测试+`scripts/automation_llm_route_smoke.py`（如判读需护栏触发计数微扩）。越域即停上交。

## 验收标准

- 单测四组+相关包全绿；
- 真栈重跑腿：场景腿 COMPLETED+载荷三件套+护栏触发与重定向亲证（或护栏零触发+模型一次即弃的形态如实记录）+组 1 判据或终局证据包；栈登记/释放全流程；§8 计费披露。

## 领取

- 2026-11-03 上午 +0800 / origin/main=15de9627 / owner=GLM-5.3 PC 执行会话（ZCode，L1，240 卡会话）/ 分支=port/m3-loop-unavailable-view-guard-1 / worktree=D:\Vit_DAW_wt_240impl
- 领取基线三件：HEAD=15de9627；impl worktree `git status --short` 空、`git diff --stat` 空（干净基线，无领取前 diff）
- 主工作树 D:\Vit_DAW 既有未提交改动（VitApp Workspace/coord runs 工件/webui design 等）与本卡文件域（agentloop/scripts）不相交，不动
- 真栈腿：重跑前按 §2.2 登记 PC-RUNTIME-STACK.md（fetch 重读 main+进程/端口亲核全零后登记，push 后 ≤60s 即时复核）

## 回执

- （执行侧回填）

## 验收

- （主管回填）
