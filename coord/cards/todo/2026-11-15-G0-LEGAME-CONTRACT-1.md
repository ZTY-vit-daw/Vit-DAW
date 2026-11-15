# G0-LEGAME-CONTRACT-1：W01 乐局设计源契约冻结草案——vit.legame_design.v0

- 发卡：GLM 主管决策流 / 2026-11-15 深夜（依据=审计 H1+[lesson-designs 课程 YAML](../../lesson-designs/music-creation-unit2/design.v1.yaml)；无工期估算）
- 派发确认：已确认（主管发卡）
- 验收负责人：主管决策流（+用户过目关键裁定项）
- 池序 370；P0；依赖=无；PC

## 目标

1. 以 `coord/lesson-designs/music-creation-unit2/design.v1.yaml`（revision 2，含 2026-11-12 裁定：调号 key_signature/拍号两层/学生作品快照归属）为输入，冻结 `vit.legame_design.v0` schema：最小意图层（字段全可选）+乐局层（objectives/materials/regions/checks/flow 必填段）分层定义（2026-11-12 §15 裁定口径）。
2. 与现有 `vit.level_card.v0`（lesson_card.go，KnownFields 拒未知字段）的迁移/适配契约：编译映射、缺项诊断、卡缓存失效策略（H3）。
3. 待用户裁定项清单（如模糊语义/两可设计点）单列，不擅自定夺。
4. 产出契约文档+schema 草案文件（`docs/` 或 `coord/`）+实施卡草案（解析/校验/装配绑定的开发卡）。

## 非目标

- 不改生产代码；不实现解析器。

## 文件域

只读+文档产出（coord/runs/G0-LEGAME-CONTRACT-1/ + 契约草案落 docs/ 候审）。

## 验收标准

schema 草案完整覆盖设计源 YAML 全字段+迁移契约可执行+裁定项清单；主管过目后转用户裁定关键项。

## 领取/回执/验收：待回填
