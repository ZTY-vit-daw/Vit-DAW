# chat 包 TempDir/波动性测试证据收集记录（2026-08-26）

## 目的

针对 `agent/internal/chat` 测试波动（重点怀疑 `TestProcessorCertification*` 的 TempDir RemoveAll 报错），做连续证据收集。只运行测试与记录，不做任何代码修改。

## 执行环境

- 目录：`D:\Vit_DAW\agent`
- 日期：2026-08-26
- 命令 A（全量，5 次）：`go test ./internal/chat/ -count=1`
- 命令 B（单跑，5 次）：`go test ./internal/chat/ -run TestProcessorCertificationStartAcceptsBroadbandCompressorCapability -count=1 -v`
- 单条命令超时上限：10 分钟（本次无一条超时）

## 结果

### A. 全量 `go test ./internal/chat/ -count=1`

| # | 结果 | 耗时 | 说明 |
|---|---|---|---|
| 1 | PASS | 50.3s | `ok vit-daw-agent/internal/chat 50.325s` |
| 2 | PASS | 51.6s | `ok vit-daw-agent/internal/chat 51.572s` |
| 3 | PASS | 51.3s | `ok vit-daw-agent/internal/chat 51.284s` |
| 4 | PASS | 50.0s | `ok vit-daw-agent/internal/chat 50.042s` |
| 5 | **FAIL** | 49.8s | 失败测试：`TestProductionFreeStateRunnerObservationsSurviveDurableSlices`（见下方输出） |

全量通过率：4/5（80%）。

### B. 单跑 `TestProcessorCertificationStartAcceptsBroadbandCompressorCapability`

| # | 结果 | 耗时 | 说明 |
|---|---|---|---|
| 1 | PASS | 0.01s（总 1.9s） | |
| 2 | PASS | 0.01s | |
| 3 | PASS | 0.02s | |
| 4 | PASS | 0.01s | |
| 5 | PASS | 0.02s | |

单跑通过率：5/5（100%）。

## 失败输出（全量第 5 次）

失败的不是 ProcessorCertification 系列，而是 `TestProductionFreeStateRunnerObservationsSurviveDurableSlices`：

```
--- FAIL: TestProductionFreeStateRunnerObservationsSurviveDurableSlices (0.09s)
    continuation_scheduler_test.go:105: slice 3 observation projection inconsistent:
    result_status=waiting_continue stop=limit_reached
    executed=[map[agent_action_id: command_name: error: preview: result:map[...obs-production-3
      ...status:ok ...tool:ccb.observation_request tool_call_id:tool_step_1 ...]]
    ids=[obs-production-2] receipts=1 ledger=1
    latest=&{... observation_id:obs-production-2 ...
      views:map[mix.multitrack_relationship:map[projection_status:unsupported_projection status:ready]
              project.structure:map[projection_status:unsupported_projection status:ready]]}
```

断言位于 `agent/internal/chat/continuation_scheduler_test.go:105`：slice 3 的观察投影与已执行动作（obs-production-3）不一致，最近的 ledger/latest 仍停留在 obs-production-2，且其 views 报 `projection_status:unsupported_projection`。

## 频率与结论要点

- `TestProcessorCertificationStartAcceptsBroadbandCompressorCapability`：单跑 5/5 PASS，全量 5 次中也未失败——**本轮未复现任何 TempDir RemoveAll 报错**。
- 实际复现的波动是 `TestProductionFreeStateRunnerObservationsSurviveDurableSlices`：全量下 1/5 失败（20%），单跑未针对它单独验证。
- 推测该失败与并发/时序相关（slice 边界处 ledger 与执行记录不同步），且只在全量（其它测试并行、有 TempDir 磁盘 IO 干扰）环境下出现。

## 约束遵守

- 仅运行测试与记录，未修改任何代码。
- 未执行 git commit。
- 无单条命令超时。
