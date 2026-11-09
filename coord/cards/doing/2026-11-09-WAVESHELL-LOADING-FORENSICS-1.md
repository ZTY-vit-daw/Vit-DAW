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
- 回执：（执行侧回填）
- 验收：（主管回填）
