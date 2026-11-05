# SMOKE-GUIDANCE-CHECK-DISCRIM-1：smoke 脚本 guidance 判别面盲区修复（全仲裁形态误报）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=251 验收裁定 [2026-11-05-M3-GROUP3-DISCRIMINATE-1-pass.md](../../rulings/2026-11-05-M3-GROUP3-DISCRIMINATE-1-pass.md) §三-1）
- 派发确认：已确认（主管发卡；用户口头开工即有效）
- 验收负责人：GLM 主管决策流
- 池序 252；目标=scripts/automation_llm_route_smoke.py（+dev_agent_smoke.ps1 若涉及退出码传递）；P2 / 0.25 天 / 依赖=251 已在 main ✓；L1 / PC 执行会话；不占真实栈（纯脚本改+离线单测/构造 fixture 验证）

## 背景（251 实证）

组 3 全仲裁形态（12/12 semantic entry unresolved，零 message-loop 轮）下：prompt_render 文件永不创建（message_loop.go:532 唯一插桩点未进入）→ check_prompt_guidance（automation_llm_route_smoke.py:1298-1306）读缺失文件 return False → GUIDANCE_MISSING stop condition 2 → exit 2 → ps1:8779 throw exit 1。判定数据实际完整落盘，exit 1=结构性误报。装配面无缺陷（ccb_model_prompt.go:61 无条件追加，251 亲证非 242 回归）。

## 目标

1. **判别修复**：guidance 检查区分三态——①文件在场+needle 在场=ok；②文件在场+needle 缺席=真装配缺陷（维持 fail）；③**文件缺失+零 message-loop 轮（全仲裁形态）=指导面不适用，跳过 guidance 检查不触发 stop condition 2**（输出面如实标注 guidance=n/a(all-arbitration)）。
2. 「零 message-loop 轮」判定依据=telemetry 行 source 分布（全 semantic_entry 族即全仲裁）或等价可核信号；不引入对 prompt 内容的猜测。
3. 复验：构造 fixture（或最小重放 251 归档 telemetry/console）验证三态各自出口；不重跑真栈（本卡验收=脚本面单测+fixture，真栈复验挂下次全仲裁场景 run 搭车）。

## 约束

- 文件域=automation_llm_route_smoke.py+（如需）dev_agent_smoke.ps1 退出码传递段+新增测试；**不动 agent/kernel 生产代码**。
- 不改变既有非全仲裁形态的行为与退出码语义（243 以来的组过滤/判据口径字节不动）。
- AGENTS §5：扩展现有脚本体系，不另起测试机制。

## 验收标准

- 三态 fixture 各自出口正确（②仍 fail、③不再误报 exit 2/1）；
- 现有非全仲裁路径回归零变化（对照组 fixture 输出 diff=0）；
- `python -m py_compile`/脚本自检绿；回执含 fixture 构造说明与输出摘录。

## 领取

- 2026-11-05 18:42 / origin/main `76305d87` / owner=GLM-5.3 PC L1 执行会话（ZCode，本会话）/ 分支 `port/smoke-guidance-check-discrim-1` / worktree `D:/Vit_DAW_wt_252impl`
- 领取基线状态：worktree 新建于 `76305d87`，`git status --short` 空、`git diff --stat` 空（clean checkout，无领取前 diff）
- 领取提交=本提交（claim commit，hash 记入回执）；文件域核对：doing 两卡（GEN-SUNO-WEB-DESIGN-1=webui 设计面、253 M3-TIMEWINDOW-INTENT-ARM-1=agent 词表面）与本卡 scripts/ 域无交集

## 回执

- （执行侧回填）

## 验收

- （主管回填）
