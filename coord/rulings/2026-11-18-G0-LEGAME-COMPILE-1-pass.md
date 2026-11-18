# G0-LEGAME-COMPILE-1（424，W01-B）验收：PASS（零栈 Go）

- 卡：`2026-11-18-G0-LEGAME-COMPILE-1`（池序 424）；主管=lead；执行=lead/legameparse423。
- commit `a9a720fc`（父 c91e8417；6 文件 +3710，零修改现有文件）；已合入 main `780e5671` 并推送。

## 主管复核（亲跑）

| 项 | 结果 |
|---|---|
| `go test ./internal/legamedesign/ -count=1` | exit 0；52 top-level / **256 RUN** 全 PASS |
| `go build ./...` | exit 0 |
| `go vet ./internal/legamedesign/` | exit 0 |
| `gofmt -l compile*.go` | 空 |
| `git diff --check` | exit 0 |

## 交付要点

- `Compile`: 源校验 → **强制 ValidateCompileReadiness**（R5）→ 材料准入/MAT-FILE → T-1..T-21 → yaml → `edu.ParseLevelCard`/`ValidateLevelCard` 终检；fixture 产物 0 E（23W/16I）并过 `edu.LoadLevelCard`。
- manifest `vit.legame_compile_manifest.v0`：四元组缓存键 + 映射表版本 + 卡 sha256 + W 清单 + receipt；`VerifyCompiled` 区分 stale/tampered/missing；`RefreshCompiled` 失效重编译、重编译失败保留旧产物但阻断使用（fail-closed）。
- R5：无覆盖=3E（W-BUDGET 等级 E），给覆盖=0E；覆盖优先并出 I-NOTE。R3 声明序+映射记录、R4 `id#k`、R6a/b/c 落地。
- first-course 卡：2/4、bpm=80、5 runtime_slots、2 assets、11 卡目标、8 nodes、allow 9 命令、budget 60/120；无文本生成判定。

## 接受边界（转 W01-C/后续）

1. T-14/T-17 只建“结构表+first-course 条目”，运行期命令面/每轮注入接线归 W01-C（425）；`c-reports` 的 submission_snapshot 在卡 v0 无判定面，暂映射最弱结构叶 `track_non_empty(result-track)`（语义张力已在代码注释记录，425 需复核/必要时回契约修订）。
2. T-13 catalog 达标口径=`generated_symbolic_checked`；扩状态须扩装配表。`sound_check=not_performed` 以 I-NOTE 可见。
3. 卡 v0 资产 key 仅大调（edu.prepareKey 限制）；R6a 小调在设计源合法，但编译期不写 key + W-MAP 可见降级（消费方限制，非 schema 拒绝）。
4. 不做 chat/装配接线；零真栈/零 webui。
