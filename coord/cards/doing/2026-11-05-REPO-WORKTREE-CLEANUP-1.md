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

- **Phase 1 完成（检查点上交）2026-11-05**：报告=[coord/reports/2026-11-05-worktree-cleanup.md](../../reports/2026-11-05-worktree-cleanup.md)。
- 盘点结果：142 项注册 worktree 全在场、无 locked/prunable；A 类 78（干净+已合入，建议删）/ A₀ 27（干净未合入，删树留支待裁）/ B 19（dirty，弃置 12+归档 3+上交 4 组建议）/ C 13（codex 8+claude 1+本流 detached 3+E 盘 1，不判删）/ 硬保留 5；分支层 241 支已合入 port/coord（147 孤儿可直接 -d）；磁盘预估 A 档 ~55GB / 全弃置档 ~89GB（±30% 抽样法，Phase 2 逐项 du 前后对照出精确值）。
- 勘误记录：dirty 尺寸首轮相对路径误测，已全量绝对路径重算（dirty_detail2.txt）；A₀ 表 20 支 ahead 数未逐支计（同型 1–2），Phase 2 按需补。
- **等待主管+用户裁定 §9 七项后再领 Phase 2**；Phase 1 零 git 写操作（除协调提交）、零真栈、零 LLM 成本；与 253 无 git 写冲突窗口（其 worktree 独立 index，本卡写操作仅领取/回执两笔协调提交，瞬时完成）。
- 主检出 status 与领取基线一致（差异仅本卡卡面+报告两个 coord 文件）。
- 推送冲突处置记录（一次性）：回执首推被拒（远端期间进 252 领取/done+253 占用登记三笔，文件域零重叠）；主检出未提交改动不可 stash/rebase，按 PROTOCOL §2.1.3/§3 走临时协调 worktree cherry-pick 重放推送成功（origin/main=6d57de55）。**主检出本地 main 停在等价补丁 e9efb0cd（ahead1/behind4）**，主管侧下次 `git pull --rebase` 会自动跳过该等价提交自愈；工作树与基线一致不受影响。

## 验收

- **Phase 1 pass（2026-11-05，GLM 主管决策流）**——盘点质量核可（142 项全分类+勘误自曝+分支层+磁盘抽样+推送冲突 §2.1.3 合规处置；主检出自愈已亲核 local=origin）。**七项裁定如下（主管部分；用户 go 后 Phase 2 生效）**：
  1. A 类 78 项：**批准删**（worktree remove+分支引用 `git branch -d` **仅限 -d**——git 自拒未合入支为机械保护；例外：wt_252impl 删树**留支** port/smoke-guidance-check-discrim-1，其实现系 cherry-pick 入 main，tip 不可达，引用保留）；
  2. A₀ 类 27 项：采纳建议=**删树留支**；分支废留另裁（不急，引用零成本）；
  3. B 类：弃置组 12 **批准**（含 klte1 2.7GB——KERNEL-L3-TIMEENERGY-1 卡已验收合入+run exit 0 归档，§6.1 留存件使命完成）；归档组 3 **批准**（pgtdc_compat 前置=研究报告先入库）；上交组 3 项=**Phase 2 领取时逐条现场裁定**；
  4. C 类：**不发起跨流主动确认**（codex 8/claude 1/E 盘演练 1 保留原地不动）；本流 detached 3 项**并入 A 组处置批准**；
  5. 硬保留 5 项维持；wt_228 **不降级**（kernel f9e3da0a 复用源+build 面在役）；wt_249run/wt_251run **降级可删**（卡已验收+工件入库，payload 留存件弃置）；
  6. 147 支已合入孤儿分支：**随本轮一并 -d**（仅 -d）；
  7. 磁盘回收：**最大档**（A+A₀ 树+B 弃置组+两降级 worktree），Phase 2 逐项 du 出前后对照精确值。
- Phase 2 待用户对删除量 go 后领取执行。

## Phase 2 裁定补充（2026-11-05 用户 go 后，主管补裁）

1. **上交组 3 项终裁：保留**——wt_topoplan154（timefx-bus-topoplan-1，ahead=12）/wt_writegate155（timefx-bus-writegate-1，ahead=2）/wt_tracklist1（track-list-gap-1，ahead=5）均含**未合入实现提交**，属权威输入（AGENTS §12）；标记为 **A-F 主线开线勘察输入**，归属勘察后另裁。本轮不动。
2. klte1 终裁：build-klte1/ 2.7GB **弃置**；worktree 按 **A₀ 式删树留支**（分支 kernel-l3-timeenergy-1 ahead=2 不删）；xml 改动随树弃置（卡已验收+run 归档）。
3. wt_253impl 状态更新（盘点时在飞，现已验收合入）：按 A 类处置（树删；分支 port/m3-timewindow-intent-arm-1 系真合并可达，`-d` 可删）。
4. 主检出内残件（todo/ 下 252 空白模板）由主管自清，不属 Phase 2 域。
5. Phase 2 执行域=原裁定 1/2/3（弃置+归档组）/4（detached 并 A）/5（wt_249run/wt_251run 降级）/6/7 不变；执行前后各跑一次 `git worktree list`+`git status --short` 存证，磁盘回收以逐项 du 前后对照出精确值落报告 Phase 2 节。
