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
