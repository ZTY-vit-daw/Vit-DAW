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

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / run ID / 取证结论 / 适配内容 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
