# WAVESHELL-LOADING-FORENSICS-1：WaveShell Grand Rhapsody 装载/license/壳层静音取证（309 M1 判别转卡）

- 发卡：GLM 主管决策流 / 2026-11-09（依据=[309 终验](../done/2026-11-07-WAVESHELL-OFFRENDER-FIX-1.md) 裁定②——M1=RT-SILENT 判别性结果：GR 在四套 harness[chat-rack M0a2/直挂渲染/离线渲染/实时播放]一致全零静音，E1ctrl Surge 同装置出声证装置健康→静音归属=装载/license/壳层[S2/S3 族]，超 309 文件域转卡）
- 派发确认：已确认（309 终验裁定②）
- 验收负责人：GLM 主管决策流
- 池序 329；P2 / 1 天 / 依赖=309 工件在库（stackleg_20261109_1906 八 run+RUNBOOK-STACKLEG）；L1-L2 / PC；取证先行零栈，复现腿需栈

## 目标

1. **取证面锚定**：WaveShell1-VST3 包装器的装载路径（shell→shellnode 插件实例化时序）、license 检查（iLok/Waves Central 在无 GUI/离线渲染上下文的行为）、壳层初始化失败时的日志/返回码——vendored JUCE 与 Waves shell 交界（297/301/309 三代取证在卷，先归并）；
2. **假设分列**：至少分出「license 在非交互上下文拒绝激活」「shell 延迟实例化在离线窗口外」「采样率/块长组合触发壳层哑火」三假设的判别实验设计（§8 口径成文：运行次数/成功条件/失败分类/止损线）；
3. **判别腿候栈**：一次栈窗口跑判别实验（可用 309 装置复用：-WaveshellStacklegSmoke + vit309-stackleg engine 分支遥测在本地可续用）；
4. **结论面**：可修（host 侧 workaround）→ 给修法；不可修（闭源二进制内）→ 定「黑名单+替代路径」策略方案（309 原停止条件的正式出口）+warn_only 默认维持的产品确认。

## 文件域

取证=只读；判别腿=scripts+工件（coord/runs/WAVESHELL-LOADING-FORENSICS-1/）；engine 本地分支续用不推远端；VitApp 改动候判别结果另裁。

## 验收标准

取证报告（三假设判别设计+§8）+（栈腿后）判别结论+修法或策略方案。零 LLM 优先（判别实验全确定性）。

## 停止条件

判别需要 Waves 侧调试符号/license 环境侵入 → 取证上交转纯策略方案裁定。

## 领取/回执/验收

- 领取：2026-11-09 21:0x +0800 / origin/main=e8ff96c184ca6a8fde7f04132e6661595bbd31c4 / owner=ZCode GLM-5.3 L2-worker PC 子代理（329 取证腿）/ 分支 port/waveshell-loading-forensics-1 / worktree D:/Vit_DAW_wt_329impl（领取基线：HEAD=e8ff96c1，status 全净，diff --stat 空）/ 领取提交=本笔 coord/329-claim（卡 329 todo→doing）。本腿=取证先行零栈（判别腿候栈不在本腿）。
- 回执（取证腿完成 2026-11-09，判别腿候栈另领）：
  1. **工件**：`coord/runs/WAVESHELL-LOADING-FORENSICS-1/report.md`+`PROGRESS.md` @ port/waveshell-loading-forensics-1（领取 a53137f9→工件 9a9426c9，均已推远端）；零栈零 LLM 零源码改动（Waves 侧全只读，未运行组件、未投放开关文件；两棵 engine 树只读）。
  2. **三代归并表**（report §1）：297=三候选并陈+守卫主嫌疑（渲染图/MIDI/rack/合路全链锚点入卷）→301=守卫零命中排除+M2/M3 对照定性插件特异性（实例在+prepared+process 被调而输出全零=静音在闭源插件内部）→309=M1=RT-SILENT 推翻「non-realtime 特异」（四 harness 一致全零）+S1 确证但非成因+修法 B warn_only 落地。329 前提=静音在「装载后、处理前」的壳内初始化/license 面，与渲染模式无关。
  3. **装载路径锚定**（report §2）：host 层 L1-L7 四道壳感知拦截面（class_selection/component_create/identity_validation/hasSharedContainer）全带 `[WavesVST3UI]` 留痕（pinned snapshot 既有 patch）且 309 全绿=host 装载未失败；Waves 闭源内（strings）=proxy 架构（WCPluginLoader::LoadPlugin 与 VST3 实例化分离，eFaildToLoadPluginDLL/eInitPlugin 显式错误枚举）+**license 状态机（IsPluginLicensed/SetIsSessionLicensed/UpdateDemoMode）与 demo 静音机制（IsMuteOnDemo/WP_SetMute/WP_SetOutputMute）+license GUI 对话框**双侧（壳+GrandRhapsody.dll）在卷。
  4. **本机 license 链物证（核心新发现，report §3）**：wle=第三方破解授权（DeviceName=Team V.R、DeviceID 7b2c15836a10≠本机 08BFB841E625、NextSync=99990909 伪造、批量同 LicenseID）；LicenseEngine.dll 与 wle 同日 mtime（2025-03-23）；last.lgn 本机记录 NextSync=20260825 已过期；**LastRefreshLicenseFile.txt mtime=2026-11-09 19:07=309 E2a 装载窗口 touch=装载期 license refresh 路径真实执行**；LicenseEngine 有 valid/invalid 校验分支与 noautowlesync.txt/tmpDisableLicenseDialog.txt 免侵入开关（未投放）。五 WaveShell 版本共存（14.12/14.21/16.7/17.1/ARA）。
  5. **三假设判别设计**（report §4-5，§8 口径成文）：H1 license 非交互拒绝（**主嫌疑**——破解 wle×正版校验不匹配+无 GUI 上下文对话框不可显→IsMuteOnDemo 静音，与四 harness 全零逐项吻合）/H2 壳延迟实例化（次，与 H1 非正交）/H3 sr-bs 组合（弱，M1 实时 480 同静音已压制）。判别腿：**D1 GUI 会话对照**（open_plugin_editor→license 弹窗/demo banner 观测→关窗后电平复测，H1 判别器）+D2 license 文件/进程差分（WavesPluginServer spawn 观测）+D3 延时 60s 对照（H2）+D4 bs=480 对齐渲染（H3 完备性）；每腿单 run 零 LLM，复用 309 装置（-WaveshellStacklegSmoke+vit309-stackleg 遥测+`[WavesVST3UI]` 既有日志面免插桩）。
  6. **两出口判据**（report §6）：出口 A（可修）=D1 GUI 预热/D3 就绪等待/D4 bs 对齐任一出声→VitApp-only workaround（与 warn_only 正交）；出口 B（不可修）=D1-D4 全静音→静音锁定闭源 license 校验（本机破解授权物证已显著上调出口 B 概率；正版化属用户环境域）→黑名单+替代路径（Surge XT）+309 warn_only 默认维持的产品确认建议。
  7. 停止条件对照：D1/D2 不需 Waves 调试符号不侵 license 环境；若 D1-D4 全无法判定→按卡面停止条件转纯策略方案（出口 B 判据直接生效）。
- 验收：（主管回填）
