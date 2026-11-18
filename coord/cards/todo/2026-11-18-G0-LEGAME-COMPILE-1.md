# G0-LEGAME-COMPILE-1（W01-B）：编译器 design→level_card + 缺项诊断 + manifest

- 池序 424；P0；PC；发卡=本会话 PC 主管；**todo 候 423 合入**（依赖 W01-A 的解析/诊断类型）。
- 权威：`docs/LEGAME_DESIGN_CONTRACT_V0.md` v0-frozen §6（T-1..T-21）；卡片草案 `impl-cards-draft.md` W01-B。
- 目标：实现 §6.2 映射表（按 R3 声明序/mapping manifest、R4 拆分 `id#k`、R5 fail-closed、R6 三裁定修订），输出 LevelCard+compile manifest；终检复用现行 `ValidateLevelCard`。
- 文件域：`agent/internal/legamedesign/compile*`（或主管指定新包）；单向依赖 compile→edu；禁改 edu/projectpackage/chat。
- 要点：T-14 判定面三分支白名单（deterministic→结构叶；rubric→verbal_rubric；user_judgment→event 映射）；白名单外降 advisory+W-CHK，禁止文本生成判定；T-13 材料准入+catalog 状态；T-11 clip_origins/bar_span；T-17 write_boundary 表结构（接线归 425）；manifest 四元组键+W 清单+fail-closed 失效。
- 验收：`go test ./internal/legamedesign/...`（music-creation-unit2 编译产物过 ValidateLevelCard 0 拒绝；W 清单对账；缓存键变化重编译；手改 compiled 卡被 manifest 拒绝）；`go build ./...` 0。
- 依赖/停止：待 423 合入后由主管发卡；映射表全行有测试；不做 chat/装配接线。
