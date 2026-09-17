# BOOTSTRAP-MAC：Mac 执行侧对话流引导与收工提示词（Mac 端注记）

- 建立：2026-09-16，用户直令入仓；同日用户裁定「双端同一套口令中枢」后，**口令权威定义上移至 [BOOTSTRAP.md](BOOTSTRAP.md)**，本文件只保留 Mac 端专属内容与备用提示词
- 用法：首选在新流输入口令（工作区级 `~/.zcode/workspace/default/AGENTS.md` 调度中枢路由到 BOOTSTRAP.md）；下方长提示词为**备用**（工作区文件缺失/未生效时粘贴用）

## 备用开流提示词（粘贴到新流）

```
读 ~/Documents/Vit-DAW/coord/PROTOCOL.md 与 AGENTS.md，担当 Mac 执行侧值班（模式：先报告再动手）。
检查 ~/Documents/Vit-DAW/.git/watcher_alive：超过 5 分钟未更新才启动 §4 watcher v2
（run_in_background；主仓实际路径 ~/Documents/Vit-DAW，指纹命令用 shasum），
若已有存活 watcher 则不重复启动，直接汇报。
git fetch 后汇报：main 新提交、coord/ 变化、~/vit-transfer/inbox-pc/ 到达件、todo 卡建议。
批准前不做任何写操作（watcher 运维与只读命令除外）。
```

## 备用决策开流提示词（粘贴到新流；本机亦可担当决策侧，2026-09-17 用户裁定）

```
读 ~/Documents/Vit-DAW/coord/PROTOCOL.md、BOOTSTRAP.md 与仓库根 AGENTS.md，本会话担当决策侧
（模式：先报告再动手；不启 watcher）。git pull 后汇报：done/ 待验收卡与证据摘要
（diff/测试/端测边界声明）、blocked/、todo/ 队列与下一批派卡建议、decisions/ 与 rulings/ 新项、
~/vit-transfer/inbox-mac/ 到达件、origin/port/* 分支状态。批准前不做任何写操作（只读命令除外）；
「开工 <卡ID>」= 授权启动该卡验收流程。
```

## 备用收工提示词（关闭流前粘贴在旧流）

```
收工 gate：汇报本日交付与未竟事项，确认工作树干净、卡状态已推、回执完整，
停掉本流 watcher（2h 心跳自动化会跨流自愈），下一条流由用户新开。
```

## 流纪律（用户裁定 2026-09-16）

- 每日/每任务独立开流，不跨流依赖会话记忆——状态一律以仓库为准（AGENTS §0 的既定纪律）
- 同一时刻只激活一条执行流（避免重复 watcher；开流提示词已含防重复守卫）
- 流可回访：旧流打字即唤醒原会话，切换不是销毁
- 全部流关闭的空窗期由 2 小时心跳自动化兜底（工作区级定时任务，跨流存活，会自愈重启 watcher 并轮询）
- watcher 为对话流的后台子进程：流关闭即停，属预期行为
- 「决策」口令激活的会话担当决策侧（角色跟会话不跟机器，2026-09-17）：不启 watcher，与值班流互不代启
