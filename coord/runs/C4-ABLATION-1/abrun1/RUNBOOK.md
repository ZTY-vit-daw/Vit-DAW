# C4-ABLATION-1 abrun1 真栈 RUNBOOK（§8 口径——跑前成文）

- 卡：C4-ABLATION-1（池序 263，A-F 主线首拆）｜执行 owner=ZCode GLM-5.3 L2 PC 执行会话（263 卡会话）
- 被测：worktree `D:\Vit_DAW_wt_263impl` @ `port/c4-ablation-1`，HEAD=`fe368796`（实现提交：Go 缝 2 文件 20 行 diff+配对单测+py/ps1 ablation 参数；C++/kernel 面零改动）；agent=worktree 构建（ps1 -RestartAgent 自建）
- kernel：`D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe` SHA256 `F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A`（2026-11-06 10:5x 亲核，=249/251/253/225/254 同款只读复用）
- 引擎：`-LlmModelOverride deepseek-flash`（238-253 先例）

## 完整命令

```
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_263impl -StartKernel -RestartAgent -AutomationLLMRouteSmoke -LlmRouteGroups "1,3" -LlmRouteAblationHook -LlmModelOverride deepseek-flash -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe
```

## 钩生效验证（三重）

1. ps1 console 行 `automation LLM route ablation hook: VIT_AUTOMATION_INTENT_ABLATION=1`；
2. py summary `ablation_hook: true`；
3. run 内零 `lexicon_hit` verdict——任何 lexicon_hit=钩未达 agent=配置失败，py exit 4 `INVALID_ABLATION_HOOK`。

## 运行矩阵（单 run 无重跑）

- 组 1「这段让它越来越突出一点」×5（回归面：钩不改变其既有纯 LLM 路径，预期与 253/225 稳定形态一致 ≥4/5）
- 组 3「30 秒之后动起来」×5（**ablation 目标面**：253 词表武装拆除后纯 harness 复现；卡面「三样本起步」→实跑 5 样本覆盖起步地板并以字面 ≥4/5 判据评定，LLM 化轮合计 ≤10 控预算）
- 组 2 不重跑（249 判别定案）；组 4 与钩结构性不兼容（判面需 armed clarify 出口），py 已前置拒绝

## 判据（能力复现≠固定面绿；ablation 模式）

- 每组 successes ≥ 4/5；success = {tool_chain_ok, boundary_ok}（lexicon_hit 在钩模式下不算成功）
- choreo-pending 腿 = `skipped_ablation_hook`（词汇 arm 被钩禁用，结构 n/a，如实记录不算失败）
- 组内止损：3 连同型失败中止该组，证据保全，不再花轮

## 判定分叉（A 态输入）

- 两组均 ≥4/5 → C4 判 A（可拆）输入成立，判定面入报告
- 任一组不足 → 缺口清单如实记录转卡（重点候选：semantic_entry 仲裁器 unresolved→确定性模板拦截=251 形态「动起来无声学目标/仅时间范围不构成请求」；该面本卡禁改，不硬修凑过）

## 失败分类与止损

- verdict 词表沿用 220/230/232：no_route / capability_blocked / in_progress_timeout / wrong_tool / confirm_broken / write_fail / env_interrupt(exit 3)
- exit 2 = guidance 部署缺陷锚点 → 取证上交不重跑
- exit 4 = INVALID_ABLATION_HOOK（配置失败，零有效样本）→ 按环境中断处置：修配置后至多重跑 1 次（256 RUNBOOK 先例）
- 断言失败取证不重跑；§8 单 run 无重跑

## 预算

≈¥2-4 上界（≤10 LLM 轮 deepseek-flash；换算基准=253 回执 5 LLM 轮≈¥1.7-2.3、225 回执 8 LLM 轮≈¥2.2-2.4）；telemetry 亲数 in/out 入回执

## 拆栈

ps1 完成态不自带 teardown（设计如此）→ owner 亲拆 VitAgent/VitApp/Godot 前端，TEARDOWN_ALL_ZERO 亲核（四端口 7878/5555/5556/8787 + 18787/UDP 4444/4445 + 三类进程=0）后释放登记
