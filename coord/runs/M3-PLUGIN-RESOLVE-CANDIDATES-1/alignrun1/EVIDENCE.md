# M3-PLUGIN-RESOLVE-CANDIDATES-1（236）真栈重验腿证据包——alignrun1

- run：`coord/runs/M3-PLUGIN-RESOLVE-CANDIDATES-1/alignrun1`（worktree `D:/Vit_DAW_wt_236impl` @ `port/m3-plugin-resolve-candidates-1`，实现 7781678a）
- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_236impl/scripts/dev_agent_smoke.ps1 -RepoRoot D:/Vit_DAW_wt_236impl -RunArtifactsDir <本目录> -StartKernel -RestartAgent -KernelExe D:/Vit_DAW_wt_228/.../VitApp.exe -AutomationLLMRouteSmoke`
- 退出码如实分列：**AutomationLLMRouteSmoke 场景腿 `[DONE] status=COMPLETED exit=0`**；**ps1 整体 exit 1**＝场景腿完成后的后续 Chat route smoke 腿 `Invoke-WebRequest 操作超时`（dev_agent_smoke.ps1:491，与组 1 判读无关的腿；235 同命令该腿通过，本轮 agent 在 95 次 LLM 调用后 HTTP 未及时响应，环境/时序类，非本卡功能面——本卡改动仅 error 披露文本）。按 §5 字面门槛（退出码 0）未满足，交主管裁定
- 栈：VitApp wt_228（只读复用，C++ 面零改动，pid 38452）+ 本 worktree 构建 VitAgent（pid 26260，含本卡披露面修复）；引擎=deepseek-flash（user config 亲核）——同码（唯一 diff=本卡目标 1/2）同 fixture 同引擎三同成立
- 占用登记 c32f5e70（登记前+启动前双复核进程/端口全零，push 后 ≤60s 即时复核全零）；拆栈 TEARDOWN_ALL_ZERO（VitAgent 26260/VitApp 38452 手动停，TCP 5555/5556/7878/8787/18787/5178=0，UDP 4444/4445=0）

## 组 1 门重判结果（判据三件，如实分列）

| 判据 | 结果 |
|---|---|
| no_route 消失 | **✗ 未达**：`[no_route, no_route, no_route]`（llm_samples=3），successes=0，r1-r3 三连同型止损，组 2/3 skipped |
| set_points 包络形态 | **✗ 未达**：三轮 `automation.set_points` 零调用（payload+rounds 全量核） |
| 候选恢复亲证 | **✓ 达成**：模型实收候选清单 error 后调用 `plugin.get_parameters {"plugin_id":"1011"}`（payload row32/33/35/36），回传 `status=ok`（row34/37）——卡面预期恢复路径（从候选选 Volume & Pan→按 id 重试）完整走通 |

## 三件套

### 1. 实收载荷（payload capture 亲证）

- 文件：`logs/agent_llm_payload_capture.jsonl`（95 行=95 次调用全量，与 telemetry 1:1）
- **候选清单实收亲证**：error 全文 `multiple visible plugins match: [1011 Volume & Pan Plugin, 1012 Level Meter]; specify plugin_id or plugin_name` 在模型 request 上下文回传在场（r2 内 4+ 处；`Volume & Pan Plugin, 1012 Level Meter` 候选串 4 处）——**本卡披露面修复生效且被模型消费**
- 教学段 needle 延续在场：`payload_guidance.rows_with_guidance=81/85`

### 2. 收敛数据

| 轮 | verdict | 形态 |
|---|---|---|
| g1_r1 | no_route | 2 exchanges，CCB 观察兜圈（未走到 plugin.get_parameters；reason 含"属于随时间变化的音量/自动化走向"=时变判别正确） |
| g1_r2 | no_route | **走到 plugin.get_parameters**（r2 rounds 内 13 处提及；payload 实际调用 8 次） |
| g1_r3 | no_route | 未再调 get_parameters，CCB 兜圈 |

- 引擎计费：deepseek-flash 95 调用，1,963,632 in（含 cache）/122,019 out
- probe view：setup 正常（双轨 deterministic clips、acoustic partial×2、ride track 1010 volume plugin 1011 setup 亲证）

### 3. 形态分型（r2 载荷逐调用链亲证）

1. 模型调 `plugin.get_parameters {"track_id":"1010"}`（reason："写入音量包络前必须先解析这条轨的音量参数标识与取值区间"——教学路径第二步）→ 收**新 error 含候选清单** `[1011 Volume & Pan Plugin, 1012 Level Meter]; specify plugin_id or plugin_name`
2. **模型从候选清单恢复**：下一调用 `{"plugin_id":"1011"}`（reason："解析该轨音量控制的可写参数标识与取值范围"）→ 回传 **status=ok**
3. **新确定性断点钉死（停止条件第一条触发）**：ok 回执的上下文快照 `"command_name":"get_plugin_parameters","status":"ok","summary":{}`——**参数面数据零披露**（row37 全 request 核：`parameter_id`/`parameters`/`value_range`/`min`/`max` 全 0 计数；仅有的 `param_id`×4/`gain_db`×2 全部来自 system 规则文本非结果数据）
4. 模型拿不到 param_id/取值范围 → 按教学段纪律拒写 set_points（"Do not output ... any plug-in parameter ID or value you have not resolved from returned evidence"）→ 部分回退裸调 track_id（再撞歧义）+ CCB processor.identity_and_controls 兜圈 → 结算 no_route

**断点机理锚点（本卡文件域外，上交不连修）**：`agent/internal/harness/harness.go:12879` `publicPluginParametersResult`——参数行仅在 `include_parameters`（或 `include_full_parameters`/`include_parameter_snapshot`）为 true 时组装进 `out["parameters"]`（:12910 门控）；模型调用（教学段未教该键）不带 → publicResult 只剩 parameter_count/quick_controls 元数据；且上下文快照 recent_observation 的 summary 为空回执，参数面整个不在模型可见面。恢复链（歧义→候选→plugin_id 重试→ok）已通，断点后移至成功路径的结果披露面。

## 分型定性（执行侧如实分列，裁定归主管）

1. **本卡目标 1（披露面修复）完全达成**：候选清单在场、模型实收、恢复路径走通（组1 三件之一 ✓）。
2. **组 1 门未过**：no_route×3 + set_points 零调用——但失败形态已与 235 质变：235=模型无从恢复（error 无候选）；本轮=模型**恢复成功**后拿不到参数面数据（ok 回执空壳）。确定性工具缺口后移一层：error 披露→结果披露。
3. **停止条件第一条触发**（"候选披露后出现新确定性断点（参数面不含预期键）→锚点上交，不折第三张小卡连修"）：锚点=harness.go:12879 include_parameters 门控+快照 summary 空。换引擎对该断点无效（任何引擎拿到 ok+summary{} 都无数据可写——同 235 对裸 error 的论证同构）。
4. ps1 整体 exit 1（无关腿超时）如实记录，不掩盖场景腿 COMPLETED 的事实，也不把场景腿完成说成整体 exit 0。

## 工件清单

- `logs/agent_llm_payload_capture.jsonl`（6.3M，95 调用全载荷——候选实收+恢复链+断点形态三面亲证源）
- `logs/agent_llm_telemetry.jsonl`（159K）+ `logs/agent_prompt_render.jsonl`（3.5M）
- `automation_llm_route_summary.json`（1.7M 含 exchanges）+ `rounds/`（g1_r1-r3+setup）+ 双 console + `ps1_exit.txt`（PS1_EXIT=1 如实在卷）
- `automation_llm_route_clip.wav` 排除入库（确定性可再生，同 235 先例）
