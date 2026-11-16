# RUNBOOK：382 SCORE-ENTRY-REACH-1 真栈验收腿（-ScoreWorkbenchSmoke berth + J1/J2/J3 旅程断言）

## 跑前口径（AGENTS §8，跑前成文）

- 运行次数：**正式 1 次**，单 run 无重跑；装置域修复后重跑须分记 attempt。
- 成功条件：`dev_agent_smoke.ps1 -StartKernel -RestartAgent -ScoreWorkbenchSmoke -NoChatSmoke -KernelExe <run>/kernel_runtime/VitApp.exe` **exit 0**；summary 含 A1-A8+**J1/J2/J3 旅程断言全 PASS**（J2=非 120bpm〔80bpm 模板〕谱面面板装载无 pageerror+桥网格化披露 10.75；J1=工程面「打开工作台」→workbench surface→score 页签→面板装载；J3=返回工程条回 project surface）。
- 失败分类：`env_interrupt`（栈起不来/端口/超时）/`assertion_failure`（产品断言红）/`probe_defect`（装置缺陷）。环境中断只有原始日志才可排除有效轮次。
- 止损线：60 分钟或环境故障两次→释放栈位记录分类。同断点连败 2 次禁原样重跑（先取证/上交）。
- LLM 计费上界：**0**（-NoChatSmoke+全确定性 invoke/ZMQ/UI 面；被测 webui=worktree dist、agent=worktree 构建）。

## 被测物 provenance（§9）

- 内核=run 目录拼装 VitApp.exe sha256 `A59473614152882B…48E`（主仓 04:33 Nov-16 构建 place-copy；374/377/384 同一二进制；**本卡零内核改动**〔取证 verdict=template_xml_unit_mismatch〕）。
- 插件库 Settings sha256 `5ABF7D95…E076`（staging 只读拷贝，含 Surge XT；377/384 同款）。
- boot 工程=ps1 共享隔离块 git-clean 拷 HEAD `VitApp/Workspace/default_project.xml`（sha256 `1F1AEAF9…8D488`；120bpm/4/4——P0 fixture 基线）。
- webui=worktree `npm run build` dist（tsc 0 错+vite built 22:0x）；agent=ps1 现场 go build（worktree 源，GO_BUILD_OK）。
- 栈位：kernel+agent berth（VitApp 5555/5556+UDP 4445；agent 7878 HTTP；Chromium headless 自起；不启 UI/Godot）。
- S10 recents：before.json=807226F4AF3F…E353 落 run 目录；ps1 berth 块还原+哈希亲证。

## 断言面增量（相对 377 的 DG1-A A1-A8）

- J2-non120-panel-load / J2-non120-clip-grid-snapped / J2-non120-error-boundary-absent：377 attempt3 崩溃点（80bpm 模板选旋律 clip）修复回归。
- J1-project-surface-reachable / J1-workbench-entry-present / J1-workbench-surface-switched / J1-score-panel-from-project-journey：374 M4「没看到」旅程级回归（工程面→工作台→谱面）。
- J3-return-button-present / J3-return-to-project：返回工程闭环。
- 既有 A1-A8 十九断言零改动（A7/A8 编辑缝维持 373 B4 tool-face 同款=卡面指定判据形态）。

## 步骤

1. 占栈登记推 main 亲核 → ≤60s 复核净空；
2. `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev_agent_smoke.ps1 -StartKernel -RestartAgent -ScoreWorkbenchSmoke -NoChatSmoke -KernelExe <run>/kernel_runtime/VitApp.exe -RunArtifactsDir <run>`；
3. exit 0 → teardown（脚本 berth 自拆+owner 复核 TEARDOWN_ALL_ZERO）→ recents 还原哈希亲证 → 释放行推 main。
