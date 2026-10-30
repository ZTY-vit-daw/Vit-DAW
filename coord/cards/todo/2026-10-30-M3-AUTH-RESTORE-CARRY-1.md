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

（待领取：记录 HEAD/status/diff --stat；真栈错峰登记 PC-RUNTIME-STACK.md）
