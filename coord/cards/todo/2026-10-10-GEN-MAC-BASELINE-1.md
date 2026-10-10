# GEN-MAC-BASELINE-1：Mac 生成线基线对齐+生成链首勘（Mac 线公共前置）

- 发卡：GLM 主管决策侧 / 2026-10-10 夜（依据=用户裁定 Mac=生成线+GEN-CAP-RECON-1 §4.1 首卡建议）
- 派发确认：已确认（用户 2026-10-10 裁定 Mac=生成）
- 验收负责人：GLM 主管决策流
- 池序 60；目标仓库=D:\Vit_DAW（Mac 执行侧；同步+构建+烟测，零生产源码改动）
- 优先级 / 预估 / 依赖：**P1** / 0.5-1 天 / 无（Mac 线其余生成卡的公共前置）
- 模型分级：L1 / Mac 执行流（PORT 系列先例）；PC 侧只读协同

## 目标（GEN-CAP-RECON §3.2 三件事打包）

1. **同步对齐**：Mac 仓 fetch/rebase 到 origin/main 最新（今夜 ≥e3af3c4e），核对 Mac 侧遗留 worktree/分支与在飞卡（RLM-PROFILE-2 done 待验收/CCB-PARAM/MACRO-RECON 领取记录）文件域无冲突。
2. **内核重建**：Mac 侧 VitAgent 内核+agent 构建过（PORT-A4/A5 先例流程），记录二进制哈希。
3. **生成链五命令真栈首勘**：`project.new`/`save_as`、`assets.generated.*` 三命令（GeneratedAssetService.cpp:160-448，catalog.go:1062-1065 注册面）、`midi.*` 命令族（MidiService 九 handler，CommandDispatcher.cpp:2609-2664）——Mac 真栈逐命令首勘（发令→回执形态记录），产出 **Mac 生成线缺口清单与可跑面结论**（PORT 系列未覆盖面即本卡产出）。
4. 建 `coord/resources/MAC-RUNTIME-STACK.md`（PROTOCOL §2.2 语义，Mac 栈独立占用记录）。

## 文件域

`scripts/`（Mac 烟测扩展，按 PORT-SMOKE-MAC 体系模式）+ coord 工件；**零生产源码改动**（发现缺口如实记录不修）。

## 验收标准

Mac 栈烟测 exit 0（五命令勘测脚本）；缺口清单+可跑面结论+二进制哈希入回执 coord/runs/GEN-MAC-BASELINE-1/；MAC-RUNTIME-STACK.md 建立。

## 停止条件

Mac 栈构建断裂（基线漂移）→ 断点+证据上交（对照 PORT-A5 最后绿基线）。

- 领取：（时间 / origin/main hash / owner / worktree / 领取提交）
- 回执：（run ID / 缺口清单 / 可跑面结论）
- 验收：（裁定文件）
