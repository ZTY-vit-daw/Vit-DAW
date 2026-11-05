# AF-D1-LOUDNESS-SETTLE-1 armrun1 真栈回执（2026-11-05 晚窗）

## 结论

**PS1_EXIT=1——D1 runner（非本卡面）settle 链未在 600s 烟测窗口内排干；本卡新增的 LW 门与 D1 settle 门未执行到（脚本在 LW 阶段前 throw）。§8 单 run 无重跑兑现，取证上交。**

单测面与实现完整性不受本 run 影响（见卡面回执）；gate4 缺陷腿已在跑前证据关线。

## Run 事实

| 项 | 值 |
|---|---|
| run stamp | 20261105_211940（21:19:40 栈起，runner 21:20:50 起） |
| 命令 | `run_free_state_d1_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_254impl -PublicManifest D:\Vit_DAW\temp\...fixture_manifest.json -PromptFlavor reverb -ExpectDomain reverb -LoudnessWatch -D1LoudnessSettle -LlmModelOverride deepseek-flash -SkipBuild -KernelExe D:\Vit_DAW_wt_228\...\VitApp.exe`（完整形态见卡面 §8） |
| 被测 agent | worktree 构建 `bin/VitAgent.exe` SHA256 `F321AF5E268CE2C14FB83BDA15BE1C4F73083AFC3C3BC620299E486720C6E1AD`（实现 c687c9a1；console `[lw] binary face` 亲证） |
| kernel | wt_228 `F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A`（225/249/251/253 同款只读复用，console 亲证），pid 42928 |
| LW 预配置 | ok（digest `0079aa999954...`，ebu_r128 口径，`lw_prepare.json`） |
| runner 结果 | **fail**：`D1 durable continuation did not drain within the smoke timeout`（`d1_smoke_report.json` status=fail） |
| 栈健康 | kernel ZMQ 5555/5556 在场、agent HTTP 7878 在场、settle 链推进 executed=5 边界 / stored_receipts=18 / ccb_observations=5（agent 日志 21:38:50 `[free-state.ledger] boundary` 行）——**非环境中断、非栈故障** |
| 停泊形态 | continuation `waiting_continue / stop=limit_reached limit=timeout`（agent 日志 21:38:50 `[continuation.arm]` 行；runner 600s 先到，continuation 在 runner 退出后仍被 claim） |
| 拆栈 | 脚本 throw 路径未自 teardown；owner 手动拆 VitAgent 41732/VitApp 42928/Godot 前端进程组，**TEARDOWN_ALL_ZERO 亲核 21:40:37**（四端口 0 + VitAgent/VitApp/vsphub 0 + Godot 前端 0 + python 0） |

## 失败分类与归因候选（§8 口径对照）

- 归类 **a) D1 runner settle 链收敛随机性**（133 run1 先例同族：彼为 `free_state_continuation_budget_exhausted`，本次为 `limit_reached(timeout)` 停泊未 drain）——失败发生在 free-state settle 链本体（非本卡文件域、非本卡断言面；LW 门与 D1 settle 门均未执行）。
- **归因候选披露**：本 run 按 §8 口径注入 `-LlmModelOverride deepseek-flash`；133 run2（同 fixture 同话术 -PromptFlavor reverb -ExpectDomain reverb）**未注入**（默认引擎）跑出 exit 0。settle 链轮次节奏与引擎选择的关联是未排除变量——处置选项（重跑不注入 / 加大 --timeout-sec / 认定模型分支维持单 run 结论）归决策侧裁定。
- 计费：**51 调用全 deepseek-flash**（50 ok + 1 client `context deadline exceeded`），tokens in 790,951 / out 138,039（`llm_telemetry_run_window.jsonl` 亲数）——按 225 卡计价口径折算 **≈¥1.1-1.3 上界**，预算 ¥2-4 内。

## 调用形式备注（如实记录）

本 run 前有两次**启动前**调用形式失败（栈未起、零 LLM、零断言，不构成有效运行）：①Git Bash 反斜杠转义吞路径（AGENTS §5 陷阱，PS 报脚本不存在即退）；②worktree 无 gitignored temp fixture，补显式 `-PublicManifest`（主检出 fixture）后第三次调用成功起栈。占用登记（319384b3）不受影响——第三次起栈在登记后 ≤60s 即时复核净空后进行。

## 工件清单（本目录）

- `console.log`——完整 PS console（栈起/binary face 亲证/LW 预配置/runner 失败 throw）
- `d1_smoke_report.json`——runner 终态报告（status=fail + `D1 durable continuation did not drain within the smoke timeout`）
- `llm_telemetry_run_window.jsonl`——run 窗口 51 行 telemetry 切片
- `agent_log_tail120.txt`——agent 日志尾 120 行（boundary/continuation.arm 停泊实证）
- `lw_prepare.json`/`lw_binary_face.json`/`loudness_watch_configure.json`/`lw_project_open.json`——LW 预配置与二进制亲证小件
- 大件（project 工作区、完整 agent 日志）留盘 `D:\Vit_DAW_wt_254impl\artifacts\free_state_d1_s1\20261105_211940\` 可回指
