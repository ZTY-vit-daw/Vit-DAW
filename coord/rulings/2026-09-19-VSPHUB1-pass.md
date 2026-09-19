# Ruling：PORT-VSPHUB-1 pass（2026-09-19）

- 卡：`coord/cards/done/2026-09-19-PORT-VSPHUB-1-mac-runtime.md`
- 实现：`port/vsphub1-mac-runtime` @`bb26c54`（冒测入口 941 行 + SMOKE_TESTS 条目）→ cherry-pick 入 main `f5f9e30`；前端翻回 `08ecdb3`（本地仓）
- 裁定：**pass——vsphub mac 运行时正式就位，三件套（内核+vsphub+agent）成为 mac 可用栈形态**

## 决策侧核验

1. **六项验收全过**：全栈形态 run `20260919-195832` PASS 全门（决策侧直读 summary：bounded_fail_self_stop/kernel_ports/agent_health/registration_retry_pending/hub_health/registration_success/readonly_probe/ws_probe 八门全 true）；A5 两件套回归 run `20260919-200044` exit 0（legacy 通道无损）；vsphub sha256 入档；工件三段齐备。
2. **有界重试两向实证**（收编 7793927 的运行时首证）：负例 29 pending→attempts=30 自停（61s）、正例 hub 起 2s 内注册成功且 session 由内核签发（hello 打穿 hub→kernel）。
3. **只读往返**：codex probe 语义 10 检查全过（register→在册→snapshot→event.poll→unregister→除名，内轨 id 零泄漏）。
4. **翻回核验**：决策侧直审前端仓——fix5 文件对 `9987c5a^`、fix6 文件对 `b55c2b7^` diff 均为 0 行（逐字节复归，Windows 臂零变化由构造证明）；翻回后不设 ENABLE 的 ws 活体探针 exit 0 且零 TCP no-delay 告警。翻回理由（根因消除故整体复归，不留双臂分叉）采认。
5. **§9 纪律**：领取时遗留栈取证在案、external-kernel 形态避让、用户关闭编辑器后补跑全栈——端口争用处置规范。

## 移交

- 编辑器手测建议（非闸门）：翻回后首次 Godot 会话目检 ws 状态与告警面板（headless 探针已证数据通道，UI 面未覆盖）。
- SMOKE-MAC-1 的⑤⑦曾以"hub 未移植"按两件套口径适配——hub 现已就位，后续端测可逐步切三件套口径（不必回改已验收件）。
