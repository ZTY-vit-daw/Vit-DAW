# Ruling：CCB-PARAM pass（2026-09-29，决策侧）

- **裁定：pass**。合入 main=2c5ab4c1（cherry-pick 7f6858e1）。
- **决策侧独立复验（非转述）**：独立 worktree @port/ccb-param 尖全量 `go test ./... -count=1` → **87 包 ok、0 FAIL**；blob gofmt 6/6 清；锚点抽核=FreeStateObservationCatalogForDimension→GetViewsForDimension 生产接线在位（free_state_observation.go:205-220，F7 钩子由此获得唯一生产消费者）；harness 7 例契约测试亲核（含三组反例：RejectsUnknownFreshnessClass/RejectsMixViewNarrowedByTargets/RejectsTopKViolations）+capabilitycontext 7 例。
- **越域申报采信**：实触 6 文件中 harness×3+tools/catalog.go 超出卡面字面域（capabilitycontext），但为目标 1"schema 追加"（CommandSpec 所在）与目标 2"接线"（ccbObservationCatalog 处理器）的实际所在面，ccb_masking 为 time_window 唯一真实应用面（探测区间）——语义内执行+显式申报，**裁定合规**。
- **采信要点**：①四增量语义护栏齐（mix.*/project.* 缩窄拒、>8 拒、replay 拒、top_k 字段⊆声明集、freshness 词表拒）；②time_window 仅 masking 预备探测真实生效+bundle limitation 如实声明（诚实边界）；③T11 半边（on 态透传/off/shadow 不变）测试锁定；④独立 worktree 纪律执行到位（本卡是三卡中唯一规范使用 §3 worktree 的）。
- **移交注意**：tools/catalog.go 与 IMPL-C 的 CommandSpec 注册同文件不同 hunk——已注记 IMPL-C 卡面，领取时实锚。
