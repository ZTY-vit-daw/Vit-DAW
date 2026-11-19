# W02-A1-HANDTEST-V0 prep 记录（脚本生成）

- 生成时间：
- 结论：passed / failure_class=
- 模式：-SameStackPrep（工程已打开、不 project.open；import/enter/S1.5/S2 + rename/save/copy；不跑 S3-S6、零 LLM、不启停栈）
- 模式标记：同栈（same_stack，手测主用）
- 栈：http://127.0.0.1:7878（owner=lead/pc-20261119）
- RunRoot（临时工件）：C:\Users\timoz\AppData\Local\Temp\vit_w02_a1c\run_20261119_215117
- git head：ef30ca3ef1124c0366c9af63fd97b4f01eb1de66

## 预置工程

- 原工程：D:\Vit_DAW_handtest\W02-A1-v0\live.vit
- live：D:\Vit_DAW_handtest\W02-A1-v0\live.vit
- pristine：D:\Vit_DAW_handtest\W02-A1-v0\prep-20261119_2152\pristine\live.vit
- prep 会话 conversation_id："w02-a1ht-20261119_215117"
- 预期节点："n2-rhythm"（第 3 关 / clip 节点，乐局已导入、已开局）
- card_id："music-creation-unit2-demo"；level 绑定：D:\Vit_DAW_handtest\W02-A1-v0\.vit_agent\vitproj_020340e8ae5d452c967f0fd1bf29bcba\legame_binding.json

## 轨道与 clip

- 目标轨：我的旋律（选第二个 clip）（track_id=1010）
- 参考轨：参考（空白）（track_id=1007）
- clip_id=1013 start= notes=60@0/1,64@1/1,67@2/1
- clip_id=1014 start= notes=67@0/2,71@2/0.5,74@2.5/1.5
- 第一个 clip（控制组）：1013
- 第二个 clip（手测要选的那个）：1014

## 保存与验证取证

- 保存命令："project.save"，kernel status="ok"，lifecycle="save"
- .vit 大小 2328 -> 4403 字节；sha256 CB3B7E6DF498898BBC122310F3E24A4B44B59BC5EF20C568906964DA89136BF7 -> 85C174BFBE2CE4F52E3398D81B3B4ACEA7F6FA50141BEA180C76FDD870D0A260
- pristine 快照：文件数=134，.vit sha256=85C174BFBE2CE4F52E3398D81B3B4ACEA7F6FA50141BEA180C76FDD870D0A260（同栈：reset=重跑 prep，不做文件还原）
- live 验证（未重开）：用户轨 2（track_count=2），目标轨 clip 数=2，参考轨 clip 数=0
- 同栈 prep 会话：stop_reason="edu_jam_status"，current_node="n2-rhythm"，期望 "n2-rhythm"
- 持久化状态探针（支撑证据）：node_active（file=D:\Vit_DAW_handtest\W02-A1-v0\.vit_history\.sessions\vitproj_020340e8ae5d452c967f0fd1bf29bcba\session_20261119T135123_75fe874f\workspace\state\agent_runtime_state.json，node_state=active）
- 零 LLM：telemetry 新增行=0（本会话），可用=True

## 已知边界

- 本脚本只操作**已存在的栈**，不启停进程、不占端口；prep 结束后栈仍由 lead 拥有，由 lead 停栈后核 process/TCP/UDP 零。
- 重开发生在同一 agent 进程内：它证明 .vit 里有 tracks/clips，也证明 prep 会话在重开后仍报 n2；**冷启动恢复**（新进程/新栈）以手测为准，见 coord/runs/W02-A1-handtest/RUNBOOK.md。
- teardown **不关闭** prep 会话（否则 fixture 不再是 n2）；recents / Settings.xml / default_project.xml / authority 仍按字节还原。
- 运行工件（requests/responses/snapshots/evidence）在 RunRoot；本文件与 summary.json 是本轮的 prep 记录。
