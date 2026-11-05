# REPO-WORKTREE-CLEANUP-1：worktree 积压整理（盘点→裁定→清理）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=gate 2026-11-05 首排「worktree 整理」+本日盘点 160+ worktree 积压实测）
- 派发确认：已确认（主管发卡；用户口头开工即有效）
- 验收负责人：GLM 主管决策流
- 池序 250；目标仓库=主仓 worktree 注册表；P2 / 0.5 天 / 无依赖；L1 / PC 执行会话（本地 git 操作，零 LLM 成本）；**不占真实栈**

## 背景

2026-11-05 盘点：主仓注册 worktree 160+，大量为 207-248 链时代遗留的 impl/claim/coord/stack worktree。多数 HEAD 已合入 main、工作树干净，可安全回收；少数含 dirty 文件（多为按 §6.1 工件纪律「留 worktree」的 payload/prompt_render 留存件），删除前须裁定归档或弃置。

## 目标（两阶段，中间设裁定检查点）

1. **Phase 1 盘点（只读）**：逐 worktree 产出分类清单——
   - A 类=HEAD 已在 main（`git branch --merged main` 覆盖或 tip 哈希可达 main）+工作树干净 → 建议删；
   - B 类=dirty（列全部 dirty 路径+大小；payload/日志留存件单独标注）→ 待裁定归档/弃置；
   - C 类=detached HEAD / codex·claude 流 worktree（`C:/Users/timoz/.codex/*`、`D:/Vit_DAW/.claude/*`、`codex/*` 分支）/ E 盘恢复演练 worktree → 列清单不自动删（跨流归属，须各流确认）；
   - **硬保留**：`wt_228`（kernel f9e3da0a 只读复用源，235-249 在用）、`wt_249run`（若已建，249 判别 run 在飞）、主工作树 `D:/Vit_DAW`。
2. **检查点**：A/B/C 三清单+磁盘回收预估上交主管与用户裁定（B 类逐条给出归档或弃置建议）。
3. **Phase 2 执行（裁定后）**：仅删除裁定通过项；worktree remove 遇 dirty 需 `--force` 时，仅限已裁定弃置项；同步删已合入的 `port/*`、`coord/*` 分支引用。

## 约束

- 禁止 `git reset` / `git clean` / 覆盖式 checkout（AGENTS §12）；删除动作仅限 worktree remove+分支 -d/-D（-D 仅限已裁定项）。
- 与 249（M3-GROUP23-DISCRIMINATE-1）错峰：249 建 worktree/入库的 git 操作窗口内暂停本卡 git 写操作（index.lock 冲突面）；Phase 1 只读可随时。
- 不触碰真实栈进程/端口；不动 main 工作树。

## 验收标准

- 清理报告入库 `coord/reports/2026-11-05-worktree-cleanup.md`（删除/保留/待裁定三清单+各分支处置+磁盘回收量前后对照）；
- `git worktree list` 只余裁定保留项；`git fsck` 无新错误；主仓 `git status` 与开工基线一致（本卡零源码 diff）。

## 领取

- 2026-11-05 18:41 / origin/main=76305d87056ff0e4ec3f1eb217e9bbbb7d7f4f80（=本地 HEAD，ff 可推）/ owner=GLM-5.3 PC L1 执行会话（ZCode 本会话）
- 无开发 worktree：本卡只读盘点在主检出 `D:/Vit_DAW` 执行（发卡侧授权）；协调提交直接落 main
- 领取基线：HEAD 76305d87；`git status --porcelain` 369 项（快照存 `$TEMP/wtcleanup250/baseline_status.txt`）；`git diff --stat` 8 文件 4469+/4429-——全部为领取前既有改动（含 coord/cards/blocked→done 的 2026-10-30-M3-CHAT-AUTH-STAMP-1 未提交移动与 VitApp/Workspace 运行态），非本卡产物，本卡不碰
- 卡面在领取前为本地未跟踪状态（发卡侧尚未入库）：领取提交连同卡面一并入库（todo→doing+回填一次提交）

## 回执

- （执行侧回填）

## 验收

- （主管回填）
