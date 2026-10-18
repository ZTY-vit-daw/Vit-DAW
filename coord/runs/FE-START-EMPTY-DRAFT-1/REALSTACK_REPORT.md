FE-START-EMPTY-DRAFT-1 真栈复验腿 — run 记录（执行侧）

- 卡：coord/cards/doing/2026-10-15-FE-START-EMPTY-DRAFT-1.md（池序 118，P1）
- owner：DSH PC 执行流 exec-start-draft；分支 port/fe-start-empty-draft-1 @ 1dcb20f8（worktree D:/Vit_DAW_wt_fedraft1）
- 栈占用：PC-RUNTIME-STACK 登记 commit 450c4c47（Lead 登记；窗口标注 14:32–15:00）
- 实际执行窗口（本机钟）：14:10:56 首次启栈 → 14:18:52 拆栈复核（本次记录以下均用本机钟）
- 结论：**真栈复验 PASS（8/8 断言）**，被测 agent = 本分支修复构建；用户真实 draft 根零写入（0 新增）；拆栈后四端口/本腿进程=0。

## 1. 被测二进制与栈配置

| 项 | 值 |
|---|---|
| 内核 | `D:\Vit_DAW\Export\staging\runtime\VitApp.exe`（主检出 stock，只读）SHA256=`B6565DCF85D1DA867E16333052A7B928A5413565940603ADBCF4ABD59DD3123B` |
| agent | 本分支构建 `D:/Vit_DAW_wt_fedraft1/agent/bin/VitAgent.exe` SHA256=`69B8D77C8A9CCBAA59222350DAA2869EAC27ABBAB0A71EC03598166808F06949` |
| 启动方式 | `dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_fedraft1 -StartKernel -RestartAgent -KernelExe D:\Vit_DAW\Export\staging\runtime\VitApp.exe -NoChatSmoke -NoStripSilenceSmoke`（不启 Godot UI）；脚本 bringup 后为获得"干净隐式 draft"窗口，执行侧重启 agent（同二进制、同 env、同参数），kernel 保持 |
| draft 隔离根 | `<run>/ProjectHistory/drafts`（env `VIT_HISTORY_DRAFT_ROOT`；路径含 `ProjectHistory\drafts` 段以复刻用户真实根的 `IsDraftProjectPath` 语义） |
| 用户真实根 | `C:\Users\timoz\AppData\Local\Vit\ProjectHistory\drafts`：全程只读；运行前 10177 目录、运行后 10177 目录，14:00 后无新增（见 `real_draft_root_check.txt`） |
| §8 | **LLM=0**（agent LLM telemetry 最后一行 14:05:14，本腿窗口内零新增行；probe 仅发 kernel 命令+agent 生命周期工具调用）；失败分类=assertion_fail/environment_interrupt；未触发止损 |

## 2. 探针与断言（最终 PASS 轮：`probe_final/`，14:18:37–14:18:40）

用户等价场景（模拟"上一会话 draft 存在"——四轮用户样本 100% 具备的前置）：隔离根内预置与 **内核启动工程 uuid** 匹配的种子 draft（runtime state 指向种子 draft，无 BOM JSON），agent 启动恢复会认领种子 draft，使隐式默认 draft 保持 pristine。

1. 启动恢复认领种子 draft（旧 uuid `vitproj_2978f634…`）✓
2. 通知前隐式默认 draft **仍为 pristine**（`snapshot_before_cancel` 中 `history=false/agent=false`；本最终轮未发生启动期 reader 物化，`canceled_reader_artifacts=[]` 即无需取消）✓
3. 内核 ZMQ `new_project` 切换工程 → 新 uuid `vitproj_2508a2ff…`（复刻前端先换内核工程的顺序）✓
4. agent `/agent/invoke {"tool":"version.project_new","args":{"project_uuid":新uuid}}`（前端同款通知）后：**draft 根无新增目录**（旧行为=再建 1 个）✓
5. 隐式 draft `draft_20261018T061835_d2743dd5` 被复用为新工程 draft ✓
6. 复用 draft 只含新工程 uuid，无旧 uuid workspace 泄漏 ✓
7. 种子恢复 draft 保留旧 kernel uuid workspace ✓
8. 全程用户真实 draft 根零写入 ✓

结果工件：`realstack_d_clean_20261018_1421/probe_final/probe_final_result.json`（exit 0，failures=[]，3/3 断言 pass + 4 项附加校验 pass）。

## 3. 同腿过程记录（如实登记，非最终结论）

同窗口内做过三轮探针迭代，均为**场景/夹具问题**且已定位（保留证据，不计入产品失败）：

| 轮次 | 目录 | 结果 | 归因 |
|---|---|---|---|
| A：空隔离根（Lead 字面场景） | `realstack_a_20261018_1410/` | FAIL（双 draft） | 空根内不存在与内核启动 uuid 匹配的历史 draft → 启动恢复无处可去，隐式 draft 被 activation 认领并初始化（非 pristine），ProjectNew 按设计新建第二个 draft。**这暴露了最小修复的适用边界：reuse 只在"通知到达时隐式 draft 仍 pristine"时生效**，而空根场景下 agent 自身启动会先认领它；与用户四轮样本（A 保持 75B）不同。 |
| B：种子用 Windows 默认 ANSI/带 BOM 写 JSON | `realstack_b_seeded_20261018_1415/` | FAIL | 种子 runtime state 带 UTF-8 BOM，`json.Unmarshal` 静默失败→恢复未命中；改为无 BOM 后恢复命中。 |
| C：修复 BOM 前的用户等价根 | `realstack_c_userequiv_20261018_1420/` | 中途诊断轮 | 用于验证恢复/激活路径与 `IsDraftProjectPath` 前缀要求；部分结论被 D/E/F 取代。 |
| D/E：用户等价根 + 自主 reader 竞态 | `realstack_d_clean_20261018_1421/probe_run/`、`probe_race/` | FAIL（发现真问题） | 若探针在 agent 启动**之后**才开跑（~2s 后快照），agent 启动后约 1.5–2s 会出现**自主历史 reader**（日志 `harness.invoke tool=version.list / version.worktree_list`，无外部请求来源）先物化隐式 draft 的 `.vit_history/vitproj_<旧uuid>/state/worktrees`，使 pristine 前置丢失。该 reader 正是新卡 `FIX-START-DRAFT-SIDE-EFFECT-1` 的"读路径物化 draft"面（本卡修复不覆盖）。**最终轮改为探针与 agent 启动并发**（TCP/日志 80ms 轮询，activation 后立即快照），在 reader 落盘前完成了 pristine 快照→kernel new_project→agent 通知，因而最终轮为自然 pristine、无需任何取消动作。 |

## 4. 拆栈证据

- 本腿进程：VitApp pid=10100、VitAgent pid=3620/17352（各轮重启）——14:18:50–14:18:52 只 Stop-Process 自起 pid（记录 `teardown.txt`）。
- 14:18:52 复核：五端口（7878/5555/5556/8787/4445/19876）监听=**0**；本腿 VitApp/VitAgent=0；本腿未启 Godot/vsphub。
- 注意事项（如实登记）：14:18:52 复核时发现另一进程 `VitApp.exe pid=20128`（`D:\Vit_DAW_wt_mastervol1\...`，parent=powershell 运行 `D:/Vit_DAW_wt_mastervol1/coord/runs/FIX-MASTERVOL-CALIBRATION-1/run_master_volume_diag.ps1`，创建 14:18:50）——属 **134/FIX-MASTERVOL-CALIBRATION-1 栈腿**，非本腿、未触碰；本腿四端口已释放，该腿可自行绑栈。
- 14:19:07 终核：本腿所有 pid 均不存在，端口=0。

## 5. 工件索引（本目录）

| 工件 | 说明 |
|---|---|
| `realstack_d_clean_20261018_1421/probe_final/probe_final_result.json` | **最终 PASS 轮**完整断言/快照/取消 reader 物化清单（exit 0） |
| `realstack_d_clean_20261018_1421/probe_race/probe_race_result.json`、`probe_run/probe_result.json` | 竞态/中间轮证据（reader 物化时间线、recovery 未命中即场景边界） |
| `realstack_a_20261018_1410/probe_result.json`、`script_stdout.log` | 空隔离根字面场景失败证据与栈日志 |
| `realstack_d_clean_20261018_1421/agent_last_snapshot.txt` | 清洁 agent 启动日志（activation/version.list reader 时间戳） |
| `realstack_d_clean_20261018_1421/teardown.txt`、`real_draft_root_check.txt` | 拆栈与真实 draft 根零写入复核 |
| `scripts/probe_pristine_race.ps1` / `probe_race.ps1` / `probe_draft_root.ps1` / `kernel_new_project.py` | 可复跑探针（自包含；kernel 侧只读/生命周期命令，agent 侧仅工具调用） |
| `ATTRIBUTION.md` / `FIX.md` | 归因件与修复记录（见本卡主题目录） |

## 6. 结论与边界

- **通过**：在"通知到达时隐式 draft 为 pristine"的契约下，本分支修复在真实栈上确定性生效——最终轮自然 pristine（无取消动作），`version.project_new` 不再新建第二个 draft，复用隐式 draft 且无旧 uuid 历史泄漏；用户真实 draft 根零污染。
- **适用边界（重要，交 Lead/新卡）**：真实栈存在启动期自主历史 reader，会在 activation 后约 1.5–2s 内物化隐式 draft，使 pristine 前置丢失（本腿 D/E 轮实证；最终轮以并发启动的探针在 reader 落盘前完成通知而通过）。用户四轮样本中 A 保持 75B，说明用户 frontend 真实时序下通知早于/绕过了该 reader；根因修复面归 `FIX-START-DRAFT-SIDE-EFFECT-1`。
- **未覆盖**：空隔离根（首装用户）场景仍会 2 个 draft（本腿 A 轮实证：启动恢复无历史可认领时隐式 draft 被 activation 认领初始化，ProjectNew 按设计建新 draft）；`track.list` 0 轨未在本腿验证（kernel 空工程，无意义）；UI 手测 4 条保留给用户补充。
