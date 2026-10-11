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

- 领取：2026-10-11 11:29 CST / origin/main=32fcab06609e64eafd5d6e7cbdb6911538df1a42（fetch 直连覆盖代理；卡面「≥e3af3c4e」已满足，实际基线为当日最新）/ owner=GLM-5.3 Mac 执行流（ZCode 会话）/ worktree /Users/timozty/Documents/Vit-DAW-genbase1 / 分支 port/gen-mac-baseline-1
- 回执：**done（自验 exit 0）**——权威 run `coord/runs/GEN-MAC-BASELINE-1/run4-pass-20261011-122948/`（summary 30/30 门绿，schema gen.chain.survey.mac.v1）。四件事：①基线 32fcab06（fetch 直连；远端 doing 仅 MIXLAYER-M1-1=PC/agentloop 域不相交，卡面"RLM-PROFILE-2 待验收"已过时按现状清点）；②内核重建 sha256 `9a70f064d2a20f023f35389c28fa87956595ce46dad4eb35bda571d243f54057`+vitagent `d3dc1662…`（K1/K2 均为祖先，79b8ecdc..HEAD 对 VitApp/Source 零改动；FetchContent 代理坑用 GIT_CONFIG_* 空覆盖直连解）；③五命令+扩展面首勘全绿（project 周期/midi 九命令数据级闭环含 mutate-by-id/delete-by-id 回读数学一致/ingest×2 take 栈/switch_take/ghost/render 双腿——K1 同步拒绝语义+完成不冻结 Mac 首证/render.profile.bind Mac 首勘 ok/fail-closed 负探针；17 确认卡双形态）；④MAC-RUNTIME-STACK.md 建立+占用+释放闭环（de3a3b6d 登记，12:34 复核空闲）。缺口清单与可跑面结论见 [report.md](../../runs/GEN-MAC-BASELINE-1/report.md)（A1 维持开放；新发现=mutate argspec 与内核校验漂移待决策侧定；探针迭代 3 轮均脚本缺陷，栈本体零功能红）。实现 commits：f95f4760/bd079a09/d2d8070e/9560deba/dfcb2947 @ port/gen-mac-baseline-1；零生产源码改动（VitApp/Source 与 agent/ 零触碰）。端测边界（AGENTS §5）：两件套命令/回执面；Godot 前端/webui/旅程面与插件扫描面不在本卡。
- 验收：**pass**（2026-10-11 晚窗，双验收会话合流——并行决策会话独立复验复跑+主管复核，同 pass 结论）：run4 30/30 门 overall=PASS 双方亲核；零生产源码 diff 双方核对；**独立复跑 exit 0 全门绿零残留**（并行会话，Mac 栈）；MAC-RUNTIME-STACK 建立→释放闭环。裁定文件双档：[rulings/2026-10-11-GEN-MAC-BASELINE-1-pass.md](../../rulings/2026-10-11-GEN-MAC-BASELINE-1-pass.md)+[rulings/2026-10-11-EVENING-BATCH-rulings.md](../../rulings/2026-10-11-EVENING-BATCH-rulings.md)。实现 cherry-pick 入 main=**fcc2743c..ff98ab99** 五提交（port 原提交 f95f4760..dfcb2947 留远端；本侧同批 cherry-pick rebase 自动去重）。**裁定随卡**：argspec 漂移=对齐 argspec 到内核 by-id 语义（新卡 MIDI-ARGSPEC-ALIGN-1 池序 69）；A1 装载维持开放归 A1-MAC-FORM-1；坐标系/SMF 注记备查不立卡。AIGC-DRUM-GEN-1 依赖解锁。