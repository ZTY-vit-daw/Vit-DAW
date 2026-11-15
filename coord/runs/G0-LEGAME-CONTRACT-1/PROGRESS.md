# PROGRESS — G0-LEGAME-CONTRACT-1（池序 370，P0）

任务：W01 乐局设计源契约冻结草案 vit.legame_design.v0（分层 schema+level_card 迁移契约+裁定项清单+实施卡草案）。

## 基线（领取时）

- 领取时 HEAD：`9cc3048ccdda11c3f889476ebfd85eea3cd46c25`（= origin/main，无被顶）
- 工作树状态（领取前已有改动，权威输入，本卡不触碰不提交）：
  - `M CURRENT-STATE.md`
  - `M VitApp/Workspace/Settings/Settings.xml`（8693 行级改）
  - `M VitApp/Workspace/default_project.xml`
  - `M coord/decisions/2026-11-11-legame-demo-experience-first.md`（+8 行）
  - `M tracktion_engine`（子模块指针）
  - 大量未跟踪文件（.vit_derived/、.zcode/、agent/coord/、agent/webui/design/* 等）
- 本卡文件域：`coord/cards/`（领取/回执）、`coord/cards/POOL-INDEX.md`（370 行）、`coord/runs/G0-LEGAME-CONTRACT-1/`、`docs/` 契约草案候审。不改生产代码。

## 步骤

| # | 状态 | 内容 | 产物/下一步 |
|---|---|---|---|
| 0 | done | 建目录+PROGRESS.md+基线记录 | 本文件 |
| 1 | done | 读卡+全部输入（design.v1.yaml rev3 / TEXTBOOK-ALIGNMENT / lesson_card.go / 裁定 §13-15） | 无产物；结论已入步骤 2 |
| 2 | in_progress | 领取协议：卡 todo→doing+POOL-INDEX 370+领取节，提交推 main | 领取 commit hash 见本表步骤 7 回填；下一步=契约主体 |
| 3 | done | 契约文档主体（分层 schema 定义+全字段覆盖表+迁移/适配契约+缓存失效+候用户裁定项清单） | docs/LEGAME_DESIGN_CONTRACT_V0.md |
| 4 | done | schema 草案（结构骨架 YAML，含注释） | coord/runs/G0-LEGAME-CONTRACT-1/legame-design-v0.schema-draft.yaml |
| 5 | done | 现行设计源对 schema 草案逐段核验（覆盖性+诊断模拟） | coord/runs/G0-LEGAME-CONTRACT-1/design-source-coverage-check.md |
| 6 | done | 实施卡草案（解析/校验/装配绑定）×3 | coord/runs/G0-LEGAME-CONTRACT-1/impl-cards-draft.md |
| 7 | pending | 卡面回执+转 done 候验收+POOL-INDEX 更新，提交推 main | 待做 |

## 命令记录（长命令先记账）

- `git add` 限定本卡文件域 + `git commit` + `git push origin main`（领取/收尾各一次；被顶则 `git pull --no-rebase`）。

## 结尾报告

- 终态：进行中（收尾时改 DONE）。
