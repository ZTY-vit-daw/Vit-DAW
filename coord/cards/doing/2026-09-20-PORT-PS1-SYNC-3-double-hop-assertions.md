# PORT-PS1-SYNC-3：④⑤ 断言按双跳确认工作流重设计（PS1-SYNC-2 后续，用户裁定 b）

- 优先级 / 预估 / 依赖：P1 / 0.5-1 天 / 依赖 PORT-PS1-SYNC-2（已合入 main：settle 720/needle 三词/waiting_confirmation 合成面全在位）
- 模型分级：L2 / GLM-5.3（双跳工作流断言语义设计 + 双端同步 + 真实栈跑绿）
- **背景（用户裁定 2026-09-20，[decisions/2026-09-20-ps1-45-disposition-b.md](../decisions/2026-09-20-ps1-45-disposition-b.md)）**：双跳确认是产品设计——第一次确认=改善性提案确认；随后 agent 发起工具申请；第二次确认=工具申请执行确认；然后才应用。探针已证端到端走通（Track 1010 电平 -1dB：确认①→improvement_proposal_native_tool_confirmation_required（workflow=mix_tick）→确认②→已应用并回读验证 0→-1dB→终态 d1_post_action_evaluation_required）。④⑤ 现行断言编码经典单跳（mix.tick.pending 事件等待/单跳 applied_reobserved/提案时 derive 投影）——按用户裁定改测试不改产品。
- 目标：
  1. **④ `run_mix_single_tick_e2e` ps1+_mac.sh 断言重设计**：observe 回合 needle 命中后走双跳确认链——断言第一次确认后到达工具级确认面（workflow=mix_tick 的 pending/needs_confirmation，"Track … 待确认"形态）；断言第二次确认后**已应用+回读验证**（期望值变化，如 -1dB 0→-1）+ 终态 `d1_post_action_evaluation_required`；废弃 `[mix.tick.pending] stored` 日志等待与单跳断言（判据来源：探针对话 conv `mix_single_tick_e2e_20260920_182906` 与 PS1-SYNC-2 四件套工件 `coord/runs/PORT-PS1-SYNC-2/`、`VitApp/Workspace/Artifacts/smoke/product_path_20260920_*`）
  2. **⑤ `run_vit_product_path_smoke` ps1+_mac.sh vocal focus 断言重设计**：settle 至 needs_confirmation（提案确认面）为正路径；废弃"pending route 含 mix.derive"断言（derive/apply 经 ④ 探针证实发生在确认后，提案停车时不执行）；route 断言维持 ccb.observation_catalog 别名组（PS1-SYNC-1 契约）；若脚本内驱动确认链，按 ④ 同款双跳断言
  3. **PC 跑绿**：④⑤ 两件 ps1 权威 run exit 0（flash 口径不换模型）
  4. SMOKE_TESTS.md ④⑤ 条目契约描述同步（双跳语义、两次确认的不同确认面、d1 终态）
- 文件域：`scripts/`（④⑤ 的 ps1+_mac.sh 共 4 文件断言块 + SMOKE_TESTS.md；最小 diff 不重构）；`agent/`、`VitApp/`、前端仓零触碰
- 验收标准：①④ 双跳断言 diff（两次确认语义分层：提案面/工具面）；②⑤ vocal focus 断言 diff（needs_confirmation 正路径+derive 断言废弃理由注释）；③PC ④⑤ 各权威 run exit 0 工件（§8 ≤3 轮、同断点两败止损）；④SMOKE_TESTS.md 同步
- 停止条件：双跳链断言在新设计下仍红且红因在断言语义之外 → 取证上交（届时指向 agent 侧，另开卡）；需改 agent/内核才能绿 → 立即域外上报；环境中断如实记录
- 附注：mac 侧 ④⑤ 复跑不在本卡（mac 自行安排，可并入其复跑批次）；双端断言语义必须一致
- 领取：2026-09-20 19:30 / PC 执行侧（GLM-5.3） / origin/main=07d0e732f96674865b9d629fa82d88fcf635bdef / 分支 port/ps1-sync-3 / 领取时工作树：M VitApp/Workspace/default_project.xml（烟测已知保留项）、D coord/cards/doing/{2026-09-19-PORT-PS1-SETTLE-1,2026-09-20-PORT-PS1-SYNC-2}（决策侧 done 归档的 doing 删除遗留，不代提交）、?? coord/runs/ + extension 构建产物 + godot-cpp/（均域外不动）
- 回执：
- 验收：
