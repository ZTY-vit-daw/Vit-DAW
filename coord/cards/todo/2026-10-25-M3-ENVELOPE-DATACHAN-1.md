# M3-ENVELOPE-DATACHAN-1：automation 数据通道+写回底座（M3 步骤③实施 C1）

- 发卡：GLM 主管决策流 / 2026-10-25 晚窗（依据=[M3-ENVELOPE-LANE-RECON-1 验收](../done/2026-10-25-M3-ENVELOPE-LANE-RECON-1.md) 标注点裁定①②③+拆分建议 C1）
- 派发确认：已确认（勘察卡验收裁定链——phase-ordering v2 §M3 步骤 3 实施序列）
- 验收负责人：GLM 主管决策流
- 池序 187；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；P2 / 0.5 天 / 依赖=M3-KERNEL-1 合入（✓）+186 勘察验收（✓）
- 模型分级：L1 / PC 执行会话
- 并行：与 185（app/rack+stack_probe L 腿）文件域不相交可并行；与 C2（车道 UI）前期并行。

## 目标（依 RECON §二案 A+§4.1 语义底座）

1. **新文件 `app/kernel/clients/automation_client.gd`**：三方法 `get_curve` / `set_points` / `clear_region`，走 RackClient 同通道（kernel_command_client UDP 4445 legacy 面；get_curve 已入内核只读白名单，零新请求面——186 已核）；payload 形态同 `rack_client.gd:68-85` track_send_* 先例；`rack_client.gd` 预期零改（若实现发现必须改它，先上交再继续）。
2. **回执解析**：set_points/clear_region 回执字段 plugin_id/param_id/mode(region|full)/region/written/point_count/time_base（PluginRackControlService.cpp:2193-2200 形态）；get_curve 回执 points[{t,v,c}]+bypass+time_base+param_id。
3. **错误分类面**：beats 曲线拒收（set_points 只收 time-based）/空点集拒收（clear_region 承担清除）/目标缺失——错误文案原样透传内核串（184 本地化先例不适用于内核技术错误，呈现层归 C2）。
4. **工作副本底座**：曲线工作副本持有+区域替换语义纯函数（拖动编辑域→完整点集发包的最小纯函数面，放共享 rules 件或本 client——形态执行侧定，C4 直接消费）；**单点 add/move 面不存在是有意省略，不得造**。
5. **变更通知形态（裁定①③落地面）**：首版=按需拉取+每笔写回执到达后回读+既有工程刷新链覆盖；**实施时核定 delta_update 是否携带 plugin state 变更**（现消费面只见 track/clip：project_store.gd:187-212；核内核 state.delta 生产者）——结论写回执，不定案不扩面。

## 文件域

新文件 `app/kernel/clients/automation_client.gd` + 探针/测试新文件（不得插 stack_probe_sendroute_macro.gd）。越域即停。

## 验收标准

- 独立探针（新文件）断言：三方法 payload 形态/回执解析/错误分类/工作副本纯函数区域替换语义；
- 真栈腿（§2.2 登记）：get_curve 空曲线→set_points 三点→回读逐点一致→clear_region→回读空（复用 M3-KERNEL-1 冒测 ps1 模式扩 GUI client 腿）exit 0；
- 既有探针零回退；工件入主仓 coord/runs/M3-ENVELOPE-DATACHAN-1/；端测边界声明。

## 停止条件

回执形态与 186 勘察锚点不符（内核已重构）→ 锚点上交再裁。

- 领取：（时间 / origin hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / 探针断言面 / 真栈 run ID / delta_update 核定结论 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
