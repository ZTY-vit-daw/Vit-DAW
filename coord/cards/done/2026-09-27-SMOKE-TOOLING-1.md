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
- 领取：2026-09-27 05:18 UTC / 7f8ef6299d1555fcab10c4987c9839e8fcb5a098（origin/main）/ port/smoke-tooling-1
- 回执（2026-09-27 mac 夜池；补填提交——首推 0b47b3b 时回执填充脚本编码失败空推，本提交为实际回填）：实现 `80d014a`（port/smoke-tooling-1 已推 origin）。三件：①`dev_agent_smoke` 增 `-UiPortWaitSeconds`（默认 20=零行为变化，仅作用于 UI 内核端口等待处，泛用 WaitSeconds 不动）；②方括号字符类通配四处全转 `.Contains()` 字面匹配（cont_stall_repro_smoke :326/:333/:440+cont_stall_two_goal_smoke :356），**grep 复验零残留**（精确模式 `-like "[^"]*\[[^]]*\][^"]*"` 全 scripts/ 空）；③tim_assert_smoke 十处工件写出转 `Write-Utf8NoBom`（`[System.IO.File]::WriteAllText`+`UTF8Encoding($false)`）。**偏离卡面字面说明（决策侧核）**：未用 `-Encoding utf8NoBOM` 字面——Windows PowerShell 5.1（烟测宿主）无该枚举值会抛错，而 PS7 上 `utf8` 本就无 BOM；.NET 写法两端均产无 BOM 字节，达成卡面意图（BLIND-BOM-1 族根治）。**附带修复**：cont_stall_repro_smoke 三处内嵌真实 CR 字节的 `-replace` 正则字符串（原为 `"<CR>?<LF>"` 字节形态）规范为 `\r?\n` 转义文本（正则语义等同：可选 CR+LF）——python 文本读写误伤的修复，全 scripts/ CR 清点 before 3/after 0。**验证边界（卡面口径）**：mac 无任何 PowerShell（pwsh/powershell 均缺）——①参数化短跑与③BOM 字节检查**PC 复验待决策侧**（建议复验点：`-UiPortWaitSeconds 60` 冷启动一次+run_report.json 首 3 字节非 EF BB BF+任一 cont_stall 脚本断言日志行命中）；②grep 复验已在本机完成。
