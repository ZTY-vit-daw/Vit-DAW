# AF-D1-LOUDNESS-SETTLE-1 armrun2 补证腿回执（2026-11-05 晚窗，interim §三）

## 结论

**PS1_EXIT=1——D1 runner（非本卡面）再次失败，但形态≠排干超时：settle 链 2 分钟完整收敛（goal completed/stop=done），卡在 `D1 must contain exactly one forward mutation` 断言（模型动作数量分支）。引擎变量排除证据链闭合（两次 run 同引擎 deepseek-flash、两种不同失败形态、均不在本卡断言面）→ 按裁定 §三分叉：取证上交不重跑，D1 runner settle 链断言面转取证卡归主管。**

## Run 事实

| 项 | 值 |
|---|---|
| run stamp | 20261105_215624（21:56:24 栈起；登记 b8d892e1 后 ≤60s 即时复核净空 21:56:16 → 起栈） |
| 命令 | 同 armrun1 **去掉 `-LlmModelOverride`**（裁定字面），其余逐字不动（-PublicManifest 主检出 fixture/-PromptFlavor reverb/-ExpectDomain reverb/-LoudnessWatch/-D1LoudnessSettle/-SkipBuild/-KernelExe wt_228） |
| 被测 agent | `bin/VitAgent.exe` SHA256 `F321AF5E...`（与 armrun1 同一二进制，不重建；console `[lw] binary face` 亲证） |
| kernel | wt_228 `F9E3DA0A...`（同款复用亲证），pid 29668 |
| 默认引擎口径 | 跑前亲读本机持久化 config `~/.vit/config.json` **defaultModel=deepseek-flash**——去注入后引擎与本机默认同值（telemetry 本窗口 10 调用全 deepseek-flash 实证）；与 armrun1 显式注入同引擎，**引擎选择变量两次 run 恒等** |
| runner 结果 | **fail**：`D1 must contain exactly one forward mutation`（`d1_smoke_report.json` status=fail） |
| settle 链形态 | **完整收敛非超时**：goal `status=completed stop=done`（agent 日志 22:00:02 `[continuation.arm]`），executed=5 边界 / ccb_observations=4 / stored_receipts=0——链路健康，模型动作数量与 D1-S1「恰好一次 forward mutation」断言不匹配 |
| 本卡门 | LW 门与 D1 settle 门再次未执行到（脚本 LW 阶段前 throw） |
| 计费 | **10 调用全 deepseek-flash，0 error**，in 154,472 / out 13,743（telemetry 切片亲数）≈¥0.2-0.3 |
| 拆栈 | 脚本 throw 路径未自 teardown；owner 手动拆 VitAgent 37272/VitApp 29668/Godot 前端，**TEARDOWN_ALL_ZERO 亲核 22:01:13**（四端口 0+三进程 0+Godot 0+python 0） |

## 引擎变量排除论证（交主管转卡依据）

| | armrun1 | armrun2 |
|---|---|---|
| 引擎 | deepseek-flash（显式注入） | deepseek-flash（本机默认，无注入） |
| settle 链 | 未排干：41 轮×9-20s 晚高峰延迟，600s 窗口耗尽，limit_reached(timeout) 停泊 | 2 分钟收敛：goal completed/stop=done |
| 失败断言 | `did not drain within the smoke timeout` | `must contain exactly one forward mutation` |
| 本卡面 | LW/D1 settle 门未执行 | LW/D1 settle 门未执行 |

同引擎同 fixture 同话术两次 run 卡在 **D1 runner 两个不同断言**上（排干窗口×延迟交互 / mutation 计数×模型动作分支）——133 时代 D1 runner 断言面与模型行为分支的适配是系统性问题，与引擎选择无关（armrun2 无延迟问题仍失败，排除「延迟」作为唯一根因；armrun1 无 mutation 断言问题，排除「断言」作为唯一根因）。**转卡建议：D1 runner settle 链取证卡（排干窗口×轮次节奏×mutation 断言×模型分支四点复核，133 时代面）**。

## 工件清单（本目录）

- `console.log`——完整 PS console（栈起/binary face/LW 预配置/runner 失败 throw）
- `d1_smoke_report.json`——runner 终态（status=fail + `exactly one forward mutation`）
- `llm_telemetry_run_window.jsonl`——10 行 telemetry 切片（全 deepseek-flash）
- `agent_log_tail150.txt`——agent 日志尾（boundary executed=5 + continuation.arm completed/done 实证）
- `lw_*`/`loudness_watch_configure.json`——LW 预配置与二进制亲证小件
- 大件留盘 `D:\Vit_DAW_wt_254impl\artifacts\free_state_d1_s1\20261105_215624\` 可回指
