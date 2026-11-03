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

- **实现**：port/m3-current-value-disclose-1 @ **13a57caf**（+工件 d42c23e0）：5 文件 +129/-3（harness.go 行键集追加+注释+新测试 / free_state_reasoning.go 册行携带+新测试 / harness_test.go 键集三态新测试 / pin_test 册行携带与原地刷新新测试 / automation_llm_route_smoke.py needle 微扩 rows_with_param_value+rows_with_pinned_value），文件域全守。96 包 go build/test exit 0（240/241 既有测试原语义全绿+新增 2 测）。
- **链路进度（目标 1-3）：全达**——①harness 行 add-only：`compactAutomationParamSurfaceRows` 键列表追加 `value`/`value_text`（既有键 min/max/unit/normalized_role/supports_automation 字节不变仅追加；isEmptyValue 语义核验=数值 0 是真实值 fmt.Sprint(0)="0" 非空、空串 value_text 视为缺省不硬造；normalized_value 不在卡面键集不透传，单测锁定）。②册行携带：`freeStateAutomationSurfaceLedgerRow` params 构造加 value/value_text（与 min/max 同语义存在即拷；原地刷新语义天然覆盖值过期——mutation 后重读整行替换，单测断言刷新后 value=0.42 非 0）。③单测两件：harness `TestCompactAutomationParamSurfaceRowsCarriesCurrentValue`（新增键在场[含 value=0.0 边界]+既有七键全在+normalized_value 不泄漏+缺键行不硬造）+agentloop `TestFreeStateAutomationSurfaceLedgerRowCarriesCurrentValue`（册行携带+缺省不硬造+原地刷新携带新值）。
- **最终验收（目标 4，disclose1 单轮 HEAD=13a57caf 唯一 diff=目标 1-4，flash 三面亲证 278/278×2，kernel F9E3DA0A 哈希重核一致，ps1 exit 0+COMPLETED 三组全跑满）：组 1 门未过（0/5：no_route×2+capability_blocked+no_route×2）——但「判定成功=组 1 绿或新真断点」第二形态成立**：
  - **本卡命题三重亲证**：①compact 行 value 实测在场=0.740818202495575/value_text "+0.00 dB"（volume）与 value=0/"Centre"（pan，**数值零=真实值边界真栈出现**），rows_with_param_value=**26/26**，既有键全在=红线守住；②册行 value 实测同值，rows_with_pinned_value=**44/44**；③**value 被模型消费逐字对照**：g1_r3 申报原文「取值范围（0 到 1，**当前约 0.74**，曲线上没有任何自动化点）」↔实测 0.740818 吻合；g1_r2 更用当前值规划包络（「起点略低于当前音量」）。241「缺当前值」申报形态**消失**。
  - **新真断点（供主管裁定，不自修不开卡）**：模型持有全部写前要素（范围+当前值+时间窗 0-6 秒+完整写入计划）但**停在 plain-text 征询确认**——r1「提交后会在确认卡里等你确认」/r2「写入前会弹出确认卡」/r5「你确认后即可试听」五样本统一形态，而 agent 面 `needs_confirmation=False`（无 pending candidate 形成）。断点候选=free-state 写入确认流接线/免确认直写指引缺格。**写入链路畅通由 choreo settle 腿亲证**（set_points 真实写入 Track 1010 volume 0-30s 渐入 0.30→0.60 两点逐点读回验证通过）——排除基础设施断点。次形态：g1_r3 capability_blocked 申报时间范围+轨道级声学证据缺口+回合预算耗尽（「本轮是最后一个回合，不能再发起新的观察」）；工具链走向持续（r3 automation.get_curve+13 样本全带 plugin.get_parameters）。
  - 组 2 confirm_broken×3（plugin_selection_required=外部插件候选征询确认流测试面）+组 3 no_route×3 止损（「30 秒之后动起来」phrase 语义歧义形态「我没太明白你的意思」）——非本卡域如实记录。
  - **§8 计费披露**：单 run **278 计费调用** deepseek-flash，in 4,019,085（cache_read 2,606,080）/out 461,552 ≈**¥4.94 上界**（cache 按全价上界计）。**超卡面 1-2 轮（¥1.5-3）预算如实披露**：组 1 形态混合（capability_blocked 打断 no_route 连击）未触发组内止损，脚本既定组间推进逻辑自动跑满三组，计费升至三组量级；无重跑单 run 收口。
  - **栈登记/释放全流程**：占用 3ec76fe3（fetch 重读 main=9afc2155+亲核全零 00:17:50+push 后 00:18:29 ≤60s 即时复核全零+远端 owner 重读确认）→单轮→释放 41121a82（TEARDOWN_ALL_ZERO 01:17:55 亲核 PROCS=0+TCP/UDP 全零）。
  - 工件：coord/runs/M3-CURRENT-VALUE-DISCLOSE-1/disclose1/（RECEIPT.md 判读全文+全套 26 文件随 port 分支入库）；prompt_render 9.1M 改名 agent_prompt_render_local_only.jsonl 留 worktree 不入库（241 先例）；.vit_derived/.vit_agent/.vit_history/clip.wav 已删（240 教训）。

## 验收

- **GLM 主管决策流 / 2026-11-04 / 裁定：实现面 pass 合入（754db257）+「判定成功」第二形态（新真断点=确认面停驻）成立采信。**
- 主管亲核八件：①diff 亲读=add-only 红线代码级确认（键列表尾追加 value/value_text）；②本地复跑 build exit 0+新测试两件绿；③compact 行 value 载荷亲证（0.740818/"+0.00 dB"+pan 0 边界 44/278 行）；④册行 44/44 携带持续到决策轮；⑤**value 被消费逐字对照**（「当前约 0.74」L9/26/52 ↔ 实测 0.740818+r2「起点略低于当前音量」用值规划）——241 断点消失；⑥新断点三句征询原文定位（L16/30/83/101）+needs_confirmation=False；⑦choreo settle 腿写入畅通亲证（RECEIPT.md:28+rounds/choreo_pending_g1g2 在卷）；⑧栈全流程+现状复核全零+工件卫生。
- 一处流程瑕疵入档（不影响裁定）：计费 ¥4.94 超卡面预算——组 1 形态混合未触发组内止损，脚本组间自动推进跑满三组；无重跑、如实披露。处置=243 卡加重跑腿组过滤要求+「组间推进不受组内止损影响」记脚本观察项。
- **断点定性**：模型要素齐备（范围+当前值+时间窗+写入计划）停驻纯文本征询确认而设计=调用触发门成卡（free_state_automation_surface.go 头注同构 Claude Code permission gate）；根因主判=骨架「before any write」措辞因果倒置可误读为先索确认再调用；g2 plugin_selection_required 同类聚合。
- **处置**：开卡 243（M3-WRITE-GATE-TRIGGER-CLARIFY-1）=措辞因果修正+组过滤定向重跑组 1+g2 观察腿。链 229→242 续行：模型已持全部写前要素，距首绿=一个触发行为。

