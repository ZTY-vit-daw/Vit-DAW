# M3-GROUP3-DISCRIMINATE-1：组 3 单组判别 run（链收口判别补腿）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=249 验收裁定 [2026-11-05-M3-GROUP23-DISCRIMINATE-1-pass.md](../../rulings/2026-11-05-M3-GROUP23-DISCRIMINATE-1-pass.md) §三-2；组 3 未采样=249 卡面设计缺口，非执行侧）
- 派发确认：已确认（链内续卡，主管发卡；用户口头开工即有效）
- 验收负责人：GLM 主管决策流
- 池序 251；目标仓库=主仓 agent；P1 / 0.25 天 / 依赖=249 已在 main（0298790e）✓；L1 / PC 执行会话；栈空闲（249 释放 10:06 亲核）

## 背景

249 `-Groups "2,3"` 单 run 中组 2 连续 3×no_route 触发脚本止损，脚本既有行为连带 skip 组 3（automation_llm_route_smoke.py:1561）——组 3「30 秒之后动起来」（时间窗 boundary 形态）在 v2/v3 措辞链新栈上的行为仍未判别。242 时代组 3=no_route×3（语义歧义「我没太明白你的意思」）。

## 目标

1. **单组判别 run**：`-LlmRouteGroups "3"` 单 run（deepseek-flash，载荷+agent log 全开）——组 3 话术 ×5（止损线=组内连续 3 同型，可能 3-5 样本）。
2. **判定**：脚本组判据（组内 ≥4/5 正确行为轮=过）；逐轮 verdict+断点形态如实分列，不追绿；与 242 时代组 3 语义歧义形态对照（消失/转化/复现）。
3. 产出直接供链收口报告与统一手测清单（组 3 时间窗话术=手测关键场景之一）。

## 预期与诚实边界

- §8 预算 ≈**¥2-3.5**（依据=249 实测 ≈15 调用/样本×3-5 样本+setup/choreo 腿；单轮 ≈¥0.4-0.6）；单 run 无重跑。
- 成功条件=脚本判定 exit 0（组判据过）或新形态取证包（逐轮分列，不原样重跑）。
- 本卡**零代码改动**（纯判别 run+判读）；新断点只取证不修（转卡）。
- **执行要点（249 教训入卡）**：新建 worktree 凡 `-StartKernel` 必显式 `-KernelExe`（wt_228 VitApp.exe f9e3da0a 只读复用，起栈前哈希亲核）。

## 停止条件

- 脚本功能断言失败→取证包上交（逐轮 verdict+载荷坐标），不重跑；
- 栈/环境中断→按 §9 记录处置（零 LLM 损耗的配置修正重启合规）；
- 任何代码修复冲动→越域即停（本卡无实现面）。

## 文件域

零代码改动；产出=coord/runs/M3-GROUP3-DISCRIMINATE-1/ 工件+卡面回执。

## 领取

- 领取时间：2026-11-05 10:29 +0800
- 领取时 origin/main：9adfea3107466a4c3eb8a61a8e641e89e3696af0（=local HEAD，已核对依赖 249 验收 pass 9adfea31 在内）
- owner：GLM-5.3-Flash（ZCode PC 执行会话，L1）/ 机器=PC / 卡=M3-GROUP3-DISCRIMINATE-1
- 分支：port/m3-group3-discriminate-1（基线=9adfea31）
- worktree：D:\Vit_DAW_wt_251run（新建，领取时干净）
- 领取基线工作树状态（主检出 D:\Vit_DAW，他流遗留与本卡文件域不相交，不动）：tracked 修改 8 文件（VitApp/Workspace 两 xml、CHAT-AUTH-STAMP-1 卡 blocked→done mv 未提交、UX-AB-AUDITION-IMPL-1 工件 4 文件）；未跟踪副本若干（agent/coord/、webui/design/、coord/runs/ 多目录、todo/2026-11-05-REPO-WORKTREE-CLEANUP-1.md 等）——均不属本卡文件域
- 栈登记：见 coord/resources/PC-RUNTIME-STACK.md（本卡单 run 前另行登记占用，推送 main 后 ≤60s 内起栈）

## 回执

- （执行侧回填）

## 验收

- （主管回填）
