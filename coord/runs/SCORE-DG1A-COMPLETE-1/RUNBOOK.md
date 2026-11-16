# RUNBOOK — SCORE-DG1A-COMPLETE-1（377）DG1-A A7/A8 补完真栈腿

冻结时间：2026-11-16 19:2x +0800（跑前成文，AGENTS §8 / STACKGATE §1.6）

## 1. 目标

`scripts/score_workbench_smoke.mjs` 补 A7（谱面编辑→内核出声探针非静音）+A8（编辑前后 WAV 差分）
断言，判据复用 373 DG1-B（`scripts/src_template_smoke.py` B2/B4 同参同阈）；DG1-A 全断言
（A1-A8，STACKGATE §3.1）真栈轮 exit 0。

## 2. 装置

- 形态：kernel+agent berth（`dev_agent_smoke.ps1 -StartKernel -RestartAgent -ScoreWorkbenchSmoke
  -NoChatSmoke`，不启 UI）+断言体自起 Chromium（headless，真浏览器驱动 /app/ 谱面面板）。
- 被测仓：worktree `D:/Vit_DAW_wt_377dg1a` @ `port/score-dg1a-complete-1`（基线 d5c20765+
  本卡 scripts/score_workbench_smoke.mjs 单文件 diff）。
- agent：ps1 现场 go build（worktree 构建即被测）；webui dist=worktree npm run build 产物（/app/ 服务面）。
- 内核：run 目录拼装 `kernel_runtime/VitApp.exe`
  sha256 `a59473614152882b21433dcee685f8b13ae0bd5b632a97cd5e2c8765bb18348e`
  （源=374 run_dg1a_20261116_0455 同一二进制只读复用；二进制 grep 亲证含
  set_time_signature×3 / l2_render_probe×20 / start_render×3 面；本卡零内核改动，
  VitApp/ 在 322a82fd→d5c20765 间唯一差异 aea3c37a=375 已含）。
- 音源：373 模板资产 `coord/lesson-designs/music-creation-unit2/templates/melody-drum-v1/`
  （template.v1.xml sha256 对账 manifest a188defd…；place-copy 进 run 目录，源只读）。
- 端口面：脚本标准 berth（7878 agent HTTP+5555/5556 内核 ZMQ+4445 UDP）；P5 探针/渲染经
  内核 ZMQ 命令面（STACKGATE §3.0 生产缝隙，python+pyzmq 27.1.0 内嵌子进程=373 已验传输）。

## 3. §8 口径（跑前冻结）

- **运行次数**：正式 1 次单 run 无重跑。环境中断类（端口/内核启动/python 传输/Surge 前件缺失/
  recents 还原失败）不计为正式轮，须有原始日志/退出证据才可排除；同断点环境故障两次→停卡
  上交（卡面记录失败分类）。断言失败（face_defect）一律禁原样重跑。
- **成功条件**：脚本 exit 0 且 summary 断言全绿（A1-A6 十五项+A4-template-2-4+A7-edit-readback+
  A7-probe-non-silent+A8-edit-differential）+工件齐（summary/WAV 对/探针 JSON/回读 JSON/截图/
  recents before 快照+还原哈希亲证）。
- **失败分类**：exit 1=assertion_failure/face_defect；exit 2=env_interrupt（分相位记录：
  env-agent/env-fixture/env-context/env-playwright/env-browser/runtime 装置时序/P5-kernel-zmq/
  P5-handles〔Surge 前件〕）。
- **阈值**（373 逐字沿用，summary.thresholds 随工件记录）：PROBE_MAX_ABS_MIN=0.01；
  WAV_NON_SILENT_PEAK=500；WAV_NON_SILENT_RMS=20.0；WAV_DIFF_MAX_MIN=500（int16）。
  支持性证据：probe sum_abs 相对 delta（不断言）。
- **LLM 预算**：计费上界=0（-NoChatSmoke+VIT_AGENT_LLM_BASE_URL=http://127.0.0.1:1 死端点
  override+全确定性 invoke/UI/ZMQ 面）。
- **止损线**：同代码版本同确定性断点连续两次失败→停卡取证上交；不得改阈值/断言自愈。

## 4. 命令

```powershell
cd D:\Vit_DAW_wt_377dg1a
$env:VIT_AGENT_LLM_BASE_URL = "http://127.0.0.1:1"
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev_agent_smoke.ps1 `
  -StartKernel -RestartAgent -ScoreWorkbenchSmoke -NoChatSmoke `
  -KernelExe "D:\Vit_DAW_wt_377dg1a\coord\runs\SCORE-DG1A-COMPLETE-1\run_dg1a_20261116_1945\kernel_runtime\VitApp.exe" `
  -RunArtifactsDir "D:\Vit_DAW_wt_377dg1a\coord\runs\SCORE-DG1A-COMPLETE-1\run_dg1a_20261116_1945"
```

（console 全量重定向落 run 目录 ps1_console_full.txt）

## 5. 工件契约（STACKGATE §1.4）

run_id、起止时间、HEAD+status、内核/agent 二进制哈希、fixture（模板）源哈希前后对账、
recents before 快照+teardown 还原哈希亲证（ps1 ScoreWorkbenchSmoke 块自带）、探针 WAV 对
（render_before_edit.wav/render_after_edit.wav）+差分报告（summary.p5_renders）、回读 JSON、
每腿判定 JSON（score_workbench_summary.json）、截图 p1-p5、agent 原始日志（ps1 留 run 目录）。

## 6. 隔离（AGENTS §10）

VIT_PROJECT_XML=脚本共享 GUI 隔离块（git-clean run-dir 副本）；草稿根重定向 run 目录；
recents 快照→还原哈希亲证（mismatch=装置非零退出按环境中断）；模板源只读（copy 后源哈希复算）。

## 7. 前置门（已过）

- 三门：vitest 668/668（exit 0）/npm run build（exit 0）/go build ./...（exit 0）——worktree 内。
- 脚本 node --check 通过；内嵌 python helper py_compile 通过+pyzmq 27.1.0 在场。

## 8. attempt 台账（§8 失败分类如实分记）

- **attempt1（run_dg1a_20261116_1945，19:27-19:28）=装置缺陷（driver_defect），非产品断言失败、非栈环境中断**：
  mjs 模块装载即崩——`createHash` 误从 `node:fs` 导入（应为 `node:crypto`；`node --check` 只查语法
  不查命名导出故预检漏过）。栈本身健康（内核 26016+agent 27368 起动正常，ZMQ 5555/5556 在听）；
  腿零断言执行零工程写面（recents 哈希 807226F4… 与 before 快照一致亲证，无需还原）。PS 包装器
  EAP=Stop 在 NativeCommandError 处中断（recents 还原段未及执行——已人工核对源态=快照态）。
  残留栈 owner 亲拆 19:30:06（VitAgent 27368/VitApp 26016 亲停，六 TCP+UDP 4445+Vit 进程=0 复核）。
  修复=导入行改 `node:crypto`（装置域单行修，非断言/阈值自愈）；副本预检通过（模块装载→env-agent
  诚实失败为止）。非原样重跑：代码已变更。
- attempt2（run_dg1a_20261116_1932）=修复后正式轮。
- **attempt2 实况（19:31-19:33）=环境中断（env_interrupt@P5-handles，环境类第 1 次），A1-A6 十五断言全 PASS**：
  腿推进到 P5 模板相位，旋律轨 plugins 披露=[vst3 全路径名|Volume & Pan|Level Meter] 而非
  ["Surge XT"|…]——组装的 kernel_runtime 缺插件数据库（fresh Workspace/Settings/Settings.xml
  3,534 字节空壳 vs 374 同位 424,113 字节含全 VST3 描述符表；两份含 Surge XT 条目亲证
  [wt_score1 副本与 Export/staging 部署各 4 处命中]）。活栈取证（同一占用窗口内只读）：
  l2_render_probe reply=error "no_renderable_audio_content (MIDI tracks have no instrument)"
  ——插件未实例化（路径名=占位），健康音源前件不成立=按设计 fail(2, P5-handles)。非产品缺陷
  （373 同模板同判据在其运行时 PASS）；非判据问题。PS 包装器同 attempt1 在 stderr 行中断，
  recents 还原段未及执行→owner 人工自 before 快照还原+哈希亲证一致 807226F4…（腿内
  save_as/save/open 写面曾触发，7bfada2d…→还原后=快照态）。残留栈 owner 亲拆 19:35:23
  （VitAgent 10060/VitApp 7728 亲停，六 TCP+UDP 4445+Vit 进程=0 复核）。
  修复（装置组装域）=attempt3 kernel_runtime 补齐部署插件数据库：
  Workspace/Settings/Settings.xml 自 D:/Vit_DAW/Export/staging/runtime 只读拷贝（源哈希随工件记录）。
- attempt3（run_dg1a_20261116_1936）=补齐插件数据库后的正式轮。
- **attempt3 实况（19:36-19:38）=域外产品缺陷发现（D4），非本卡断言失败**：插件库修复生效
  （p5_handles melody_plugins=["Surge XT",…] 按名装载亲证；A1-A6 十五断言全 PASS），P5 面板
  装载崩溃：活栈只读复现取证=[pageerror] "score model error [grid_misaligned]:
  beats=10.666666666666666 not on grid"→[data-score-root] 消失。根因链（域外三层，锚点上交）：
  ①内核 get_midi_clip_data 对 80bpm 工程 8 拍 clip 披露 length_seconds=8.0（拍数冒充秒；真值
  6.0s；VitApp/Source/Service/MidiService.cpp:1758-1768）②webui 桥换算 clipLengthBeats=10.6667
  （kernel.ts:131-140）③面板 applyPatch useMemo 渲染期抛 ModelError 无边界→整树卸载
  （ScoreWorkbenchPanel.tsx:401+model.ts:96-102）。120bpm fixture 下 8*120/60=16 恰整数网格
  不触发（374 A1-A6 未暴露的原因）。分类=face_defect（产品）但**在 377 文件域外**——按
  AGENTS §11 上交锚点+最小复现，不扩修。recents 还原+栈亲拆 19:42:53（写面曾触发 23a6d31b…
  →自快照还原 807226F4… 亲证一致；VitAgent 11336/VitApp 22904 亲停；六口+UDP+进程=0 复核）。
  **缝裁定（STACKGATE §3.0 装置锚）**：A7/A8 编辑缝改走 373 DG1-B B4 逐字同款（agent/kernel
  tool face mutate_midi_notes，占位音高 60→79@beat0——正是被指定复用的判据原编辑形态）；
  面板手势受阻记为 D4 边界；A1-A6 保持面板手势（P1-P4，120bpm fixture）不受影响。
- attempt4（run_dg1a_20261116_1945b）=tool-face 编辑缝正式轮。
- **attempt4 实况（19:45-19:47）=装置缺陷（driver_defect：wavMeter fmt 块内偏移），16 断言全 PASS 后停在渲染计量**：
  A1-A6 十五断言+A4-template-2-4-disclosed 全 PASS（music_context 80bpm/2/4 披露；Surge XT 按名装载；
  melody_readback_before_edit.json 8 四分音符在场；render_before_edit.wav 3,072,784 字节真实产物）。
  崩点=本腿 wavMeter 读 fmt 块 bitsPerSample 用了绝对 offset+14（=chunk 体 +6=byteRate 高半字→恒 0）
  ——正确为 chunk 体 +14=绝对 offset+22。python wave 模块旁证同文件 channels=2/width=16bits/
  48kHz/768,000 帧/16.0s/peak 16282/rms 2328.12——渲染链与判据输入全健康，纯计量函数偏移缺陷。
  修复后离线复算同文件 channels=2/width=16/dataLen=3,072,000 验证通过。非产品断言失败；
  栈/recents 亲拆还原 19:47:58（79cb4122…→快照态 807226F4… 亲证一致；VitAgent 1912/VitApp
  27476 亲停；六口+UDP+进程=0 复核）。非原样重跑：计量函数已修复。
- attempt5（run_dg1a_20261116_1949）=wavMeter 修复后正式轮。
- **attempt5 实况（19:48-19:50）=装置缺陷（driver_defect：wavMeter width 单位——同函数第二处）**：
  attempt4 修复后 bitsPerSample=16（位）被当字节与 `!==2` 检查比较→"unexpected sample width 16"。
  与 attempt4 同断点（P5-render-before wavMeter）但不同缺陷点、代码已变更（非原样重跑）。
  修复=width=bits/8 字节语义。**修复后离线全链验证**（attempt5 真实 WAV）：meter
  {channels 2, width 2, peak_abs 16166, rms_abs 2602.3}+受扰副本差分 max=4693 亲证。
  栈/recents 亲拆还原 19:51:15（7a84d0c6…→快照态 807226F4… 亲证；VitAgent 10028/VitApp
  23880 亲停；六口+UDP+进程=0 复核）。
- attempt6（run_dg1a_20261116_1952）=width 单位修复后正式轮。
- **attempt6 终判（19:52-19:53）=PASS exit 0（ps1 exit 0+腿 exit_code 0，19 断言全绿，duration 27.5s）**：
  A1-A6 十五断言（fixture 面板手势链）+A4-template-2-4-disclosed（music_context 80bpm/2/4）+
  A7-edit-readback（占位 60→79 落核：edited 在场+占位消失+数恒 8）+A7-probe-non-silent
  （l2_render_probe ready/nonzero/max_abs≥0.01/nan0）+A8-edit-differential（改前
  peak 14622/rms 2247.43、改后 peak 13094/rms 2207.16 双侧非静音≫阈，max_abs_diff=18334≫500；
  render_done 双完成；支持性 probe sum_abs rel_delta=0.0008 如实记录——单音改音能量近恒，
  差分判据在 WAV 面成立）。**LLM 计费上界=0 兑现**（-NoChatSmoke+死端点 override+全确定性面，
  全程零 chat 调用）。装置 recents 还原哈希亲证 807226F4AF3F（ps1 自带块，console 在卷）。
  owner 亲拆+TEARDOWN_ALL_ZERO 亲证 19:53:59（VitAgent 27564/VitApp 21272 亲停；TCP
  7878/7879/8787/5555/5556/9333=0+UDP 4445=0+Vit/Godot 进程=0+recents=807226f4 终核）。
  工件=本 family 目录六 run 全保真（正式轮 run_dg1a_20261116_1952 + attempt1-5 分记）；
  WAV 对（各 3,072,784 字节）留 worktree run 目录本地核验（仓库先例不入库，计量/差分在 summary）。

## 9. 有效轮/无效轮总账（§8）

- 正式有效轮=attempt6（PASS exit 0）。
- 无效轮分记：attempt1=driver_defect（mjs 模块导入 node:crypto）；attempt2=env_interrupt#1
  （kernel_runtime 缺插件数据库→Surge 不装载）；attempt3=域外产品缺陷发现 D4（非本卡断言
  失败非环境——120bpm 恒等巧合掩盖的拍数冒充秒披露链）；attempt4=driver_defect（wavMeter
  fmt 偏移）；attempt5=driver_defect（wavMeter width 单位）。零产品断言失败；零原样重跑
  （每轮代码/装置均有变更）；环境类 1 次（未触两次止损线）；D4 锚点上交决策侧。
