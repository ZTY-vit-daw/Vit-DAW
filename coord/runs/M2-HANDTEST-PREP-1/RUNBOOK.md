# M2-HANDTEST-PREP-1 RUNBOOK（§8 跑前口径，跑前成文冻结）

- 卡：coord/cards/doing/2026-11-08-M2-HANDTEST-PREP-1.md（池序 311）
- 写：311prep 执行子代理 / 2026-11-08 晚窗（候栈期间成文；跑前不再改口径）

## 腿构成

1. **282 离线证据腿（零栈，先行独立执行）**：在 wt_282impl（`port/edu-ask-anchor-1` @ 873c0e9c，只读）复跑 282 回执四组测试命令——主管预裁：282 未合入 main（origin/main catalog.go 无 transport_set_loop，领取时亲证），真栈轻腿降级为离线证据。
   - **执行结果（2026-11-08 晚窗，候栈期间）**：四组全绿——`go build ./...`=0；`go test ./rules/... ./internal/tools/ ./internal/policy/ -count=1`=0（ruleset 0.565s/tools 0.568s/policy 0.501s）；`go test ./internal/chat/ -count=1`=0（162.201s 全包）；`go test ./internal/agentloop/ -count=1`=0（6.043s）。零栈零 LLM。
2. **真栈主腿（单 run）**：完整命令（281 smokerun3 同型+StartUI+双 No 开关最小面）：
   `powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_311prep\scripts\dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_311prep -StartKernel -RestartAgent -StartUI -M2HandtestPrep -NoChatSmoke -NoStripSilenceSmoke -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe -RunArtifactsDir <run 目录>`
   （kernel=wt_228 只读复用 SHA256 F9E3DA0A…D67A 已核；agent=现场构建；-NoStripSilenceSmoke 新增于 281 形态——默认 strip-silence 腿不在本卡判定面且会动隔离副本工程，关掉保最小面；三件套；Godot 前端进程在场但 GUI 面不在断言面），driver=`scripts/m2_handtest_prep.py`：
   - **Phase P 布位**（内核 ZMQ，确定性零 LLM）：new_project → save_as_project(`%APPDATA%\Vit-DAW\projects\手测·小星星乐局.vit` 持久位，非源码树非 run 目录，§10 合规) → add_track×3 → t1 insert_midi_clip(52 beats)+42 theme 音（edu_jam_smoke.py PHRASES 同表=level.yaml notes.theme 孪生）→ t2 insert_midi_clip+42 音 corrupted 表（beat 37 处 pitch 68 替 69，level.yaml deviations 同款）→ set_mute(t2)（initial_state=muted 防剧透）→ t3 留白 → save_project → open_project(同路径)（重开加载验证+recent 置顶）→ get_recent_projects 断言首项=布位路径 → get_project_state 断言 3 用户轨 + 双 clip 42/42 音回读对账（t2 含 37 拍 68 锚）。
   - **Phase J 旅程**（HTTP chat，确定性零 LLM）：workspace 激活轮询（A2 同型）→「开始乐局」enter（edu_jam_entered/first-run-twinkle/n0，门拒绝有界重试≤6）→「写好了」n0 过（clip_exists+human_replied→n1）→「结束乐局」exit（exited/user_exit；终态清面，用户手测新会话可干净开局）→ edu-jam conversation telemetry 零 LLM 行断言。
   - **Phase G 生成路由**（HTTP chat，轻 LLM）：同一 conversation 生成类话术（步骤清单 B1 同款语义）→ 客观断言=prompt render 工件出现 gen.generate 披露（303 词典命中面）+ 响应进编排（task_id/goal_id）或回复属生成域受理形态；不等 wav（278 全链 exit 0 在案）。VIT_GENAPI_CONFIG 指 run 工件 providers（yue-local@8087，引擎**不在场**：gen.generate 若被调即连接拒绝快速失败，路由证据已在 render+响应面成立）。

## 运行次数与止损

- **真栈主腿单 run 无重跑**；同型确定性断点一次失败即停（exit 4 取证上交，禁止原样重跑）。
- 282 离线腿=测试命令复跑（确定性，无 LLM 无栈）。

## 成功条件

- driver py exit 0 → PS1 exit 0 = 手测放行（三目标齐：布位可开+旅程 enter/n0 可达+生成路由命中）。

## 失败分类（driver exit code 预写）

- **exit 0**=PASS。
- **exit 2=环境**：栈/传输故障、布位内核命令失败、激活/等待超时（含 enter 门重试耗尽——by-design 门拒绝不计缺陷）。
- **exit 4=确定性面缺陷**：recent 断言失败、工程重开对账失败、enter 未拦截/词形漂移、n0 判定失败、生成路由 3 轮内词典面未命中（render 无 gen.generate）——取证上交，不重跑。

## LLM 预算

- 乐局面（Phase P+J）：**零 LLM**（确定性拦截；telemetry 按 conversation 过滤断言零行）。
- 生成路由（Phase G）：**1-3 调用 deepseek-flash**，上界 ≈¥0.1。
- 合计上界 ≈¥0.1。

## 端测边界声明（AGENTS §5）

- 真栈面=agent HTTP/ZMQ 断言（布位/旅程/路由）；渲染面与用户旅程面（Godot 启动页点选工程、聊天面板视觉、webui DOM）**不在断言面**——GUI 最后一米=用户手测本体。
- 用户手测前置补记（步骤清单回填）：拉起 Godot 前同一 shell 设 `$env:VIT_EDU_LEVELS_DIR = "D:\Vit_DAW\coord\levels"`（勘察结论：前端自启 agent 无此 env 时乐局 enter fail-closed——edu_jam_chat.go:1109-1114 回退 `coord/levels` 相对 cwd，而前端拉起 agent 的 cwd=Godot 工程目录，回退不可达；AGENTS §5 在案形态=同一 shell 先设变量再拉 Godot）+旅程 B 前置（YuE2 引擎 E:\AI\YuE2 启动+providers 配置 ~/.vit/genapi_providers.json 或 VIT_GENAPI_CONFIG——278 run 形态）。

## 勘察结论（跑前冻结）

- **启动页工程列表来源**：Godot 启动页 → project_store.refresh_recent_projects() → project_client.get_recent_projects() → 内核 ZMQ `get_recent_projects` → VitGlobalProjectConfig（`%APPDATA%\Vit-DAW\global_project_config.json`，recent_projects 最多 10 条，open/save 时 prepend 置顶）——**内核侧全局配置，非 agent workspace/projects 体系**（卡面勘察方向的修正）。布位持久位=`%APPDATA%\Vit-DAW\projects\手测·小星星乐局.vit`（用户数据域；agent 工作区 .vit_history/.vit_agent/.vit_derived 随 .vit 同目录生成，同域干净）。
- **表对账（离线干跑绿）**：driver theme 表 42 音逐音==level.yaml notes.theme（pitch/start/length 全等）；corrupted 表单点偏差 beat37 69→68（level.yaml deviations 孪生）。
