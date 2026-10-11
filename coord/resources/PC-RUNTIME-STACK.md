# PC 真实运行栈占用记录

本记录是共享卡池协作的资源登记面，操作协议见 [PROTOCOL.md](../PROTOCOL.md) §2.2。
初始化“未核查”只表示尚未核对当前进程；不声明栈空闲，不接管任何现存进程。

## 当前占用

- **占用中**：2026-10-11 22:55 起 / FE-GUI-SHRINK-IMPL-1 真栈烟测腿（GLM-5.3-Flash · PC · ZCode flash 执行流会话；前端仓分支 port/fe-gui-shrink-1 @ c56e38f，主检出 D:/Godot/project/vit-daw-frontend）。计划命令=`dev_agent_smoke.ps1 -StartKernel -StartUI -MixSmoke`（7878/5555/5556 + Godot 前端）。登记前实测（M1-4 释放记录后复核）：端口 7878/5555/5556 监听=0，VitAgent/VitApp/Godot 进程=0。

## 最近一次释放

- 2026-10-11 22:04–22:11 +0800（本机钟）/ MIXLAYER-M1-4（GLM-5.3 PC 执行流，ZCode 会话；worktree D:/Vit_DAW_wt_mixlayer_m1_4，分支 port/mixlayer-m1-4）：fs_largeproject_smoke.ps1 -D1D3Observation 一 run=**PASS exit 0**（D1D3OBS 五门全过：观察报告 schema 行 d1d3.premaster.observation.v0+D1/D2/D3 三面行+D3 参考曲命令缺口登记行+只读声明+A4 固定编排标记族零命中；A1/A4 pass；A2/A3 软记录，classification=capability_blocked 与 FS-LARGEPROJECT-SMOKE-1 基线同族归其返工线——M1-2 ruling 已接受同语义；栈自 teardown torn_down=true，agent pid 5956 / kernel pid 17452 已停）；22:12 后复核端口 7878/5555/5556 监听=0、VitAgent/VitApp 进程=0。占用登记 8e0e52ad（登记时间误写 22:35，实际提交 22:04:34、run 启动 22:06:35——此处修正，同 M1-3 修正先例）；被测=工作树（=实现 commit 7a31c6d5 内容，run 时实现未提交，head.txt=93e23954 在卷）；run 工件精简件随 7a31c6d5@port/mixlayer-m1-4 入库（coord/runs/FS-LARGEPROJECT-SMOKE-1/fs_largeproject_smoke_20261011_220635），详见卡回执。

- 2026-10-11 21:04–21:15 +0800（本机钟）/ MIXLAYER-M1-5（GLM-5.3 PC 执行流，ZCode 会话 sess_155c5ae2；worktree D:/Vit_DAW_wt_mixlayer_m1_5，分支 port/mixlayer-m1-5）：fs_largeproject_smoke.ps1 -M1QueueRoundtrip -M1FiveThreeView 两 run——210405=**首轮 PASS exit 0**（M15 八门全过：mix_report invoke ok+schema+最近 settle 结论行+queue 节点状态行 23 节点+E1 延期可见+双折叠一致 0=0+settle 行真实性+A4 零标记；M1Q 四门+A1/A4 pass；round 形态 goal_terminal 无 settle→settle 行走无记录诚实分支）；210903=第二轮同形态 PASS exit 0（§8 止损：不再加跑，有结论分支归单测覆盖）；两轮均脚本自 teardown（ports free, processes gone），末轮后实测端口 7878/5555/5556 监听=0、VitAgent/VitApp 进程=0。占用登记 435ebd4b；被测 HEAD=8e9ce5f1（实现 commit，两轮 head.txt 在卷）；run 工件精简件随 6cc0ec06@port/mixlayer-m1-5 入库（coord/runs/FS-LARGEPROJECT-SMOKE-1/fs_largeproject_smoke_2026101{1_210405,1_210903}），详见卡回执。

- 2026-10-11 20:49–21:16 +0800 / MIXLAYER-M1-3（GLM-5.3 PC 执行流，ZCode 会话；worktree D:/Vit_DAW_wt_mixlayer_m1_3，分支 port/mixlayer-m1-3）：fs_largeproject_smoke.ps1 -C3Observation 一 run=**PASS exit 0**（C3OBS 三门全过：观察报告 schema 行+只读声明+A4 标记族零命中；A1/A4 pass；A2/A3 软记录，classification=capability_blocked 与 FS-LARGEPROJECT-SMOKE-1 基线同族归其返工线——M1-2 ruling 已接受同语义；栈自 teardown torn_down=true）；21:16 后复核端口 7878/5555/5556 监听=0、VitAgent/VitApp 进程=0。占用登记 e7645a65（登记时间误写 21:05，实际占用自 20:49 run 启动——此处修正）；run 工件 coord/runs/FS-LARGEPROJECT-SMOKE-1/fs_largeproject_smoke_20261011_204949（随实现 commit 入库），详见卡回执。

- 2026-10-11 20:05–20:16 +0800 / MIXLAYER-M1-2（GLM-5.3 PC 执行流，ZCode 会话；worktree D:/Vit_DAW_wt_mixlayer_m1_2，分支 port/mixlayer-m1-2）：fs_largeproject_smoke.ps1 -M1QueueRoundtrip 两 run——194717=首轮（M1Q 读面过/注释命令被语义入口吞→chat 拦截修复+门槛语义补丁，round 形态 improvement_proposal 与 FS-LARGEPROJECT-SMOKE-1 基线同族）；200448=**末轮 PASS exit 0**（M1Q 读→标注 E1 延期→重读往返四门全过+A1/A4 pass+A2/A3 软记录，栈自 teardown）；20:16 后复核端口 7878/5555/5556 监听=0、VitAgent/VitApp 进程=0。占用登记 cf46f950；run 工件 coord/runs/FS-LARGEPROJECT-SMOKE-1/fs_largeproject_smoke_2026101{1_194717,1_200448}（已随实现 commit 6511d700 入库精简件），详见卡回执。

## 历史释放（M1-2 期间覆盖的上一条）

- 2026-10-11 11:05–12:18 +0800 / MIXLAYER-M1-1（GLM-5.3 PC 执行流，ZCode 会话 sess_30083b31；worktree D:/Vit_DAW_wt_mixlayer_m1_1，分支 port/mixlayer-m1-1）：settle_deliver_smoke.ps1 -M1Survival 共 9 run（105423/110451=judgment_not_armed 模型随机分支×2 触发止损→脚本场景修复；111730/112607=判定命中但行面失败→取证定位原卡 $NeedleSettle/$Round2Text B64 非法 UTF-8 遗留 bug 并修复；113450=M1 门通过但重启探针暴露 draft 隔离→门改跨会话读回；114450/120538=无 settle 形态；115519=M1 门完整通过（当轮 exit 1 系行面坏 needle）；121359=**末轮全链 exit 0**：判定 POST→M1 记录落盘+新会话黑板读回(improved)→行面 1 行→round2 无 stale→重启探针留痕 empty_draft_isolation），各 run 均脚本自 teardown；12:18 后复核端口 7878/5555/5556 监听=0、VitAgent/VitApp 进程=0。占用登记 32fcab06；内核复用主检出 B6565DCF85D1DA86（C++ 面零改动）；run 工件在本机 coord/runs/SETTLE-DELIVER-SMOKE-20261011_*（worktree 内），详见卡回执。

## 历史释放（归档）

- 2026-10-10 20:16–20:50 +0800 / FS-LEDGER-PERSIST-1（GLM-5.3 PC 执行会话，ZCode D:\Vit_DAW；worktree D:/Vit_DAW_wt_fslp1）：三 run（201713/202326=取证插桩复现轮，203348=修复后回归轮，均脚本自 teardown：末轮 console "stack released: ports free, processes gone"），20:50 复核端口 7878/5555/5556 监听=0、VitAgent/VitApp 进程=0。占用登记 0d3d90cc；实现 commit db6f2b11@port/fs-ledger-persist-1；回归 verdict FAIL(A3 模型行为) 但 A2 站图 PASS=缺陷修复面，详见卡回执。E:\ 三轮 mid-check 干净。

## 历史释放（归档）

- 2026-10-10 18:30–19:15 +0800 / FS-LARGEPROJECT-SMOKE-1（GLM-5.3 PC 执行会话，ZCode D:\Vit_DAW；worktree D:/Vit_DAW_wt_fsLPS1）：四 run（183305/183914=工具性中止，栈由执行侧人工拆除核清；184500=$pid 崩溃后人工拆除核清；185519=末轮脚本自 teardown torn_down=true）；19:15 复核端口 7878/5555/5556 监听=0、VitAgent/VitApp 进程=0、Godot vit-daw-frontend=0。占用登记 de1d441b；实现 commit 2291d055@port/fs-largeproject-smoke-1；verdict FAIL（A1/A4 pass，A2/A3 fail）详见卡回执。期间 185519 一轮干净自释放，前三轮人工拆除均在 ABORTED_NOTE/RUN_NOTE 留痕。

- 2026-10-09 21:41–21:47 +0800 / G3-ATTRIB-1（GLM-5.3 执行会话，D:\Vit_DAW_wt_g3a1）：harness_ab 四相 run G3-ATTRIB-1/20261009_214138_harnessab SCRIPT-LASTEXITCODE=0；释放证据=脚本 berth teardown（agent pid=23600 / kernel pid=18636 已停）+ 21:52 复核端口 7878/5555/5556 监听=0、VitApp/VitAgent 进程=0；期间无脚本缺陷重跑、无栈残留。登记提交 82a654ad（被测 HEAD c4c015c5，rebase 后实现提交 hash 以分支为准）；内核复用主检出 B6565DCF85D1DA86（C++ 面零改动）。

## 历史占用（归档）

- 2026-10-09 19:47–20:10 / L1-5-IMPL-D（GLM-5.3 执行会话，D:\Vit_DAW_wt_l5d）：harness_ab 双模 run L1-5-IMPL-D/20261009_195558 exit 0；已释放（20:10 核对进程=0、端口 7878/5555/5556 监听=0）；详见卡回执。

