# PROGRESS.md — WAVESHELL-OFFRENDER-FIX-1（309）取证腿断点续跑协议

## 卡与腿

- 卡：coord/cards/doing/2026-11-07-WAVESHELL-OFFRENDER-FIX-1.md（309）
- 本腿=**取证腿（零栈先行）**：不起栈、不改代码、零 LLM、纯读档。产出 recon-design.md（取证+嫌疑分列+M1 设计+三案评估+自检断言形态+裁定请求）。
- 后续腿（不在本腿范围）：M1 实时对照（真栈，候栈排 296-R2/310 之后）/插桩加密观测/验证腿。

## 身份与基线

- owner=ZCode GLM-5.3 L2-worker PC 子代理（309 取证腿）；worktree=D:/Vit_DAW_wt_309recon @ port/waveshell-offrender-fix-1；领取基线 origin/main=8c68c186（2026-11-08，status 全净 diff 空）。
- 取证输入树：主仓 `D:/Vit_DAW/tracktion_engine` @ 60ab1a49（=b4397498 te 快照+301 插桩独立提交，子模块内干净）——**只读**。主仓子模块指针 M（gitlink b4397498 vs HEAD 60ab1a49）=301 插桩在位的预期态，不触碰不整理。
- 301 工件：`D:/Vit_DAW_wt_301repro/coord/runs/KERNEL-OFFRENDER-REPRO-1/`（evidence.md，只读）；297 工件：`D:/Vit_DAW_wt_297forensic/coord/runs/KERNEL-OFFRENDER-FORENSICS-1/report.md`（只读）。

## 断点续跑

若本腿中断，续跑者按以下步骤接续：

1. `git -C D:/Vit_DAW_wt_309recon fetch origin`，核对 port/waveshell-offrender-fix-1 远端状态与本文件进度。
2. 已完成步骤见下方清单（勾选项）；未完成项从断点继续，**不重做已勾选项**。
3. 零栈纪律不变：不起进程、不改源码、零 LLM 调用。
4. 完成后将回执回填卡面并推送 port 分支（不合入 main）。

## 步骤清单

- [x] 必读材料（AGENTS.md/PROTOCOL §0§2/卡面/301 判决链/297 报告/301 evidence.md）
- [x] 领取：worktree 建立+基线记录+卡面回填+PROGRESS.md
- [ ] 推送领取提交（origin/port/waveshell-offrender-fix-1）
- [ ] 取证 R1：tracktion_engine 渲染循环 non-realtime 全链源码读（Renderer/NodeRenderContext/ExternalPlugin/PluginNode/JUCE VST3 wrapper setNonRealtime 语义）
- [ ] 取证 R2：VitApp 渲染入口层参数面读（TransportAudioService/VitProductionCoordinator）
- [ ] 取证 R3：WaveShell 包装器架构行为链（主 DLL+Shell 子插件）与 301 三 run 观测对账 → 嫌疑分列（S1..Sn，代码锚点+吻合度）
- [ ] M1 实时对照腿设计（复用 301 M2 直挂装置）
- [ ] 修法三案评估（预热/黑名单+替代路径/包装器 workaround）
- [ ] 渲染前自检断言形态建议（2-3 形态+利弊）
- [ ] recon-design.md 成文
- [ ] 回执回填卡面+推送 port 分支（终态）
