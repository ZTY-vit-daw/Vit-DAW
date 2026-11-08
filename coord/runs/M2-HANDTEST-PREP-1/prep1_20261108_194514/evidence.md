# M2-HANDTEST-PREP-1 (311) prep1 — PS1 exit 0 PASS（手测放行信号）

- run: `prep1_20261108_194514`（19:45:14 ps1 启动，driver stamp 20261108_194530，19:45:5x PS1_EXIT=0）
- 完整命令：
  `powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_311prep\scripts\dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_311prep -StartKernel -RestartAgent -StartUI -M2HandtestPrep -NoChatSmoke -NoStripSilenceSmoke -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe -RunArtifactsDir D:\Vit_DAW_wt_311prep\coord\runs\M2-HANDTEST-PREP-1\prep1_20261108_194514`
- 退出码：**py exit 0 → PS1 exit 0**（§5 真栈门槛达成=手测放行）
- 被测二进制：agent=现场构建 SHA256 `7CB6E18A5BCE9F1209299B6830AD0B0A7647C36164F558F90CF06DC25EC272C8`（mtime 19:45:24，@ wt_311prep `c70276a7` port/m2-handtest-prep-1，agent 源码零改动）；kernel=wt_228 只读复用 SHA256 `F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A`（225-310 同款）
- 测试时 HEAD：wt_311prep `c70276a7`（净树：run 前后 git status 仅 run 工件未跟踪）；worktree `D:/Vit_DAW_wt_311prep`
- 栈占用：登记 `1e555212`（19:44 推 main；净空两轮亲核 19:42:46+19:43:41 全零+登记后 ≤60s 即时复核 19:44:59 全零 owner 311 亲证）；PS1 完成态不自拆 → owner 亲拆 VitAgent 9020/VitApp 12024（Godot 已随 run 收尾退出）；**TEARDOWN_ALL_ZERO 亲核 19:47:10**（六口 TCP 5555/5556/5557/7878/8787/18787 listen=0+UDP 4444/4445=0+VitApp/VitAgent/vsphub/Godot/python 全零）；释放登记随本笔

## 一、各相全绿读数（m2_prep_summary.json exit_class=pass）

- **Phase P 布位**：`C:\Users\timoz\AppData\Roaming\Vit-DAW\projects\手测·小星星乐局.vit`——track 1007/1010/1013+clip 1016/1017；**42+42 音回读**（theme 逐音==level.yaml notes.theme 离线预对账在 RUNBOOK；重载后自 state clips[] 取 id 回读）；**beat37=pitch68 锚**（corrupted 单点偏差）+theme 侧 beat37=69 对照；**track2 mute=true**（防剧透预置）；track3 留白（clips 空）；**get_recent_projects 首项=布位路径**（启动页列表同源面=Godot start_page→project_client→内核 get_recent_projects→%APPDATA%\Vit-DAW\global_project_config.json）
- **Phase J 旅程**：activated=true（agent_last.log:158 workspace activated 布位工程+conversations=0；:139 shadow tracks=3）→ **enter_attempts=1** 一次过（stop=edu_jam_entered/level=first-run-twinkle/n0；回复「乐局开始：一个旋律的变奏（小星星乐局） 莫扎特…」）→ **n0「写好了」过**（stop=edu_jam_node_done/node_done=n0→n1 读谱纠错）→ **exit 终态**（stop=edu_jam_exited/user_exit；用户手测新会话可干净开局）→ **乐局面零 LLM**：J 完成时点 telemetry 文件尚未创建（rows=None=文件不存在的零调用旁证）；G 轮后文件 4 行**全部**属生成路由轮（conversation_id 逐行核对 m2prep_journey_20261108_194530 的 G 段 4 行——第 1 行语义分类+3 行编排内轮，乐局 3 轮零行）
- **Phase G 生成路由**：turn 1 即中——**三证据齐**：①accepted=True（task_id/goal_id 编排受理）②gen_tool_trace=True（响应含 gen.generate 调用痕迹）③disclosed=True（prompt render 2 行含 gen.generate 披露）；**stop_reason=needs_confirmation**（治理确认门正确拦截——RiskConfirm 零裸写红线保持：模型披露并请求执行生成，等用户确认，未实际调 8087——路由证据完备且比引擎失败更干净）；VIT_GENAPI_CONFIG 指 run 工件 providers（yue-local@8087 引擎不在场，未到达）

## 二、计费（如实）

telemetry 4 行 deepseek-flash（全 m2prep conversation 生成路由段；error 全空；http_ms 2432/2438/1569/5123）——**申报预算 1-3 超限 1 行**（summary.budget_exceeded=true 如实记录，¥ 量级仍 ≪¥0.1 上界，deepseek-flash 4 调用）；乐局面零 LLM。

## 三、282 轻腿（主管预裁条件化兑现）

282 未合入 main（领取时亲证 origin/main catalog.go 无 transport_set_loop）→ 轻腿改**离线证据**：wt_282impl（@873c0e9c 只读）四组复跑全绿——build 0/rules+tools+policy 0（0.565/0.568/0.501s）/chat 全包 0（162.201s）/agentloop 0（6.043s）。set_loop 体验随 282 合入后生效（步骤清单已标注）。

## 四、端测边界声明（AGENTS §5）

真栈面=agent HTTP/ZMQ 断言（布位/旅程/路由）；**渲染面与用户旅程面不在断言面**——Godot 启动页点选工程、聊天面板视觉=用户手测本体（最后一米）。Godot 前端进程在本 run 在场（-StartUI 三件套）但 GUI 未断言。

## 五、工件清单（本目录）

console_ps1.txt（ps1 全程+PS1_EXIT=0）/ m2_prep_console.txt（driver 四行 PASS 面）/ m2_prep_summary.json（exit_code 0+四相读数）/ chat_responses.json（4 回合原始响应）/ agent_last.log（346 行；:139/:158 激活证据行）/ logs/agent_llm_telemetry.jsonl（4 行）/ logs/agent_prompt_render.jsonl（含 gen.generate 披露）/ genapi_providers.json+downloads/ / default_project.xml+gui_project_isolation.json（隔离副本）/ binary_sha256.txt / rundir.txt

## 六、持久布位物（run 外持久在场，用户手测直接消费）

`%APPDATA%\Vit-DAW\projects\手测·小星星乐局.vit`（16K，3 用户轨 fixture）+ 同目录 `.vit_agent\`/`.vit_history\`（agent 工作区，vitproj_08c5c009…）+ `%APPDATA%\Vit-DAW\global_project_config.json` recent 首项（启动页列表第一位）。
