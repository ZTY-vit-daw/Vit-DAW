# G0-LEGAME-PARSE-1（W01-A）验收：PASS（零栈 Go）

- 卡：`2026-11-18-G0-LEGAME-PARSE-1`（池序 423）；主管=lead；执行=lead/legameparse423。
- 分支 `codex/legame-parse-423`，commit `fefb0939`；已合入 main `69760e78` 并推送。
- 交付：新包 `agent/internal/legamedesign/` 6 文件 2906 行（types/parse/validate/diagnostics/tests/testdata 副本）；未改 lesson_card.go/projectpackage/chat/webui/内核。

## 主管复核（亲跑）

| 项 | 结果 |
|---|---|
| `go test ./internal/legamedesign/ -count=1` | exit 0（22 顶级/169 子测试，191 RUN 全 PASS） |
| `go build ./...` | exit 0 |
| `go test ./internal/edu -count=1` | exit 0（现有行为不变） |
| `go vet ./internal/legamedesign/` | exit 0 |
| `gofmt -l internal/legamedesign` | 空 |
| fixture 完整性 | testdata 与 `coord/.../design.v1.yaml` HEAD blob sha256 均 `DBF3781A…6FB7`，content_equal=true（工作树 CRLF 文件哈希差异属换行形态，非内容） |
| `git diff --check` | exit 0 |

## 口径裁定（两点实现张力）

1. **R5 落点（采纳）**：源校验允许设计期显式 `null`（fixture 源校验 0E/3W/11I）；R5 的 fail-closed E 由同包 `ValidateCompileReadiness(d, CompileOverrides{})` 在编译就绪入口执行（无覆盖=3E：tempo+2 budget；给覆盖=0E）。规则号保留契约 `W-BUDGET`、等级 E（注释已说明）。**W01-B 编译入口必须调用该函数**，否则 R5 不生效。
2. **Decode 级 E-SCHEMA Path 为空（接受为边界）**：未知字段/类型错/多文档等解码期诊断在 Message 中带 yaml 行号+字段名，暂不提供机器字段路径；如后续需要 schema walker，另开小卡，不阻塞 W01-B。

## 边界

- 未实现且明确归 W01-B/C：MAT-FILE 文件存在性/哈希、catalog 核验状态 W-MAT 第三触发、§4.3 空 evidence→W、§4.11 false→W、T-15/T-16 映射降级/E-FLOW（契约 §5.2 未给规则号）。
- 不做编译器（424）、工程绑定/启用判定（425）；非乐局解析路径未实现（仅登记预留版本）。
- 零真栈/零 webui/零 LLM。
