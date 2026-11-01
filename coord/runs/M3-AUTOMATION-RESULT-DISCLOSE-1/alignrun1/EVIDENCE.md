# M3-AUTOMATION-RESULT-DISCLOSE-1（237）真栈重验腿证据包——alignrun1

- run：`coord/runs/M3-AUTOMATION-RESULT-DISCLOSE-1/alignrun1`（worktree `D:/Vit_DAW_wt_237impl` @ `port/m3-automation-result-disclose-1`，实现 978d5dc9）
- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_237impl/scripts/dev_agent_smoke.ps1 -RepoRoot D:/Vit_DAW_wt_237impl -RunArtifactsDir <本目录> -StartKernel -RestartAgent -KernelExe D:/Vit_DAW_wt_228/VitApp/build_release/VitApp_artefacts/Release/VitApp.exe -AutomationLLMRouteSmoke`
- 退出码：**AutomationLLMRouteSmoke 场景腿 `[DONE] status=COMPLETED exit=0`；ps1 整体 exit 0（dev agent smoke completed）**——236 记录的无关腿超时**未复现**（Chat route smoke 本轮通过：improvement_proposal_confirmation_required 正常路由）。§5 门槛达标（场景腿 COMPLETED exit 0+工件在场+栈登记/释放全流程）。
- 栈：VitApp wt_228 f9e3da0a 只读复用（C++ 面零改动，pid 26928）+ 本 worktree 构建 VitAgent（pid 4792，含本卡双层修复）；引擎=deepseek-flash（user config 亲核）——同码（唯一 diff=本卡目标 1/2/3）同 fixture 同引擎三同成立（对照=236 alignrun1）。
- 占用登记 c90e64ec（登记前 fetch 重读 main=03d257a3 无他人占用+进程/端口亲核全零；push 后 ≤60s 即时复核全零）；拆栈 TEARDOWN_ALL_ZERO（VitAgent 4792/VitApp 26928 owner 手动停，PROCS=NONE，TCP 5555/5556/7878/8787/18787/5178=0，UDP 4444/4445=0）。

## 组 1 门重判结果（判据三件，如实分列）

| 判据 | 结果 |
|---|---|
| no_route 消失 | **✗ 未达**：`[no_route, no_route, no_route]`（llm_samples=3），successes=0，三连同型止损；组 2/3 skipped |
| set_points 包络形态 | **✗ 未达**：三轮 `automation.set_points` 零调用（payload 91 行全量核，rounds+payload 双面） |
| 参数面可见亲证 | **✓ 达成**：row54（r3 会话）模型 request 实收 `automation_param_surface` 完整参数行——本卡双层修复在真实栈生效且到达模型 |

## 链路进度（本卡目标 1/2/3）——完全达成

1. **层 2 供数面生效**：harness out 带 ungated 紧凑键 `automation_param_surface`（volume `{min:0,max:1,normalized_role:common_gain,supports_automation:true}` / pan `{min:-1,max:1,normalized_role:common_pan,supports_automation:true}`，label 在场）。
2. **层 1 披露面生效**：`recent_observation.summary` 从 236 断点形态 `"summary":{}` 变为 `"summary":{"automation_param_surface":[volume,pan],"parameter_count":2,"status":"ok","track_id":"1010"}`——参数面整个进入模型可见面（row54 全 request 亲证）。
3. **新测量面**：`payload_param_surface={"rows":86,"rows_with_param_surface":1}`（本卡 smoke py 微扩，measurement only）——与 ok 回执 1:1（唯一一次 ok 回执即唯一一次实收）。
4. 教学段 needle 延续在场：`payload_guidance.rows_with_guidance=81/86`。

## 最终验收（组 1 门）——未过

三轮 verdict 全 no_route；`automation.set_points` 零调用。组 1 判据三件只达成一件（参数面可见亲证）。

## 三件套（停止条件第一条触发的纯模型残差证据包）

### 1. 实收载荷（payload capture 亲证，91 行全量）

- 文件：`logs/agent_llm_payload_capture.jsonl`（86 计费调用+5 非计费行，与 telemetry 计费面 1:1 对齐）
- **参数面实收亲证（row54）**：`recent_observation` 携带 `automation_param_surface` 完整两行（volume/pan 各带 min/max/label/normalized_role/supports_automation）+`parameter_count:2`+`status:"ok"`——**236 钉死的"ok 回执空壳 summary{}"断点已修通**。
- 歧义→候选→恢复链延续在场（与 236 一致）：track 级调用撞 `multiple visible plugins match: [1011 Volume & Pan Plugin, 1012 Level Meter]` 歧义 error（2 行在场），模型下一调用即 `{"plugin_id":"1011"}` 恢复成功。

### 2. 收敛数据

| 轮 | verdict | 形态 |
|---|---|---|
| g1_r1 | no_route | 纯 CCB 兜圈（observation_request/catalog），未碰 automation 面 |
| g1_r2 | no_route | 纯 CCB 兜圈，未碰 automation 面（与 236 r2 走到 get_parameters 不同——LLM 分支随机性） |
| g1_r3 | no_route | **走到 plugin.get_parameters**：歧义→候选恢复（plugin_id=1011）→**ok+参数面实收（row54）**→仍 no_route |

- 引擎计费：deepseek-flash 86 调用，1,291,342 in（含 cache）/116,112 out。
- setup 正常：ride track 1010 volume plugin 1011（console 亲证）+双轨 deterministic clips。

### 3. 形态分型（r3 载荷逐调用链亲证，rows 47-85）

1. row51：模型调 `plugin.get_parameters {"track_id":"1010","plugin_type":"volume"}`（教学段路径）→ 撞 track 级歧义 error（候选清单在场）。
2. row52-53：模型从候选清单恢复 `{"plugin_id":"1011"}` 重试 → **status=ok**。
3. **row54：模型实收参数面**（volume/pan 完整值域行）——但模型 response 判定"**缺少可自动化的音量控制标识与取值范围的权威披露**"，继续请求 `ccb.observation_request view_ids=[processor.identity_and_controls]`。
4. row55-84：纯 CCB 观察兜圈 ~30 次（processor.identity_and_controls/track.time_dynamics 等反复请求），**再未调用任何 automation 工具**，set_points 零调用 → 结算 no_route。
5. row68/69：模型曾再试 track 级 `{"track_id":"1010","plugin_type":"volume"}`（回执被同轮 CCB 观察覆盖，参数面未再现）→ 继续兜圈至轮终。

**残差定性**：参数面证据（param_id+min/max+supports_automation=true）已在模型 request 上下文在场（row54 亲证），模型仍宣称"缺少权威披露"并转 CCB 视图无限兜圈——**指引（教学段 81/86 在场）+工具面（参数面实收）双全下的纯模型行为残差**。与 236 的"ok 回执空壳"（确定性工具缺口）质变：本轮工具面缺口已闭合，断点后移至模型对在野证据的采信/收敛行为。

## 停止条件判定（卡面第一条）

**触发**：参数面可见（载荷亲证模型实收含参数行 ✓ row54）+ 三连同型止损（no_route×3 ✓）→ **纯模型残差证据包上交（本文件三件套），转主管换引擎裁定，不再折第三张披露卡**。

- ps1 无关腿超时：未复现（本轮 exit 0）——不触发 flaky 卡建议。
- 既有 mix/CCB summary 披露回归：无（全量 go test ./internal/... exit 0+零回退单测在卷）。
- 新确定性断点：无（模型拿到了完整参数面；不采信属行为面非工具面）。

## 工件清单

- `logs/agent_llm_payload_capture.jsonl`（参数面实收 row54+歧义恢复链+兜圈形态三面亲证源）
- `logs/agent_llm_telemetry.jsonl` + `logs/agent_prompt_render.jsonl`
- `automation_llm_route_summary.json`（含 exchanges+`payload_param_surface` 新测量面）+ `rounds/`（g1_r1-r3+setup）+ `automation_llm_route_console.txt` + `ps1_exit.txt`（PS1_EXIT=0 如实在卷）
- `automation_llm_route_clip.wav` 排除入库（确定性可再生，同 235/236 先例）
