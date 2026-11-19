# W02-A2a 零栈自检记录（2026-11-19）

- 执行 owner：`w02fb`（DeepSeek-V4.1-Flash，PC）。
- worktree：`C:/Users/timoz/.codex/worktrees/pc433-feedback/Vit_DAW`
- 分支：`codex/w02-a2a-feedback-433`；HEAD `2794d135859f5fb6368791005a92c7d344cd01dd`（main 基线）。
- 主机：Windows；Windows PowerShell `5.1.26100.9818`（生产入口语义按 5.1 验证）。
- 边界：本记录只覆盖**零栈**自检。**未启动/重启/停止任何栈进程，未监听端口，未发起真实 LLM 请求，未消耗正式轮。**

## 1. 结论表

| 项 | 命令/方式 | 结果 |
|---|---|---|
| PowerShell 5.1 解析 | `[Parser]::ParseFile(...)` | **PARSE_OK**，tokens=17424，errors=0 |
| 文件编码 | 读前 3 字节 | **UTF-8 BOM**（239,187,191），保证 5.1 正确解码中文 chat 命令 |
| 无授权（status 默认） | 不带 `-StackAuthorized` 运行 | **exit 3**（只打印 runbook 摘要） |
| 无授权（work） | `-QuestionMode work` 且不带 `-StackAuthorized` | **exit 3** |
| Dry-run（status） | `-DryRun` | **exit 0**（打印 S0–S6 计划，零 HTTP） |
| Dry-run（work） | `-DryRun -QuestionMode work` | **exit 0**（打印 S0–S3 + S4W 计划，S5/S6 标注跳过） |
| 静态卫生 | `Start-Process/Start-Job/TcpListener/HttpListener/WebRequest/netstat/Get-NetTCPConnection/Stop-Process` | **0 hits** |
| 证据 helper 合成测试 | `coord/runs/W02-A2a/selftest/w02a2a_helper_test.ps1` | **HELPER_TESTS_OK**，exit 0 |

## 2. 解析与编码

```powershell
$p = '<worktree>/scripts/run_w02_a1c_smoke.ps1'
$t = $null; $e = $null
[void][System.Management.Automation.Language.Parser]::ParseFile($p, [ref]$t, [ref]$e)
if ($e.Count -eq 0) { 'PARSE_OK tokens=' + $t.Count } else { $e }
```

输出：`PARSE_OK tokens=17424`；首 3 字节 `239,187,191`（UTF-8 BOM）。

## 3. 三个零栈退出码

```powershell
# 无授权（status / work）：expected 3
powershell -NoProfile -ExecutionPolicy Bypass -File <script> -RepoRoot <worktree>
powershell -NoProfile -ExecutionPolicy Bypass -File <script> -RepoRoot <worktree> -QuestionMode work

# dry-run（status / work）：expected 0
powershell -NoProfile -ExecutionPolicy Bypass -File <script> -RepoRoot <worktree> -DryRun
powershell -NoProfile -ExecutionPolicy Bypass -File <script> -RepoRoot <worktree> -DryRun -QuestionMode work
```

输出（逐行）：

```text
NOAUTH_STATUS_EXIT=3
NOAUTH_WORK_EXIT=3
DRYRUN_STATUS_EXIT=0
DRYRUN_WORK_EXIT=0
HYGIENE_HITS=0
```

- 无授权路径在解析参数后即 `exit 3`，不写 RunRoot、不发 HTTP。
- dry-run 路径在授权门前 `exit 0`，只打印计划，不写 RunRoot、不发 HTTP、不占端口。
- 脚本内不存在启动/停止进程或监听端口的命令（静态卫生 0 hits）；真实栈由 lead/owner 启动，脚本只访问已运行的 `http://127.0.0.1:7878`。

## 4. 证据 helper 合成测试

测试文件：[selftest/w02a2a_helper_test.ps1](selftest/w02a2a_helper_test.ps1)。它用 PowerShell AST 从真实执行器提取 `Get-Prop/Read-AppendedText/Test-NotesReadToolName/Find-ClipIdInObject/Get-ToolCallRowsFromEvents/Find-PromptRenderToolResults/Find-KernelNotesRead`，喂合成数据并断言正例/反例：

- `/agent/events`：`item.started` + `item.completed` 的 `payload.tool=midi.read_notes`、`command_name=get_midi_clip_notes`、`args.clip_id=1014`、`result.notes` 3 行 → 命中 1 条，notes_count=3；
- 嵌套 `command_raw.legacy.args.clip_id`（legacy 命令形态）→ 仍能取到 1014；
- prompt render：同一 `model_snapshot` 字段内 `<tool_result>` + notes 工具名 + clip 1014 → 命中 1 行且 `has_notes_key=true`；
- 反例：只有工具目录 + clip id（无 `<tool_result>`）→ 0；`<tool_result>` 属于 clip 1013 而查询 1014 → 0；
- kernel log：`"cmd":"get_midi_clip_notes"` + `"clip_id":"1014"` + `tool_call_id` → received=1、tool_call_id_hits=1；clip 1013 → 0；`Command executed: get_midi_clip_notes` 单独计数=1；
- 真实 A1c S4 prompt render 行（无工具回执）→ 0（不把工具目录/选中 id 误判成 notes 回执）。

运行：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\coord\runs\W02-A2a\selftest\w02a2a_helper_test.ps1
```

输出：

```text
HELPER_TESTS_OK
HELPER_EXIT=0
```

## 5. 未覆盖（诚实边界）

- 未跑真实栈：S0–S3/S4W 的真实 HTTP/内核/LLM 路径未被本记录执行；真实证据面是否出现由 lead 的正式轮决定。
- 未验证 free-state 问题轮是否一定暴露 `midi.read_notes` 工具；若模型不调用或工具面不提供，正式轮会落到 `status=model_boundary`（exit 5），这是设计内 fail-honest 结果，不是脚本缺陷。
- 合成测试只验证解析/匹配逻辑；无法替代真实 agent log 的字段形态。首次正式轮若出现字段形态差异，按 §6/§7 的 fail-honest 语义记录并交 lead 判断，不补假证据。
