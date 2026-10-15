# Worktree 存量盘点（只读勘察 · 2026-10-15）

- 执行：闲时只读勘察（offpeak-96080b7c）/ 依据=2026-10-14 gate §3-6 挂账"worktree 历史存量清理（codex/claude/jsss1 等）另排卫生项"
- 方法：`git worktree list --porcelain` 全量 18 项（主检出+17 外置）；逐项核磁盘存活、`git status --short`/`git diff --stat` 采样、分支 `merge-base --is-ancestor main` 合入态、`rev-list --count main..br` 独有提交数；对照 `coord/cards/{done,doing,todo,blocked}` 归属。
- **本任务零改动**：未执行 prune/remove/clean/reset，未触碰任何既有文件；本报告为唯一新建文件。

## 总览

- 17 处外置 worktree **全部存活于磁盘，无一 prunable**——本次无需 `git worktree prune`。
- 带未提交残迹 3 处（1 处 tracked 改动、2 处未跟踪目录/残迹），其余 14 处工作树净。
- 分支合入态：7 支已合入 main；8 支未合入（多为拆 worktree 不删分支——worktree 拆除不动分支 ref，分支去留另行裁定，不在本卫生项）。

## A 类：可拆候选（卡已收口 + 磁盘无未提交改动）—— 9 处

| 路径 | 分支/HEAD | 合入 | 卡归属与依据 |
|---|---|---|---|
| D:/Vit_DAW_wt_startfeed1 | coord/startfeed1-b @3345badc | MERGED | FE-GUI-START-FEED-1 done（10-12 验收）；tip 即卡回执 commit |
| D:/Vit_DAW_wt_region_intent_wire_1 | port/region-intent-wire-1 @bd60a799 | MERGED | REGION-INTENT-WIRE-1 done（10-04） |
| D:/Vit_DAW_wt_region_time_1 | port/vitnote-region-time-1 @0364bcc5 | MERGED | VITNOTE-REGION-TIME-1 done（10-03） |
| D:/Vit_DAW_wt_sess_sem_1 | port/webui-session-semantics-1 @45840390 | MERGED | WEBUI-SESSION-SEMANTICS-1 done（10-03） |
| D:/Vit_DAW_worktrees/l1-5-c-review-coord | codex/l1-5-c-review-done @6da1492d | MERGED | L1-5 独立评审线 done（10-09 卡组） |
| C:/Users/timoz/.codex/worktrees/l1-5-a-review-publish/Vit_DAW | codex/l1-5-impl-a-review-1-report @7b567089 | 未合入（ahead=1，评审文档 commit） | L1-5-IMPL-A-REVIEW-1 done；评审报告已另存主仓 coord/reports（2026-10-09-L1-5-AC-review-audit.md 等） |
| D:/Vit_DAW_jsss1 | port/judgment-settle-stall-1 @1a83e462 | 未合入（ahead=1） | JUDGMENT-SETTLE-STALL-1 done（10-02，卡面裁定为准） |
| D:/Vit_DAW_wt_notestream2 | port/vitnote-notestream-2 @618d69f9 | 未合入（ahead=1） | VITNOTE-NOTESTREAM-2 done（10-02） |
| D:/Vit_DAW_wt_webui_ia_1 | port/webui-ia-redesign-1 @f1faad78 | 未合入（ahead=2，tip=卡回执） | WEBUI-IA-REDESIGN-1 done（10-02） |

注：三支 jsss1/notestream2/webui_ia 未合入分支的独有 commit 在拆 worktree 后仍存于分支 ref，不因拆除丢失；分支合并或删除由决策侧另裁。

## B 类：带残迹/无卡归属，需核对后处置（不可直接拆）—— 7 处

| 路径 | 状态残迹 | 处置前置条件 |
|---|---|---|
| D:/Vit_DAW/.claude/worktrees/xenodochial-poincare-1c86a8 | **tracked 改动 2 文件 70 行删除**（agent/internal/orchestration/builtins.go + orchestrationruntime/runtime.go）；分支 claude/xenodochial-poincare-1c86a8 @f2faf805（2026-07-21 refactor: retire legacy SPAL and VPS layers，未合入 ahead=46），无对应卡 | 工作树改动视为权威输入：先归属核对（7 月 claude 实验线？），决定改动是否还有价值 |
| C:/Users/timoz/.codex/worktrees/harness-review-evidence/Vit_DAW | tracked 净；未跟踪 coord/runs/L1-5-CANCEL-ADAPTER-AUDIT-20261009/（主仓已有同名留存）+ **coord/runs/L1-5-REVIEW-AUDIT-20261009-local-preserved/（主仓无 → 疑唯一副本）**；HEAD 47ce5d75=L1-5-IMPL-A-CANCEL-FIX-1 修复 commit（卡 done） | 先把 local-preserved 目录搬移主仓（或确认他处已存）再拆 |
| D:/Vit_DAW_worktrees/g7-branch-worktree-multi-agent | 未跟踪 agent/internal/chat/.vit_history/（T3 已知历史泄漏目录残迹）；分支 @36873718（2026-08-20 feat: branch worktree multi-agent，未合入 ahead=91），无对应卡 | 残迹无保留价值但按 §12 纪律先核对；分支 8 月 codex 线去留另裁 |
| C:/Users/timoz/.codex/worktrees/3ca4/Vit_DAW | 净；detached @46176efd（2026-04-22 chore: stop tracking export artifacts） | 无卡时代产物（4 月）；无未提交内容，仅分支/detached ref 去留需裁定 |
| C:/Users/timoz/.codex/worktrees/8bf5/Vit_DAW | 净；codex/phase-c-closeout @9ee055be（2026-08-23 feat: close free-state phase c runtime，未合入 ahead=110） | 8 月 codex 线；free-state Phase C 与现行 FE-GUI Phase C(92) 无关；去留另裁 |
| D:/Vit_DAW_worktrees/eq-vps-control-graph | 净；codex/eq-vps-control-graph @6b8ee0c3（2026-07-28 feat(eq): external VPS control graph，未合入 ahead=57） | 7 月 codex 线；与后续 EQ/VPS 相关卡是否有承继关系需核对 |
| D:/Vit_DAW_worktrees/g3-gap-composer-layout-fix | 净；codex/g3-gap-composer-layout-fix @f1f9822a（2026-08-20 fix(webui): compact composer layout，未合入 ahead=91） | 8 月 codex 线；G3 复评今日挂账——若复评涉 composer 布局可先核对再处置 |

## C 类：必须保留 —— 1 处

- **D:/Vit_DAW（主检出，main @1f85627e）**——当前唯一活跃开发检出（今日 M2 体验场次二进制重建基线）。
- 说明：doing 卡 114（FE-CEF-FOCUSFIX-1）为 Mac 线（前端仓独立 git + Mac 侧 worktree），不占本仓 worktree；92 尚未领取无 worktree。本仓当前实际无在飞外置 worktree。

## 范围外登记

- E:/Vit_DAW_Backups/ADR_AGENT_CLEANUP_PHASE0A_20260721-101438/restore-drill/phase0b_commit_test_v1（detached @4a80fba7，2026-07-21 架构基线捕获）——**备份演练件**，属备份目录体系不在本卫生项清理范围；去留归 ADR 归档裁定。
- 主仓自身的未跟踪 coord/runs 工件目录（B2-RACK-VIZ-SKETCH page yml、FS-* repro 等）为运行工件非 worktree 问题，不列入。

## 建议处置序（供决策侧裁定，本任务未执行）

1. A 类 9 处：核对无新增依赖后逐个 `git worktree remove`（长路径 Windows 环境按 fsdc1/fsdc1r2/fix106 先例备 rmdir 兜底）；
2. harness-review-evidence：先搬移 local-preserved 审计目录入主仓 coord/runs，再拆；
3. claude xenodochial：先归属核对 70 行删除改动，再定拆/弃；
4. g7：核对后拆（.vit_history 残迹随拆消除）；
5. 4 处无卡时代 codex 线：分支去留裁定后拆（建议同批处理减少 Windows 路径操作次数）；
6. 全部完成后 `git worktree prune`（本次无 prunable 项，仅未来拆除后收尾用）。
