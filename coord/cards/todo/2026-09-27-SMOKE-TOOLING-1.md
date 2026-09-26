# SMOKE-TOOLING-1：烟测工具修缮三小件（登记项收口）

- 优先级 / 预估 / 依赖：P2 / 0.2 天 / 三项均已登记在案：TIM-SMOKE-1 验收附带（dev_agent_smoke 20s 预算+tim_assert_smoke BOM）+TIM-SMOKE-1 边界声明（cont_stall 通配符）
- 模型分级：L1 / flash 可接（脚本小修，零生产代码）
- **执行侧（mac 会话，夜池——注意三件都是 Windows ps1，mac 上修改+PC 复验，或转 PC 白天；夜池执行则测试面声明边界）**
- 目标（三小件）：
  1. **dev_agent_smoke 等待预算参数化**：UI 端口等待 20s 提为参数（默认保持 20，可 -UiPortWaitSeconds 60+）——三例同形态环境失败（内核 994 表冷加载）的根治。
  2. **cont_stall_repro_smoke.ps1:440 通配符修复**：`-like "*[tim.assert]*"` 方括号是字符类通配（TIM-SMOKE 上交的同款问题）→ `.Contains()`；grep 全 scripts/ 同模式扫一遍一并修。
  3. **tim_assert_smoke.ps1 BOM 修复**：run_report.json 等 Out-File 加 `-Encoding utf8NoBOM`（BLIND-BOM-1 同族根治）。
  4. 验证：每件的最小验证（1 件参数化后短跑或语法检查/2 件 grep 复验零残留/3 件写出的 json 无 BOM 字节检查）——**mac 上不能跑 ps1 真验证的，修改后声明"PC 复验待决策侧"**，决策侧白天复验。
- 约束：零行为变化（参数默认值不变）；文件域 scripts/*.ps1。
- 验收：三件修毕+验证记录（或 PC 复验声明）+grep 零残留
- 停止条件：无（纯工具卡）
