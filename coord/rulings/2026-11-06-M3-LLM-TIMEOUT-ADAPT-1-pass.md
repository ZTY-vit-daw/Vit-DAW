# Ruling：M3-LLM-TIMEOUT-ADAPT-1（239）验收裁定——pass

- 裁定：GLM 主管决策流 / 2026-11-06 深夜（自主批次）
- 输入：卡面回执（doing）+ diff（6c0af013）+ 主管交叉复跑（llm 包 ok 6.5s）

## 一、分面裁定

| 面 | 裁定 | 依据 |
|---|---|---|
| 适配实现 | pass | resolveDefaultAttemptTimeout 硬界 1-3600s 回落 60s+attemptHTTPClient 纯函数+warnLLMTimeoutFallback 注入点；11 回落+5 红线+2 httptest 行为传导子例 |
| 红线 | pass | 显式 req.Timeout 优先/NoTimeout/TimeoutIsTotalBudget 三语义零改动（测试钉） |
| 脚本面 | pass | ps1 -LlmTimeoutSec env 透传（WriteLease/telemetry 同窗口）+Parser 解析过；真栈端到端按卡面无义务（诚实边界申报接受） |
| 回归 | pass | llm/agentloop/chat 三包零回退+build 全量 |

## 二、口径取舍四条均接受

①建议界按硬界实现（更严）；②stderr 日志径每请求记录（无全局可变状态、坏 env 每请求可见）——**telemetry 面不扩**符合 §11 持久化兼容精神，如后续需要 run 级归因再议；③CompleteImageUnderstanding 10 分钟固定 fallback 未动（如实申报，不属本卡）；④换引擎判别腿按 2026-11-01 用户修订撤销。

## 三、验收动作

- 合入零冲突（4 文件无他流触碰）；卡 todo→done 随本裁定；POOL-INDEX 更新。
