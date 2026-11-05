# M3-TIMEWINDOW-INTENT-ARM-1 armrun1 真栈回执（执行侧自验）

- run ID：`automation_llm_route_20261105_184935`（run 目录=worktree `D:\Vit_DAW_wt_253impl\VitApp\Workspace\Artifacts\smoke\automation_llm_route_20261105_184935\`，大件留此不入库）
- 被测 HEAD：`port/m3-timewindow-intent-arm-1` @ `0593e05a`（基线 c7c22e43；本卡实现唯一 diff=`agent/internal/chat/automation_intent.go` +8/-1 与 `automation_intent_test.go` +35，2 files changed 亲核；零 C++/kernel 面改动）
- 启动：18:49:35（占用登记 f2719977 推送+远端 owner 重读 18:49:00 后，≤60 秒即时复核 PROCS=0+TCP=0 于 18:49:01，随即启动）
- 完整命令：

```
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_253impl -StartKernel -RestartAgent -AutomationLLMRouteSmoke -LlmRouteGroups "1,3" -LlmModelOverride deepseek-flash -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe
```

- kernel：`D:\Vit_DAW_wt_228\...\VitApp.exe` SHA256 `F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A`（18:45 亲核，前缀 f9e3da0a 匹配卡面）
- 退出码：python 腿 `[DONE] status=COMPLETED exit=0`（finished 11:03:35Z=19:03:35 +0800）；**ps1 最终 `PS1_EXIT=0`**——§5 真栈交付门槛过
- §8 单 run 无重跑：一次收口，无止损触发，无环境中断
- Strip Silence 前置腿：全绿零 warn（needs_confirmation→确认→completed「已删除 3 个静音区域」，与 251 健康形态一致）
- 计费：90 调用全 deepseek-flash，in 1,376,092 / out 121,208（telemetry 覆盖 ps1 全程各腿，非仅 LLM route 腿）；≈¥1.7-2.3（换算基准=251 回执口径 9,079 in+3,452 out≈¥0.01-0.02），在卡面预算 ≈¥2-3 内

## 组 1 回归证据（首要红线：绿）

- `llm_samples=5`，verdicts=`[no_route, tool_chain_ok, tool_chain_ok, tool_chain_ok, tool_chain_ok]`，successes=4，**group_pass=true**，stop_loss=null，lexicon_overflow=false，**choreo_pending_leg=pending_intact**
- r1 no_route=首轮 goal failed 无写无卡无 capability 面（模型行为样本如实记录，不凑通过）；r2-r5 均 confirmed write reached kernel（points=4）；r5 在 choreo-pending 窗口内完成且 pending 完好
- 结论：组 1 稳定绿未回退（≥4/5 组判据过，与既往稳定形态一致）

## 组 3 逐轮 verdict（本卡目标：达成）

| 轮 | verdict | counts_as_llm_sample | stop_reason | workflow |
|---|---|---|---|---|
| r1 | lexicon_hit | false | automation_choreo_clarify_category | section_automation_choreo |
| r2 | lexicon_hit | false | automation_choreo_clarify_category | section_automation_choreo |
| r3 | lexicon_hit | false | automation_choreo_clarify_category | section_automation_choreo |
| r4 | lexicon_hit | false | automation_choreo_clarify_category | section_automation_choreo |
| r5 | lexicon_hit | false | automation_choreo_clarify_category | section_automation_choreo |

- successes=5/5，**group_pass=true**，choreo_pending_leg=pending_intact；verdict_distribution={no_route:1, tool_chain_ok:4, lexicon_hit:5}
- 词表补词（秒之后/秒后+动起来）后「30 秒之后动起来」首轮即确定性武装走既有澄清卡（category 空问参数，不猜），零 LLM 判定轮——与卡面预期形态逐字一致；251 时代全仲裁 unresolved 形态（12/12 模板回复）消失

## payload needle 面（未回退）

- payload_guidance / payload_automation_surfaces / payload_param_surface 全 `available=true`；`prompt_guidance_rendered=true`；capture 文件=worktree run 目录 `logs/agent_llm_payload_capture.jsonl`（6.5MB，留 worktree）

## 拆栈与释放

- owner 手拆 VitAgent(21444)/VitApp(31936) 后 **TEARDOWN_ALL_ZERO 亲核 19:09:05**：PROCS=0+TCP(7878/5555/5556/8787/18787)=0+UDP(4444/4445)=0；释放登记随卡面状态变更同批推 main

## 本地验证（实现提交 0593e05a 前）

- `go build ./...` OK；`go test ./internal/chat -count=1` 全绿（127.152s，存量零回退）；`TestAutomationIntentTimeWindowArming`+`TestAutomationIntentArmingBoundary` verbose PASS（正 3/负 5 按卡面清单，含组 2 基线「帮我把空间铺开」不武装、静态正例「把这条轨存在感调强一点」不变、裸跨度「30 秒」/「别动」/「等 30 秒再说」负例）

## 工件与端测覆盖边界

- 本目录（分支=`port/m3-timewindow-intent-arm-1`，随实现分支同批入库）：`automation_llm_route_summary_slim.json`（7.6KB）、`agent_llm_telemetry.jsonl`（154KB）、`automation_llm_route_console.txt`（python 腿）、`dev_agent_smoke_console.txt`（ps1 全程）
- 留 worktree 大件：payload capture 6.5MB / prompt_render 3.6MB / summary 全量 3.2MB / rounds/ 2.9MB / project.vit；`automation_llm_route_clip.wav` 已删（卡面收口要求）
- 端测覆盖边界声明：本卡改动=确定性词表路由面（chat 包），真栈腿覆盖 Groups 1,3 全判定面；**组 2 话术未在本 run 重跑**——组 2 判别基线维持 251/249 结论，本卡以单测负例守住其入口不变（「帮我把空间铺开」不武装，词表只增不改删）；webui 渲染面/用户旅程不在本卡域

## 需主管裁量事项

- 组 1 r1 no_route 为模型行为样本（本轮 goal failed 形态），组判据内不构成红线；如需归因可从 rounds/g1_r1.json（worktree）取证，不在本卡扩域
