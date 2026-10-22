# FS-NAMED-DISCLOSE-SUPPLY-1：user_named frontier 披露行确定性供应（159 判读缺口②继卡）

- 发卡：GLM 主管决策流 / 2026-10-22 深夜（依据=159 判读：A5/A3 两轮 unt 已解析（1032/1012）但渲染 prompt 全程无 user_named:<id> frontier 披露行，同 fixture 同 unt 的 A1 有——fold/披露面非确定性缺失 → 模型声明轨身份未披露而边界自报/域漂移；156 §① 观察供应面同族）
- 派发确认：已确认（159 RESULTS §2 判读落定）
- 验收负责人：GLM 主管决策流
- 池序 170；目标仓库=D:\Vit_DAW；**P1（比 169 更重：直接卡到达率与意图保真——A5 边界自报+A3 域漂移同源）** / 0.5 天 / 依赖=无（可与 169 并行，文件域不相交）；模型分级：L2 / GLM 执行会话

## 范围

第一步=锚点取证：user_named frontier 披露行的生成与折叠链路（prepare 侧 user_named_target 已解析 → candidate/frontier 投影 → 渲染 rows 的 fold 点）——找出 A1 命中/A5 缺失的分叉条件（候选集非平稳？frontier 竞争丢弃？fold 时机竞态？）；第二步=确定性供应：unt 非空时披露行**必须**在场（同 A1 形态），缺失=缺陷而非可选项；第三步=单测钉死（同 unt 两形态必一致）+真栈 chat 腿复刻 A5 话术对照（159 工件在卷可复用 fixture 配方）。

## 文件域

`agent/internal/chat/audio_closure_controller.go`（audioClosureUserNamedCandidates 单函数面）+`agent/internal/chat/fs_named_target_test.go`（同文件族单测）。**勘察结论固定（2026-10-22）**：分叉点=合成候选的覆盖抑制条件，不在 free_state_reasoning_loop.go——与 169 卡文件域不相交，无需等 169。越域即停。

## 验收标准

单测（unt 非空→披露行必在场+同 unt 幂等）+真栈 chat 腿=A5 话术复刻（unt=1032 已解析**且渲染披露行在场**，模型不再边界自报）+全量 0 FAIL+gofmt 净；端测边界声明。真栈腿登记 PC-RUNTIME-STACK（与 169 错峰）。

## 停止条件

披露行生成点在 semantic_entry 分类器或 CCB bounded disclosure 契约内（注入受红线约束）→ 锚点上交再裁定。

- 领取：2026-10-22 20:09 / origin/main=4e82e41e（领取时远端 tip；本卡发卡面自主管本地 main ee0050de 摘取入共享 main，同 787063ec 先例只同步 coord 面）/ owner=GLM-5.3 L2 PC 执行会话（ZCode）/ 分支=port/fs-named-disclose-supply-1 + 协调分支 coord/fnds170-claim / worktree=D:/Vit_DAW_wt_fnds170（实现期，勘察期零改动暂不建）/ 领取提交=coord/fnds170-claim tip
- 回执（2026-10-22 深夜，实现+单测腿完成，真栈腿待错峰）：
  - **锚点取证结论**：分叉条件=**生成侧覆盖抑制，非候选集非平稳/竞争丢弃/fold 竞态**（三候选假设中①为载体、②③排除）。实证链：A1/A5 同 unt=1032 同 fixture，每变体独立 conversation/goal、loop 本轮新建 unt 已解析（chat 回执 workflow_data 在卷）、fold 所有权门通过；A5 终态 frontier 有 4 个 machine 候选，其中 `candidate:17ba83a15fc30877 tracks=1017,1027,1032`（mix.multitrack_relationship/mid_band_overlap）**覆盖 1032** → `audioClosureUserNamedCandidates`（audio_closure_controller.go:1184-1190 旧行）覆盖抑制触发 → `user_named:1032` 合成候选不被 append；A5 turn2/turn3 渲染 frontier 披露行在场但列表无 user_named 行（159 判读「全程无」实为此形态——初次 grep 命中 2 次系跨臂累积快照文件里 A1 轮遗留行）。模型侧后果：1032 仅作为 machine 候选三轨之一出现，无指名绑定语义披露 → 「人声轨身份未出现在已披露工程事实」边界自报。A1 命中=其 4 个 machine 候选（tracks 1007/1012/1022 与 1017/1022/1027）恰好都不含 1032 → 不覆盖 → 合成在场。machine 候选集取决于模型当轮 CCB 观察折出（非平稳载体），覆盖纯属偶然 → 披露行时有时无。
  - **实现**：beb75ae3@port/fs-named-disclose-supply-1（已推 origin；基线 origin/main 899170dc）——抑制条件从「任何候选 track_ids 覆盖目标轨」改为「合成候选 ID 已在」（幂等语义保持），契约注释更新（覆盖≠披露）；unt 非空→披露行必须在场，空 unt no-op 不变。
  - **单测**：`TestFSNamedDiscloseSupplyDeterministicRow`（新增）——同 unt 两形态（machine 候选含/不含目标轨）合成行必一致（A1 形态逐字段钉死）+machine 事实零改写+折叠级在场+重复 fold 不堆叠；既有 `TestFSNamedTargetFoldSynthesizesCandidate` covered 段改按新契约（覆盖时仍合成）。
  - **验证**：`go test ./internal/chat` ok（106s 全量）、`./internal/agentloop` ok、全量 `go test ./...` **96 包 ok 0 FAIL**；gofmt——LF 规范化后两文件 gofmt 幂等（checkout CRLF 全仓现象与基线一致，非本卡引入）。
  - **停止条件核查**：生成点在 audioClosureUserNamedCandidates（fold 面），不在 semantic_entry 分类器或 CCB bounded disclosure 契约内——未触发。
  - **真栈腿完成（2026-10-22 21:45–21:49，用户派发补证；run=fs_disclose_chat_20261022_214407，占用 2f6c942d→释放 b1bee0d8）**：agent=主检出 **6CE34E54**（主管预备二进制，21:40 构建，含 169+170——时间线 21:37:5x 合入→构建+169 别名表特征 bytes 亲核，170 以行为面兜底）+vsphub B10CC412+K3 722A79D7 只读；驱动 softguide_exp.py（169 件复用）`--arm B1 --variants A5,B1`（A5=159 逐字失败话术「给人声加一点板式混响」复刻；B1=用户口述「给人声加点空间感」话术——话术考据：用户指令引「空间感」系 159 B1 原文，故两话术都跑）；fixture=159 六 stem 配方确定性重建。**驱动 exit 0+断言器 exit 0 三面全过**：①A5/B1 unt 均=1032 已解析 ②渲染 turn2/3 turn_directives 披露行 `user_named:1032 tracks=1032` 在场（159 A1 对照形态逐字一致）③reply/events 零命中 159 F-gate 签名（「没有出现在任何已披露的工程事实/无法圈定/任务在能力边界停止」），终局 A5=停泊确认卡（waiting_confirmation，157 A1 同族合法停泊）/B1=completed——159 A5 同话术为能力边界自报 completed。**最强对照**：两轮终态 frontier 均 5 候选=4 machine+user_named:1032，且 machine 候选恰覆盖 1032（`candidate:64f4…/f2d5… tracks=[1017,1027,1032]`，与 159 A5 抑制触发候选 17ba… 同形态）而 user_named 行共存——159 覆盖抑制形态行为翻转直接实证。LLM=12 次全 deepseek-flash（159 同模型）0 error；0 环境中断 0 原样重跑。RESULTS.md 判定面+工件随 8bce68e9@port/fs-named-disclose-supply-1 入库（重件留盘 run 目录）。
  - **端测边界声明**：真栈腿=agent HTTP/ZMQ 面 chat 链（真实内核+真实 deepseek 链+工程副本隔离）；不含 Godot GUI/webui 渲染面断言（本卡零渲染面文件）；A5 停泊确认卡/B1 completed 的提案面到达/装载/总线执行非本卡判定面（159 判读域=披露供应面）；行为面（不再边界自报）为单轮真栈事实+159 复刻对照，非稳定性证明（§8 口径，RESULTS §2 在卷）。
- 验收：（裁定文件 / 验收 commit）
