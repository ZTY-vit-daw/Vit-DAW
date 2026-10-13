# TIMEFX-D1-EXIT0-DEEPSEEK-1：deepseek 基线下 D1 reverb 收口腿达成（域选择适配）

- 发卡：GLM 主管决策侧 / 2026-10-13 深夜（依据=用户基线裁定「改用 deepseek，不等 gpt-6.1-sol」+[FIX-TIMEFX-AXISFILTER-1 裁定补记二](../../rulings/2026-10-13-FIX-TIMEFX-AXISFILTER-1-pass.md)）
- 派发确认：已确认（用户裁定直授权）
- 验收负责人：GLM 主管决策流
- 池序 108；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P1 / 0.5-1 天 / 无（FIX-106 已合入 main）
- 模型分级：L2 / GLM 执行会话（PC，取证+适配+真栈腿）

## 背景与 blocker 画像

LLM 基线已切换 deepseek-chat（api.deepseek.com/v1；PC 决策侧探测实证：chat=非推理模式直出正文，flash=推理模式勿用）。收口腿两轮（run 20261013_212900/213535，工件在 main）**NOT_EXERCISED exit 3**：reverb 引导话术在场（"残响拖沓…混响尾巴"），模型自主域选择两轮同落 broadband_compression（proposal 锚点 "compressor threshold move within +/-10 dB on a dual-channel surface"）；轮内 LLM 全健康（1.35-4.4s err=false）。定性=模型分支（意图识别/域选择），非服务、非修复面（七处修复未被触达，gpt-6.1-sol 下已两轮实证）。

## 任务

1. **取证**：读两轮 report+agent 日志，钉死域选择发生的轮次与输出形态；对照 intent 词表/域菜单组装面（PromptFlavor→系统提示→自由态循环），回答"为什么 reverb 话术被 deepseek 判到压缩面"（候选：话术措辞偏问题描述而非意图声明/域菜单顺序/示例偏置）。
2. **适配**（边界硬约束）：只允许把**用户意图表达得更清楚**（更接近真实用户会说的话，如"给这条轨加一点混响（衰减短些）"——10-12 手测 B 话术先例）；**禁止**域关键词堆砌或任何绕过模型自主选择的硬路由——那属于倒推验收，违反 AGENTS §8。若涉 agent 侧提示组装面改动，附回归测试。
3. **收口**：`run_free_state_d1_smoke.ps1 -PromptFlavor reverb -ExpectDomain reverb` **exit 0**（建议连过 2 轮：一轮销三笔挂账，一轮给"体验场次可靠性"以稳定性证据——§8：单次 exit 0 只证存在性）；delay 族同话术一轮可选加分。需登记 PC-RUNTIME-STACK 占用。

## 文件域

以取证定界（候选=scripts/run_free_state_d1_smoke.ps1 话术面 + agent 提示组装/intent 表面）；领取时核对 doing 卡。

## 验收标准

取证锚点+适配 diff（或"无需代码适配、仅话术调整"的明确结论）+D1 reverb exit 0（工件含 console+summary）+`go test ./... -count=1` 0 FAIL（如涉 agent 面）+gofmt 净；模型口径行（deepseek-chat）随工件入卷。

## 停止条件

取证证明 deepseek 在**自然话术**下无法可靠选中 reverb 域（需堆砌才命中）→ 带证据上交决策侧裁基线策略（选项：deepseek 全局基线+混音线专用路由 / provider 复议），不硬凑。

- 领取：（2026-10-13 23:09 / origin/main=0a7c1cc49a34a85a83b56a30f2e9e8339e410e1c / GLM-5.3 L2 执行会话·PC·ZCode 本会话 / port/timefx-d1-exit0-deepseek-1 / D:/Vit_DAW_wt_d1ds1 / 领取提交 78b00ea1）
- 回执：（2026-10-13 深夜）
  - **commit**：话术适配+取证工件 0814a3ae、agent 日志补遗 9c50bc7f（均在 port/timefx-d1-exit0-deepseek-1，已推远端）；取证报告=[coord/runs/TIMEFX-D1-EXIT0-DEEPSEEK-1/FORENSICS.md](../../runs/TIMEFX-D1-EXIT0-DEEPSEEK-1/FORENSICS.md)
  - **run**：20261013_231930 / 20261013_232817（真栈三件套：fix106 agent 0f9de7ed 复刻+主检出内核 b6565dcf+Godot；deepseek-chat @ api.deepseek.com/v1 实流，LLM 11 次调用全健康 1.5-4.0s err=false）；栈占用 7913e2a2→释放记录同批提交
  - **取证结论（①）**：域选择发生在第 3 轮 LLM 的 `free_state_decision.v1` 提案 JSON `action_domain` 字段。根因=原话术是纯问题描述型（"残响拖沓…不要用增益或 EQ…给一个建议"），对工具选择留白且仅排除增益/EQ——"收紧尾巴"映射到宽带压缩是语义合法读法；提案期披露菜单按设计只含 ≥2 候选族（reverb/delay 白名单单载体永不入菜单，通过轮 181843 的 74 份披露同样无 reverb），非菜单缺陷非路由缺陷，是 deepseek 把菜单当封闭集 vs gpt-6.1-sol 把域规则（11 域含 reverb 在册）当权威列表的差异
  - **适配内容（②）**：`scripts/free_state_d1_smoke.py` PROMPT_FLAVORS reverb/delay 两句改意图声明型（手测 B 先例形：「…想给其中一条合适的轨加一点混响（衰减短些）…」），盲法四要素保持（无轨名/无参数值/排除句/先观察），无堆砌无硬路由；无测试 pin 旧串（grep 全仓），py_compile 过。**效果：4/4 提案 reverb（旧话术 0/2），2 次完整准入（typed=reverb_decay_adjust decay_pct=3/4）——原 blocker（域选择）已解决**
  - **exit 0 未达成（③）→ blocked 上交**：两轮同断点 exit 1 "D1 must contain exactly one forward mutation"。机制=末轮提案 JSON 同时携带空态 `experiment_materiality={state:none,evaluation:not_ready}` 且 `final=true`（模型幻觉"已应用"，工程实际零插件）→ `free_state_experiment_runtime.go:649` 按字段在场判结算企图拒收 → `goalrunner_chat.go:3119` owed 扣留后不再 arm（lifecycle=none）→ 物化/装载/执行链不发生 → interventions=0；驱动器 nudge 门（free_state_d1_smoke.py:3319 仅 interaction_requests 非空才发"继续"）错失模型明确邀请的续跑。§8 两轮同断点停止重跑。**候选修复菜单（决策侧裁定）：A=驱动器 drain 后对已准入未应用实验发 nudge（scripts 面）/ B=settleReportCarried 容忍空态 materiality 或 owed 后 arm 物化（agent 面+回归）/ C=提案轮提示明令禁填 settle 三字段（提示组装面，概率性）**——详见 FORENSICS.md 第四节
  - **端测边界声明**：本卡改动面=烟测话术字典（无 agent/Go/内核/webui 改动，go test/gofmt 不适用）；真栈两轮覆盖域选择/准入/自由态循环全链；**应用后路径（interventions/settlement/A-B）因新阻断未触达**；LLM 口径行（deepseek-chat）随工件入卷
  - **三笔挂账**：销账条件（deepseek 基线 D1 reverb exit 0）未满足，维持挂账；话术适配已就位，任一候选修复落地后可直接复验
- 验收（主管裁定 2026-10-13 深夜）：**适配部分采信**——话术 diff 亲读（意图声明型/盲法四要素保持/括注为自然用户语非堆砌）+两轮 run 证据亲核（各 4×action_domain=reverb、4×reverb_decay_adjust 准入、2×空态 materiality、零 interventions、同断点 exit 1）+FORENSICS 采信；交付已合入 main（merge 随本批）。**新阻断处置=立 FIX-FS-SETTLE-DEADCHAIN-1（109，B 主修方向）**，本卡收口腿转 109 收口后复验销账（三笔挂账维持）。卡维持 blocked（依赖 109）。
