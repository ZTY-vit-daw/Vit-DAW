# KERNEL-RENDERGAIN-1：stock 内核离线渲染逐声道恒定 ~2x 增益归因（取证先行）

- 发卡：GLM 主管决策侧 / 2026-10-17 晚窗（依据=KERNEL-SENDROUTE-IMPL-1 验收中的域外发现上交，最小复现在 [PC-RUNTIME-STACK 释放记录](../../resources/PC-RUNTIME-STACK.md) 2026-10-17 节）
- 派发确认：**已确认（用户 2026-10-18 早窗 /morning 四线确认）**；验收负责人：GLM 主管决策流
- 池序 132；目标仓库=D:\Vit_DAW；P2 / 取证 0.5 天（归因后修复另裁）/ 无依赖
- 模型分级：L2 / GLM 执行会话（内核渲染链归因）
- 领取（2026-10-18 午窗）：2026-10-18 13:41 +0800 / origin/main=0c433634 / owner=**DSH PC 执行流 exec-rendergain**（本机 DSH 团队会话，12:00-18:00 决策流派发；用户 2026-10-18 午窗选择 118+132 并行）/ 分支 **port/kernel-rendergain-1**（自 origin/main 0c433634 建）/ worktree **D:/Vit_DAW_wt_rendergain1** / 领取提交=（本提交）。执行口径=**取证先行、零生产代码改动**（代码/历史考古不占栈；最小复现/差分实验需内核栈——不启 Godot UI，`dev_agent_smoke -StartKernel` 体系）；真栈腿与用户 92 手测/118 验证腿串行排队（PC-RUNTIME-STACK 排队条），Lead 放行前不得启栈；跑完拆净复核四端口/四类进程=0。协作模式（DSH 团队）：协调面由 12-18 决策流单写，实现面由执行流在独立 worktree 写。

## 缺陷实锤（K1 烟测腿发现，主检出复现=非本卡线引入）

1. stock 内核（`Export/staging/runtime/VitApp.exe`，零 K1 改动）离线渲染对渲染结果施加**逐声道恒定 ~2x（+6dB）增益**；
2. 最小复现在卷：`clear_project → add_track → import_audio(48k 立体声 0.5 幅度正弦) → start_render(16bit) → 逐声道幅度 0.9976`（输入 0.5 → 输出 ~1.0）；
3. 左独占源复现 L=0.9976 / R=0.0000——排除声道求和，增益为逐声道独立；
4. 44.1k 源（含重采样路径）同现——排除单采样率路径；
5. 诊断工件：`D:/Vit_DAW_wt_sendroute1_build/diag_*.wav` + `Workspace/Artifacts/smoke/sendroute_20261017_204856/`。

## 取证问题

1. 增益注入点归因：VitProductionCoordinator / te::Renderer / 写入器（16bit/浮点差别？）链上哪一层引入；恒定 2x 是故意的 headroom 补偿还是缺陷（查 git 历史与注释证据）；
2. **用户面影响判定**：导出/产物音频是否系统性 +6dB 偏热（对比工程内监听电平）——这决定本卡是 P2 卫生还是升级 P1；
3. 与既有烟测断言的关系：比值断言免疫（已在卷），绝对幅度断言有哪些现存面受影响。

## 文件域

取证零生产代码改动（诊断脚本+工件入 coord/runs/KERNEL-RENDERGAIN-1/）；修复腿待归因裁定后另立。

## 验收标准

三问有证据答案（代码锚点+最小复现对照+历史考古）；用户面影响有明确判定；修复建议（修 or 记录为已知行为）上交决策侧裁定。

## 停止条件

归因需要动渲染链核心且风险大 → 取证如实上交，不擅自改渲染路径。
- 回执（2026-10-18 午窗，exec-rendergain 执行，Lead 转写）：分支 **port/kernel-rendergain-1**（19b15661 archaeology → f2038da9 berth runner → **e3b4ac32** 栈腿结果，已推 origin）。**考古结论**=+6.000dB 注入点=**工程 master volume 节点**（Edit 级 VolumeAndPanPlugin 状态=fader 位置 1.0；pin 引擎 tracktion_AudioUtilities.cpp:136-156 `volumeFaderPositionToDB(1.0)=+6.000000dB`，0.7408182=0dB、0.6376282=-3dB）；排除 Renderer 归一化/写入器/import/track 增益（左独占逐声道/16-24-32bit 恒定）；模板三处硬编码 `volume="1.0"`（VitPaths.h:25-29/73-77 + 仓库 default_project.xml:14-18）+ gen_vit_project.py 的 -3dB 期望 → **缺陷（非故意 headroom）**。**真栈**=run_20261018_135710 exit 0（10/10 腿 PASS）：E1 ratio 1.995181=+5.9996dB（peak 0.99762 复现 K1）；E2 use_master_plugins A/B **1.995181 vs 0.999959**（注入点锁定 master 节点）；E3 五份仅 MASTERVOLUME 不同 fixture 映射 1.995181/0.999959/0.707918(无 MASTERVOLUME)/0.707918/0.999959 与 fader 换算精确对应；E4 new_project(blank) 1.995181；E5 16/24/32bit 1.995181/1.995121/1.995121 恒定+0.9 源 16bit peak 1.00003 clamp、24bit 1.00000 clamp、**32bit float 1.79566 不 clamp**；E6 幅度扫描恒定；E7 44.1k/单声道/左独占逐声道 1.995（L 1.995181/R 0）；E8 `list_tracks` master `volume_db=null`、内核命令面无 master 命令=不可见。被测 stock 内核 SHA256=B6565DCF…（只读）。拆栈=只杀自起 pid 22544，14:05 复核六端口（5555/5556/5557/7878/8787/4445）=0、四类进程（含 Godot 变体）=0；用户 Godot 编辑器 17188 未碰。偏差：首轮非隔离尝试向部署目录 `Export/staging/runtime/Workspace/Logs/` 写/追加 4 个运行时日志（非源码；后续 run-dir 隔离复验零写入；日志保留作证，登记 BATCH3 §二）。零生产代码改动。
- 验收：**pass（取证腿·2026-10-18 午窗）**——[rulings/2026-10-18-BATCH3-rulings.md](../../rulings/2026-10-18-BATCH3-rulings.md) §一；决策侧独立复验=远端 hash 亲核（e3b4ac32）+run artifacts 亲核（diag_summary.json 10 腿全 PASS/容差内）+teardown 六端口/进程=0 现场复核+部署目录日志偏差登记；三问均有证据答案；**用户面影响判定=所有新建/存量工程 master 总线恒 +6dB 过 unity、16/24bit 导出削波提前 6dB、无 UI/命令可读可控 → 决策侧裁定升级 P1 候选**，并立修复卡 **FIX-MASTERVOL-CALIBRATION-1（池序 134，todo，待用户确认派发）**；比值类断言免疫/绝对幅度受影响面清单见 RESULTS §0③。卡归 done。
