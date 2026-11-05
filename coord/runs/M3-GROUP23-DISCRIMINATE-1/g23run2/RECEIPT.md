# RECEIPT — M3-GROUP23-DISCRIMINATE-1 / g23run2（组 2/3 判别 run）

- 执行：ZCode GLM-5.3-Flash PC 执行会话（249 卡会话，L1）；worktree D:\Vit_DAW_wt_249run @ port/m3-group23-discriminate-1
- 被测 HEAD：a8c90dd3（=领取提交+占用登记，**本卡零代码改动**，agent=worktree f1b7d204 基线构建，VitAgent.exe SHA256 前缀 c66ee6104a732977）
- kernel：wt_228 VitApp.exe SHA256 前缀 f9e3da0a66263f0b 只读复用（09:34:44 哈希亲核），-KernelExe 显式指定
- 栈占用登记：a8c90dd3（2026-11-05 09:34）；拆栈 TEARDOWN_ALL_ZERO 亲核 10:06:22（PROCS=0+TCP 六口=0+UDP=0，owner 手拆 VitApp 40784/VitAgent 48816）

## 一、命令与退出码

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev_agent_smoke.ps1 `
  -RepoRoot 'D:\Vit_DAW_wt_249run' -StartKernel -RestartAgent -AutomationLLMRouteSmoke `
  -LlmRouteGroups '2,3' -LlmModelOverride deepseek-flash `
  -KernelExe 'D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe'
```

- **ps1 exit 0**（腿执行完成，model-behavior findings 在 summary——按 247/248 先例口径，exit 0=腿完成；组判据结果=summary group_pass 字段）
- run 目录：`VitApp\Workspace\Artifacts\smoke\automation_llm_route_20261105_094045\`（=工件原始坐标，summary 中 run ID 同源）
- console：`[DONE] status=COMPLETED exit=0`（09:41:01→09:57:21，16 分钟）

## 二、组判定（以脚本组判据为准）

| 组 | 话术 | LLM 样本 | verdicts | successes | 组判据 |
|---|---|---|---|---|---|
| 2 | 帮我把空间铺开 | 4 | capability_blocked, no_route×3 | 0 | **group_pass=false**；r4 后连续 3×no_route 触发脚本止损线（stop_loss={group 2, no_route, consecutive 3}） |
| 3 | 30 秒之后动起来 | 0 | —（skipped: stop_loss_in_earlier_group） | — | **未采样** |

- verdict_distribution={capability_blocked: 1, no_route: 3}；choreo_pending_leg=null（组 2 止损在末轮 choreo 窗口前，choreo 腿未达）
- 判别目标达成（新形态取证包逐组分列，**不追绿、无重跑**）

## 三、逐轮断点形态（判定坐标=rounds/g2_r1..r4.json）

- **r1 capability_blocked**：首轮 reply 为合理混音建议（Track 2 立体声相关性 ≈0.68、建议挂空间混响总线小幅加长衰减）+needs_confirmation=True（improvement_proposal_confirmation_required，确认面挂起=治理链正常形态）；continuation 腿自由态推理**诚实申报能力边界**（工程级关系观察为部分结果：成对掩蔽维度不可用；两次实验提案未受理=目标不属候选前沿/候选缺失目标轨；历史观察取回失败）。G1-G8 准入全 pass（admission_receipt.status=needs_experiment）。**无未经确认写入，非 confirm_broken**。
- **r2 no_route**：reply 为合理文本建议（Track 1 声像左偏错开）+needs_confirmation=True，continuation 结算失败（「任务在形成有效结算前失败；失败原因与已有证据已保留」），无 automation 工具面。
- **r3 no_route**：reply=「我还在继续处理这个任务」，stop_reason=limit_reached（turn 预算截停族，220 时代同族形态），continuation settled **failed** 无写入/确认/能力面。
- **r4 no_route**：同 r3 形态，continuation settled **completed** 仍无写入/确认/能力面。

## 四、对照记录（vs 242 时代 g2/g3，2026-11-04 M3-CURRENT-VALUE-DISCLOSE-1 全组 run ¥4.94/13 轮）

| 维度 | 242 时代 | 249 本 run | 判别结论 |
|---|---|---|---|
| 组 2 confirm_broken（plugin_selection_required 外部插件候选确认面停驻）×3 | 在场 | **0/4，已消失** | 242 时代聚合的「措辞修复候选」方向有效；v2 触发权句（243）/carry（245）/v3 终局路由（246）/guard 准入（247）落地后该停驻形态不复现 |
| 组 2 转化形态 | — | capability_blocked×1（诚实边界申报+确认面正常挂起）+no_route×3（结算失败/limit_reached 零面） | **形态已转化**：确认面停驻 → 治理链边界申报 + turn 预算截停族残差（220 时代已知族） |
| 组 3（30 秒之后动起来，语义歧义 no_route「我没太明白你的意思」）| no_route×3 | **未采样**（组 2 止损连带 skip） | **无法判别**（诚实边界）；lexicon_hit 成功形态是否达成未知，待统一手测或后续单组 run |

- 残差复发率（供统一手测参考）：组 2 内 no_route 3/4（其中 limit_reached→continuation settled 零面型 2/4=r3/r4）；与组 1 时代 248 run3 r4/r5 no_route×2 残差同族但形态细节不同（本次含 limit_reached 截停前置）。

## 五、措辞链部署面（payload needle，61 载荷行——243/246 修复效果面健康）

- guidance needle「Action Path Selection…」：**52/61 在场**；243 触发权句「never forms a confirmation card」：**52/61**；时变豁免句：**52/61**
- v3 终局直调句：7 行在场；**旧二选句 0、旧终局二选句 0**（零残留）
- prompt_guidance_rendered=True（242 时代 GUIDANCE_MISSING 装配分支缺陷未复现）
- automation_param_surface / automation_surfaces：0/61（与组 2 零 automation 工具调用一致，无矛盾）

## 六、setup 前提与前置腿

- setup：ride track 1010 + volume plugin 1011（no external plugin, **card group-2 premise** 成立）+双轨 deterministic clips（1007:1/1010:1）+observation ready after 1 attempt（acoustic excerpt=partial/partial）+隔离工程 VIT_PROJECT_XML
- 前置 Strip Silence 腿两 warn（chat confirmation did not complete / suggest 未产出期望 pending regions）——非本卡场景腿，如实记录不影响判定（ps1 仍 exit 0）；与 242 时代全组 run 的前置腿形态一致性未逐行核对（超本卡域）

## 七、计费实测（§8）

- **61 调用，deepseek-flash 全部，in 893,296 / out 135,847 tokens**（逐调用在 agent_llm_telemetry.jsonl）
- 按 248 单价口径同比例外推 ≈**¥2-2.5 上界**（248: 364 调用 in 5.70M/out 445K≈¥9.26）——在卡面 ¥4-7 预算内；g23run1 abort LLM 零损耗
- 单 run 无重跑达成；r1 首轮首 reply 后 continuation 腿轮数放大（61 调用/4 样本≈15 调用/样本，与 242 组 1 capability_blocked 打断形态的轮窗放大同因）

## 八、诚实边界与执行偏差披露

- **g23run1 aborted（环境中断，执行侧启动配置错误）**：首次启动遗漏卡面指明的 `-KernelExe`（wt_249run 新 worktree 无 Export/staging 产物）→ kernel 未启动（脚本 warn 非 fail）→ ZMQ 5555/5556 不通，python 腿 setup 卡住；**LLM 零损耗**（console 空、telemetry 不存在、无 rounds）；owner 手拆（python 49332+VitAgent 39456）复核全零后修正参数重启。取证=同目录 ABORT-NOTE.md。教训：凡 `-StartKernel` 于新建 worktree 必须显式 `-KernelExe`。
- 组 3 未采样（组 2 止损连带 skip）——组 3 判别目标未达成，是否开单组续卡由主管裁定；本卡不重跑（卡面停止条件）。
- success 条件落点：卡面「成功条件=脚本判定 exit 0（组判据过）或新形态取证包（逐组分列）」——本 run=**后者**（ps1 exit 0 但组判据 group_pass=false，新形态+对照已取证）。

## 九、工件坐标

- 入库（本目录）：RECEIPT.md、automation_llm_route_summary.json（**slim 版 12KB**：判定面+needle+usage+setup+逐轮 verdict 坐标，原样 4.4MB 留 worktree）、ps1_console_full.log（ps1 全量 console）、automation_llm_route_console.txt（python 腿 console，UTF-16 原样）、agent_llm_telemetry.jsonl（计费证据）、rounds_verdicts_extract.json（逐轮 verdict+detail+reply 摘要）、ps1_exit_code.txt
- 留 worktree（不入库）：`automation_llm_route_20261105_094045/logs/`（agent_llm_payload_capture.jsonl 4.6MB、agent_prompt_render.jsonl 872KB、summary 原样 4.4MB、rounds/g2_r1..r4.json 原样 r1=1.4MB 含完整 exchanges/workflow_data）、`.vit_agent/.vit_derived/.vit_history`
- 已删：automation_llm_route_clip.wav（1.15MB）
- g23run1/：ABORT-NOTE.md+ps1_console_full.log（abort 取证）
