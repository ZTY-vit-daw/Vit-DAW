# M3-NEUTRAL-ACTION-PATH-1：中性族行动路径教学（时变语义→automation 链）+实收载荷捕获 env 门控+同引擎重验（234 附记指引层定性承接，222 同域）

- 发卡：GLM 主管决策流 / 2026-11-01 晨（依据=234 gate 附记终局定性修正「指引层」+本晨主管对 alignrun 工件字段级亲核；2026-10-31/11-01 gate 首排）
- 派发确认：已确认（gate 首排记录：235 中性族指引补全卡，222 同域）
- 验收负责人：GLM 主管决策流
- 池序 235；目标仓库=**主仓 agent**；P1 / 0.5–1 天 / 依赖=234 已在 main（27828950）✓；L2 / PC 执行会话（装配层判断+LLM 重验判读）

## 标的（234 附记+主管工件亲核定案，不重考）

主管本晨对 alignrun1/2 `agent_prompt_render.jsonl` 字段级复核（130+116 行）钉死的机理：

- 模型每 neutral 轮实收两面：**system_prompt 17,045B**（含 222 指引段于 35% 位，教工具存在）+ **turn_directives 18,322B**（user 侧动态区每轮注入，模型实际服从的显著层）。
- turn_directives 教学面 **100% 静态域**：improvement_proposal 示例 action_domain=track_gain|static_eq|broadband_compression（`ccb_model_prompt.go:105`）；D1-S1 域表同源（`freeStateD1AdmittedDomainRuleForBudget`←`experiment/d1s1_domains.go`）；模式识别指引同（`freeStatePatternRecognitionGuidance`）。set_points/get_plugin_parameters/**时变→包络路径映射** 130/130 行零出现（`envelope` 字样在场但均为治理语境非路径教学）。
- 时变语义唯一合法写入路径=决策轮直呼 automation 面（骨架规则已准入：plugin.get_parameters/automation.get_curve 只读 + automation.set_points/clear_region 治理卡 mutation，`ccb_model_prompt.go:687`）；该路径的**「何时走」判别教学从未到达 directives 层**。
- 结论：234 三型形态（观察重复×6/中性空转×3/静态步选择×2）全部=服从实收指令。222 教存在（system 层）未教路径（directives 层）。

## 目标（三面）

1. **行动路径教学（生产面）**：时变语义行动路径指引到达 **turn_directives 层**。教学内容（英文，承现有 directives 文体）：时间渐变/riding 类听感目标（参数随时间推移上行/下行）映射到 automation 包络路径而非静态域——观察定位目标 → plugin.get_parameters 读参数范围 → automation.set_points（点值在范围内，过治理卡）→ automation.get_curve 回读；与 improvement_proposal 静态域的关系（何时该走直呼面、何时该走提案面）一并成文。插入点勘察定案（`ccb_model_prompt.go` directives 生成器 vs 222 段扩深 vs 双面），判据=**到达模型实际服从的显著层**（234 证据=directives 层；若勘察证据支持其他位形，带证据定案）。红线：纯 prompt 文案+装配，**Go 判定逻辑零改动**；教学=路径存在性+语义特征判别（"riding a parameter up or down over a time range" 类特征描述，承 222 文体），**不硬编码话术→动作映射**（词表全量退役裁定 2026-10-31）；文案版本纪律+锁定测试（222 先例）+render/directives 取证标记（新段可机检在场）。
2. **实收载荷捕获 env 门控（测量面）**：`agent/internal/llm` 补 env 门控全载荷捕获（**默认 off**，如 `VIT_AGENT_LLM_PAYLOAD_CAPTURE=<dir>`）：每次调用 dump 完整 request messages（system+user+history 全量字节）+response content（reply 文本+tool_calls）到 jsonl。现状缺口亲核：telemetry 仅元数据（alignrun1 140 行零载荷）；render 不含全消息列与响应体。写入门控承 `VIT_AGENT_LLM_TELEMETRY_PATH` 先例；产物目录由调用方指定（smoke run 目录），**默认路径禁写**（§10 不落源码树）。
3. **重验腿（干净判别门）**：**同码**（除目标 1/2 外零 diff）**同 fixture**（组 1 同话术「这段让它越来越突出一点」）**同引擎**（deepseek-flash 不换）重跑 automation_llm_route_smoke。判据（229 口径第六承延续）：组 1 no_route 消失 + `tool_chain_ok`（写入形态=automation.set_points 包络：确认卡→确认→写入→get_curve 读回）；234 收敛测量面继续采集；**载荷捕获亲证新指引在场**（directives 含路径教学段）。组 1 门过且预算余量时组 2/3 按 220 口径跑满（非验收门）。

## 预期与诚实边界（发卡时即声明）

指引层定性=**可推翻假设**：directives 层显著性有 234 证据支持但非因果证明。若载荷捕获亲证新指引实收在场仍 3 连同型止损→指引层定性被推翻，剩余变量独占模型/引擎面，换引擎实验（同码同 fixture 只换引擎）转主案——本卡载荷捕获+测量面即该实验的测量基座。不预设通过率承诺；组级 ≥4/5 为门，<4/5 走停止条件上交证据包（实收载荷+收敛数据+形态分型三件套）。

## 停止条件

- 勘察发现决策轮直呼 automation.set_points 治理路径在当前真栈不可达（确认卡死循环/准入新拦截）→锚点上交不折卡（注：229–234 链已修通写入链且 234 亲证 mix tick 写入全绿，本条预期不触发，留保险丝）；
- 新指引实收亲证在场（载荷捕获工件）仍组 1 三连同型止损→**指引层定性推翻证据包上交**（三件套），转主管换引擎裁定，不开第三张指引卡；
- 教学段插入造成确认话术分支/词表路由回归（既有装配测试红）→锚点+两案权衡上交；
- LLM 面栈不可用→环境中断如实记录。

## 文件域

`agent/internal/agentloop/ccb_model_prompt.go`（directives 教学段）+`agent/internal/agentloop/neutral_automation_route.go`（如扩 222 段）+`agent/internal/llm/`（载荷捕获）+`scripts/automation_llm_route_smoke.py`（载荷捕获启用/判据如需微扩）+各自测试。越域即停上交。

## 验收标准

- 单测：教学段装配在场断言+版本锁定测试；确认话术分支零回退断言；载荷捕获 env 门控 on/off 行为+目录创建+默认路径禁写负例。
- 真栈腿（必跑，§5 门槛）：ps1 体系 AutomationLLMRouteSmoke 组 1 重跑 exit 0 + 载荷捕获工件在 run 目录；栈登记/释放全流程（PC-RUNTIME-STACK.md）。
- 回执分列**链路进度**（指引在场+载荷捕获可用）与**最终验收**（组 1 判据三件：no_route/set_points 形态/指引在场亲证）；§8 计费披露。

## 领取

- 领取：2026-11-01 10:09 +0800 / origin/main=5c5ee2ea / owner=GLM-5.3 L2 PC 执行会话（ZCode，本会话）/ 分支 port/m3-neutral-action-path-1 / worktree D:/Vit_DAW_wt_235impl / 领取基线：worktree HEAD=5c5ee2ea、worktree status 干净（主检出存在他卡运行工件与设计稿改动，与本卡文件域不相交，已核对未动）/ 领取提交=（本提交）

## 回执

- （执行侧回填）

## 验收

- （主管回填）
