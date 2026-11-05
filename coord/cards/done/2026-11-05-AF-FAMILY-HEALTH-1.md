# AF-FAMILY-HEALTH-1：A-F 全族体检（回归扫描+自主性分级首盘）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=用户裁定 2026-11-05「一定要 A-F 的全族体检卡」+[2026-11-05-af-scaffolding-endstate.md](../../decisions/2026-11-05-af-scaffolding-endstate.md) §四挂点）
- 派发确认：已确认（用户明示）
- 验收负责人：GLM 主管决策流
- 池序 257；目标仓库=主仓（以跑腿+盘点为主，产出工件；修复零散小问题须逐项过域检查）；P1 / 1 天 / 依赖=无（与 256 文件域不相交：256=scripts/run_free_state_d1_smoke.ps1，本卡只读它）；L1 / PC 执行会话；真栈腿需占用（分批）

## 目标（双面）

### 面 1：全族回归扫描（横向）

对 A-F 各族既有烟测腿/场景腿在**当前 main** 上分批重跑，产出健康矩阵：

| 族 | 腿来源（勘察到的，领取时再核） | 跑法 |
|---|---|---|
| B1-B4 | static mix 契约/能力族既有腿（static_mix_capability_contract 状态面+mix 工具腿） | 逐腿真栈或最小栈 |
| C1 | frequency_cleanup 面（fine_mix.frequency_cleanup.v0/v1 schema+脚本如有） | 同上 |
| C2 | dynamic_control 面（fine_mix.dynamic_control.* 既有验证腿） | 同上 |
| C3 | space_depth 面 | 同上 |
| C4 | 已由 M3 链今日全验（三组终态）——**不重跑**，引用收口报告 | 引用 |
| D1 | 254/256 在飞——不重跑，状态记 in-flight | 引用 |
| A/E/F1 | A=journey/probe 时代腿可选抽查；E/F1=空白记 not_built | 抽查/记录 |

分批跑（每批一栈占用一释放）；LLM 参与的腿按 §8 跑前成文逐腿计预算（多数腿确定性零 LLM 预期）；断言失败=取证分列不追绿（D1 教训：先判「断言过时×模型分支」还是「真回归」再定处置）。

### 面 2：自主性分级首盘（拆掉准备度）

按裁定三态（F=仅固定路径/H=harness 路径已测存在/A=已过 ablation）逐节点盘点：

1. 逐族读代码+测试，钉死「固定路由面清单」（capability_owner 意图词/choreo 词表/固定 runtime 入口）与「harness 可达面清单」（工具对通用循环的暴露、prompt 指引覆盖）；
2. 已有测试证据里 harness 形态盘点（如 C4 组 1 tool_chain_ok=H 证据；225 handoff=判断权上移证据）；
3. 产出**拆减准备度矩阵**：节点×三态×证据锚点×拆掉所需前置（哪些指引/工具暴露缺口）；
4. 本卡**不执行 ablation run**（首盘只盘点+设计 ablation 腿模板），ablation 分族挂后续卡。

## 产出

- coord/reports/2026-11-05-af-family-health.md：健康矩阵（面 1）+拆减准备度矩阵（面 2）+问题清单（断言过时/真回归/缺口三类分列）；
- 问题处置建议逐条（转卡/小修域内/记录观察）。

## 约束

- 文件域=本卡工件目录+报告；**各族脚本/生产代码零改动**（发现问题→取证分列转卡，含对 256 域的只读）；
- 真栈分批占用协议照旧；§8 逐腿跑前口径；与 256 会话错峰用栈；
- C4/D1 引用既有结论不重跑（省钱纪律）。

## 停止条件

- 某腿连续两轮同型断言失败→止损取证（§8）；
- 栈/环境中断按 §9；发现生产面崩溃级缺陷→立即取证上交。

## 领取

- 2026-11-05 22:24 +0800 / 基线 origin/main=15ef1050 / owner=ZCode GLM-5.3 L1 PC 执行会话 / 分支 port/af-family-health-1 / worktree D:/Vit_DAW_wt_257impl / 领取提交=本笔（hash 见回执）。领取时核对：卡在 todo、派发已确认（用户明示）、doing 仅 256（文件域 scripts/run_free_state_d1_smoke.ps1，本卡只读不相交）；真栈被 256 占用中（VitApp 13372/VitAgent 44016/Godot 39304，登记 15ef1050）——本卡执行序=先零栈/只读腿（面 2 盘点+脚本勘察+最小栈测试），真栈腿等 256 释放后按 §2.2 分批登记错峰跑。

## 回执

- 2026-11-05 23:00 +0800 / 报告=[coord/reports/2026-11-05-af-family-health.md](../../reports/2026-11-05-af-family-health.md)（健康矩阵+拆减准备度矩阵+问题 G1-G7+处置建议）随 port/af-family-health-1 入库；工件 coord/runs/AF-FAMILY-HEALTH-1/{deesser_matrix_20261105_224002, mix_tick_20261105_224459}/ 同批。

**面 1 摘要**：零栈全绿（C1 冒测七包+agentloop B 族契约+chat 全量复跑——首轮 5 失败取证=测试隔离缺陷非回归，删全局 Temp 残留后全绿）；真栈批 1 de-esser=**腿过时**（9/9 PCA 装载门拦，py invoke 无 context，2026-08-08 门落位后腿未跟上，compressor 系列同根因）；批 2 mix_single_tick=**模型行为分支非链路回归**（观察工具链三步亲证健康，模型对「先等我确认」话术走终局文字建议未挂卡——256 措辞漂移假说同族，needs_confirmation 契约存废**交主管裁定** G6）；批 3 b13b 不跑（同族风险+预算节制，已记录）；C4 引用 M3 收口（组 1 稳定绿 13/15+组 3 5/5）、D1 in-flight（256）。

**面 2 摘要**：固定路由面逐族钉死（capability_owner.go 词表梯度：B2 12+词>B3/B4/C1>C2/C3 仅显式标记；C4 automation_intent 词表；B1 agentloop preflight；D1 settle 意图词；queue fastpath=记账非派发）；harness 可达面=工具 Invoke 全 case 清单（观察/执行/A4/D1/自动化五族）+指引面（read-only 白名单+full-access autonomy 指令+能力目录披露）；拆减准备度矩阵 23 节点（**A=0/H=1（C4）/F=B1-B4+C1-C3+D1+A 族观察面/not_built=C5-D2-D4-E-F1**）；ablation 腿模板四件套（§六，未执行）。

**问题清单**：断言过时 0 定性+1 待裁定（G6）；真回归 0；缺口 7（G1/G2 flaky 测试隔离、G3 脚本退出码缺口、G5 matrix 腿族过时、G6 行为面待裁定、G7 环境陷阱记录）。处置建议见报告 §七（G1-G5/G6-G7/C4 首拆/B4 次选/C2 最后/D1 等 256）。

**计费**：零栈+批 1 零 LLM；批 2 deepseek-flash 257 行 telemetry ≈¥0.2-0.4（预算 ¥1-3 内）。栈占用 22:39-22:51（登记 4a416ef0/TEARDOWN_ALL_ZERO 22:51:27），与 256 错峰满足。

**红线核对**：各族脚本/生产代码零改动（worktree git diff 仅 coord/+工件；256 域 scripts/run_free_state_d1_smoke.ps1 只读未触碰）。端测边界声明：本卡为体检盘点卡，无渲染面/用户旅程改动（§5 两层层级门槛不适用）。

## 验收

- （主管回填）
