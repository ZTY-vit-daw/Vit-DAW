# Ruling：PORT-SMOKE-MAC-3 pass（2026-09-20，VSP 五件套全绿）

- 卡：`coord/cards/done/2026-09-20-PORT-SMOKE-MAC-3-vsp-suite.md`
- 实现：`port/smoke-mac-3` @`b246305`（6 文件 +3729：五件 mac 脚本 + SMOKE_TESTS）→ cherry-pick 入 main
- 裁定：**pass——VSP 五件套在 mac 三件套口径全绿，现行套件 PC-only 项清零**

## 决策侧核验

1. 五件权威 run 各 exit 0（§8 零失败轮；① 首 run 退出码被控制台管道吞的复跑取证诚实，补直采 run 为权威）：extension 7 门 / codex 10 检查 / asset py 9 检查（原样复用+sha256 入件）/ websocket 5 检查（**内嵌 stdlib RFC6455 客户端**替代 .NET ClientWebSocket——握手校验/掩码/分片/ping-pong，零外部依赖）/ lifecycle 11 门（决策侧直读 summary=PASS）。
2. **栈与二进制账目严格**：每件独立三进程起停、五端口归属断言（8787|hub / 7878|agent / 5555+5556|kernel，含 inode 比对与运行二进制 sha256 核对）、teardown 排空复核；agent/hub 自 HEAD=b15b35b 新建、内核 §9 合规复用；主仓 Workspace 零触碰（fake-root 运行态）。
3. 语义对照与边界：ps1↔mac 逐件对照在案；未移植边界（Godot autostart 生命周期本体/-Reuse*/60-track/GUI 探针——需前端作栈主）显式声明，采认（归后续前端作栈主的扩展口径）。
4. LLM 零参与（纯协议确定性断言），key 面不适用。

## 记分板更新（mac 端测全绿 10 → 15 件）

readonly / A5 / C2 / journey / ab_result / preflight / stems / b1_group_reset / b1_3_full / b4_low_end + **VSP 五件**。剩余：④⑤ 双跳断言重设计（PS1-SYNC-3，用户已裁方案 b，待派）→ 终局 N=5 统计复跑（SMOKE-MAC-4）即端测时代收官。
