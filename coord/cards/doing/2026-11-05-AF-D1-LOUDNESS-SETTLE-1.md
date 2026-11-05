# AF-D1-LOUDNESS-SETTLE-1：D1 全局响度与峰值能力落地（A-F 主线首卡）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[2026-11-05-af-line-survey.md](../../reports/2026-11-05-af-line-survey.md) §三）
- 派发确认：已确认（主管发卡；用户 2026-11-05「先勘察，发卡」口头授权）
- 验收负责人：GLM 主管决策流
- 池序 254；目标仓库=主仓 agent；P1 / 1 天 / 依赖=无（M3 链已收口）；L1-L2 / PC 执行会话；验证腿需真栈（一次）

## 背景（勘察锚点）

- D1「全局响度与峰值」节点现状：观察面已收口 `pre_master.observation.v0`（capability_routing.go:658，D1-D3 共用只读 pack）；**无执行/判断 runtime**；`agent/internal/loudnesswatch/` 包在库（trigger.go 590 行+compare.go+config.go 含测试）。
- 前置腿：port/loudness-watch-trigger-1-d1 分支（worktree wt_loudness_watch_b 保留中）有 **D1 settle 重锚 LW 真栈腿脚本**（scripts/run_free_state_d1_smoke.ps1 +601 行）与两 run 工件；其 gate4 被**产品缺陷阻断上交**（缺陷描述见该分支工件，worktree dirty 72MB 内含 run 记录）。

## 目标

1. **缺陷诊断**：从 wt_loudness_watch_b 工件读取 gate4 阻断形态，在当前 main 定位该产品缺陷（只读取证→根因→最小修；若已被 M3 链时代修复则证据关线）。
2. **D1 能力落地**：以 loudnesswatch 包为判断底座，点亮 D1 节点——工程全局响度/峰值观察（复用 pre_master.observation.v0 pack）+阈值/目标判断+**待确认动作面**（如全局增益调整建议/推子动作，走既有 mix.propose_tick→确认→执行治理链；禁裸写）；capability ledger 记录使 D1 在 queue view 从 not_started 点亮。
3. **真栈验证**：真栈单 run（隔离工程+双轨 clips 同 249 先例），判定面=D1 观察正确读取全局 LUFS/峰值域可用值+判断输出+动作面挂确认卡；§8 运行口径跑前成文（预算 ≈¥2-4，单 run 无重跑）。

## 约束

- 文件域=agent/internal/loudnesswatch/（+chat 侧 D1 路由/账本接线最小面）+scripts/run_free_state_d1_smoke.ps1（承接 90 腿脚本，从 worktree 取材后改造）+测试；**不触 automation_intent/semantic_entry**（M3 域）；
- loudnesswatch 既有行为不回退（包测试全绿为底线）；
- 确认/执行走既有治理链（247 以来口径），LLM 提议≠写入；
- 判断权冻结令适用：固定文案只做识别/执行/兜底，判断归 LLM（如涉）。

## 停止条件

- gate4 缺陷根因超出本卡文件域→取证上交转卡；
- 真栈断言失败→取证包上交不重跑；栈/环境中断按 §9；
- loudnesswatch 包测试回退→停。

## 领取

- 2026-11-05 20:46 +0800 / origin/main=b46a9096（领取基线，与本地一致）/ owner=GLM-5.3 L1-L2 PC 执行会话（ZCode，254 卡会话，sess 本流）/ worktree D:/Vit_DAW_wt_254impl / 分支 port/af-d1-loudness-settle-1 / 领取基线 HEAD=b46a9096，status --short 空、diff --stat 空（干净基线）/ 真栈占用：领取时最近记录=2026-11-05 20:11 M3(225) 释放 TEARDOWN_ALL_ZERO；真栈腿启动前另行登记（§2.2），与 255 错峰

## 真栈腿跑前口径（§8，2026-11-05 21:16 成文）

- 命令（在 D:\Vit_DAW_wt_254impl 下执行）：`powershell -NoProfile -ExecutionPolicy Bypass -File .\scriptsun_free_state_d1_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_254impl -PromptFlavor reverb -ExpectDomain reverb -LoudnessWatch -D1LoudnessSettle -LlmModelOverride deepseek-flash -SkipBuild -KernelExe D:\Vit_DAW_wt_228\VitAppuild_release\VitApp_artefacts\Release\VitApp.exe`
- 运行次数=1，无重跑；LLM=deepseek-flash（VIT_AGENT_LLM_MODEL 注入，D1 runner 轮次；D1 settle 四门与 LW 四门零 LLM），预算 ≈¥2-4 上界。
- 被测 agent=worktree 构建 bin/VitAgent.exe SHA256 F321AF5E268CE2C14FB83BDA15BE1C4F73083AFC3C3BC620299E486720C6E1AD（实现 c687c9a1）；kernel=wt_228 显式 -KernelExe SHA256 F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A（225/249/251/253 同款只读复用）。
- 成功条件（脚本 exit 0）＝①D1 runner exit 0（133 run2 同形态 -PromptFlavor reverb -ExpectDomain reverb）＋②LW 四门回归不回退（133 已 PASS 面）＋③D1 settle 四门：settle 轮 stop_reason=d1_loudness_settled+观察 loudness_peak_status=present+payload checked/config_basis/mutation_performed=False；verdict store 落 d1_settle trigger 记录；动作面诚实（breach→mix_tick_confirmation 卡+取消腿 mix_tick_rejected / 非 breach→无卡，verdict kind fixture-dependent 不硬断言=90 卡先例）；mix_queue_view D1 节点≠not_started。
- 失败分类预写：a) D1 runner 模型随机分支失败（materiality missing/budget exhausted，133 run1 先例）→取证上交不重跑；b) 栈/脚本环境中断→§9 记录后人工处置；c) settle/LW 门断言失败（实现缺陷面）→取证包上交不重跑。
- 止损线：单 run 后停（无论结果）；同代码版本同断点不烧第二轮。占用登记 coord/254-stackclaim 推 main 后 ≤60s 即时复核进程/端口再启动。

## 回执

- （执行侧回填：完整命令/退出码/run ID/HEAD/计费/判定/D1 点亮证据[gueue view 状态]/gate4 缺陷处置结论）

## 验收

- （主管回填）
