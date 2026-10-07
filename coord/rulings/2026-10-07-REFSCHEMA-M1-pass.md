# Ruling：REFSCHEMA-M1 验收 pass（2026-10-07，决策侧）

- 对象：legacyPrefixRegistry `observation:` 翻译条目补全（实现 680dda80，cherry-pick → main **00f96200**）
- 裁定：**pass**。

## 证据链（决策侧亲核）

1. **diff 逐字核**（680dda80，2 文件 +73/-1，卡面文件域内）：条目五字段与卡面规格逐字一致（`observation:` / evidence_scheme_uri / TargetKind 空 / slot=snapshot / anchor mom/evidence.go:34-39）；注释块补 M1 来源行；测试新增 2 函数（`TestObservationPrefixDoesNotHijackAudioObservation` 双前缀互不劫持、`TestObservationHeadNoLongerOpaqueCounted` 头出 opaque 计数+observation_id:/合成头仍计）+三表行（三态/翻译/注册表契约第 15 条）。
2. **我方复跑**：`go build ./...` exit 0；`go test ./internal/agentprotocol -count=1` ok（cherry-pick 前后各一次）；消费面包 `com`/`queryengine`/`materialize` -count=1 全 ok；全量 `go test ./... -count=1` exit 0（IMPL-A 态本方跑+M1 态执行侧 88 ok 0 FAIL 声明，注册表追加零行为面）。blob gofmt 净（materialstore.go 不净系领取前既有，M1 未碰，非本卡引入）。
3. **收尾动作（决策侧代行）**：worktree `D:/Vit_DAW_wt_refschemam1` 已移除、分支 `port/refschema-m1` 已删；REF_SCHEMA_V1 §4 表一更新 15 条+条目行、G1 终审记录 §3 落账、CURRENT-STATE 计数同步（随验收 commit）。

## 备注

- 执行侧四注意点处置：①14→15 计数过时→本 ruling §2.3 已修；②worktree→已清；③materialstore.go gofmt 既有不净→记 hygiene 债（不混卡，随下次触碰该文件的卡带走或单独小卡）；④未推送→随本批验收统一推 origin。
