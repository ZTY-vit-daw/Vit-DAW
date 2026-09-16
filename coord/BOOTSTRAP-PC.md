# BOOTSTRAP-PC：PC 决策侧对话流引导与备用提示词（PC 端注记）

- 建立：2026-09-16；口令权威定义在 [BOOTSTRAP.md](BOOTSTRAP.md)，本文件只保留 PC 端专属内容与备用提示词
- 机器事实：两台电脑物理并排放置，用户可随时在 PC 侧打字激活任务——无需传令机制

## PC 端机器事实（新流恢复状态用，不依赖会话记忆）

- 主仓 `D:\Vit_DAW`（决策侧守护 main）；transfer 仓本地克隆 `D:\vit_transfer`
- shell = Git Bash（指纹命令用 `sha1sum`）；GitHub 凭据已缓存（git push 直接可用）；gh CLI 可用（`/c/Program Files/GitHub CLI/gh`，经凭据令牌登录 ZTY-vit-daw，可自建私有仓）
- Layer 3 SSH 直控：`ssh -i ~/.ssh/vit_daw_mac TimoZTY@192.168.1.138`（Mac 执行侧，2026-09-16 生效）
- watcher v2 命令见 PROTOCOL §4；watcher 是对话流的后台子进程，**流关闭即停，属预期行为**，「值班」自愈
- 工作树常态：`VitApp/Workspace/Settings.xml` 与 `default_project.xml` 两个运行时文件的本地改动为已知保留项，不提交

## 备用开流提示词（工作区路由失效时粘贴到新流）

```
读 D:\Vit_DAW\coord\PROTOCOL.md 与 BOOTSTRAP.md，担当 PC 决策侧值班（模式：先报告再动手）。
检查 D:\Vit_DAW\.git\watcher_alive：超过 5 分钟未更新才按 PROTOCOL §4 v2 命令启动 watcher
（run_in_background；Git Bash / sha1sum / 主仓+transfer 双仓指纹），已有存活 watcher 则不重复启动。
git fetch 后汇报：main 新提交、coord/ 变化（done/ 待验收卡、blocked/、decisions/ 新项）、
/d/vit_transfer 的 inbox-mac/ 到达件、origin/port/* 分支新提交。
批准前不做任何写操作（watcher 运维与只读命令除外）。
```

## 备用收工提示词（关闭流前粘贴在旧流）

```
收工 gate：汇报本日验收裁定与派卡情况、待办与阻塞，确认 coord 状态已推送、
工作树仅存已知运行时文件改动，停掉本流 watcher（TaskStop 或任其随流关闭），下一条流由用户新开。
```

## 流纪律（与 Mac 侧对称，用户裁定 2026-09-16）

- 每任务独立开流，不跨流依赖会话记忆——状态一律以仓库为准（coord/ + git）
- 同一时刻只激活一条决策流（避免重复 watcher；开流提示词含防重复守卫）
- 流可回访：旧流打字即唤醒原会话，切换不是销毁
- 「开工 <卡ID>」在本侧 = 授权启动该卡验收流程（pass/rework 由证据决定；验收内写操作含于单次授权）
