# CONV-OPEN-TOOL-1（419）验收：PASS（零栈 Go）

- 卡：`2026-11-18-CONV-OPEN-TOOL-1`（池序 419）；主管=lead；执行=lead/convopen419。
- 分支 `codex/conv-open-tool-419`：`e6184e1d` + `4554e8e8`（工具名对齐 `version.conversation_open`，命令名 `version_conversation_open`）；已合入 main `8d9b8f4e` 并推送。
- 交付：5 文件 +786/−1，全部在文件域：新增 `agent/internal/harness/conversation_open.go`（324 行）+ `conversation_open_test.go`（454 行，7 测试）；`harness.go` 分派+turn 守卫；`harness_test.go` 守卫表；`catalog.go` 注册（RiskConfirm）。未动 webui/内核/Godot/history 包。

## 主管复核（亲跑）

| 项 | 结果 |
|---|---|
| `go build ./...` | exit 0 |
| `go test ./internal/harness -run 'ConversationOpen|Worktree' -count=1` | exit 0 |
| `go test ./internal/tools -count=1` | exit 0 |
| `go test ./internal/history -count=1` | exit 0 |
| `git diff --check` | exit 0 |
| 代码抽检 | 新文件仅一处 `withProjectReload`（末尾）；失败路径在 reload 前返回；`target_materialized/kernel_reloaded/branch_warning/refresh` 披露字段在位；catalog `needs_confirmation` |

## 语义（B2b 需知）

- 参数 `name` 或 `project_file_path`（至少一个；两个同给必须同一工作树）、可选 `branch`、必填 `node_id`；未知/冲突身份拒绝。
- 同工作树：分支 head 节点→分支 checkout 语义；历史节点→detached node checkout；均单 reload。
- 跨工作树：只读校验→复用 worktree switch autosave→目标副本物化→active worktree/branch/node 复核→**恰好一次** `withProjectReload`（1×open_project+1×get_project_state）。
- turn 运行中/stop-requested 拒绝（error_code=`checkout_blocked_while_agent_running`）；**分支激活**仅在目标节点属于该分支且分支 HEAD=该节点 commit；否则 detached 并在 `branch_warning` 披露——B2b 传“分支+该分支 head 节点”即可走非 detached。
- 失败：reload 前任何失败不 reload、当前工程不动（披露 target_materialized）；reload 失败返回显式 error。

## 边界/后续

- `agent/internal/chat/turn_control.go` HTTP 层预守卫未列新工具（harness 层权威守卫已覆盖）；`goalrunner_chat.go` 模型侧白名单未加（UI 组合工具，不需要模型可达）——如需另开小卡。
- B2b 前端（421）接线 + 真实栈组合验证（422）待做；本卡只证 Go 语义与 fake kernel 单次 reload。
