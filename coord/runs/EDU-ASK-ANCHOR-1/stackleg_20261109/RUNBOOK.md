# 282 EDU-ASK-ANCHOR-1 真栈轻腿 RUNBOOK（§8 跑前冻结，2026-11-09）

## 场景（单栈窗口，两个阶段）

1. **阶段 A（既有行为不回退）**：`scripts/dev_agent_smoke.ps1 -RepoRoot D:/Vit_DAW_wt_282impl -StartKernel -RestartAgent -KernelExe <wt_228 内核>`——默认腿（ChatSmoke 路由不回退 + StripSilence 链不回退）exit 0 即既有 pack 行为零变化的真栈冒测证据（parity 证据已在卷，本腿只测不回退）。
2. **阶段 B（transport loop 注册链真栈断言）**：`transport_loop_leg.py`（本目录）——内核 ZMQ 保底工程 → chat 驱动两命令 → 断言 → 内核 ZMQ 独立读面双核。

## 被测

- agent = wt_282impl 现场构建（dev_agent_smoke.ps1 构建 `cmd/vitagent`，含实现 commit 873c0e9c 注册四面：catalog spec + policy 兜底 + chat 门控第四面 + 引导段）。
- kernel = `D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe` SHA256 `F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A`（225-319 多卡只读复用同款，本卡零内核改动）。

## 话术（覆盖边界②应对）

- 轮1（set）：含「循环播放/反复听」——「播放」在 agentLoopTextHasAny transport 词表，transport 能力面激活。
- 轮2（clear）：含「循环播放」同上。

## 断言面（三层）

1. **chat 轨迹层**：`/agent/events` 中 `item_type=daw_action` 的 item.completed 事件携带 `transport.set_loop` / `transport.clear_loop` 真实工具调用（item_id/payload.tool/payload.command_name 面，278 run6 先例——turn body 文本回显不算）。
2. **工具回执层**：set 轮 daw_action payload.result 含内核回执（status=ok + loop_enabled/loop_start_seconds/loop_end_seconds）。
3. **内核独立读面**：chat 后经内核 ZMQ `stop`（幂等，不改 loop 态）读 buildTransportReply——set 后 loop_enabled=true 且 loop range 与工具回执逐字段一致；clear 后 loop_enabled=false。

主断言=读面与回执一致（不硬编码模型参数选择）；「参数恰为 10/15」作软记录。

## 运行次数 / 预算 / 失败分类 / 止损线

- **单 run 无重跑**；环境类中断（零 LLM 轮次前的进程/端口/构建失败）允许一次重建；功能面失败直接取证上交不自动重跑。
- **LLM 预算上界**：8 调用 deepseek-flash（chat set 轮 1 + clear 轮 1 + 治理确认兜底 approve hop ≤2 + 阶段 A 默认 ChatSmoke 2 + StripSilence ~1）；in ≤120k / out ≤12k；计费上界 ≈¥0.1。
- **成功条件（exit 0）**：阶段 A PS1 exit 0 + 阶段 B py exit 0（两层 chat 断言全过）。
- **失败分类**：
  - exit 2 = 环境中断（栈起不来/端口冲突/LLM API 无响应）；
  - exit 3 = LLM 行为类（轮完成但模型未发起 transport 工具调用——no_tool_call，取证含当轮披露目录）；
  - exit 4 = 装置/链路类（工具被调但回执失败：no_active_edit/参数拒绝/工具不可见=注册面断裂）。
- **止损线**：功能面同型断点 2 连败停止；本腿单 run 设计，首败即收口上交（人工判型后再定是否重跑）。
- **r2 型治理确认兜底**：chat 终态若出现 pending 确认卡（interaction_requests），按 278/306 先例 `/agent/interaction/respond` approve 同款驱动，≤2 hop；approve 后重新收终态。
