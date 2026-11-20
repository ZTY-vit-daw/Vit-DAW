# PROGRESS — VST3-SIGNAL-HYGIENE-1 (池序 331, 产线 sighyg331)

- 开始: 2026-11-20, HEAD main ef13421a6a814c1950749d3a9db004247df69470
- 卡: coord/cards/todo/2026-11-09-VST3-SIGNAL-HYGIENE-1.md
- 无前次续跑点（工件目录首次创建）。

## 步骤

- S0 [done] 工件目录+本文件创建；读取任务卡+309 卡 S1 段。
  - 发现①: 派发提示词猜测代码域 agent/internal/vst3host 与 processorregistry/pluginprobe —— grep setupProcessing/kOffline/kRealtime/kPrefetch/ProcessSetup 零命中；实际信令面在 C++ 内核 tracktion_engine（vendored JUCE + NodeRenderContext），与卡面文件域一致。按卡面执行。
  - 309 S1 机理链六锚点: juce_VST3PluginFormatImpl.h:2489-2493(早退守卫)/:2647-2648(per-call mode)、juce_AudioProcessor.cpp:409-413、ivstaudioprocessor.h:138-143、tracktion_NodeRenderContext.cpp:135-139、Renderer.h:61/:80、VitProductionCoordinator.cpp:857-871。
  - 309 裁定: C 原型在本地分支 vit309-stackleg (9b8cc8a)，主仓 gitlink 不动；本卡评估转正或重做。
- S1 [done] 勘察完成。关键结论：
  - **C 原型 9b8cc8a 与遥测 cd957f4 在本机不可达**（栈腿 worktree D:/Vit_DAW_wt_309recon 已删，vit309-stackleg 是其子模块克隆的本地分支从未推远端；主模块仓库 .git/modules/tracktion_engine 无该分支/对象）→ 卡面目标 2「评估能否直接转正」答案=**直接转正不可行，需重做**；依 309 回执文字重做（C=per-call 强制 kRealtime 与 setup 一致；语义上 offline 应 setup 级 kOffline，且 E2b 已证 C 对 GR 无效）。
  - **机理精化（卡面前提成立但修法面更准）**：当前检出 juce_VST3PluginFormatImpl.h:2505 setup 行已是 `isNonRealtime()?kOffline:kRealtime`（非硬编码 kRealtime）；不一致成因=守卫 :2490-2493（isActive+同 sr/bs 即 return）**不含模式条件**，标志翻转后渲染窗 prepareToPlay 早退→模式感知 setup 行永远不可达→setup 停留装载期 kRealtime，per-call :2648 送 kOffline。E2a 运行时签名与此吻合。
  - 六锚点全部亲核：①guard :2490-2493 ②per-call :2648 ③setNonRealtime 仅置标志 processors/juce_AudioProcessor.cpp:410-413（std::atomic<bool> nonRealtime）④spec ivstaudioprocessor.h:138-143 ⑤NodeRenderContext ctor :136-138 setAllPluginsRealtime→nodePlayer->prepareToPlay→flush，dtor :161 恢复 true ⑥Renderer.h blockSizeForAudio=512/realTimeRender=false 默认；VitApp VitProductionCoordinator.cpp（Source/Service/）有 309 fix-B warn_only+opt-in flip（:1046/:1201）。
  - 第二个 setupProcessing 调用点=setupIO() :3407（实例化期固定 44100/1024/kRealtime，装载扫描语境，模式正确，需同步维护 lastSetup 跟踪）。
  - 修法定案：**守卫加 lastSetupProcessMode 模式感知 + setupIO/prepareToPlay 维护跟踪 + VIT331_SETUP/VIT331_SIGNAL 一次性 log-only 断言面 + env 门 VIT331_CONSISTENT_SETUP（默认关=生产行为不变，真栈 E4 腿 A/B 后由决策侧翻默认）**。防回归断言=VIT331_SIGNAL consistent 键（门开时 setupMode==perCallMode 必须成立）。
- S2 [in_progress] 建 worktree+分支+领取提交。命令（依序）：
  1. `git -C D:/Vit_DAW worktree add D:/Vit_DAW_wt_pc331 -b pc331-sighyg`（主仓从 main HEAD）
  2. `git -C D:/Vit_DAW/tracktion_engine switch -c pc331-sighyg`（从 detached 60ab1a49，零树变更）——**engine 分支放主模块仓库而非 worktree 子模块克隆**：309 的 vit309-stackleg 因放 wt_309recon 独立克隆、worktree 删除即失，本卡防复发。
  3. 卡面「领取」节回填+commit main（只动本卡 coord 文件）。

## 下一步

S4 轮询构建结果 → S7 收尾。

## S5/S6 落盘记录

- S5 [done] Go 双门全绿（worktree wt_pc331/agent）：
  - `go build ./...` exit 0；
  - `go test ./internal/vst3host/... ./internal/processorregistry/... ./internal/pluginprobe/... -count=1` exit 0（三包全 ok；本卡 Go 源零改动，此为卡面名义域最近证据面）。
- S6 [done] 规范锚点文档：`coord/runs/VST3-SIGNAL-HYGIENE-1/SPEC-ANCHOR.md`（规范原文/六锚点现状/机理精化/修法评估+定案/改动面+断言面/缺陷登记/真栈腿 lead/边界）。
- S4 [done] 编译门 PASS：configure exit 0（77.4s）+ `cmake --build --config Release --parallel` exit 0（日志零 `error C`/`error LNK`；我改头文件所在 TU juce_audio_processors_headless.cpp 编译通过；仅既有 C4819 代码页警告）；产物 `D:/Vit_DAW_wt_pc331/VitApp/build_release/VitApp_artefacts/Release/VitApp.exe` 31,995,392 B，SHA256=`0abdc136a024e4b09dd3b8dbf89098d410b524535c94919cd4cf156a5202823d`；未运行 exe（真栈禁启）。
- S7 [done] 收尾完成：分支 pc331-sighyg 提交 `862ede3d`（卡面回执+git mv done/、证据两件、ROSTER sighyg331 行）→ main FF 合并成功（d170d36a→862ede3d，MERGE_EXIT=0，无冲突）；main HEAD=862ede3d。

## 终态

**DONE（2026-11-21，执行侧自验 done 待主管验收）**——四件套：
- 执行的命令+退出码：cmake configure exit 0 / cmake --build Release --parallel exit 0 / go build ./... exit 0 / go test（vst3host+processorregistry+pluginprobe 三包，-count=1）exit 0 / git 提交×3（883a0db9 领取、98fb0200 engine、862ede3d 回执+done）与 FF 合并 exit 0。
- 产物路径：engine 修复=`D:/Vit_DAW/tracktion_engine` 分支 `pc331-sighyg` @ `98fb0200`（1 file +92/-1）；卡面=`D:/Vit_DAW/coord/cards/done/2026-11-09-VST3-SIGNAL-HYGIENE-1.md`；证据=`D:/Vit_DAW/coord/runs/VST3-SIGNAL-HYGIENE-1/`（本文件+SPEC-ANCHOR.md）；ROSTER=`D:/Vit_DAW/coord/agents/ROSTER.md` sighyg331 行；构建产物=wt_pc331 VitApp.exe（SHA256 0abdc136…823d）。
- 遇到的问题：①派发提示词猜测 Go 代码域（vst3host/processorregistry/pluginprobe）grep 零命中→实际信令面=C++ engine，按卡面文件域执行；②C 原型 9b8cc8a/遥测 cd957f4 因 309 栈腿 worktree 删除+本地分支未推而永久丢失→C 直接转正不可行，依回执文字重做（setup 级 kOffline 方向）；③engine 分支改放主模块仓库防 309 失分支复发；④构建日志仅存 tail（管道缓冲），零 error 以退出码+链接行+grep error=0 三重亲证。
- 断了几次：0（单次执行无重派感知）。
- 真栈腿（lead 留决策侧）：栈协议两条登记+E4 Surge 不回退+A/B 两 run 断言 VIT331_SIGNAL 判定键（SPEC-ANCHOR §5/§7）；PASS 后翻默认+landing engine 分支。

## S2/S3 落盘记录

- S2 [done] worktree `D:/Vit_DAW_wt_pc331`+分支 `pc331-sighyg`（基线 main 0e659626）；engine 分支 `pc331-sighyg` @ 60ab1a49（主模块仓库）；领取提交 main **883a0db9**（只动卡文件）。
- S3 [done] engine 提交 **98fb0200**（pc331-sighyg，92 insertions/1 file）：juce_VST3PluginFormatImpl.h 四处——守卫模式感知（:2489 邻域）/prepareToPlay 跟踪+VIT331_SETUP（phase=prepareToPlay）/processAudio VIT331_SIGNAL 一次性配对断言/setupIO 跟踪+VIT331_SETUP（phase=setupIO）；成员 lastSetupProcessMode（std::atomic<int>）+vit331SignalTraceArmed+vit331ConsistentSetupEnabled()+vit331ProcessModeName()。

## S4 构建命令（跑前成文；断线重派时先查 build_release 是否已有产物再决定重跑）

1. configure: `cmake -S D:/Vit_DAW_wt_pc331/VitApp -B D:/Vit_DAW_wt_pc331/VitApp/build_release -G "Visual Studio 18 2026" -DVIT_TRACKTION_ENGINE_DIR=D:/Vit_DAW/tracktion_engine`
2. build: `cmake --build D:/Vit_DAW_wt_pc331/VitApp/build_release --config Release --parallel`
   （冷构建预估 15-40 min，后台跑+轮询；engine dir=主模块仓库的 pc331-sighyg 检出）
3. 产物核验: VitApp.exe 存在+构建日志零 error；本卡真栈腿禁启（不运行 exe）。

## 纪律锚点

- 禁 git reset/clean；不启真实栈（真栈腿留 lead）；engine 子模块独立提交（301/309 同款）。
- 回归红线: Surge XT/全包渲染不回退（E4 同款断言，真栈腿候栈）。
- 停止条件: 修法引发非 WaveShell 插件回退 → 回退修法转纯登记。
