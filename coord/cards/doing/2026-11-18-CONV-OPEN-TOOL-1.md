# CONV-OPEN-TOOL-1：agent 组合工具（打开工作树并继续对话，单次工程重载）

- 池序 419；P1；PC；发卡/验收=本会话 PC 主管（lead）；派发确认=已确认（用户 2026-11-18 晚同意新增组合工具；交互模型见 decisions/2026-11-18-entry-rail-conversation-model.md）。
- owner=`lead/convopen419`（DeepSeek-V4.1-Flash，PC）；分支 `codex/conv-open-tool-419`；worktree=`C:/Users/timoz/.codex/worktrees/pc419-open/Vit_DAW`（主仓）；基线=`origin/main`。
- 目标：新增 agent 侧组合工具（暂名 `version.conversation_open`，最终以 catalog/实现为准），供 B2b 左栏【打开此工作树并继续此对话】使用：**校验/守卫 → 物化目标工作树+目标对话节点 → 最后仅一次 withProjectReload（内核 open_project+get_project_state）**。

## 语义（必须实现）

1. 参数：目标工作树标识（name 或 project_file_path）、可选 branch、目标 node_id、当前上下文由 harness 注入；拒绝缺少目标身份/节点不存在等输入。
2. 同工作树：等价于现有 node checkout（materialize 目标 commit、写 active branch/node），成功后一次 project reload。
3. 跨工作树：校验目标工作树存在且节点属于该工程历史；复用现有 worktree switch 的 autosave/守卫语义（当前工程未保存先按既有行为处理，不得静默丢改动）；把目标 commit 物化到目标工作树副本、更新 active worktree/branch/node；**最后只调用一次** `withProjectReload`（fake kernel 断言恰好一条 open_project + get_project_state）。
4. 守卫：turn 运行中（checkout_blocked/active status）拒绝并返回明确错误；未知工作树/分支/节点拒绝；不得 grep/猜测。
5. 失败语义：reload 之前的任何失败 → 当前内核工程不 reload（当前工程不受影响；目标工作树文件可能已准备，需在结果中如实披露）；reload 失败 → 明确失败态与原因，不静默。
6. 旧工具（version.worktree_checkout / version.checkout / node_checkout）行为不变；本工具是组合缝，不改其语义。

## 文件域（Go；≤6）

`agent/internal/harness/harness.go`（工具分派+组合实现/复用 helper）、必要时 `agent/internal/harness/history.go` 辅助、`agent/internal/tools/catalog.go`（注册）、`agent/internal/harness/*_test.go`（fake kernel/工作树 fixture 测试）。不动 webui、内核 C++、Godot、主检出。

## 验收（零栈；亲跑原文）

1. `cd agent && go build ./...` exit 0；`go test ./internal/harness -run 'ConversationOpen|Worktree' -count=1` exit 0（新增测试覆盖：同工作树单 reload；跨工作树单 reload+autosave；turn 运行中拒绝；未知工作树/节点拒绝；reload 前失败当前工程不 reload；结果披露）；
2. 对新工具做 fake kernel 命令数断言（成功路径恰好 1 次 open_project + 1 次 get_project_state）；
3. `go test ./internal/tools -count=1`（catalog 注册）与全量相关包回归 exit 0；
4. `gofmt`/`git diff --check` exit 0。

## 停止与回执

同确定性断点两次失败→停；需改内核/协议/webui→停报。回执：commit、文件、命令+exit、测试清单、fake kernel 命令计数、失败语义证据、边界（零真栈）。
