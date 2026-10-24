# FE-SEND-LEVEL-FADER-1 (183) 真栈 SendRouteGuiSmoke 腿 RESULTS

- run（收口，PASS）：`sendroute_gui_20261024_191155` —— 脚本 exit 0（"dev agent smoke completed"），stack probe **72/72 PASS**（SUMMARY_JSON checks_failed=0 = 182 后基线 64 + **本卡 L11 拖动腿 8 断言**）。
- run（首轮，环境污染留档）：`sendroute_gui_20261024_190938` —— 67/72，5 失败全围绕一条**预存外来 send 边 1032 vocals→1037 "Reverb Bus" @+6.0**（L2/L4 读回集合多一条、L5 macro 量被外来边覆盖 6.0≠1.537、L6 disconnect 后 list 非空、L7 repository row 不匹配）。
- 污染归因（证据链）：非本卡改动引入。SendRouteGuiSmoke 非 journey berth（`dev_agent_smoke.ps1` StartKernel 分支：journey 之外内核工作目录=exe 所在目录）→ kernel 加载**主检出** `VitApp/Workspace/default_project.xml`（运行时状态，M 长期在卷，本轮含 1032→1037 auxsend×2 边——grep 亲核 1037/"Reverb Bus"/auxsend 在场）；182 当时（2026-10-24 00:0x 前）该文件无此边故全绿；本卡改动域=前端仓（内核/脚本/工程零触碰）。stack worktree git 版工程 grep 同词=0（干净）。
- 处置与复跑：不触碰主检出运行时工程状态（AGENTS §10/§12——权威输入）；改跑法=VIT_PROJECT_XML 指向 stack worktree **git 干净版工程的隔离副本**（`lvlfader_iso_project/default_project.xml`，173/174 先例同款隔离面，主检出 workspace 零写入亲核）复跑同命令 → 一次过 72/72。
- L11 断言面（两轮均过）：推子在场+值 label / 初始 unit 映射读回 -5.5 / **真实事件管线拖动**（motion→press→motion…→release 全程 push_input，禁 emit 直调）/ 拖动中间帧 repository 读回不变（拖中零落值）+ 值 label 本地回显 / 释放读回收敛到量化落值 / 推子位随拓扑回执同步。
- 被测：前端 worktree `D:/Godot/project/vit-daw-frontend-lvlfader1@port/fe-send-level-fader-1@060a5e9`（含对卡推子+167 槽位同升+L11 探针腿）；kernel=主检出 K3 `722A79D7A3F53797F5713209D812D8CE9F2888CB2A7E743286FA56B99629DFEC` 只读（占用前亲核）；RepoRoot=主仓 stack worktree `D:/Vit_DAW_wt_lvlfader1_stack@port/fe-send-level-fader-1-stack@0a6dd0bf`（Go 零改动）；Godot 4.6.1 console headless。
- §8 记账：LLM=0（全确定性探针+脚本）；两轮均登记占用（0a6dd0bf）内执行；零原样重跑（复跑=改跑法后首跑，非同签名重试）。
- 拆栈：19:12 owner 手拆本轮 VitApp 7480/VitAgent 10288（首轮残留 14624/14856 同会话手拆）→ 复核六端口 tcp listen=0、4445/5555 udp=0、VitApp/VitAgent/vsphub/非编辑器 Godot=0；用户 Godot 编辑器全程未动。
