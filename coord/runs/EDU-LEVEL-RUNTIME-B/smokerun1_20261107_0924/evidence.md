# EDU-LEVEL-RUNTIME-B (281) smokerun1 取证 — py exit 4（§8 预写确定性面缺陷类）

- run: `smokerun1_20261107_0924`（09:24:02 启动 ps1，09:24:14 py 起跑，09:24:18 结束）
- 完整命令：
  `powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_281impl\scripts\dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_281impl -StartKernel -RestartAgent -EduJamSmoke -NoChatSmoke -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe -RunArtifactsDir D:\Vit_DAW_wt_281impl\coord\runs\EDU-LEVEL-RUNTIME-B\smokerun1_20261107_0924`
- 退出码：**py exit 4（face_defect）→ PS1 exit 1**（dev_agent_smoke.ps1:9019 throw，预期映射）
- 被测二进制：agent=现场构建 `agent/bin/VitAgent.exe` SHA256 `B73212AC31D7D248A6A90EBE7DF63A9DEEF3CD0A22CC26A7E753E422B126095C`（worktree c57b9a43 @ port/edu-level-runtime-b）；kernel=wt_228 只读复用 SHA256 `F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A`（225-277 同款，零 C++ 改动）
- 栈占用：24a8247b（09:23 登记，净空两轮亲核+登记后 60s 即时复核 09:23:44 全零）；TEARDOWN_ALL_ZERO 亲核 09:24:55（VitAgent 1704 / VitApp 20740 owner 亲拆；六口 TCP listen=0+UDP 4444/4445=0+五类进程=0）

## 一、失败面（与 §8 预写分类对账）

- Phase A fixture：**PASS**（track 1007/1010/1013 + clip 1016 + 42 音回读计数过；edu_jam_summary.json phases.fixture.verified=true）
- B1 enter（「开始乐局」）：**PASS**——workflow=edu_jam / stop=edu_jam_entered / level=first-run-twinkle / current_node=n0_photo_score_open / 回复含「乐局开始」标题行（chat_responses.json 第 1 回合）
- B2 n0 推进（「写好了」）：**FAIL（本取证对象）**——workflow=semantic_entry / stop=semantic_entry_unresolved（conf 0.20，「ambiguous completion statement」）
- B3-B5 与 Phase C 零 LLM 断言：未及执行（B2 中断）。n1 judge 的 fixture↔卡 notes.theme 对账未到达。

## 二、根因链（agent_last.log + 代码锚点）

时序（agent_last.log，全部在 turn 1 的 623ms HTTP 请求窗口内）：

1. 09:24:14 Phase A `new_project`+`save_as_project` 落 run 目录 → 内核当前工程切换为 fixture 工程，但 **agent 侧工作区未激活**（此前工作区=脚本临时草稿 `VitStackDrafts\dev_agent_smoke_20261107_092402\...\Unsaved.vit`）。
2. 09:24:15 turn 1「开始乐局」：挂载点（goalrunner_chat.go:190，durableInvocationID=="" 门）正常进入 → eduJamEnter 正常执行——projectUUID 经 kernel 回退路径读到 fixture uuid（edu_jam_chat.go:945-964）→ `storeEduJamSession` 写入内存会话表（:262）→ **同一请求内**，工作区管理器检测到内核工程变更，激活 fixture 工作区：`[workspace] activated project=…edu_jam_fixture_project.vit conversations=0`（log 09:24:15）。
3. 激活路径 server.go:7542-7568 读该工程持久化 runtime state——**fixture 是全新工程，盘上无记录 → state 为空** → `restoreProjectAgentRuntimeStateLocked` 执行 `s.eduJamSessions = s.restoreEduJamSessionsLocked(state.EduJamSessions)`（server.go:7899）→ **把第 2 步刚写入的乐局会话整表空覆盖清掉**（该行注释「无记录（所有既有工程）→ 空覆盖层零行为变化」对既有工程成立，对本场景=静默丢弃在途会话）。
4. 09:24:17 turn 2「写好了」：parseEduJamCommand 命中 check 词形（edu_jam_chat.go:96-100）→ eduJamAdvance → `eduJamSessionFor` 查表为空 → **按设计放行**（edu_jam_chat.go:401-404「没有乐局时写好了是普通会话轮」）→ 落入 semantic_entry → LLM 分类器计费 1 行（deepseek-flash，本机默认引擎无注入）→ unresolved → py exit 4。

**缺陷定性**：乐局会话生命周期与工作区惰性激活的竞态——enter 允许在「工作区未激活」时开局（kernel 回退读 uuid），而激活时空态 restore 会无条件覆盖内存会话表；两者组合使「enter 后紧接推进」在同一切换窗口内必然丢会话。属 281 文件域内（edu_jam_chat.go + server.go 快照接线）的确定性接线缺陷；非环境、非 LLM 随机性（turn 2 的 LLM 分类是放行的下游后果，放行本身是确定性的）。

**单测为何没拦**：段 1 的 9 个测例（edu_jam_chat_test.go）全部在单一预置工作区状态下跑，未建模「请求内工作区切换/激活」这一交互面。

## 三、计费（如实）

telemetry 共 **2 行**（logs/agent_llm_telemetry.jsonl，均 deepseek-flash 本机默认）：
- `dev_strip_silence_chat_20261107_092408` ×1——默认 Strip Silence **chat** 子腿（-NoChatSmoke 只关 9391 行 chat 路由腿；strip silence 链内的 chat 子腿仍计费——登记文「-NoChatSmoke 保全 run 零 LLM」申报不完整，如实更正）；
- `edu_jam_smoke_20261107_092414` ×1——turn 2 放行后 semantic_entry 分类（缺陷下游后果；若跑达 Phase C 零 LLM 断言也将以 1≠0 失败）。
合计 ≪¥0.01（两条 flash 短分类）。

## 四、工件清单（本目录）

console_ps1.txt（ps1 全程+PS1_EXIT=1）/ edu_jam_console.txt / edu_jam_summary.json（fail_phase=n0）/ chat_responses.json（2 回合原始响应）/ agent_last.log（282 行，含 workspace activated 关键行）/ logs/agent_llm_telemetry.jsonl（2 行）/ default_project.xml+gui_project_isolation.json（隔离标记）/ edu_jam_fixture_project.vit（fixture 工程）/ binary_sha256.txt / .vit_history+.vit_agent（会话运行态，34K/4K）

## 五、处置（交主管，本腿不扩修）

§8 exit 4=取证上交不重跑已兑现。修复面在 281 文件域内，候选方向（不预设）：
- A（最小）：激活路径对 EduJamSessions 做 merge 而非整表覆盖（保留在途会话；需定义同 conversation 冲突时新态优先）；
- B：enter 强制要求工作区已激活且绑定=内核当前工程（fail-closed 拒绝 pre-activation 开局，冒测腿需先触发激活再 enter）；
- C：eduJamSessionFor 查无时回退读内核工程绑定/盘上快照再判（恢复链兜底）。
附带：单测补「激活窗口竞态」建模用例（enter→激活→推进）。
