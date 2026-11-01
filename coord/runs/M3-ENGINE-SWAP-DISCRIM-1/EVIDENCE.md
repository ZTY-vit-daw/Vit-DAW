# M3-ENGINE-SWAP-DISCRIM-1（238）证据包——换引擎判别实验（flash→v4-pro，同码同 fixture 只换引擎）

- run：`coord/runs/M3-ENGINE-SWAP-DISCRIM-1/alignrun1`（worktree `D:/Vit_DAW_wt_238impl` @ `port/m3-engine-swap-discrim-1`，基线=main HEAD 24374340 原样，agent 生产代码零改动）
- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_238impl/scripts/dev_agent_smoke.ps1 -RepoRoot D:/Vit_DAW_wt_238impl -RunArtifactsDir <本目录> -StartKernel -RestartAgent -KernelExe D:/Vit_DAW_wt_228/VitApp/build_release/VitApp_artefacts/Release/VitApp.exe -AutomationLLMRouteSmoke -LlmModelOverride deepseek-v4-pro`
- 退出码：**AutomationLLMRouteSmoke 场景腿 `[DONE] status=COMPLETED exit=0`；ps1 整体 exit 1**——失败腿=无关腿 Chat route smoke（`dev_agent_smoke.ps1:501` Invoke-WebRequest 超时；其 2 次 LLM 调用全撞 60s timeout）＝**卡面停止条件 4 命中**（env 注入造成非引擎面行为差异）：如实分列，**判读只取 LLM 路腿**（236 alignrun1 无关腿超时同族先例）。
- 引擎注入亲证：ps1 console `engine override: VIT_AGENT_LLM_MODEL=deepseek-v4-pro`（env 块行）+ telemetry/payload 每行 `model=deepseek-v4-pro` + summary `usage.models={"deepseek-v4-pro":...}`——**用户 config 零触碰**（defaultModel=deepseek-flash 只读亲核在卷，run 后未复核到任何变更源）。
- 三同成立：同码（agent=main HEAD 原样，唯一 diff=ps1 +20 env 注入块）/同 fixture（AutomationLLMRouteSmoke 组1 同卡矩阵同 setup：ride track 1010 volume plugin 1011+deterministic clips，console 亲证）/同栈（kernel wt_228 VitApp.exe f9e3da0a 只读复用 235/236/237 同款二进制）——对照=237 alignrun1（flash 基线）。
- 栈：占用登记 eb4b30d6（登记前复核全零；push 后 ≤60s 即时复核全零 21:06:46）；拆栈 TEARDOWN_ALL_ZERO（VitAgent 40708/VitApp 27324 owner 手动停，PROCS=NONE，TCP 5555/5556/7878/8787/18787/5178=0，UDP 4444/4445=0）。

## 预检腿产物（详见 precheck/PRECHECK.md）

- `/models`：账号可用 id=`deepseek-flash`、`deepseek-v4-pro`；**`deepseek-v4` 非有效 id**→按卡面采用 `deepseek-v4-pro`（单调用探针 200/echo 亲证）
- 单价（官方 pricing 页 2026-11-01）：v4-pro in 未命中 ¥3/M（flash ¥1/M）、out ¥6/M（flash ¥2/M）、cache 命中 ¥0.025/M（flash ¥0.02/M）——**约 3×**

## 三面对照表（flash=237 alignrun1 vs v4-pro=238 alignrun1）

### 面 1：组 1 verdicts

| | flash（237） | v4-pro（238） |
|---|---|---|
| verdicts（g1） | [no_route, no_route, no_route] | [no_route, no_route, no_route] |
| successes | 0（stop_loss no_route×3） | 0（stop_loss no_route×3） |
| verdict_detail 同型性 | 三轮各异（兜圈/兜圈/参数面后兜圈） | **三轮逐字同型**：「本轮因 LLM 服务临时错误暂停，可以继续恢复尚未完成的判断」 |

### 面 2：逐轮收敛数据（telemetry 会话归组口径：ok+err）

| 轮 | flash 调用/超时 | v4-pro 调用/超时 | flash turns/obs/limit_reached | v4-pro turns/obs/limit_reached |
|---|---|---|---|---|
| g1_r1 | 23 / **0** | 12 = 7 ok + **5 timeout** | 22 / 12 / true | 4 / 3 / **None（超时暂停型）** |
| g1_r2 | 23 / **0** | 8 = 4 ok + **4 timeout** | 22 / 13 / true | 2 / 3 / **None** |
| g1_r3 | 39 / **0** | 8 = 5 ok + **3 timeout** | 37 / 14 / true | 5 / 3 / **None** |
| 腿合计 | 85 / **0**（+setup=86） | 28 = 16 ok + **12 timeout（43%）** | — | — |
| 无关腿 | 91 行全量 0 错误 | strip 3 ok；**chat route 2 行全超时→ps1 exit 1** | — | — |

- 调用延迟（telemetry http_ms，OK 行）：flash 中位 7.4s / p90 14.8s / max 34.2s；**v4-pro 中位 29.3s / p90 44.4s / max 44.5s**——分布尾持续贴近并越过 60s client timeout 硬线（`agent/internal/llm/client.go:171` 硬编码默认，**无 env 覆盖路径**，改=改生产代码=卡面红线）。
- 教学段 needle：flash 81/86 行在场；v4-pro 20/24（summary 快照口径）——指引面两引擎均在场。

### 面 3：形态分型（payload capture 逐调用链亲证）

| | flash（237） | v4-pro（238） |
|---|---|---|
| r1 | 纯 CCB 兜圈（observation_request/catalog）至 limit_reached | needs_observation 推进：意图识别正确（"随时间变化的响度走向"）→锁定 Track 1010→「确认参数表面」→ **3 次 60s 超时打断（response 空）**→暂停收尾 |
| r2 | 纯 CCB 兜圈至 limit_reached | 同型：needs_observation→「解析可写自动化的处理器参数面」→**超时打断**→暂停收尾 |
| r3 | 歧义→候选恢复（plugin_id=1011）→get_parameters ok→**参数面实收（row54）**→仍宣称"缺权威披露"→CCB 兜圈 ~30 次→limit | needs_observation→**行21「需要先确认这条轨道上的音量插件身份与参数范围」（参数面意识出现）→行22 即超时打断**→暂停收尾；r1 尾部一次收尾调用 status=capability_blocked（"无法提供音量参数表面"——超时打断观察链后的收尾，与 flash 的"完整观察后拒绝"因果链不同） |
| 工具面 | r3 到达 plugin.get_parameters | **三轮 0 次 plugin/automation 工具调用**（tools_called 仅 ccb.observation_request/catalog） |
| param_surface 实收 | 1/86（row54 亲证） | **0/24（模型从未收到参数面——非不采信，是未到达）** |

## 判别结论：两支均不成立——引擎面超时环境中断主导，判别未获结论，上交主管

**A 支不成立**：无收敛、无提案、set_points 零调用、无任何 automation 工具面行为。
**B 支同样不成立**：B 支的前提是"v4 持有在野参数面仍无视"——本轮 v4-pro **从未到达 get_parameters ok**（param_surface 0/24），不是行为拒绝而是**推理链被 60s 超时反复截断**（12/28 超时，打断点恰在关键推理步：r3 行21 参数面意识→行22 超时）。

按卡面停止条件归属：**条件 2（引擎面栈错误-超时类）主导**——v4-pro reasoning 时延（OK 行中位 29.3s+越线分布尾）与生产栈硬编码 60s client timeout 物理不兼容；**条件 4（无关腿异常红）并发命中**（chat route 腿 2/2 超时→ps1 整体 exit 1）。两者均为"如实分类记录、不消耗判别结论"类——**本卡判别面未获 A/B 结论，证据包上交主管**。

### 上交主管的决策面（非执行侧越权项）

1. **timeout 适配前置**：llm/client.go:171 的 60s 默认对 reasoning 引擎不足；完成判别需先有 timeout 适配（env override 或按模型配置的代码卡——本卡红线禁改生产代码，未动）。
2. 适配后重跑判别的预算：v4-pro OK 行计费口径 242,780 in/33,280 out ≈ **¥0.93 上界**（全未命中；cache 命中更低）——量级与 237 flash（≈¥1.5）同档，不构成预算障碍。
3. 有限行为面观察（样本 16 OK 调用，太薄，**不作为 B 支结论**）：v4 有效调用推理方向正确（意图分类/轨道锁定/参数面意识均对），未见 flash 式"纯 CCB 兜圈至 limit"形态——存在"引擎档位可能足够但被栈截断"的迹象，支持先做 timeout 适配再判别，而非直接判 v4 无效。

## §8 计费披露（v4-pro 单价对照 flash）

- 计费行（telemetry OK 行，全 run 含无关腿）：19 行 242,780 in / 33,280 out。
- 单价：v4-pro in 未命中 ¥3/M、out ¥6/M → **≈¥0.93 上界**；对照 flash 同口径单价约 ¥0.27（3× 价差如实披露）。
- **超时行服务端计费不可核实**：14 行 60s timeout 客户端中止无 usage 回执，服务端已发生的 reasoning tokens 无法从客户端核实（如实标注，未计入上述估算）。
- 概率性运行声明（§8）：本 run=单次存在性实验；组1 三连同型止损即止损（卡面），未开第二 run。

## 工件清单

- `precheck/PRECHECK.md`（模型 id+单价+探针）
- `alignrun1/logs/agent_llm_telemetry.jsonl`（33 行终态：19 ok+14 timeout，model 字段逐行 v4-pro）
- `alignrun1/logs/agent_llm_payload_capture.jsonl`（33 行：三轮决策链+RAW 空=超时打断形态亲证）
- `alignrun1/logs/agent_prompt_render.jsonl` + `automation_llm_route_summary.json`（usage.models 引擎亲证）+ `rounds/`（g1_r1-r3+setup）+ `automation_llm_route_console.txt` + `ps1_exit.txt`（PS1_EXIT=1+腿分列如实）
- ps1 全程 console：后台任务日志（driver_console 副本随工件入库）
