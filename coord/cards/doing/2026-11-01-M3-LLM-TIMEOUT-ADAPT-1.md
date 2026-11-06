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

- （执行侧回填：会话/worktree/分支/领取基线 HEAD+status+diff --stat/栈登记）

## 回执

- （执行侧回填）

## 验收

- （主管回填）
