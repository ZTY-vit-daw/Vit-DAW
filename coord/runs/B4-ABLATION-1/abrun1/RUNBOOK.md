# B4-ABLATION-1 abrun1 真栈 RUNBOOK（§8 口径——跑前成文）

- 卡：B4-ABLATION-1（池序 264，A-F 主线第二张 ablation）｜执行 owner=ZCode GLM-5.3 L2 PC 执行会话（264 卡会话）
- 被测：worktree `D:\Vit_DAW_wt_264impl` @ `port/b4-ablation-1`，HEAD=`98fc4ca4`（实现提交：Go 缝 2 文件生产面 29 行 diff[含注释]+配对单测三态+py/ps1 ablation 面；C++/kernel 面零改动）；agent=worktree 构建（ps1 -RestartAgent 自建）
- kernel：`D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe` SHA256 `F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A`（2026-11-06 19:21 亲核，=225/249/251/253/254/256/258/261/263 同款只读复用）
- 引擎：`-LlmModelOverride deepseek-flash`（238-263 先例）

## 完整命令

```
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_264impl -StartKernel -RestartAgent -B4AblationSmoke -LlmModelOverride deepseek-flash -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe
```

## 钩生效验证（三重）

1. ps1 console 行 `B4 ablation hook: VIT_LOW_END_RELATION_ABLATION=1`；
2. py summary `ablation_hook: true`；
3. 5 轮 verdict 全集**零 lexicon_hit**——任何 lexicon_hit（B4 canary 接管：workflow=capability_runtime_v1/canary_stage 非空/low_end_summary 在场）=钩未达 agent=配置失败，py exit 4 INVALID。

## 运行矩阵（单 run 无重跑）

- 话术=「分析一下当前工程的低频关系」（B4 低频关系族 canonical 话术，b4 py 固定面同款；生产路径词表命中 B4→canary，本 run 钩下应落 LLM 轮）×5 轮（rounds-per-group=5，263 组配置先例；每轮 fresh conversation `b4_ablation_<stamp>_r<N>`）
- fixture=py 自建三轨（Sub Bass Tone 100Hz+两轨 3s 共享 fixture，b13b masking 场景同款低频遮蔽观察域）；**不预暖音频分析**（ablation 诚实面=模型自主决定是否触发分析链）
- 判定为机械分类器（跑前固化于 py `classify_ablation_round`，优先级：lexicon_hit > semantic_entry_intercepted > write_attempt > tool_chain_ok/weak_report/boundary_ok/no_route > error_*）

## 判据（卡面两分支，不预设；能力复现≠固定面绿）

- success = {tool_chain_ok（观察族工具自主执行+完成+实质报告）, boundary_ok（诚实边界话术零虚构）}；**successes ≥4/5 → 分支①：B4 判 A 态候选**（harness 自主复现成立，附证据）
- **semantic_entry_intercepted / 守卫拦截形态 → 分支②：归类「入口识别薄层」维持 F/H**（263 裁定②同款，如实记录拦截面，不判失败、不硬修凑过）
- write_attempt=红线如实记录（B4 只读观察面出现写工具）
- 弱报告（观察执行但回复<40 字）=weak_report 如实记录不算 success
- 组内止损：3 连同型非 success 中止，证据保全不再花轮（263 先例）

## 失败分类与止损

- py exit 4 = INVALID_ABLATION_HOOK（配置失败，零有效样本）→ 按环境中断处置：修配置后至多重跑 1 次（256/263 RUNBOOK 先例）
- py exit 3 = 环境中断（agent/栈不可达）→ 至多重建栈 1 次（卡面停止条件）
- 断言/判定失败取证不重跑；§8 单 run 无重跑；换算基准=263 回执 106 调用 ≈¥1.8-2.4

## 预算

≈¥2-4 上界（≤15 LLM 调用 deepseek-flash 预期：5 轮×semantic_entry 分类/循环轮；263 同引擎 106 调用 ≈¥1.8-2.4 换算）；telemetry 亲数 in/out 入回执

## 拆栈

ps1 完成态不自带 teardown → owner 亲拆 VitAgent/VitApp（本腿无 Godot/vsphub 参与面），TEARDOWN_ALL_ZERO 亲核（五端口 7878/5555/5556/8787/18787=0+UDP 4444/4445=0+三类进程=0+Godot=0）后释放登记
