# RECEIPT — M3-GROUP3-DISCRIMINATE-1 / g3run1（组 3 单组判别 run）

- 执行：ZCode GLM-5.3-Flash PC 执行会话（251 卡会话，L1）；worktree D:\Vit_DAW_wt_251run @ port/m3-group3-discriminate-1
- 被测 HEAD：9adfea31（=领取基线，**本卡零代码改动**，agent=worktree 基线构建 VitAgent.dev-smoke.exe，f1b7d204→9adfea31 零非 coord/docs 改动亲核=与 249 被测代码同源）；VitAgent.exe 由 ps1 构建（build passed 亲核 console）
- kernel：wt_228 VitApp.exe SHA256 前缀 f9e3da0a66263f0b 只读复用（10:30 哈希亲核一致），-KernelExe 显式指定（249 教训执行到位）
- 栈占用登记：d2a9f4ad（2026-11-05 10:31）；拆栈 TEARDOWN_ALL_ZERO 亲核 10:50:22（PROCS=0+TCP 六口=0+UDP=0，owner 手拆 VitAgent 46040/VitApp 10816）

## 一、命令与退出码

启动一（环境中断，零 LLM 损耗）：10:32:50 Git Bash 调 powershell 时 `-File scripts\dev_agent_smoke.ps1` 反斜杠被 bash 转义吞掉（AGENTS §5 已知陷阱），PowerShell 报 -File 参数不存在 exit 127，ps1 未执行、零进程启动、零 LLM 损耗——按卡面停止条件第 2 条配置修正（路径改正斜杠，249 回执同款写法）重启。取证=ABORT-NOTE.md + ps1_console_abort1_bashpath.log。

启动二（本 run 主腿）：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev_agent_smoke.ps1 `
  -RepoRoot 'D:\Vit_DAW_wt_251run' -StartKernel -RestartAgent -AutomationLLMRouteSmoke `
  -LlmRouteGroups '3' -LlmModelOverride deepseek-flash `
  -KernelExe 'D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe'
```

- **ps1 exit 1**（10:36:35 起，10:37:36 python 腿 `[DONE] status=GUIDANCE_MISSING exit=2` → ps1:8779 throw stop condition 2）；栈未自带 teardown（既有行为），owner 手拆全零 10:50:22
- run 目录：`VitApp\Workspace\Artifacts\smoke\automation_llm_route_20261105_103635\`（=工件原始坐标）
- **判定数据在 exit 1 之前已完整产出**（组 3 三样本 + summary + rounds + payload 全落盘）；exit 1 根因=验证面 guidance 检查（见 §五），非组 3 判别中断

## 二、组判定（以脚本组判据为准）

| 组 | 话术 | LLM 样本 | verdicts | successes | 组判据 |
|---|---|---|---|---|---|
| 3 | 30 秒之后动起来 | 3 | no_route×3 | 0 | **group_pass=false**；r3 后连续 3×no_route 触发脚本止损线（stop_loss={group 3, no_route, consecutive 3}，组判据 ≥4/5 未过） |

- verdict_distribution={no_route: 3}；choreo_pending_leg=null（止损在第 3 样本，choreo 窗口=第 4 样本前未达，CHOREO_ARM_PHRASE 腿未执行）
- lexicon_hit（确定性词面接管，组 3 成功形态）**0/3 未发生**：裸话术「30 秒之后动起来」与跟进「0 到 30 秒」均不在确定性词表（词表已验证覆盖=富话术「0 到 30 秒，让这条轨的存在感慢慢强起来」218），话术按设计走模型语义入口仲裁
- 判别目标达成（新形态取证包逐轮分列，**不追绿、无重跑**）

## 三、逐轮断点形态（判定坐标=rounds/g3_r1..r3.json + rounds_verdicts_extract.json）

**三轮完全同型，断点层=语义入口仲裁器（semantic entry arbiter，chat/semantic_entry.go 独立小提示面，系统提示 2896 字符）**：

- 每轮 4 次交换：主话术「30 秒之后动起来」×1 + 脚本跟进 REGION_FOLLOWUP「0 到 30 秒」×3（FOLLOWUP_MAX=3，轮内每次回复=同一澄清模板→驱动补时间范围→仲裁仍拒）
- 12/12 交换 stop_reason=**semantic_entry_unresolved**；12/12 仲裁 route=unresolved（conf 0.18-0.35，逐次拒绝理由为模型真实生成、措辞各异：主话术=「太模糊/『动起来』无明确声学结果或控制目标」（r1/r2/r3 三版措辞可读于 payload 行 1/5/9）；跟进=「仅时间范围不构成请求」）
- 回复面：三轮 verdict_detail 逐字相同=「我没太明白你的意思——是想继续刚才的确认，还是提个新要求？…」——**agent 确定性模板**（chat/semantic_entry.go:395 亲核），非模型生成；「继续刚才的确认」为模板固定措辞，与 pending 状态无关
- 自由态/自动化面零到达：13/13 载荷行零 neutral-family 特征（free_state/set_points/Action Path/improvement_proposal 全否），prompt_render 插桩（message_loop.go:532）零行落盘=文件未创建
- 无重跑、无止损线外消耗：llm_samples=3 即止损，attempt 3/13

## 四、对照记录（vs 242 时代组 3，2026-11-04 M3-CURRENT-VALUE-DISCLOSE-1 全组 run）

| 维度 | 242 时代 | 251 本 run | 判别结论 |
|---|---|---|---|
| 组 3 顶层判定 | no_route×3，连续止损 | no_route×3，连续止损 | **复现**（同判定、同 verdict 分布、同止损形态） |
| r1/r2 断点层 | 仲裁层 semantic_entry_unresolved + 同款澄清模板 | 仲裁层 semantic_entry_unresolved + 模板逐字相同 | **逐字复现** |
| r3 形态 | **entry 解析成功→goal 武装→limit_reached 空转→continuation settled completed 零写入**（verdict_detail=continuation settled goal completed with no write…） | 仲裁层 unresolved（与其余两轮同型） | **未复现**（主话术解析率 242=1/3 → 251=0/3，小样本如实记录；「过仲裁但空转零写入」形态本轮零出现） |
| guidance 面 | prompt_guidance_rendered=True；payload 240/273 行带指导（组 1/2 载荷携带） | False；payload 0/13 行 | **结构性差异非缺陷回归**（见 §五：本 run 全部请求停在前置仲裁小面，自由态装配面零进入） |
| 仲裁拒绝理由可读性 | 未逐行取证（242 工件无 payload 仲裁理由分列） | 12/12 理由+置信度逐行可读 | **新证据面**：拒绝为模型语义判断（「动起来」无声学目标）非确定性门拦截 |

- 链判别意义（供链收口报告）：v2/v3 措辞链（243 触发权句/245 carry/246 终局路由/247 guard）的效应面在**自由态装配之后**；组 3 话术家族的断点在**装配之前的语义入口仲裁**，措辞链对其不可达——组 3 的改进杠杆在仲裁器对话术家族的解析（模型能力/仲裁面设计），不在动作路径措辞

## 五、ps1 exit 1 根因链（stop condition 2 = GUIDANCE_MISSING；结构性后果，非 242 装配缺陷复现）

1. 组 3 全部 13 次请求停在前置仲裁小面（2 消息、2896 字符系统提示，职责=仅分类交互意图）
2. 语义入口未解析 → message loop（自由态/neutral-family 面）零进入 → `appendMessageLoopPromptRenderDiagnostic`（message_loop.go:532 唯一调用点）零次执行 → **agent_prompt_render.jsonl 文件从未创建**（249 同位置 872KB 对照）
3. 脚本 check_prompt_guidance 读不存在文件 → False → exit 2（stop condition 2 设计语义=「deployment defect anchor」）→ ps1 throw exit 1
4. payload needle 0/13 为真实证据（引擎实收请求确实无指导文本）——但原因=全部请求为仲裁小面（该面设计上不带自由态指导），**非 242 时代装配分支缺陷复现**（装配函数 freeStateTimeVaryingActionPathGuidance 在 neutral-family directives 内无条件追加，ccb_model_prompt.go:61 亲核；只是该面本轮零到达）
5. **脚本盲区观察项上交（不修，越域即停）**：stop condition 2 在「全仲裁 unresolved」形态下误报 deployment defect——(a) prompt_render 文件缺失（零 message-loop 轮）与装配缺陷不可区分；(b) 全仲裁形态下指导缺席是结构性。供主管裁定是否开脚本判别面修订卡（如：check_prompt_guidance 在零 message-loop 轮时降级为 N/A 或与 verdict 面联动）

## 六、setup 前提与前置腿

- setup：ride track 1010 + volume plugin 1011（no external plugin，card group-2 premise 成立）+双轨 deterministic clips（1007:1/1010:1）+observation ready after 1 attempt（acoustic excerpt=partial/partial）+隔离工程 VIT_PROJECT_XML——与 249 setup 同形态
- **前置 Strip Silence 腿本 run 全绿零 warn**（import ok→deterministic suggest stop_reason=needs_confirmation「这个操作需要你确认后才会执行。」→confirm ok goal_status=completed「已删除 3 个静音区域」→cleanup ok；console 亲核）——249 g23run2 同腿两 warn（chat confirmation did not complete / suggest 未产出期望 pending regions）**未复现**，249 验收裁定的低优先观察项本 run 未见
- console 中组 3 话术乱码=ps1 console GBK 编码工件，summary/payload/rounds 内 UTF-8 正确（非发现项）

## 七、计费实测（§8）

- **13 调用，deepseek-flash 全部（13/13），in 9,079 / out 3,452 tokens**（逐调用在 agent_llm_telemetry.jsonl，归档行数=13 与 summary usage 13 一致——本 run 无窗口差，249 验收注记的窗口差异未出现）
- 按同比例口径 ≈**¥0.01-0.02 量级**（248 口径：5.70M in≈¥9.26）——远低于卡面 ¥2-3.5 预算（根因=仲裁小面调用极短+止损@3 无自由态长链；249 的 ≈15 调用/样本放大因子未出现）
- 启动一 abort LLM 零损耗；单 run 无重跑达成

## 八、诚实边界与执行偏差披露

- 启动一=执行侧启动配置错误（bash 反斜杠路径转义），环境中断、零 LLM 损耗、零进程启动，修正后重启合规（卡面停止条件第 2 条）；取证留档同目录
- ps1 exit 1=验证面 stop condition 2，**非组 3 判别中断**；按卡面停止条件第 1 条（功能断言失败→取证包上交不重跑）处置；组 3 判别数据在 exit 1 前已完整落盘
- success 条件落点：卡面「成功条件=脚本判定 exit 0（组判据过）或新形态取证包（逐轮分列）」——本 run=**后者**（组判据 group_pass=false，仲裁层新形态+242 对照已取证）
- 样本量边界：止损@3 使组 3 仅 3 样本（卡面预期 3-5）；「242 r3 形态未复现」的样本基础=3 次主话术尝试 vs 242 的 3 次（其中 1 次过），小样本下不排除概率性解析，如实记录不外推
- 本卡零代码改动达成：全程仅 coord/ 卡面+资源记录+工件目录写入，agent/kernel/scripts 零触碰（§五盲区与 §四杠杆结论均转观察项供主管裁卡）

## 九、工件坐标

- 入库（本目录，port/m3-group3-discriminate-1 分支）：RECEIPT.md、automation_llm_route_summary.json（**全量 70KB**，本 run 无需 slim）、agent_llm_telemetry.jsonl（计费证据）、agent_llm_payload_capture.jsonl（76KB 全量=仲裁理由逐行证据）、automation_llm_route_console.txt（python 腿 console）、rounds/g3_r1..r3.json（全量 20KB×3）、rounds_verdicts_extract.json（逐轮 verdict+12 次仲裁判定摘要）、ps1_console_full.log、ABORT-NOTE.md+ps1_console_abort1_bashpath.log（启动一取证）
- 留 worktree（不入库）：`automation_llm_route_20261105_103635/logs/` 下 .vit_agent/.vit_derived/.vit_history 运行态
- 已删：automation_llm_route_clip.wav（1.15MB，确定性可再生 fixture）
