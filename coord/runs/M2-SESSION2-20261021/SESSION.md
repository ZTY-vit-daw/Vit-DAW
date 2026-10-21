# M2 体验场次②实录（2026-10-21 晚窗，用户手测）

- 场次：用户从 Godot 拉起前端进入六轨工程 spv1_p06（同场次①入口）；authority=full_project_access；模型=deepseek-flash 思考档；conversation=webui_mvi4cp4y（两轮同会话）。
- 被测二进制：agent=主检出 agent/bin/VitAgent.exe 构建 60DB91F3（含 155/152/153/157 合入面，bb3d863f 追平记录）+内核 722A79D7 +前端 main=0e244f0。
- 本件=主管事后取证（现场只读，栈保持原样至用户收）。

## 轮 1（goal_103a506dd046f160，20:59-21:00）——「给人声加点空间感」

回复=模型自报能力边界，理由原文：「人声轨的目标级证据已确认该轨偏窄、相关性偏高，与"加空间感"的听感目标相符；但动作前置校验要求本次运行内已交付的整项目扫描凭据，该凭据未交付且本轮观察窗口已耗尽…」。

判读：

- ✅ **157 生效**：无 G 门代号堆叠；「人声」解析命中 1032；人声轨被观察并拿到目标级证据（对照场次①同话术=秒回 G3/G5/G6/G7/G8 代号）。
- ✅ **152 生效**：边界文案=模型自身可操作理由（proposal_missing→summary 面），非门清单。
- ⚠️ **剩余断点=156 缺口②**：新循环 ledger 无 project 级扫描凭据（G3），观察预算耗尽于目标观察——**158（FS-LEDGER-CARRY-1 种子观察 A1）即修此面**。非回归。

## 轮 2（goal_cac272d4d1e6804b，21:01-21:02）——主话术（场次①第一轮逐字）

- ✅ **147 生效**：full-access 全程零交互卡（选择/装载/typed 确认均未弹），模型自主选 Pro-R。
- ✅ **154 生效**：总线拓扑硬规则——内核铁证（21:02:10）：`track.send.connect` source=**guitar/1017**→auto_create **Reverb Bus/1037** @-6.0dB，`created_fx_track=true`、`source_count=1`；`rack_add_node` Pro-R(1047) 落 1037。目标轨=模型自选 guitar（话术允许；场次①选 bass）。
- ✅ **155 生效**：写门放行合法引用（1017/1037 均在观察披露）。
- ✅ **148 部分**：选卡计数句中文化指名面正常（30/63 计数）。
- ⚠️ 文案两处（→卡 160）：准入边界句出现在聊天回复正文（full_access_semantic_chain.go:148 前置同缀，148 合同原设计；**用户裁定=移出回复面，结构化保留**）；typed 完成面回复英文硬编码（semantic_dynamic_workflow.go:937，148③ 遗漏面）。
- ❌ **GUI 缺陷（→卡 161）**：吉他轨 SEND 槽位/机架 send 边不显示。用户诊断：重新点选+视图切换**均不出现**——视图重建路径同样不取 send 拓扑（Vit_Graph_Rack.gd 刷新仅挂 GUI 自身命令完成 _SEND_REQUEST_PREFIXES:338/_on_send_control_ipc_completed:2204+，agent 经内核写零触发）。内核数据正确、GUI 呈现盲区。

## 验证总结

五项已合入修复（147/148 部分/152/154/155/157）在真实用户旅程验证生效；衍生三卡：158（轮 1 断点=既有卡）、160（文案两处，用户裁定）、161（send GUI 刷新，前端仓）。

## 工件

- `agent_last.log`（68KB 全量，两轮 timing/ledger/continuation/shadow delta 链）
- `kernel_send_connect_excerpt.txt` / `kernel_rack_add_node_excerpt.txt`（内核命令回执铁证段）
- 原始日志：VitApp/Workspace/Logs/VitHeadlessServer2026-10-21_20-58-41.log（现场保留）
