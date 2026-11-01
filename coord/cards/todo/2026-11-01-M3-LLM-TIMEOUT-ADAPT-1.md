# M3-LLM-TIMEOUT-ADAPT-1：LLM client timeout env 适配（默认 60s 覆盖路径）+换引擎判别重跑

- 发卡：GLM 主管决策流 / 2026-11-01 深夜（依据=238 验收：v4-pro 判别被 60s 硬线物理截断（12/28 超时 43%+param_surface 0/24 未到达）；生产正当性独立于实验——引擎分层落地必需）
- 派发确认：待确认（238 验收处置连带首排，用户转交时视为确认）
- 验收负责人：GLM 主管决策流
- 池序 239；目标仓库=**主仓 agent+scripts**；P1 / 0.5 天 / 依赖=238 已在 main（5bbd679c）✓；L1 / PC 执行会话

## 标的（238 验收亲核，不重考）

- `agent/internal/llm/client.go:169-171`：`llmReq.Timeout<=0 && !NoTimeout` 时默认 `60*time.Second`，**无 env/配置覆盖路径**——reasoning 引擎（v4-pro OK 行中位 29.3s/p90 44.4s/越线尾）物理不兼容，238 判别被截断（12/28 超时，打断点在关键推理步）。
- 既有语义不动面：per-request `Timeout`（c1/c2 planner 等显式设置）与 `NoTimeout`/`TimeoutIsTotalBudget` 语义全部保持。

## 目标（三面）

1. **timeout 适配（生产面）**：默认 fallback 位补 env 覆盖 `VIT_AGENT_LLM_TIMEOUT_SEC`（缺省=60s 向后兼容；解析失败/越界→回落 60s 并在 telemetry/日志记一次，不静默；建议界 1–3600s）。红线：显式 `req.Timeout` 优先级不变、`NoTimeout` 不变、`TimeoutIsTotalBudget` 不变。
2. **单测**：env 未设=60s；env 设=覆盖生效（http client timeout 值断言）；非法值=回落+记录；显式 req.Timeout 优先于 env；NoTimeout 路径不受 env 影响。
3. **判别重跑腿（238 同设计+timeout 适配）**：`-LlmModelOverride deepseek-v4-pro -LlmTimeoutSec <勘察定值，建议 240s 起（v4-pro p90 44.4s+越线尾 3-4× 余量）>` 重跑组 1；载荷捕获+needle+param_surface 全开；判读面=238 两支结论（A 收敛→引擎分层上呈 / B 持有参数面仍无视→架构面归因）+三方对照表（flash 237 / v4-pro 截断态 238 / v4-pro 适配态本卡）。超时率目标 <10%（适配有效性自身判据）。

## 预期与诚实边界

timeout 适配为确定性栈修复；判别结论不预设（A/B 等价有价值）。预算：v4-pro 单 run ≤238 量级（计费上界 ≈¥1，超时行减少后 OK 行占比升→实际接近上界，如实披露）。单 run 存在性判别（§8）。

## 停止条件

- 适配后超时率仍 ≥20% → 引擎面时延与栈物理不兼容再评估（提高上限 vs 判 v4-pro 判别不可行）上呈，不连跑；
- 判别两支之一成立即收口成文（组 1 三连止损=止损，证据包照常）；
- 适配造成既有调用面回归（c1/c2/NoTimeout 测试红）→回退两案上交；
- LLM 面栈不可用→环境中断如实记录。

## 文件域

`agent/internal/llm/client.go`（默认 fallback env 覆盖）+`agent/internal/llm/` 测试+`scripts/dev_agent_smoke.ps1`（-LlmTimeoutSec 参数与 -LlmModelOverride 并排透传）。越域即停上交。

## 验收标准

- 单测五件+`go test ./internal/llm/` 全绿+既有 c1/c2 调用面零回退；
- 真栈判别重跑腿：场景腿 COMPLETED+超时率 <10%+载荷三件套+三方对照表+两支结论之一或超时率止损上呈；栈登记/释放全流程；§8 计费披露。

## 领取

- （执行侧回填：会话/worktree/分支/领取基线 HEAD+status+diff --stat/栈登记）

## 回执

- （执行侧回填）

## 验收

- （主管回填）
