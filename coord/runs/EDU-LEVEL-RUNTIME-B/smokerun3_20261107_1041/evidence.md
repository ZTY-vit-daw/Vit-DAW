# EDU-LEVEL-RUNTIME-B (281) smokerun3 — py exit 0 PASS（整卡烟测腿收口）

- run: `smokerun3_20261107_1041`（10:41:20 ps1 启动，driver stamp `20261107_104138`，10:41:44 PS1_EXIT=0）
- 完整命令：
  `powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_281impl\scripts\dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_281impl -StartKernel -RestartAgent -EduJamSmoke -NoChatSmoke -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe -RunArtifactsDir D:\Vit_DAW_wt_281impl\coord\runs\EDU-LEVEL-RUNTIME-B\smokerun3_20261107_1041`
  （smokerun2 命令形态同款；bash 侧路径全部加引号，无假启动）
- 退出码：**py exit 0 → PS1 exit 0**（§5 真栈门槛达成）
- 被测二进制：agent=现场构建 `agent/bin/VitAgent.exe` SHA256 `6FBDA3816DEA830943BBC457CDD86E476C29593C38FBBC25F40E1384DED6DCE7`（worktree `a6b1557a` @ `port/edu-level-runtime-b`，含修复 `b81449de`+`cdf99f80`；run 前 go build 预验 10:39:56 exit 0）；kernel=wt_228 只读复用 SHA256 `F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A`（225-281 同款，零 C++ 改动）
- 栈占用：登记 `ccdf81c0`（10:41 推 main；净空两轮亲核 ~10:37+10:40:21 全零；登记后 ≤60s 即时复核 10:41:05 全零）；PS1 完成态不自拆（设计如此）→owner 亲拆 VitAgent 35692/VitApp 48756，**TEARDOWN_ALL_ZERO 亲核 10:42:04**（六口 TCP listen=0+UDP 4444/4445=0+VitApp/VitAgent/vsphub/Godot/python 全零）

## 一、各相全绿读数（edu_jam_summary.json exit_class=pass）

- **Phase A fixture**：track 1007/1010/1013 + clip 1016 + **42 音回读计数 PASS**
- **A2 激活前置**：activated=true（agent_last.log:185 `[workspace] activated …edu_jam_fixture_project.vit` uuid=vitproj_1ba9fed9…；:169 `[shadow] initialized tracks=3`=5 服务轨过滤后 3 用户轨——smokerun2 取证形状再现，寻址已修）
- **B1 enter**：**enter_attempts=1**（激活前置步到位后一次过，零门拒绝重试）；workflow=edu_jam / stop=edu_jam_entered / level=first-run-twinkle / status=active / current_node=n0_photo_score_open；回复「乐局开始：一个旋律的变奏（小星星乐局）… 第 1 关（7 关）·拍谱开局」
- **B2 n0「写好了」拦截+判定**：stop=edu_jam_node_done / node_done=n0_photo_score_open / current_node=n1_read_and_fix；判定叶 `leaves=["structure:clip_exists=true","event:human_replied=true"]`——**smokerun2 失败叶 `clip "melody_imported" exists=false` 已消灭**（cdf99f80 过滤后 1 基用户轨下标：melody_imported track_index 1 → 用户轨 1007 clip 1016）
- **B3 n1 judge**：stop=edu_jam_node_done / node_done=n1_read_and_fix / current_node=n2_set_tempo；判定叶 `leaves=["judge:note_sequence_match=true"]`——**fixture↔卡 notes.theme 42 音对账过**（驱动器头注对账断言兑现）
- **B4 状态渲染**：stop=edu_jam_status；回复含四针 拍谱开局/读谱纠错/定速度/进行中 + 7 关全表（n0✓ n1✓ n2▶ 进行中，n3-n5 待）
- **B5 exit**：stop=edu_jam_exited / status=exited / end_reason=user_exit
- **Phase C 零 LLM**：telemetry_available=true / **llm_rows=0**（会话 edu_jam_smoke_20261107_104138 零行——确定性拦截面在 semantic-entry 之前截获，零计费实证）

## 二、计费（如实）

telemetry 全文件 1 行=`dev_strip_silence_chat_20261107_104130`（deepseek-flash in 706+cache_read 512/out 789 tok ≪¥0.01）——默认 strip silence chat 子腿（-NoChatSmoke 不关该腿），登记文预申报形态兑现；edu-jam 主面零 LLM 行。

## 三、次级观察（如实记录，不追查）

- strip silence 默认子腿两行 warn（chat confirmation did not complete / suggest 未产预期 pending apply regions）——ps1 既有 warn 非断言（该腿不在本卡判定面），smokerun2 同形态。
- HTTP state 阶段 shadow tracks=6（run-dir default_project.xml 拷贝工程形状，非 fixture）——共享隔离块预期形态。
- 10:41:28 一次 draft 工作区激活（Unsaved.vit，栈启动后首请求路径），A2 激活前置步随后把工作区切到 fixture 工程——与激活门设计交互一致（enter 门在激活后才放行）。

## 四、工件清单（本目录）

console_ps1.txt（ps1 全程+PS1_EXIT=0）/ edu_jam_console.txt（驱动器三行 PASS 面）/ edu_jam_summary.json（exit_code 0+五相读数）/ chat_responses.json（5 回合原始响应）/ agent_last.log（270 行；:169 shadow tracks=3 与 :185 workspace activated 证据行在卷）/ logs/agent_llm_telemetry.jsonl（1 行）/ default_project.xml+gui_project_isolation.json / edu_jam_fixture_project.vit / binary_sha256.txt / .vit_history+.vit_agent（会话运行态，不入库）
