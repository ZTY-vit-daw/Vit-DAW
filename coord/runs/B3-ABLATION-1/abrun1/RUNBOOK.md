# B3-ABLATION-1 abrun1 真栈 RUNBOOK（§8 口径——跑前成文）

- 卡：B3-ABLATION-1（池序 275，A-F 第二拆 ablation）｜执行 owner=ZCode GLM-5.3 L2-worker PC 执行子代理（275 卡 L2 会话，续占）
- 被测：worktree `D:\Vit_DAW_wt_275impl` @ `port/b3-ablation-1`，HEAD=`b4d1da04`（实现提交：645b25da Go 钩缝+py ablation 面，b4d1da04 ps1 -B3AblationSmoke 腿；分支基线自 origin/main 319e922c 重建=merge 531ab4a6+0ccfa71a，含 281 -EduJamSmoke 腿/295 修/298 栈记录修正；C++/kernel 面零改动）；agent=worktree 现场构建（ps1 -RestartAgent 自建，SHA256 随 run 工件记）
- kernel：`D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe` SHA256 `F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A`（=225-298 同款只读复用，-KernelExe 显式）
- 引擎：`-LlmModelOverride deepseek-flash`（238-264 先例；264 同款注入）

## 完整命令

```
powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_275impl\scripts\dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_275impl -StartKernel -RestartAgent -B3AblationSmoke -LlmModelOverride deepseek-flash -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe
```

（264 同款形态：不带 -NoChatSmoke（默认 suite 与 B4 run 同构），工件落 worktree `VitApp\Workspace\Artifacts\smoke\b3_ablation_<stamp>\`，run 后拷 `coord/runs/B3-ABLATION-1/abrun1/`；ps1 console 全文留档。）

## 钩生效验证（三重）

1. ps1 console 行 `B3 ablation hook: VIT_PAN_LAYOUT_ABLATION=1`+`B3 ablation engine override: VIT_AGENT_LLM_MODEL=deepseek-flash`；
2. py summary `ablation_hook: true`；
3. 5 轮 verdict 全集**零 lexicon_hit**——任何 lexicon_hit（B3 canary/legacy 接管：workflow=capability_runtime_v1 或 pan_layout/canary_stage 非空/layout_summary 在场）=钩未达 agent=配置失败，py exit 4 INVALID；telemetry LLM 行佐证五轮为真实 LLM 轮。

## 运行矩阵（单 run 无重跑）

- 话术=**候选 A**「主唱留在中间，两把吉他分别放到左右两侧，帮我规划整个工程的声像布局」（预扫主选：词表命中可证+轨道指定/声像方向/目标布局/动作四要素密度=264 级；ps1 不传 --phrase=py 默认 B3_ABLATION_QUESTION 即候选 A，summary `phrase` 字段记录实际话术佐证）×5 轮（rounds=5，263/264 组配置先例；每轮 fresh conversation `b3_ablation_<stamp>_r<N>`）
- fixture=py 自建 7 轨（Lead Vocal/Guitar Double 1-2/Backing Vocal 1-2/Percussion 1-2，test_target_3s.wav 共享导入，264 py 自建先例）；**不预暖音频分析**（ablation 诚实面=模型自主决定是否触发观察链）
- 判定为机械分类器（跑前固化于 py `classify_ablation_round`，优先级：lexicon_hit > semantic_entry_intercepted > raw_write > tool_chain_ok/confirm_without_observation/advice_only/weak_report/boundary_ok/no_route > error_*）

## 判据（卡面口径，不预设）

- **判定线 = tool_chain_ok ≥4/5 且零 lexicon_hit → B3 判 A 态候选**（B3 A 面=观察族工具自主执行+实质布局建议（≥40 字）+征询确认收口形态；A 态终局确认归主管）
- **B3≠B4 写面红线**：B3 能力面含执行写面（mix.apply_pan_layout_batch）——单轮内任何裸写工具执行（含 .set/apply 前缀泛化规则）=raw_write 如实记录永不计 success（raw_write 优先于确认形态判定）；断言面以建议+确认形态收口，不断言裸执行；rounds 前后 mix pan/volume 快照 diff=零写入亲证
- boundary_ok 如实记录**不计 B3 A 线**（与 264 successes 含 boundary_ok 的口径差异已在预扫+py 注释声明；如主管改判可从 summary verdict_counts 直接复核无需重跑）
- semantic_entry_intercepted / 守卫拦截形态 → 归类「入口识别薄层」如实记录（264 §三方法学：入口命运由话术要素明确度决定；候选 A 预扫=四要素明确应放行）
- 组内止损：3 连同型非 tool_chain_ok 中止，证据保全不再花轮（263/264 先例）

## 失败分类与止损

- py exit 4 = INVALID_ABLATION_HOOK（lexicon_hit，零有效样本）→ 按环境中断处置：修配置后至多重跑 1 次（256/263/264 RUNBOOK 先例）
- py exit 3 / 栈不可达 = 环境中断 → 至多重建栈 1 次（卡面停止条件）；环境故障先记环境中断再定夺
- 断言/判定失败取证不重跑；§8 单 run 无重跑；真栈 run 连续两轮同型断言失败→止损取证上交（卡面停止条件）
- 换算基准=264 回执 16 调用 deepseek-flash ≈¥0.9-1.3

## 预算

≈¥1-3 上界（≤30 LLM 调用 deepseek-flash 预期：5 轮×[semantic_entry 分类+消息循环轮]，B3 轮含观察链工具循环或高于 B4 单轮 1 调用形态；264 同引擎 16 调用 ≈¥0.9-1.3 底线换算）；telemetry 亲数 in/out 入回执

## 拆栈

ps1 完成态不自带 teardown → owner 亲拆 VitAgent/VitApp（本腿无 Godot/vsphub 参与面），TEARDOWN_ALL_ZERO 亲核（六口 TCP 5555/5556/5557/7878/8787/18787 listen=0+UDP 4444/4445=0+VitApp/VitAgent/vsphub/Godot/python 进程全零）后释放登记
