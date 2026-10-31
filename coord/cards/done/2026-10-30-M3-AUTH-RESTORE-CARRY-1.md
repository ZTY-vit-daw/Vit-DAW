# M3-AUTH-RESTORE-CARRY-1：authority_mode 跨重启恢复——RecoverWorkingSession 播种面修复（10-29 用户事故真身，223 重启腿定案承接）

- 发卡：GLM 主管决策流 / 2026-10-30 深夜（依据=223 真栈重启腿红定案：RecoverWorkingSession 仅 pending-work 恢复+新 session 从 canonical 播种，前一 session 的 authority_mode 跨干净重启永不携带；锚点 working_session.go:94-101/165-215+server.go:7517-7548 在卷——10-29「权限模式未生效」事故真身，219 面 5 三窗口勘察被此定案部分推翻）
- 派发确认：已确认（用户 2026-10-30 手测第 5 步「重启后仍完全权限」即诉求，卡 223 验收转出）
- 验收负责人：GLM 主管决策流
- 池序 226；目标仓库=**主仓 agent**；P1 / 0.5 天 / 依赖=223 已合入（39076e2e）✓；L1-L2（恢复链语义判断）

## 标的（223 定案，不重考）

干净重启（无 pending work）后新 session 从 canonical 播种，不携带前一 session 的 authority_mode/authority_mode_explicit → 用户设置的完全权限重启即失。219 面 5 只修了「切换时 workspace 未激活静默」窗口；本卡修「恢复链不播种」真身。

## 目标

1. **恢复播种**：session 恢复（含干净重启的 canonical 播种路径）携带持久化的 authority_mode（含 explicit 闩语义——恢复的 full 态等价于控制面显式设置过；**裸模式串永不授权红线维持**：播种源=已持久化的 runtime state（history.WriteAgentRuntimeState 链，219 面 5 已勘察），非请求体）。
2. **跨工程隔离维持**：A 工程完全权限不漏入 B 工程（AUTHORITY-LOST-1 server.go:7469-7483 既有裁定零回退）。
3. **223 重启腿复验**：chat_auth_stamp_smoke.py 两相腿（-ChatAuthStampSmoke）重启 phase 转绿——after=restore 直执 span 采纳面+manual 确认卡恢复+伪造 body 409 负例全过。

## 探针义务

单测：干净重启形态 restore→authorityModeSnapshot=持久化值（含 full+manual 双向）；无持久化态=默认 manual（fail-closed）；跨工程切换隔离负例。真栈腿：223 的两相 smoke 重启腿转绿（必跑，exit 0 为门）。

## 文件域

`agent/internal/chat/working_session.go`+`server.go`（恢复链 7517-7548 邻域）+测试+scripts（复用 223 断言体）。越域即停。

## 停止条件

- 播种与 pending-work 恢复路径状态机冲突（恢复中的权限语义不可判定）→锚点+两案上交；
- 发现 canonical 本身不含 authority_mode 持久化位（需动持久化格式）→§11 兼容义务上交（旧工件缺省语义+往返测试）。

## 领取

- 时间：2026-10-31（PC 执行会话领取）
- origin/main：33568012affd812383f15854fd9a406f5e694458（=fetch 后核对一致）
- 领取时 worktree 基线：`git rev-parse HEAD`=33568012；`git status --short`=空；`git diff --stat`=空（新建 worktree 干净）
- owner：ZCode GLM-5.3 PC 执行会话（226 卡会话）/ PC (Windows)
- 分支：port/m3-auth-restore-carry-1
- worktree：D:/Vit_DAW_wt_authcarry1（实现）+ D:/Vit_DAW_wt_authcarry1_coord（协调）
- 领取提交：3f7b856a（claim，已 push main 重读确认）+ 49445823（真栈占用登记）+ 本收口提交（释放+done）
- 真栈错峰：229（M3-LLM-TURN-BUDGET-1）并行中——本腿先登记（11:05 条目），229 真栈腿应等待本条释放；本卡真栈腿已收口释放

## 回执

- **结论：验收门全达——单测全绿 + 真栈两相腿 exit 0（restart_restore 真身腿转绿）**。卡留 done 待主管验收。
- 实现 commit：`4a94938e`（port/m3-auth-restore-carry-1）；真栈+smoke 修正 commit：`b0c3fcf3`（同分支，已 push）
- 改动面（域内）：`agent/internal/history/working_session.go`（LatestPersistedAuthorityMode 只读扫描+persistedAuthorityAt+ResetProjectIdentityRegistryForTest 测试帮手）+`agent/internal/chat/server.go`（恢复链 7527-7548 邻域 seed+写回）+测试 2 文件 8 例+`scripts/chat_auth_stamp_smoke.py`（223 断言体计数面缺陷修正，见下）
- 机制：干净重启（无 pending work）→EnsureWorkingSession 从 canonical 播种的新 workspace state 缺/滞后 authority_mode（最新值在前一 session workspace 的 WriteAgentRuntimeState 面）→恢复链采纳 LatestPersistedAuthorityMode（canonical+全部 session workspace 按 saved_at 取最新）并 persistActiveProjectWorkspaceLocked 写回新 workspace（防 scheduler 250ms disk replay 冲回）。播种源=已持久化 runtime state 链，非请求体（红线维持）；explicit 闩语义=stamp 面恒显式（恢复的 full 等价控制面显式设置过）；AUTHORITY-LOST-1 7469-7483 跨工程闩清零零改动（隔离负例单测全绿）
- 单测（探针义务）：`go test ./internal/chat/ -run TestCleanRestart -count=1` 4 例 PASS（full/manual 双向+无持久化 fail-closed manual+跨工程隔离负例含 replay 不回滚/写回断言）；`go test ./internal/history/ -run TestLatestPersistedAuthorityMode -count=1` 4 例 PASS（canonical 滞后/最新值胜/无值 false+异工程不漏/裸值透传）；全包回归 `go test ./internal/chat/ ./internal/history/ -count=1` ok（131s/5.6s）；`go build ./...` 无错
- 真栈（exit 0 为门）：run=coord/runs/M3-AUTH-RESTORE-CARRY-1/（RESULTS.md 在卷）——round1 restart_restore **PASS**（日志亲证 `clean-restart seed adopted mode=full_project_access`）但 tracks 腿红=**223 断言体从未执行过的计数面断言缺陷**（总数==3 对 kernel 全轨树不成立：agent 重启后 per-project 过滤上下文丢失暴露 parent 默认轨 1002-1006+setup 3 轨=8；工程文件恰 3 轨+隔离副本 uuid=parent 佐证）→scripts 域内修正为 before summary setup 轨 id 成员断言（本意不弱化，223 round1 自修断言体先例）→round2 两相全绿 **ps1 exit 0**（restart_restore+track_a_holds+full_span_direct+manual_card+forged 409+cleanup）
- §7 分开声明：链路进度=实现+单测+两轮真栈证据齐；最终验收=round2 exit 0 COMPLETED（脚本门）+单测全绿；主管裁定为最终验收
- 端测边界声明：本卡改动面=agent 恢复链+scripts，不涉 webui 渲染面与 demo 旅程面；真栈覆盖=agent HTTP/ZMQ 面（E2E-WEBUI-1/JOURNEY-1 层级不适用）
- 遗留观察（供主管知悉，非本卡缺陷）：round1 FAIL 后 10:42:24 重启 agent 有一次自发 add/delete_track（kernel revision 5→6 终态回 3 轨），不在断言窗口内，round2 未复现异常；如复发按取证卡处理
- 真栈占用：11:05 登记→两轮腿→拆栈复核进程/端口全零（VitApp/VitAgent pid 路径亲核=本腿所属）→释放条目同批

## 验收（GLM 主管决策流 / 2026-10-31）

- **pass 合入 7bd2dfc1**：红线三面亲核（播种源=已持久化 runtime state 非请求体；跨工程隔离负例单测+AUTHORITY-LOST-1 server.go:7469-7483 零改动亲核；§11 无新持久化字段、旧工件 fail-closed 语义不变）。单测 8/8 主管分支复跑全绿+chat/history 全包回归复跑 ok（115s/5.5s）+build ok。真栈 round2 两相腿 summary_after.json 亲证（restart_restore=full_project_access=223 红腿转绿+full_span_direct 直执+manual 确认卡恢复+伪造 body 409+cleanup）；round1 红腿=223 断言体从未执行的总数断言缺陷，定案成立（per-project 过滤上下文随旧 agent 进程丢失→kernel 全轨树 8 条暴露），改成员断言不弱化（setup 轨 id+volume plugin 存活断言更精确）。
- 卡面路径笔误记录：working_session.go 实际在 `agent/internal/history/`（223 锚点 RecoverWorkingSession 扫描环 :94-101 亲核），非卡面所写 chat/ 包——语义域不变，非越域。
- 遗留观察留档：round1 FAIL 后 10:42:24 agent 重启窗口一次自发 add/delete_track（kernel revision 5→6 终态回 3 轨；断言窗外、round2 未复现）——复发按取证卡处理，本卡不阻塞。
- 用户手测欠账第 5 步（重启后仍完全权限）随本卡解除。
