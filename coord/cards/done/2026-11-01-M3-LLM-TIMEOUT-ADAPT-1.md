# M3-LLM-TIMEOUT-ADAPT-1：LLM client timeout env 适配（默认 60s 覆盖路径）——纯适配卡，判别腿已撤销

- 发卡：GLM 主管决策流 / 2026-11-01 深夜；**2026-11-01 深夜修订（用户裁定）**：撤销换引擎判别重跑腿——用户模型排序修正 flash(1017)>v4-pro（pro 为更弱引擎，向更弱引擎换装不构成有效判别）；本卡缩为纯 timeout 适配小卡，**P2 排后**（240 护栏卡先行）。
- 派发确认：已确认（238 验收处置+2026-11-01 用户方向裁定）
- 验收负责人：GLM 主管决策流
- 池序 239；目标仓库=**主仓 agent+scripts**；P2（排后）/ 0.25 天 / 依赖=238 已在 main（5bbd679c）✓；L1 / PC 执行会话

## 标的（238 验收亲核，不重考）

- `agent/internal/llm/client.go:169-171`：`llmReq.Timeout<=0 && !NoTimeout` 时默认 `60*time.Second`，**无 env/配置覆盖路径**——reasoning 引擎（v4-pro OK 行中位 29.3s/p90 44.4s/越线尾）物理不兼容，238 判别被截断（12/28 超时，打断点在关键推理步）。
- 既有语义不动面：per-request `Timeout`（c1/c2 planner 等显式设置）与 `NoTimeout`/`TimeoutIsTotalBudget` 语义全部保持。

## 目标（两面）

1. **timeout 适配（生产面）**：默认 fallback 位补 env 覆盖 `VIT_AGENT_LLM_TIMEOUT_SEC`（缺省=60s 向后兼容；解析失败/越界→回落 60s 并在 telemetry/日志记一次，不静默；建议界 1–3600s）。红线：显式 `req.Timeout` 优先级不变、`NoTimeout` 不变、`TimeoutIsTotalBudget` 不变。
2. **单测**：env 未设=60s；env 设=覆盖生效（http client timeout 值断言）；非法值=回落+记录；显式 req.Timeout 优先于 env；NoTimeout 路径不受 env 影响。

（原目标 3 换引擎判别重跑腿已按 2026-11-01 用户裁定撤销；ps1 的 -LlmTimeoutSec 参数透传保留在文件域，供未来 reasoning 引擎使用。）

## 预期与诚实边界

纯确定性栈适配，生产正当性=未来任何 reasoning 引擎/引擎分层必需；不附实验腿。

## 停止条件

- 适配造成既有调用面回归（c1/c2/NoTimeout 测试红）→回退两案上交；
- LLM 面栈不可用→环境中断如实记录。

## 文件域

`agent/internal/llm/client.go`（默认 fallback env 覆盖）+`agent/internal/llm/` 测试+`scripts/dev_agent_smoke.ps1`（-LlmTimeoutSec 参数透传）。越域即停上交。

## 验收标准

- 单测五件+`go test ./internal/llm/` 全绿+既有 c1/c2 调用面零回退；纯适配卡无真栈腿义务（单测面收口，主管复跑）。

## 领取

- 领取时间：2026-11-06（池序 239 派发执行）
- 执行 owner：GLM 主会话 flash-worker 子代理（PC 执行会话）
- 领取基线：origin/main = `7864930a8de2e3a67634e37f8eaa0e63e015502c`（依赖 238=5bbd679c 已在 main ✓）
- worktree：`D:/Vit_DAW_wt_239impl`（新建，基线 7864930a，工作树干净）
- 分支：`port/m3-llm-timeout-adapt-1`（基于 origin/main 7864930a）
- 领取时主仓 `git status --short`：既有改动仅 VitApp/Workspace 设置、coord/runs 工件、.zcode/ 等非本卡文件域文件（原样保留不动）；本卡文件域（`agent/internal/llm/`、`scripts/dev_agent_smoke.ps1`）无预先改动；`git diff --stat` 同源（无本卡文件域 diff）
- 栈登记：纯适配卡零真栈腿（单测面收口，主管复跑），不占用 PC-RUNTIME-STACK

## 回执

- 执行完成：2026-11-06；实现 commit=`6c0af013`（分支 `port/m3-llm-timeout-adapt-1`，已推送 origin；基线 7864930a）；领取 commit=主仓 main `132a7c0c`+`9eebccdf`。
- diff 清单（4 files，+272/-5）：
  - `agent/internal/llm/timeout.go`（新增）：`resolveDefaultAttemptTimeout`（env 值→fallback timeout 纯函数）+`attemptHTTPClient`（构造 http.Client）+`warnLLMTimeoutFallback` 包级注入点；
  - `agent/internal/llm/client.go`：默认 fallback 位（原 ：168-172）接入 `attemptHTTPClient(llmReq, os.Getenv(...))` + `os` import；TimeoutIsTotalBudget 段零改动；
  - `agent/internal/llm/timeout_test.go`（新增）：三组测试；
  - `scripts/dev_agent_smoke.ps1`：param 块新增 `[string]$LlmTimeoutSec = ""`（LLM 参数簇、LlmRouteGroups 后）+ agent 启动前 env 注入（`if ($RestartAgent)` 之前，与 WriteLease/telemetry 同窗口）。
- 测试命令与退出码（均在 worktree `agent/` 下）：
  - `go build ./...` → exit 0；
  - `go test ./internal/llm/ -count=1` → exit 0（PASS 6.561s；新增 TestResolveDefaultAttemptTimeout 11 子例+TestAttemptHTTPClientRedlines 5 子例+TestCompleteRequestEnvTimeoutReachesHTTPClient 2 子例全 PASS，既有测试零回归）；
  - c1/c2 调用面回归：`go test ./internal/agentloop/ ./internal/chat/ -count=1` → exit 0（agentloop 7.657s / chat 129.960s，零回退）；
  - ps1：`Parser::ParseFile` 语法解析 → PARSE_OK exit 0。
- 口径取舍记录（卡面目标节 vs 修订说明）：
  1. 「建议界 1–3600s」按硬界实现：`<1` 或 `>3600` 视为越界→回落 60s+记录（0/负数/3601 均有测试钉）；
  2. 「telemetry/日志记一次」取 stderr 日志径（`warnLLMTimeoutFallback` 写 stderr），不扩 telemetryRecord 结构（避免持久化面新增字段与兼容负担）；每次适配决策记录一次（非进程级 once）——无全局可变状态、坏 env 每请求可见，回执如实申报此取舍，主管可裁定是否需改 telemetry 面；
  3. 行为级传导证明采用 >48KB payload（attempts=1）+httptest 挂 2s handler 对照（env=1s 切断失败 / env 未设 2s 内成功），零真实网络零 LLM 计费；sub-test 表观 2.00s 含 `backend.Close()` 等待 handler 睡满，client 侧实际 ~1s 收到 timeout 错误（err!=nil 断言已证）；
  4. 红线核对：显式 req.Timeout 优先级不变（env 只触 no-Timeout 槽位）、NoTimeout 不变（Timeout=0）、TimeoutIsTotalBudget 段逐字未动；`CompleteImageUnderstanding` 的 10 分钟固定 fallback 不在卡面标的内，未动（如实声明越界边界=未触碰）。
- 诚实边界：纯适配卡按验收标准单测面收口（无真栈腿义务）；ps1 参数透传未经真栈端到端验证（语法+模式先例核对），主管复跑时可用 `-LlmTimeoutSec 300` 验证 agent 日志/env 继承。

## 验收

- （主管回填）
- 验收（2026-11-06 深夜，主管）：**pass**——硬界 1-3600s 回落+红线三语义零改动（测试钉）+ps1 透传；口径四条取舍均接受（telemetry 面不扩=§11 精神）；交叉复跑 llm 包 ok+三包零回退。详见 [rulings/2026-11-06-M3-LLM-TIMEOUT-ADAPT-1-pass.md](../../rulings/2026-11-06-M3-LLM-TIMEOUT-ADAPT-1-pass.md)。
