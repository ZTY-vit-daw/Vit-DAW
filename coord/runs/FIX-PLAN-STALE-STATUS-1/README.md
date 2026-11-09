# FIX-PLAN-STALE-STATUS-1 工件（330 卡，零栈单测）

- 领取基线：origin/main `b9b8c76ae85293b3aaa00d01d1e8a0a70e935932`（含 327 合入 c8bb576e）
- worktree：`D:/Vit_DAW_wt_330impl`（分支 `port/plan-stale-status-1`）
- Go：go1.26.2（本机 PATH）；§8 口径：全部为确定性 Go 单测，**零 LLM 参与、零真实栈占用**（无 run ID、无计费；未登记 PC-RUNTIME-STACK——零栈卡）。

## 红转绿记录（卡面目标③）

- `red_run_prefix.txt`：新增三态测试对**修前代码**先跑——8/8 FAIL，逐条钉住修前缺陷形态：
  - 重复 approve/过期 plan/未知 plan → 404 `{"status":"error","message":"plan not found"}`（决定被裸吞）；
  - stale 别名残迹 → **200 +「这个交互已处理或已过期。」+ goal_status=completed 伪造**（327 登记的 plan 面同型缺陷活体）；
  - chat 投影转换把空 goal_status 回填为 failed（同一回填兜底的另一伪造面）。
  - 命令：`go test ./internal/chat/ -run "TestPlanDecision|TestNewPlanAfterRetirement|TestHandleConfirmUnknownPlan|TestChatResponseFromPlanDecision" -count=1` → exit 1（预期红）。
- `green_run_targeted.txt`：修复后同命令 `-v` → 8/8 PASS，exit 0。
- 既有断言核对：全树 `*_test.go` 无任何测试钉住 plan 面旧罐头文案或 plan-not-found 404（grep 亲证，webui 仅按状态词根做客户端台账守卫、不依赖被删文案）——故「既有断言修订」以红转绿新增测试在卷替代，无存量断言需改。

## 回归

- `chat_full_package.txt`：`go test ./internal/chat/ -count=1` → **ok 141.838s，exit 0**（全包不回退，含 327 六测与全部存量）。
- `green_run_interaction_family.txt`：327 族定向 `go test ./internal/chat/ -run "TestInteractionRespondStale|TestInteractionRespondTimely|TestInteractionSurvivesWorkspace|TestInteractionOlderThanNewer|TestInteractionGuardExpiry|TestNewProposalAfterRetirement" -count=1` → ok，exit 0（跨卡语义无回退）。
- `go build ./...` → exit 0；`go test ./internal/pendingmanager/ -count=1` → ok exit 0（邻接域）。
- `go vet ./internal/chat/` → exit 0（semantic_treatment_strategy.go 两处 unreachable code 为领取基线既有、本卡未触该文件，域外不动）。

## 端测边界声明（AGENTS §5）

本卡自验=Go 单测面（chat 全包绿）。真实栈面/渲染面/用户旅程未测：plan 决策面回执的端侧形态（webui 呈现新两态文案）未起栈验证；两态回执为确定性服务端逻辑（与 327 已真栈活体证成的 interaction 面同构），候主管裁定是否足以进入验收或需搭后续栈窗口便车腿。
