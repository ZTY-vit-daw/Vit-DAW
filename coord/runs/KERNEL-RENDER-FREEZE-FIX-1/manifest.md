# KERNEL-RENDER-FREEZE-FIX-1 真栈运行清单（§9 回执契约）

- 执行：PC 执行侧值班会话，2026-10-06 深夜；领取时 origin/main `97af10a9`，分支 `port/kernel-render-freeze-fix-1`
- 被测内核：`Export/staging/runtime/VitApp.exe`，sha256 `b6565dcf85d1da867e16333052a7b928a5413565940603adbcf4abd59dd3123b`（本卡构建产物 `VitApp/build/VitApp_artefacts/Release/VitApp.exe` 同哈希；部署前泊位内核=`959c06cf…2bd7c9`，即 KERNEL-RENDER-FREEZE-1 取证时被测构建）
- 构建命令：`cmake --build build --config Release --target VitApp`（VitApp/ 下）exit 0；`cmake --build cmake-build-pcverify1-tests --config Release --target VitRenderWatchdogTests` exit 0

## 门与退出码

| 门 | 命令 | 结果 |
|---|---|---|
| C++ 内核编译 | `cmake --build build --config Release --target VitApp` | exit 0 |
| 看门狗单测（含 A' 双路径） | `cmake-build-pcverify1-tests/VitRenderWatchdogTests_artefacts/Release/VitRenderWatchdogTests.exe` | exit 0 |
| agent 回归 | `go build ./... && go test ./... -count=1`（agent/ 下） | exit 0，55 包 ok，0 FAIL |
| 真栈烟测（新场景，第一轮） | `dev_agent_smoke.ps1 -Scenario render_freeze -StartKernel -RunArtifactsDir …\smoke_render_freeze_1` | exit 0（17 ok 断言全过） |
| 真栈烟测（复跑，稳定性） | 同上 `…\smoke_render_freeze_2` | exit 0（25 ok） |
| midi_register 场景回归（共享脚本骨架） | `dev_agent_smoke.ps1 -Scenario midi_register -StartKernel -RunArtifactsDir …\smoke_midi_register_regress` | exit 0 |
| 渲染族回归 | `run_ab_result_smoke.ps1` | exit 0（mixboard/mom/chat ok） |

## run 目录

- `20261006_verify1/`：三腿先行验证探针（python ZMQ 直打，零 LLM）。关键实证：
  - 腿 B：MIDI-only `start_render` → 同步 `{"status":"error","reason":"no_renderable_audio_content",…}`，无 job_id，ping/get_project_state 活，idle cancel ok。
  - 空范围渲染：工程有音频但 range=[10,12] 无内容 → **render_done（静音成功）**——证明 `checkNodesForAudio`/`props.hasAudio` 按全工程判定，范围外剪辑不算无音频；此路径不能当异步失败触发器。
  - **确定性异步失败路径**：目标盘不存在（B:）→ 同步 "Render started" ok → 453ms 后 `render_failed("Couldn't write to target file")` 遥测到达 → 命令面活（startOfflineRender 既有行为：createDirectory 结果被忽略）。此为死锁形状的零竞态复现，已固化进场景。
  - cancel 竞态实证：[0,30] 渲染 + 立即 cancel，两轮分别得 failed("Cancelled")/ready——快机上渲染可 <取消往返时延完成，故场景中该组只断言"任一终态+命令面活"，不设竞态门。
  - 看门狗零误报：全程 0 条 render_watchdog 事件。
- `smoke_render_freeze_1/`、`smoke_render_freeze_2/`：定稿场景两轮（head.txt/git_status.txt/summary.json/各断言组回执 JSON 齐全）。
- `smoke_midi_register_regress/`：脚本共享面回归。

## 场景断言面（render_freeze）

1. 扫台：kernel `clear_project`（经 agent 命令目录回退路由）——track.delete 无法删最后一条音频轨（内核拒绝），故用清剪辑而非删轨。
2. 腿 B：MIDI-only（3 音符无乐器）render.start → 同步 error（reason=no_renderable_audio_content）+ 无 job_id → project.state 活 → idle render.cancel ok。
3. 音频导入（sine stem）后：
   - 腿 A（死锁反转主断言）：不可写目标盘 render.start → 同步 ok+job_id → render_failed 遥测经 kernel PUB → agent（render.profile.bind 错误语义探针：'is not ready (status "failed")'）→ project.state 活。
   - live-cancel：[0,30] 渲染 + 立即 cancel → cancel 回 ok → 任一终态遥测 → project.state 活。
   - 健康回归：空范围 [10,12] → render_done + 产物文件存在。
4. PS 5.1 工程注：`Invoke-WebRequest` 对 HTTP 错误回执在用户 catch 中流已被吸干（GetResponseStream/ErrorDetails 均空），场景的 `Invoke-JsonTolerant` 改用 `System.Net.Http.HttpClient`（非 2xx 不抛、body 可读）。

## 泊位与环境申报

- 强杀申报：探针内核 pid 7348/21252 各 `taskkill //F`（探针自身起停，非烟测 berth）；两轮 berth 自 teardown 干净（agent+kernel stopped，端口复查净，无残留进程）。
- 内核语言决策：腿 B 人话消息用 ASCII 英文实现（内核无 `/utf-8`、CP936 源码解码下中文字面量会运行时乱码；Source 内无中文运行时字符串先例）——卡面中文为人话语义描述，语义已对齐，回执申报。
- 本卡未触碰：tracktion submodule、agent 侧任何文件、CommandDispatcher.cpp、webui。
