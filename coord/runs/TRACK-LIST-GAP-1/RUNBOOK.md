# TRACK-LIST-GAP-1（139）栈腿 RUNBOOK：track.list 0 轨同帧对照探针

- owner：DSH PC 执行流 exec-rendergain；worktree `D:/Vit_DAW_wt_tracklist1`，分支 `port/track-list-gap-1`（基线 origin/main=77a0b02f）
- 边界：**零生产代码改动**；诊断脚本/工件只入本目录；不启 Godot UI；不写共享主工作树；不 push main
- 关联：卡 [2026-10-18-TRACK-LIST-GAP-1.md](../../cards/doing/2026-10-18-TRACK-LIST-GAP-1.md)；118 归因 [ATTRIBUTION.md](../FE-START-EMPTY-DRAFT-1/ATTRIBUTION.md) §5/§7

## 1. 驱动体系（无既有脚本改动）

既有 `scripts/dev_agent_smoke.ps1` 是单体脚本、无可挂自定义腿的开关；本腿新增 `run_tracklist_gap_probe.ps1`（本目录）复刻其 berth 纪律：
1. 单栈 owner 预检：六端口（5555/5556/5557/7878/8787/4445）监听=0、四类进程（VitApp/VitAgent/vsphub/Godot-frontend）=0；用户 Godot 编辑器（`--editor`）按 PC-RUNTIME-STACK 先例 out-of-scope（不碰不杀）。
2. run-dir kernel workspace 隔离（同 132 腿经验）：`kernel_workspace` 内放 `CMakeLists.txt`+`Source/` 标记使 `VitPaths::getProjectRootDirectory()` 命中 run 目录（内核 Workspace/Logs/Cache 全落 run 目录），并用 `VIT_PROJECT_XML` 指 run 目录的空白启动工程 → 对主检出部署 Workspace **零写入**。
3. 栈=**kernel + vsphub + agent**（不启 UI）：`vsphub -listen 127.0.0.1:8787 -kernel-req tcp://127.0.0.1:5555 -kernel-sub tcp://127.0.0.1:5556`；agent `-http 127.0.0.1:7878 -vsp-hub-url http://127.0.0.1:8787/vsp`（VSP canonical 命令走真实 hub→kernel 链路）。
4. finally 只杀本腿自起 pid（agent/vsphub/kernel），复核端口/进程=0。

## 2. 运行命令（Lead 放行后）

```powershell
powershell -NoProfile -ExecutionPolicy Bypass `
  -File D:\Vit_DAW_wt_tracklist1\coord\runs\TRACK-LIST-GAP-1\run_tracklist_gap_probe.ps1 `
  -RepoRoot D:\Vit_DAW_wt_tracklist1 `
  -KernelExe D:\Vit_DAW\Export\staging\runtime\VitApp.exe
```

- 自动构建 worktree 被测二进制：`agent/bin/VitAgent.exe`（cmd/vitagent）+ `agent/bin/vsphub.exe`（cmd/vsphub）；内核=主检出 stock exe（只读，SHA256 记录在 run_meta）。
- 工件：`run_<ts>/probe_summary.json`、`probe_console.txt`、`raw/*.json`、`kernel_workspace/Workspace/Logs/`、`agent_last.log`、`vsphub_last.log`、`kernel_vsp_evidence.json`、`teardown.json`、`run_meta.json`。

## 3. 同帧四面对照（探针 `tracklist_gap_probe.py`）

每次 capture 在 ~1s 内顺序抓取（只读、无中间变更；原始响应逐面存档）：

| 面 | 端点/通道 | 说明 |
|---|---|---|
| F1 kernel 直查 | ZMQ 5555 `list_tracks` + `get_project_state` | 内核权威 edit（绕过 agent/hub） |
| F2 VSP canonical 命令 | hub POST `/vsp` `command.request{command=project.tracks.list}` | 内核 legacy `list_tracks` → `TrackService::handleListTracks` 的 VSP 包装回包（payload.legacy_reply.tracks） |
| F3 VSP state snapshot | hub POST `/vsp` `state.snapshot_request{scope=project.timeline}` | 内核 edit 状态的 tracks/project_uuid/revision |
| F4 agent shadow | GET `/agent/state` | shadow Summary（track_count/user_track_count/tracks） |
| F5 agent 工具面 | POST `/agent/invoke {tool=track.list}` | 用户/LLM 看到的工具回包（含 vsp.command_ack） |
| F6 自愈面 | POST `/agent/invoke {tool=project.state}` | harness 用 VSP state snapshot 刷新 shadow |

## 4. 阶段设计（复现与排除）

| 阶段 | 动作 | 预期（待证） |
|---|---|---|
| S0 | 新栈基线（空白启动工程） | 各面一致；记录基线 |
| S1 | **外部路径**新建工程+导入音轨（kernel 直查 `new_project`→`add_track`×6→`import_audio`；等价 GUI 外部变更，不经 agent 工具） | kernel/hub/state=6；agent 工具回包=shadow（可能 0/陈旧）→ 复现「list_tracks 0 轨而工程有轨」 |
| S1b | agent `project.state` 工具刷新 | shadow/工具面追平 kernel（118 观测的 agent 自愈） |
| S2 | **外部路径**打开既有工程（load_project 2 轨 fixture） | 与 S1 同签名 tool≠kernel → 排除「新建+导入路径特有」候选② |
| S3 | **agent 路径**写操作（authority full + invoke `add_track`，可行时） | agent 路径自身同步（shadow 随 agent 命令更新），反证问题在于外部路径不刷新 shadow |

候选面裁决：
- **候选①（VSP 通道命中不同 kernel/edit 实例）**：每阶段比对 kernel 直查 / hub canonical / hub state 的 track_id 列表 + `project_uuid`；hub `/vsp/status` 记录 kernel_req 端点与 hub pid；内核会话日志统计 VSP envelope 行数 → 全部一致则排除。
- **候选②（新建+导入路径枚举差）**：S1 与 S2 同为外部路径不同来源，同签名则排除路径特异性。

## 5. §8 声明（运行前）

- LLM = **0 次调用**（全部确定性内核 ZMQ / hub HTTP / agent invoke 只读面；S3 写操作若触发确认流则记录而不强推）。
- 成功条件：脚本 exit 0（探针判定 pass：候选①排除 + 候选②排除 + tool≡shadow 面成立 + 自愈面成立 + 至少一阶段 tool≠kernel）。
- 失败分类：`environment_interrupt`（栈起不来/端口占用/进程残留）、`assertion_fail`（实测与预期不符→按阶段记录签名）、`capability_blocked`（agent invoke 被 authority/确认门拒绝时记录，不伪造）。
- 止损：同签名 assertion_fail×2 即停、拆栈、取证上交；不重试烧窗口。

## 6. 回执口径

run ID / 脚本 exit / 同帧逐面数值与 track_id 列表 / 候选①候选②结论 / 最小复现记录（S1 是否 2 次确定性出现）/ 修复建议与文件域锚点（另卡）/ 偏差（含被测二进制 sha256、out-of-scope 进程、隔离说明）。
