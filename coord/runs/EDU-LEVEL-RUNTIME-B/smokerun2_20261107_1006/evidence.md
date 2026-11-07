# EDU-LEVEL-RUNTIME-B (281) smokerun2 取证 — py exit 4（§8 预写确定性面缺陷类；与 smokerun1 不同面）

- run: `smokerun2_20261107_1006`（10:06:29 启动 ps1，driver stamp `20261107_100646`，10:07 前结束）
- 完整命令：
  `powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_281impl\scripts\dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_281impl -StartKernel -RestartAgent -EduJamSmoke -NoChatSmoke -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe -RunArtifactsDir D:\Vit_DAW_wt_281impl\coord\runs\EDU-LEVEL-RUNTIME-B\smokerun2_20261107_1006`
  （一次 bash 路径未加引号的 wrapper 假启动 10:06:1x——反斜杠被吞 ps1 未找到，**ps1 未启动、栈零启动**，10:06:29 复核 TCP/进程全零，266 verifyrun1 同型先例不计 run；console_ps1.txt 首段留痕）
- 退出码：**py exit 4（face_defect）→ PS1 exit 1**（dev_agent_smoke.ps1:9019 throw，预期映射）
- 被测二进制：agent=现场构建 `agent/bin/VitAgent.exe` SHA256 `9956E455F973C752DC2FEFE8FB65E2B0DCA9AB9791DD18931A87B6427B8A2D0F`（10:05:53 构建=run 后复核同哈希；worktree `4a5e6911` @ `port/edu-level-runtime-b`，含修复 `b81449de`）；kernel=wt_228 只读复用 SHA256 `F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A`（225-281 同款，零 C++ 改动）
- 栈占用：登记 `62c4af5f`（10:05 推 main，净空两轮亲核 10:03:55/10:04:49+登记后 ≤60s 即时复核 10:05:45 全零）；PS1 throw 后栈残留 VitAgent 11380/VitApp 20960 **owner 10:07:19 亲拆**，**TEARDOWN_ALL_ZERO 亲核 10:07:32**（六口 TCP listen=0+UDP 4444/4445=0+VitApp/VitAgent/vsphub/Godot/python 全零）

## 一、修复腿验证结论（smokerun1 缺陷面已消灭）

- **A2 激活前置步生效**：`workspace activation: {"activated": true, project_path: …edu_jam_fixture_project.vit}`（edu_jam_console.txt 第 2 行）
- **B1 enter 一次通过（enter_attempts=1，零门拒绝重试）**：workflow=edu_jam / stop=edu_jam_entered / level=first-run-twinkle / current_node=n0_photo_score_open / project_uuid=vitproj_b061d0f3…（=fixture）/ 回复含「乐局开始：一个旋律的变奏（小星星乐局）」
- **B2「写好了」被乐局面拦截（smokerun1 竞态根面已修复）**：turn 2 workflow=edu_jam（非 semantic_entry）、拦截面内闭环判定执行、会话在激活后存活
- **edu-jam 会话零 LLM 行**：logs/agent_llm_telemetry.jsonl 仅 1 行=`dev_strip_silence_chat_20261107_100641`（默认 strip silence chat 子腿，登记文预申报形态）；edu_jam_smoke 会话 0 行——若跑达 Phase C 零 LLM 断言此面应绿

## 二、新失败面（本取证对象）

- Phase A fixture：PASS（track 1007/1010/1013 + clip 1016 + 42 音回读计数）
- **B2 n0 推进 FAIL**：期望 `stop_reason=edu_jam_node_done`+`node_done=n0_photo_score_open`，实得 **`stop=edu_jam_attempt_failed`**，判定叶全反馈：
  `✗ structure:clip_exists — clip "melody_imported" exists=false`、`✓ event:human_replied`（attempt=0）
- B3-B5 与 Phase C：未及执行；n1 judge 的 fixture↔卡 notes.theme 对账未到达

## 三、根因链（代码锚点+工件实证，全在 281 文件域）

1. **观察寻址按原始数组下标**：`observeEduJam`（edu_jam_chat.go:840-846）读 `get_project_state` 的 `tracks` 原始行数组，slot 寻址 `idx := slot.TrackIndex - 1; tracks[idx]`——按「1 基用户轨号」语义直接当数组下标用，无音频轨过滤。
2. **真实内核 tracks 数组前部有 5 条服务轨**：内核 handler（CommandDispatcher.cpp:3737 `for (auto* track : te::getAllTracks(*edit))`）按轨树文档序返回全部顶层轨。fixture 工程文件实证（edu_jam_fixture_project.vit:40-48）顺序=ARRANGERTRACK(1002)→CHORDTRACK(1003)→MARKERTRACK(1004)→TEMPOTRACK(1005)→MASTERTRACK(1006)→用户轨 1007/1010/1013。Go 侧同形状钉死者：shadow_test.go:127-131（Arranger/Chord/Marker/Tempo/Master 五行 `is_audio_track=false` 前置）+shadow.go:749（按 `is_audio_track` 过滤）——本 run agent 日志 `[shadow] initialized tracks=3`（agent_last.log:194）即过滤后计数；生产消费者同型过滤先例=capability_routing.go:530。
3. **组合后果**：`melody_imported.track_index=1` → `tracks[0]`=Arranger 轨（无 clips）→ clip_exists=false。`TrackNonEmpty`/后续所有槽观察同错位（melody_corrupted 应命中 tracks[1]=Chord 轨）。n1 judge 即使 n0 过关也会因槽观察错位失败。
4. **单测为何没拦**：edu_jam_chat_test.go 的假内核 `eduJamFakeTrack`（:60-66）只回 3 条用户轨（无服务轨前缀）——**合成回复形状与真实内核不符**，下标寻址在合成形状下恰好正确。smokerun1 的激活竞态掩盖了本缺陷（turn 2 从未到达判定）。

**定性**：281 文件域内确定性接线缺陷（观察组装轨寻址面），非环境、非 LLM（turn 2 全确定性闭环，回复为判定叶反馈直出）。修复候选（交主管，本腿不扩修）：
- 过滤后再寻址：`tracks` 行按 `is_audio_track`/`track_type` 含 audio|hybrid 过滤（shadow.go:749+capability_routing.go:530 同型先例）后应用 1 基下标；配真实形状单测（假内核补 5 服务轨前缀）。
- 或寻址改轨身份（slot 绑 track_id）而非位置——改动面更大，交裁定。
- 附带：A2 激活轮询与 enter 门在真栈一次通过，无需改动。

## 四、计费（如实）

telemetry 1 行（logs/agent_llm_telemetry.jsonl）：`dev_strip_silence_chat_20261107_100641` deepseek-flash in 706+cache 512/out 430 tok ≪¥0.01（登记文预申报形态兑现——strip silence chat 子腿不受 -NoChatSmoke 关闭；edu-jam 会话零行如上）。

## 五、工件清单（本目录）

console_ps1.txt（wrapper 假启动段+ps1 全程+PS1_EXIT=1）/ edu_jam_console.txt / edu_jam_summary.json（enter_attempts=1、fail_phase=n0）/ chat_responses.json（2 回合原始响应）/ agent_last.log（253 行，含 shadow tracks=3、workspace activated、两 timing 行）/ logs/agent_llm_telemetry.jsonl（1 行）/ default_project.xml+gui_project_isolation.json / edu_jam_fixture_project.vit（轨树顺序实证件）/ binary_sha256.txt / .vit_history+.vit_agent（会话运行态）
