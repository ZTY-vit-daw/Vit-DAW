# M3-AUTH-RESTORE-CARRY-1（池序 226）真栈腿回执

- run：`chat_auth_stamp_20261031_1`（round 1，restart_restore 已绿/后续腿红=223 断言体计数面缺陷定案）+ `chat_auth_stamp_20261031_2`（round 2，**两相全绿 ps1 exit 0**）
- 被测：worktree `D:/Vit_DAW_wt_authcarry1` @ `4a94938e`（port/m3-auth-restore-carry-1；round 2 含 smoke 断言体修正）。agent 面=history.LatestPersistedAuthorityMode（working_session.go 播种读原语）+server.go 恢复链 seed+写回+单测 8 例；ps1 在 worktree 根自建 agent=同源
- kernel：主检出 `D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe` 只读复用，sha256 前缀 `C1E32DFE` 运行前亲核=223 同源零漂移（本卡 C++ 零改动）；无 UI；vsphub 不需要（agent 直连内核 ZMQ；日志中 8787 拒连 WARN=hub 未启属预期形态，不影响腿断言面）
- 隔离：VIT_PROJECT_XML=ps1 共享工程隔离段 git HEAD 提取副本（gui_project_isolation.json 在卷，source=git HEAD 4a94938e）；draft root=per-run 临时目录
- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev_agent_smoke.ps1 -StartKernel -RestartAgent -NoChatSmoke -NoStripSilenceSmoke -ChatAuthStampSmoke -KernelExe <主检出 kernel> -RunArtifactsDir <run 目录>`
- 退出码：round 1 = 1（after 相第三腿）；round 2 = **0（COMPLETED，验收门达成）**

## 一、本卡标的真栈成立（round 1 即钉死）

**leg restart_restore（223 两轮红的真身腿）round 1 转 PASS**：agent 真重启后 `GET /agent/authority` = `full_project_access`。日志锚点（重启后 agent_last.log）：`[authority] restore adopted disk mode=full_project_access` + `[authority] clean-restart seed adopted mode=full_project_access project=<run 工程> uuid=<run uuid>`——恢复链采纳播种的最新持久化值（前一 session workspace 的 WriteAgentRuntimeState 面），与实现设计一致。round 2 同腿再绿。

## 二、round 1 红腿与定案（223 断言体计数面缺陷，非本卡恢复链缺陷）

**leg tracks：`expected the 3 setup tracks after restart, got 8`**（223 该腿从未绿过：r1 红在腿序、r2 红在 restart_restore 即早退——本轮是它第一次执行）。

- 实锤证据链：kernel 实况 get_project_state=8 轨=**5 条无名轨（1002-1006）+ setup 3 轨（1007/1010/1013）**；run 工程文件 `chat_auth_stamp_project.vit` 恰 3 轨（1007/1010/1013）；隔离副本 default_project.xml 的 `vit_project_uuid`（vitproj_44a8…）=agent 日志 parent history recovery 的 parent UUID——**1002-1006=隔离 parent default 工程的轨**，save_as 分叉后一直挂在 kernel 轨树；before 相 agent 活着时 get_project_state 按 per-project 上下文过滤只报 3 轨（before setup 以 1007/1010/1013 拿到 volume plugin 全过），**agent 重启后过滤上下文随旧进程丢失→全轨树 8 条暴露**。
- 处置（scripts 在本卡文件域内，223 round 1 自修断言体先例）：after 相轨断言由错误的总数断言（==3，before 相从未观察过总数）改为**before summary setup 轨 id 的成员断言**（3 轨在场+volume plugin 仍挂；span/manual/forged/cancel 腿的轨引用同步改用确定性 setup id）。本意不弱化：重启后 setup 轨与插件存活断言更精确。
- 遗留观察（不进断言窗口）：round 1 中 FAIL 后 10:42:24 重启 agent 有一次 add_audio_track→delete_track 自发活动（kernel revision 5→6，终态回 3 轨）——发生在 python 退出后、不接触腿断言面；round 2 未复现异常断言影响。建议主管知悉，如有复发按取证卡处理。

## 三、round 2 全绿明细（exit 0）

| 相 | 腿 | 断言 | 结果 |
|---|---|---|---|
| before | setup | 隔离工程+3 轨（1007/1010/1013）+轨级 volume 插件+基线曲线空 | PASS |
| before | switch_full | POST full→200+persisted=true+workspace_ready=true | PASS |
| before | full_direct | 裸话术直执：无确认卡+autonomy 话术+written=2+内核曲线逐点 (0,0.30)/(30,0.60) | PASS |
| after | **restart_restore** | **真重启后 GET authority=full_project_access（本卡标的）** | **PASS** |
| after | restart_track_a_holds | setup 轨 a 在场+插件在+曲线保持 | PASS |
| after | full_span_direct | span clarify→「可以」裸采纳：无卡+written+曲线 (0,0.30)/(25,0.60) | PASS |
| after | manual_card | 切回 manual→确认卡恢复+mutation_performed=false+零写 | PASS |
| after | forged_body | body 伪造 full→409+状态未变+零写 | PASS |
| after | cleanup | 取消 pending 卡+恢复 baseline 权限 | PASS |

## 四、红线核对

- 播种源=已持久化 runtime state（WriteAgentRuntimeState 面），非请求体：LatestPersistedAuthorityMode 只读 canonical+session workspace 的 state 文件；请求体面伪造 full 在 manual 下 409（forged_body 腿）✓
- 裸模式串永不授权：stamp 面（bindChatAuthorityMode/stampInvokeAuthorityMode）恒显式；恢复不经请求体 ✓
- 跨工程隔离：扫描按 (projectPath→canonicalRepo, projectUUID) 域+session sameProjectPath 过滤；单测负例（A full 不入 B、B 激活不改写 A、A 值仍可恢复）全绿；server.go 7469-7483 AUTHORITY-LOST-1 闩清零逻辑零改动 ✓
- §11 兼容：无新持久化字段（authority_mode 既有字段；缺省=manual fail-closed 已有语义）；旧工件（无 authority_mode 的 state）加载行为不变（单测 fail-closed 例）✓

## 五、工件与复验

- 全部工件随 `port/m3-auth-restore-carry-1` 分支入库（run 1 红证据+run 2 绿工件；.vit_agent/.vit_history 派生态剔除按 218/222 先例）
- 单测：`go test ./internal/chat/ -run TestCleanRestart -count=1`（4 例 PASS）+`go test ./internal/history/ -run TestLatestPersistedAuthorityMode -count=1`（4 例 PASS）+全包回归 `go test ./internal/chat/ ./internal/history/ -count=1`（ok 131s/5.6s）+`go build ./...` 无错
- 真栈占用：登记/释放记录见 PC-RUNTIME-STACK.md（2026-10-31 11:05 登记→拆栈复核进程/端口全零→释放条目）
