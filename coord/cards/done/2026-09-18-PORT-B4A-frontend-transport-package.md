# PORT-B4A：Godot 前端运输包制备（供 mac 重建）+ B3 worktree 清理

- 优先级 / 预估 / 依赖：P1 / 0.5 天 / 依赖 B3 已验收（分支与开发态处置事实在案）
- 模型分级：L1 / flash（机械打包+清理；尺寸判断出错再升级）
- 目标：
  - **运输包三件**（投 transfer `inbox-pc/`，供 PORT-B4B 在 mac 重建工程）：
    1. **工作树快照 tarball**：`D:\Godot\project\vit-daw-frontend` 活树**全量**（含所有未提交开发态=AGENTS §12 权威输入），**排除** `.git/`、`addons/godot_cef/bin/`（mac 按 B2P 手册 §2.2 从上游 zip 自取 universal framework；Windows bin 对 mac 无用）、`.godot/`（导入缓存 mac 重生成）；命名 `2026-09-19-vit-frontend-snapshot.tar.gz`；**单文件须 <100MB**（transfer 硬限）——若超限继续排除并逐项列入 README（首要候选：大素材/未用资源目录），不得拆分 tar
    2. **git bundle**：`port/b3-start-page` 分支 bundle；若全史 bundle ≥100MB 则改投 `c7bcd29..port/b3-start-page` 增量 bundle 并在 README 说明（历史溯源降级为增量）
    3. **README 状态清单**：打包时 `git status --short` + `git log --oneline -5` + 未提交 diff --stat + tarball/bundle sha256 + 排除项清单及其原因
  - **B3 worktree 清理**（B3 ruling 移交项）：`git worktree remove D:\Godot\vit-b3-worktree`，回执附清理后 `git worktree list` 输出
- 文件域：Godot 工程只读打包（零文件改动）+ worktree 清理；主仓仅卡片状态变更
- 验收标准：①tarball <100MB + sha256 入 README；②bundle（或增量替代+说明）；③README 三节齐（status/log/diffstat+哈希+排除清单）；④已推 transfer（回执报 commit）；⑤worktree 已移除证据
- 停止条件：排除 godot_cef/bin 后仍 >100MB → 上交候选排除清单由决策侧裁定（不得擅自拆包或塞主仓）；打包期间活树被并行改动 → 记录重做快照
- 领取：2026-09-18 23:20 / origin/main f3479ac0 / 分支 main（仅 coord/ 卡片状态变更，符合 PROTOCOL §3 执行侧直推路径）；派发消息所附决策侧追加排除清单（2026-09-18 深夜裁定）并入执行依据
- 回执：2026-09-18 23:35 完成自验，五项标准逐条：①tarball `2026-09-19-vit-frontend-snapshot.tar.gz` = 98,298,849 B（93.7 MiB <100MB ✓，原始内容 162.8 MiB/973 成员，gzip -t 通过），sha256 `f5c4fdd4aa225f936a897dfc4fd47660608e2bd60be3570797679e96c238527b` 入 README §0；归档成员级核验：排除项逐一 grep 确认缺席、必备项（project.godot/start_page.gd/两 .gdextension/保留 3 字重含 .import 侧车）确认在场；打包前后 `git status --short` 逐行一致（无并行改动，未触发重做条件）②bundle：全史 347,550,773 B≈331 MiB ≥100MB → **按卡面既定预案改投增量** `c7bcd29..port/b3-start-page`（4,385 B，3 commits e4b0ca1/546c2ac/9fd4820，prerequisite c7bcd29，list-heads 已验），降级说明在 README §2（历史溯源降级，工程重建以 tarball 为权威）③README `2026-09-19-vit-frontend-snapshot-README.md` 191 行：§3 status 31 行/log -5/diffstat（16 文件 +1212/−476）+ §0 双 sha256 + §4 全部排除项及原因（EXR 注明再生命令：主仓 `scripts/vit_baker.py`，`bake_exdr_texture(test_wav, output_path="spectrum_data.exr")`）+ 附录 exclude-from 36 条 ④已推 transfer：commit **83a3a25**（f78bd96..83a3a25，push 前已 pull 回合 mac 侧 received 归位件，无冲突；GitHub large-file warning 属正常，93.7 MiB 在硬限内）⑤B3 worktree：remove 前 status 干净（0 行）→ plain remove exit 0 → 清理后 `git worktree list` 仅余主树 `D:/Godot/project/vit-daw-frontend c7bcd29 [codex/auto-mix-session-entry]`，分支 port/b3-start-page@9fd4820 保留。零改动声明：Godot 工程除 worktree remove 外零文件改动；主仓仅卡片状态变更。覆盖边界声明：本卡为机械打包制备，无 agent 生产代码改动，不适用烟测门槛；tarball 完整性为归档清单成员级核验（含中文目录名经 find 原样落盘），未做逐文件内容比对。执行发现（供 B4B/决策侧）：排除清单未覆盖的会话杂项按活树全量如实携带——`.vit_derived/` 4.8 MiB、`8.1test/`、`8.1test1/`、根层单文件工程 `7.31A5.vit`/`7.31test.vit`/`A5完成后.vit`（与被排除的同名 `_project/` 目录是不同条目），已在 README §4 记录；活树中 `archive/` 已整体删除（git D 态），tarball 如实缺席
- 验收：pass（2026-09-19，ruling [2026-09-19-B4A-pass.md](../../rulings/2026-09-19-B4A-pass.md)：决策侧亲测 tarball sha256 一致、93.7MiB 达标；增量 bundle 降级属卡面预案内；README 三节齐+杂项诚实申报采认；worktree 清理完成；运输包三件已由 B4B 消费并归档 received）
