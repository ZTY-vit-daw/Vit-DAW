# M3-AUTOMATION-SURFACE-PIN-1 真栈重跑判读（pinrun1）

- run：pinrun1 单轮（HEAD=384fc14a，唯一 diff=本卡目标 1-4），同 fixture 同引擎 **deepseek-flash**（-LlmModelOverride env 注入，用户 config 零触碰），kernel=wt_228 VitApp.exe SHA256 前缀 **F9E3DA0A**（235-240 同款只读复用），栈=dev_agent_smoke.ps1 -StartKernel -RestartAgent -AutomationLLMRouteSmoke，ps1 exit 0（dev agent smoke completed+场景腿 COMPLETED）。
- 引擎三面亲证：driver_console env 行 `VIT_AGENT_LLM_MODEL=deepseek-flash`；telemetry 82 行逐行 model=deepseek-flash；payload_capture 每行 model=deepseek-flash。

## 组 1 判据：门未过（no_route×3 三连）——但**新真断点成立**（卡面「判定成功=组 1 绿或新真断点」第二形态）

| 样本 | verdict | 最终 reply 形态 |
|---|---|---|
| g1_r1 | no_route | settled（limit_reached→continuation） |
| g1_r2 | no_route | settled |
| g1_r3 | no_route | settled（limit_reached→automatic_continuation_already_scheduled） |

### 形态质变于 240（残差定性**不成立**，卡面预期段「模型仍不走向 set_points 工具链」被推翻）

1. **册行被模型真实消费**：g1_r3 最终自由态决策 summary 原文「目标是随时间变化的突出度提升，需要先确认轨道音量参数的当前值与可写范围，才能规划一段渐强的音量包络；**目前只有参数范围信息，还缺当前值**」——模型申报从 240 的「缺标识/缺区间」变为「**已认参数范围，只缺当前值**」（标识+区间已入模，来自册行与归因回执）。
2. **模型走向 automation 工具链**：g1_r1 与 g1_r3 均调用 `automation.get_curve(volume)`（240「不走向 set_points 工具链」形态消失）；240 的 processor.* 同集重试零发生（denial=0 行载荷，guard 无触发面）。
3. **新真断点**：`automation.get_curve(volume)` 回执 ok 但 **point_count=0**（无自动化点=无曲线当前值）；模型要的「当前值」是**静态参数值**——内核 get_plugin_parameters 的 parameters 行**携带** value/value_text/normalized_value（PluginRackControlService.cpp:741-743 buildTrackLevelParameterDescriptors 亲核），但 harness `compactAutomationParamSurfaceRows`（automation_param_surface 行，模型唯一 ungated 参数面）只投影 param_id/label/min/max/unit/normalized_role/supports_automation——**当前值在投影层被丢弃**；include_parameters 全量面有 value 但需 include_* 门控键（骨架未教）。断点=**写前基线（当前值）在 ungated automation 面缺格**。
4. 修复方向（记入卡回执供主管裁定，不开卡自修——harness 面+237 键集红线属决策侧）：compactAutomationParamSurfaceRows 行 add-only 增 value/value_text 两键（既有键字节不变）。

## 载荷四件亲证

1. **automation_surfaces 册行在场**：38/82 行载荷携带（每个样本观察循环内从入册轮常驻至样本尾；样本边界=exchange 生命周期重置，与 240 unavailable_view_sets 同生命周期）；册行带 plugin_id=**1011**（实测值）、track_id=1010、params volume[0..1]/pan[-1..1]、project_revision=5。
2. **归因键在场**：34/82 行载荷携带 plugin.get_parameters ok 回执 summary 的 plugin_id（240 全程 0 行——丢失层 preFamily 投影删除已被 restoreFreeStateAutomationAttribution 修复，载荷实证）。
3. **note 引册**：`automation_surfaces in hand` 引用句 0 行触发——**guard 零触发（denial=0）的自然结果**（模型不再请求 processor.* 不可用集，无 denial 面即无引用句面）；引用句两态由单测 TestFreeStateUnavailableRetryIssueCitesPinnedLedger 锁定；base note 常量零字节改动（git diff 亲证 freeStateCCBProcessorRedirectBaseNote 无 diff）。
4. **护栏零回退**：240 单测 8 件原语义全绿（in-hand 断言按目标 3 授权同步为读册面）；拦截红线（同集拒/异集放行/revision 前进放行/拒绝零观察物化）测试锁定；本轮真栈 guard 零触发=模型行为面改善，非护栏失效。

## 边界与已知限制（诚实披露）

- 册的跨 exchange 持久性：chat 侧 workflow_data 的 loop 持久化面未搬运 automation_surfaces（final loop ledger ABSENT）——样本内（agentloop 内存面）全程有效，跨样本各自重新观察；240 unavailable_view_sets 同生命周期，非本卡回归。若主管裁定需要跨 exchange 持久，属 chat 持久化白名单扩展（新卡）。
- 修复轮（final_gate 重试）与 settled 后收尾轮的 user 消息不带 ledger JSON 组装面（len=213 纯 gate/新 goal 面）——册在场的准确口径=「样本观察循环内每轮注入」，非全部 82 行。

## §8 计费披露（flash ¥1/M in、¥2/M out 上界口径）

- 单轮 77 计费调用，in 1,176,938 / out 138,059（telemetry aggregate_usage，deepseek-flash 77 calls）≈ **¥1.45 上界**（含 cache_read 部分按全价上界计；实际低于此值）。卡面预算 1-3 轮 ¥1.5-2/轮之内。
- 单 run 存在性判别：无重跑（判定成功=新真断点，第一轮即收口）。

## 工件清单

- 入库（随 port/m3-automation-surface-pin-1）：driver_console/automation_llm_route_console/summary/project.vit/default_project.xml/gui_project_isolation.json/ps1_exit=0/telemetry（82 行）/payload_capture（82 行 5.4M）/rounds（g1_r1/r2/r3+setup_status）/本 RECEIPT.md。
- 不入库留 worktree：agent_prompt_render.jsonl（2.9M 渲染面与 payload 重复度高，240 先例）；.vit_agent/.vit_derived/.vit_history 运行态目录+clip.wav 已删（240 瑕疵教训排除条目）。
