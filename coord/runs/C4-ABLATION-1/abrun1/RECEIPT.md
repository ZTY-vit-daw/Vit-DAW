# C4-ABLATION-1 abrun1 真栈回执（执行侧自验）

- run ID：`automation_llm_route_20261106_104227`（run 目录=worktree `D:\Vit_DAW_wt_263impl\VitApp\Workspace\Artifacts\smoke\automation_llm_route_20261106_104227\`，大件留此不入库）
- 被测 HEAD：`port/c4-ablation-1` @ `e9807664`（实现 fe368796：Go 缝 2 文件 20 行 diff+配对单测+py/ps1 ablation 参数；零 C++/kernel 面改动）
- 启动：10:42:27（占用登记 fd9af956 推送+远端 owner 重读后，≤60s 即时复核 10:42:11 全零，随即启动）；完成 10:56:29（13m48s）
- 完整命令：

```
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_263impl -StartKernel -RestartAgent -AutomationLLMRouteSmoke -LlmRouteGroups "1,3" -LlmRouteAblationHook -LlmModelOverride deepseek-flash -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe
```

- kernel：`D:\Vit_DAW_wt_228\...\VitApp.exe` SHA256 `F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A`（10:40 亲核，225/253/254 同款）
- 退出码：py 腿 `[DONE] status=COMPLETED exit=0`；**ps1 最终 `PS1_EXIT=0`**——§5 真栈执行门槛过（判定面=模型行为判定，见判定报告）
- §8 单 run 无重跑：兑现（组 3 止损后不花 4-5 轮=止损线设计内，非重跑）

## 判定面（详版见 coord/reports/2026-11-06-c4-ablation-1-ablation-verdict.md）

- **组 1（回归面）**：`[tool_chain_ok×4, no_route×1]` successes=4 **group_pass=true**——钩不改变词表缺失话术的纯 LLM 路径，与 253（4/5）/225（5/5）稳定形态一致；r4 no_route=limit_reached 续跑未终局（模型行为样本，同 253 r1 族）
- **组 3（ablation 目标面）**：`[no_route×3]` 3 连同型止损 **group_pass=false**——三轮逐字同型 `semantic_entry_unresolved`+确定性模板（semantic_entry.go:395），conf 0.22/0.22/0.32 全部 <0.50 下限，**251 形态复现**；机制=词表拆除后裸时间窗话术被仲裁器 unresolved 拦截，message loop 零进入
- **判定：≥4/5×两组不成立 → C4 维持 H 态（A 态=0 不变）；缺口=semantic_entry 准入面（非 harness 层——组 1 同栈亲证指引/工具/确认链齐备）**；处置三选项交主管（仲裁器扩粒度卡/词表重归类入口识别薄层/组 3 话术族判据重审）

## 钩生效三重验证

1. ps1 console `automation LLM route ablation hook: VIT_AUTOMATION_INTENT_ABLATION=1`；
2. summary `ablation_hook=true`；
3. 8 轮 verdict 全集零 lexicon_hit（distribution={tool_chain_ok:4, no_route:4}）

## 计费

106 调用全 deepseek-flash，in 1,486,923 / out 114,145 ≈ ¥1.8-2.4（基准=253/225 回执口径），卡面预算 ≈¥2-4 内

## 拆栈与释放

owner 亲拆 VitAgent 46848 / VitApp 42628；**TEARDOWN_ALL_ZERO 亲核 10:59:56**：五端口（7878/5555/5556/8787/18787）listen=0+VitApp/VitAgent/vsphub=0+Godot/python=0+UDP 4444/4445=0；释放登记随本批推 main

## 本目录工件（分支 port/c4-ablation-1）

RUNBOOK.md（§8 跑前成文）/ automation_llm_route_summary_slim.json / agent_llm_telemetry.jsonl（164KB）/ automation_llm_route_console.txt / dev_agent_smoke_console.txt / g3_r1-3.json（全件，unresolved 证据）/ g1_r4_slim.json（no_route 对照，全件留 worktree）；大件留 worktree：payload capture 6.5MB / prompt_render 3.7MB / summary 全量 3.0MB / rounds/ 2.9MB

## 端测覆盖边界声明

本卡改动=chat 词表武装入口+冒测脚本参数；真栈腿覆盖 -LlmRouteGroups "1,3" 全判定面（含钩生效三重验证）；**组 2 未重跑**（249 判别定案维持，钩对其路径零影响由单测「帮我把空间铺开」不武装负例守住——组 2 话术本就 vocabulary-missing 走 LLM 路径，与组 1 同面）；组 4 与钩结构性不兼容（py 前置拒绝，已在判定报告记录）；webui 渲染面/用户旅程不在本卡域
