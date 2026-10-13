# TIMEFX-D1-EXIT0-DEEPSEEK-1 取证与上交报告

- 执行：GLM-5.3 L2 执行会话（PC，ZCode）/ 2026-10-13 深夜
- worktree：D:/Vit_DAW_wt_d1ds1（分支 port/timefx-d1-exit0-deepseek-1，基线 0a7c1cc4）
- 被测栈：agent=VitAgent.exe SHA256 0f9de7ed0fed4c3b…（与挂账轮末版逐位一致，含 FIX-TIMEFX-AXISFILTER-1 七处修复）；内核=主检出 VitApp.exe b6565dcf85d1da86…（C++ 面零改动）；LLM=~/.vit/config.json = deepseek-chat @ api.deepseek.com/v1（flash 未动；启栈前 Invoke-RestMethod 预探测 OK）
- 本卡 run：20261013_231930（exit 1）、20261013_232817（exit 1）——两轮同断点
- 对照在卷 run：FIX-TIMEFX-AXISFILTER-1 的 20261013_212900/213535（旧话术，exit 3）与 181843_FULL（gpt-6.1-sol 干预落账通过轮）

## 一、①取证结论：原 blocker（域选择偏压缩面）根因

1. **域选择发生轮次与输出形态**：自由态循环第 3 轮 LLM 调用（turn1=ccb.observation_catalog 发现、turn2=ccb.observation_request 工程级观察、turn3=无工具直接输出）。输出形态=`free_state_decision.v1` JSON 的 `improvement_proposal.action_domain` 字段（213535 轮=broadband_compression / action_kind=broadband_threshold_adjust，confidence 0.2，admission G6/G8 fail）。
2. **组装面三层输入**（两模型完全相同）：
   - user turn 动态区的 admitted-domain rule（`ccb_model_prompt.go:139` `freeStateD1AdmittedDomainRuleForBudget`，由 `experiment/d1s1_domains.go` 11 行域表渲染，**reverb 在册且明示可选**）；
   - 提案期 plugin_candidate_disclosure（`chat/free_state_plugin_candidate_disclosure.go:54` `addFamily`，**仅披露 ≥2 候选族**；白名单 reverb=1（Pro-R）/delay=1（Timeless 3）为单载体族→**永不入菜单**，常态披露 7 族）；
   - 提案 JSON 示例（`ccb_model_prompt.go:104`）的 action_domain 示例值仅 "track_gain|static_eq|broadband_compression"（示例偏置，次要因素）。
3. **决定性对照**：通过轮 181843（gpt-6.1-sol）全部 **74 份披露同样只有 7 族、reverb 从未出现**——gpt 是在菜单无 reverb 下依意图语义（"混响尾巴"→reverb）自主写出 action_domain=reverb；deepseek 则把"尾巴收干净"映射到菜单内在册的宽带压缩（hypothesis 原文："对该轨施加一个很小的宽带动态阈值收紧，可能让尾巴收束更干净"）。
4. **根因裁定**：非路由缺陷、非菜单缺项、非关键词分类器——**原话术是纯问题描述型**（"残响拖沓…不要用整体增益或 EQ…给一个有界的小步改进建议"），对"用什么工具"完全留白且只排除了增益/EQ；把"收紧尾巴"读成压缩阈值收紧是语义合法读法。deepseek 把披露菜单当封闭选项集，gpt-6.1-sol 把域规则当权威列表。卡面候选归因①（话术偏问题描述）成立为主因；②（域菜单顺序）不成立（菜单无 reverb 非决定性，通过轮反证）；③（示例偏置）为次要助因。

## 二、②适配（话术面，卡内授权范围）

`scripts/free_state_d1_smoke.py` PROMPT_FLAVORS 的 reverb/delay 两句从问题描述型改为**意图声明型**（卡面明示的手测 B 先例形态）：

- reverb：「有几轨听起来比较干，想给其中一条合适的轨加一点混响（衰减短些），不要用整体增益或 EQ 来解决。请先观察工程，再给一个有界的小步改进建议。」
- delay：「有一条轨听起来比较干、缺一点空间层次，想给这条轨加一点延迟（反馈少些），不要用整体增益或 EQ 来解决。请先观察工程，再给一个有界的小步改进建议。」

盲法四要素保持：不指定轨名、不给参数数值、保留排除句与先观察指令、case-agnostic。括注（衰减短些/反馈少些）为真实用户自然语言（与手测 B 原话一致），非域关键词堆砌；域选择仍由模型自主作出。无任何测试 pin 旧话术（grep 全仓核验）；py_compile+导入核验通过；两轮真栈实跑即最强回归。

**适配效果（原 blocker 已解决）**：两轮共 4 次提案全部 action_domain=reverb（run1 turn4=1022/turn6=1032；run2 turn3=1027/turn5=1017），其中 2 次达到完整准入（typed=reverb_decay_adjust decay_pct=3/4，语义状态 "free-state experiment runtime admitted"）。旧话术下同栈两轮 0/2 选 reverb。

## 三、③新阻断（exit 0 未达成的机制，两轮同断点）

**现象**：两轮均 `D1 must contain exactly one forward mutation`（exit 1）——准入成功但 persisted round interventions=0，插件选择→装载→typed 执行的服务端确认链从未发生。

**逐层机制（run2 链尾 23:30:34 事件序）**：

1. deepseek 模式性路径：先凭工程级证据提案（turn3，目标 1027）→ G6/G8 拒（目标无轨道级证据，`<final_gate>` 1092 字 gap 消息）→ 补轨道级观察（turn4-5）→ 重新提案获准入（末轮）。
2. **末轮提案 JSON 同时携带 `experiment_materiality={state:"none", evaluation:"not_ready"}`（空态填空，两轮同）**，且 `final=true`（模型自认已应用——回复原文"这一步已经应用好了：Track 1017混响衰减率 已调整…你回一句'继续'，我就接着把这轮的评估做完"，纯幻觉：工程插件数 0，无任何装载/写参）。
3. 运行时把"materiality 字段在场"当结算企图：`chat/free_state_experiment_runtime.go:649-663` `settleReportCarried := decision.ExperimentMateriality != nil || …` → 无 post-action 观察 → 拒收（WARN "settle report refused until the fresh post-action observation is recorded"）。**空态 {state:none} 无结算语义但同样触发**。
4. 拒收+final=true 走 `goalrunner_chat.go:2150-2165` 降级为 `settle_report_refused_awaiting_observation`，`goalrunner_chat.go:3119` owed 扣留恢复 waiting_continue 但 **不再 arm 任何 continuation**（lifecycle=none，chainEnded=true）。
5. D1 单轮被 `free_state_experiment_runtime.go:1003`（`IsD2MultiRound` 门）排除在 neverActed/欠提案引导之外——D1 下该分支只有"等永不可能落地的观察"一条路。
6. 烟测驱动器 nudge 门（`scripts/free_state_d1_smoke.py:3319`）：仅在 `interaction_requests` 非空时发"继续"；drain 后无 parked 交互→break→验证 interventions=0→FAIL。**模型明确邀请用户回"继续"，驱动器不发**。

**对照**：gpt-6.1-sol 通过轮的准入 turn 以 final=false/limit_reached 结束→continuation 正常 arm→物化轮 park 插件选择确认→烟测批准×2→Pro-R 装载→typed 执行→settle。

**§8 处置**：同代码版本同确定性断点连续两轮失败（231930/232817，同签名：空态 materiality+final=true+拒收吞链），停止原样重跑。

## 四、上交点与候选修复（决策侧裁定）

新缺陷位于卡面授权文件域（话术面/提示组装面）之外，按 AGENTS §11 上交。三个候选层（可组合）：

- **A（烟测驱动器，scripts 面）**：drain 后 goal=waiting_continue 且实验已准入未应用时发一次"继续" nudge（复用既有 MAX_CONTINUE_NUDGES 语义——"designed resume path is a user turn… a bounded neutral continue nudge stands in for that user"）。改动最小，不触 agent 面；风险=放宽驱动器行为需防止把真死链拖成超时。
- **B（运行时谓词，agent 面）**：`settleReportCarried` 不把空态 materiality（state=none/evaluation=not_ready 且无 target_response/round_decision）计为结算企图；或 owed 扣留后对已准入未应用实验 arm 一次物化 continuation。需回归测试。
- **C（提示组装，agent 面）**：提案轮明示"不得填写 experiment_materiality/experiment_target_response/experiment_round_decision"（现为负面示例外的文本约束，deepseek 2/2 违反；概率性修复）。

三笔挂账销账条件（deepseek 基线 D1 reverb exit 0）在上述任一修复落地后即可复验；话术适配（本卡交付）已具备。

## 五、端测边界声明

- 本卡改动面=scripts/free_state_d1_smoke.py 话术字典两句+注释（无 agent/Go/内核改动；未涉 webui）。
- 真栈端测：两轮 D1 冒测实跑（真实三件套+deepseek 实流），覆盖=域选择/准入/自由态循环全链取证；**未覆盖**=应用后路径（interventions/settlement/A-B）因新阻断未触达——该边界即本报告第三节主体。
- LLM 口径行：deepseek-chat @ api.deepseek.com/v1，轮内 LLM 全健康（1.5-4.0s err=false，两轮共 11 次 LLM 调用零错误）。
