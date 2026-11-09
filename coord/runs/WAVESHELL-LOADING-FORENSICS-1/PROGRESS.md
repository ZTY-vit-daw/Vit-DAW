# PROGRESS.md — WAVESHELL-LOADING-FORENSICS-1（329）取证腿断点续跑协议

## 卡与腿

- 卡：coord/cards/doing/2026-11-09-WAVESHELL-LOADING-FORENSICS-1.md（329）
- 本腿=**取证腿（零栈先行）**：不起栈、不改代码、零 LLM、只读。产出 report.md（三代归并+装载路径锚定+闭源静态勘察+本机安装面/license 链盘点+三假设判别设计+两出口判据）。
- 后续腿（不在本腿范围）：判别腿 D1-D4（真栈，候栈排队，另领；跑前按 report §5 框架在 RUNBOOK 成文 §8 口径）。

## 身份与基线

- owner=ZCode GLM-5.3 L2-worker PC 子代理（329 取证腿）；worktree=D:/Vit_DAW_wt_329impl @ port/waveshell-loading-forensics-1；领取基线 origin/main=e8ff96c1（2026-11-09，status 全净 diff 空）；领取提交已推 main（coord/329-claim）。
- 取证输入树（全部只读）：主仓 `D:/Vit_DAW/tracktion_engine` @ 60ab1a49（301 插桩在位）；`D:/Vit_DAW_wt_309recon/tracktion_engine` @ 9b8cc8a（vit309-stackleg，VIT309 遥测+C 原型）；309 工件 @ 主检出 coord/runs/WAVESHELL-OFFRENDER-FIX-1/。
- Waves 安装面（只读，未运行任何组件、未投放任何开关文件）：Common Files\VST3、Program Files (x86)\Waves、ProgramData\Waves Audio。

## 断点续跑

若中断，续跑者：

1. `git -C D:/Vit_DAW_wt_329impl fetch origin`，核对 port/waveshell-loading-forensics-1 远端与本文件进度。
2. 未完成项从断点继续，不重做已勾选项；零栈只读纪律不变。
3. 完成后回填卡面回执并推送（port 分支+卡面协调提交推 main）。

## 步骤清单

- [x] 必读材料（AGENTS/PROTOCOL §0§2/卡面/297/301/309 三代工件全读）
- [x] 领取：worktree+基线亲证+卡 todo→doing 推 main（owner 远端核对）+port 分支推送
- [x] 归并 297/301/309 三代取证（report §1）
- [x] host 层装载路径锚定：VitApp 命令面/tracktion/JUCE VST3 装载链含 [WavesVST3UI] 留痕面（report §2.1）
- [x] Waves 闭源内部静态勘察：WaveShell/GrandRhapsody.dll/WavesLicenseEngine/InnerProcessDictionary strings（report §2.2）
- [x] 本机安装面盘点：五版本共存/目录拓扑/mtime 时间线/注册表（report §3.1-3.2）
- [x] license 链物证：wle 内容（Team V.R/DeviceID 不符/NextSync 99990909）+last.lgn 矛盾+LastRefreshLicenseFile.txt 19:07 物证（report §3.3）
- [x] 三假设重构（H1 主/H2 次/H3 弱）+判别设计 D1-D4（report §4-5）
- [x] 两出口判据预写（report §6）
- [x] report.md 成文
- [x] 回执回填卡面+推送（port 分支+main 协调提交）
