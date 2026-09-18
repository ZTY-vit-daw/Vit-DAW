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
- 回执：
- 验收：
