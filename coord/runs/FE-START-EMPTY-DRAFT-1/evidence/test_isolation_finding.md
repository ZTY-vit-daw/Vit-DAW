# 测试隔离性偏差登记（FE-START-EMPTY-DRAFT-1 执行侧自验过程中发现）

- 时间：2026-10-18 14:00 +0800（本卡全量 `go test ./... -count=1` 首跑）
- 发现：仓库内既有测试存在 **draft 根隔离缺陷**——部分 history/chat/harness 测试未设置 `VIT_HISTORY_DRAFT_ROOT`，测试运行直接向真实 ProjectHistory draft 根
  `C:\Users\timoz\AppData\Local\Vit\ProjectHistory\drafts\` 物化 draft 目录（违反 AGENTS §10 测试运行时状态不得落到固定历史目录）。
- 本次新增的 4 个测试污染目录（只含测试工程历史，无用户数据；已复核无 `session.json`/`agent_runtime_state`/`.vit_agent` 等真实运行标记）：

| 目录 | 内容形态 |
|---|---|
| `draft_20261018T055959_9579be0d` | Unsaved.vit 75B + `.vit_history/projects/Unsaved_0075d9d0529e/worktrees/main/.vit_worktree.json` |
| `draft_20261018T060040_a960685a` | Unsaved.vit 75B + 测试工程历史（`project-capacity` / `project-targeting-coverage` 提交与对象） |
| `draft_20261018T060040_b128fc86` | 同上 legacy 布局（仅 worktrees item） |
| `draft_20261018T060050_49ca6cfd` | 同上 legacy 布局（仅 worktrees item） |

- 处置（未删除，可复核）：4 个目录已移出真实 draft 根 → `C:\Users\timoz\AppData\Local\Temp\fedraft1\test_pollution_quarantine\`；真实 draft 根复核=0 遗留。
- 复跑（隔离后）：`VIT_HISTORY_DRAFT_ROOT=C:\Users\timoz\AppData\Local\Temp\fedraft1\gotest_draftroot` 下全量 `go build ./...` + `go test ./... -count=1`：exit 0、0 FAIL，真实 draft 根本测试流净新增=0。
  - 注：同一窗口真实 draft 根另有 2 个目录（`draft_20261018T060416_4bcdde3a` / `draft_20261018T060418_5450cd7c`）经只读核对为**同时段另一真实栈腿**（KERNEL-RENDERGAIN-1 栈腿/`pcactl.certify_compressor` journal、VitAgent pid 11452、VitApp pid 21624、VspHub pid 7844、端口 7878/5555/5556/8787 监听）产物，**非测试流写入，保留不动**，不计入测试污染。
- 归因：与本卡修复无因果关系（基线同样存在；被隔离的 4 个目录的测试工程名与修复代码无关）。本卡不越域修复其它测试；建议另开小卡：为 history/chat/harness 测试统一加 draft 根隔离（`t.Setenv("VIT_HISTORY_DRAFT_ROOT", t.TempDir())` 或 TestMain 级隔离），并加「测试后真实 draft 根目录数不变」的守门断言。
