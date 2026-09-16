# coord/ 双端协作协议（PORT 期现行权威）

- 建立：2026-09-16，决策侧；细化并取代 [MAC_SESSION_HANDOVER_2026-09-16.md](../docs/MAC_SESSION_HANDOVER_2026-09-16.md) §0/§4 中"用户触发会话/转述"的部分——用户不再做内容中转，双端通过本目录 + git 轮询自转
- 适用：Mac 移植期（PORT 主线）。移植完成后本协议降级为历史记录，常规纪律回归 AGENTS.md

## 1. 目录语义

```
coord/
  cards/todo/        # 待领卡（决策侧写入；执行侧领取）
  cards/doing/       # 执行中（卡内回填领取信息）
  cards/done/        # 执行侧自验完成、待决策验收
  cards/blocked/     # 证据推翻前提/外部阻塞（必须附证据与上交点）
  rulings/           # 决策侧验收裁定（YYYY-MM-DD-<卡ID>-<pass|rework>.md）
  decisions/         # 决策与用户确认项记录（YYYY-MM-DD-<主题>.md）
```

## 2. 卡片状态机

```
todo → doing → done(待验收) ─ ruling pass   → 归档 done（卡内回填验收 commit/裁定文件）
                          └─ ruling rework → 移回 todo，卡内附返工意见
任何阶段证据推翻前提 → blocked（附证据，决策侧处置）
```

- **领取** = 卡片 mv 到 `doing/` + 卡内回填：领取时间、领取时 `origin/main` hash、执行分支名
- **完成** = 执行侧自验通过 → mv 到 `done/`；实现 commit（`port/*` 分支）与卡片状态变更同批 push

## 3. 写权限（越权即违规）

- **执行侧（Mac）**：`coord/cards/` 内的卡片状态变更可直接推 main——**这是执行侧唯一允许直接推 main 的路径**；其余一切改动（代码/文档/索引）一律走 `port/*` 分支等验收
- **决策侧（Windows）**：全仓权限；裁定与归档落在 `rulings/` 与卡内
- **冲突规则**：push 前 `pull --rebase`；coord 冲突以卡内"最后回填时间"较新者为准手工合并后重推

## 4. 唤醒机制：watcher 事件驱动（零 token 待机），心跳兜底

**架构**：双端各跑一个纯 shell 后台守望循环（不消耗 LLM token）——每 60 秒 `git ls-remote` 比对远端指纹（主仓 + transfer 仓），**发现变化即退出，退出唤醒本端会话**；被唤醒的会话处理完变更后必须重启 watcher。git push 即事件，无轮询等待。

**watcher 命令（决策侧 Windows，Git Bash，以 run_in_background 启动；v2——修复网络恢复假触发）**：

```bash
cd /d/Vit_DAW || exit 1
snap() {
  a=$(git ls-remote origin 2>/dev/null)
  b=$(git ls-remote https://github.com/ZTY-vit-daw/transfer.git 2>/dev/null)
  { [ -n "$a" ] && [ -n "$b" ]; } || { echo NETFAIL; return; }
  printf '%s\n%s\n' "$a" "$b" | sha1sum | cut -d' ' -f1
}
LAST=$(snap); [ "$LAST" = "NETFAIL" ] && LAST=SKIP
while true; do
  touch .git/watcher_alive
  sleep 60
  CUR=$(snap); [ "$CUR" = "NETFAIL" ] && continue
  if [ "$LAST" = "SKIP" ]; then LAST=$CUR; continue; fi
  [ "$CUR" != "$LAST" ] && { echo CHANGE_DETECTED: $CUR; exit 0; }
  LAST=$CUR
done
```

v2 修复说明：v1 中网络失败时 `ls-remote` 空输出经管道被哈希为"空指纹"（`da39a3ee...`），网络恢复后真指纹≠空指纹 → 假触发唤醒（2026-09-16 实测一次，远端零变更）。v2 以 NETFAIL 守卫拒绝半成功快照——两个 ls-remote 任一为空即视为本轮无效。**已按 v1 启动 watcher 的会话请用本节命令重启。**

**执行侧（Mac）同规格**：主仓实际路径 `~/Documents/Vit-DAW`，`sha1sum` 换成 `shasum`；watcher 唤醒后先 `git pull --rebase` 再看 `coord/` 变化。

**心跳兜底（双端，每 2 小时）**：检查 `.git/watcher_alive` 修改时间，超 5 分钟未更新 = watcher 已死 → 按 §4 命令重启；随后做一次完整状态轮询作为兜底。LLM 空转成本从每 30 分钟一次降为每 2 小时一次且通常只做自愈检查。

**唤醒后的职责（双端对称；2026-09-16 用户裁定 v2：先对接、批准后行动——见 decisions/2026-09-16-report-first-mode.md）**：
- **执行侧**：被唤醒后只做只读同步与汇报——同步远端变更、整理 `coord/` 变化与可领卡建议，**报告用户并获批准后**才领卡开工；自己 `doing/` 卡有新评审意见 → 同样先汇报获批再处理；`transfer/` 有新文件 → 汇报到达件，用户批准后归位
- **决策侧**：`done/` 有未验收卡 / `blocked/`、`decisions/` 新项 → 先向用户汇报候选裁定与建议，**获批准后**再写 ruling、归档或处置；transfer 有到达件 → 汇报
- 例外（无需批准即可执行）：watcher/心跳自身的运维（重启循环、状态同步）与一切只读命令
- 需要真实栈/用户在场的验收（AGENTS §5 烟测门槛类）**不自动执行**，卡内标注 `[等待真栈验收]` 由用户在场时触发
- 处理完毕**必须重启本端 watcher**（这是唤醒会话的最后一步）

## 5. 卡片模板

```markdown
# <卡ID>：<标题>
- 优先级 / 预估 / 依赖：
- 目标：（可验收的事实描述）
- 文件域：（越域 = 停下上报，AGENTS §11）
- 验收标准：（命令 + 期望退出码 / 工件）
- 停止条件：
- 领取：（时间 / origin/main hash / 分支名）
- 回执：（commit hash / 报告链接 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
```

## 6. 需要用户的事项

- 卡内标 `[等待用户:<事项>]`；决策侧轮询时汇总上报；用户裁定记入 `decisions/`
- 用户保持两端 ZCode 会话存活即可（SSH 直控模式就位后，Mac 侧会话存活也不再需要）

## 7. 变更权

本协议由决策侧维护；执行侧发现协议缺陷走 blocked 上报，不自行改协议。

## 8. transfer 仓：双端临时传输带（PC↔Mac 文件通道）

- 专用私有仓 `ZTY-vit-daw/transfer`（与主仓分离，避免论文/素材污染代码史），双端各自 clone 一份
- 用途：论文材料、参考资料、demo 素材等**主仓外内容**的双端传递；`git push` 即触发对端 watcher 唤醒
- 规则：文件放 `inbox-pc/`（PC 投递）或 `inbox-mac/`（Mac 投递）目录，文件名带日期；接收端 watcher 唤醒后汇报到达件并移入 `received/`；**非归档**——确认取走后定期清理；单文件 >100MB 不走此仓（GitHub 限制），改走 SSH scp（§9）

## 9. SSH 直控与大文件（Layer 3，待 Mac 侧一次性配置后生效）

- Mac 开启远程登录并配置决策侧公钥后，决策侧可直接 ssh/scp/rsync 驱动 Mac（拉码、构建、派活、传大文件）
- 大文件（>100MB）一律 SSH 直传，不走任何网盘或第三方
- 该层就位后，Mac 侧 watcher/心跳亦可由决策侧远程维护，用户无需在 Mac 侧开口
