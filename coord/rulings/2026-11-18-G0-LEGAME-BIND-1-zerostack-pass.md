# G0-LEGAME-BIND-1（425，W01-C）零栈段验收：PASS（真栈待授权）

- 卡：`2026-11-18-G0-LEGAME-BIND-1`（池序 425）；主管=lead；执行=lead/legameparse423。
- commit `b78c9969`（父 2c119458；7 文件 +2464/−78）；已合入 main `bef5e9c1` 并推送。

## 主管复核（亲跑）

| 项 | 结果 |
|---|---|
| `go build ./...` | exit 0 |
| `go test ./internal/projectpackage ./internal/legamedesign -count=1` | exit 0（1.5s） |
| `go test ./internal/chat/ -count=1` | exit 0（121.6s，含新增 4 测试） |
| `go vet ./internal/projectpackage/ ./internal/legamedesign/` | exit 0（`go vet ./internal/chat/` 的 unreachable 为既有 baseline，本卡未引入） |
| `git diff --check` | exit 0 |
| gofmt | 新 4 文件 clean；改动的既有两文件在 baseline blob 即非 gofmt-clean（CRLF/历史格式），非本卡引入 |

## 交付要点

- R2：项目包根 `legame.json`（enabled/design + R5 覆盖参数）为权威，manifest.legame 镜像；无段/enabled=false=普通工程零差异；转乐局=写元数据+指向设计源。
- M1 导入：设计源树复制进包→Compile→WriteCompiled 产物 `legame/cards/<id>/`（资产副本+sha）→启用元数据；E 诊断如实回执，失败不落卡不写启用。
- binding：UUID 目录隔离（旧父目录仅缺省兼容读）；卡缓存禁 level_id 单键→四元组 manifest 键；重开/移动/revise 装配校验（命中/重编译/mismatch fail-closed）。
- 零栈证据：A/B 两工程 binding 相异且互不改写；重开/移动 cache key 不变；revise 新卡标题生效且 key 变化；手改 compiled 卡被拒；非法 revise 返回 E 不返回旧卡；enabled=false 惰性；R5 无覆盖 import=3E 不落卡。
- 真栈腿：`scripts/run_legame_bind_smoke.ps1`（S0–S7、`-StackAuthorized` 门、recents 快照还原）已写好，**未运行**；A/B 工程+栈位待主管授权。

## 边界/后续

- 新增 `agent/internal/chat/edu_jam_legame_test.go` 测试 companion（覆盖 binding 读取点，无生产逻辑）——接受，属必要测试域。
- 真栈三断言（不串 binding/重开不切版/revise 失效）待 PC-RUNTIME-STACK 登记+书面授权后执行；执行前需准备真实栈 A/B 工程。
- 卡保持 doing；真实栈通过后转 done。
