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

- 领取：2026-10-25 晚窗 / 主仓 origin/main=c6cef2da / 前端仓 main=ee03262 / owner=ZCode GLM-5.3 PC 执行会话（M3-ENVELOPE-DATACHAN-1 专属） / 前端分支 port/m3-envelope-datachan-1 / worktree D:/Godot/project/vit-daw-frontend-m3dc1 / 领取提交=本提交
- 回执：2026-10-25 晚窗自验完成，交主管验收（详 [runs/M3-ENVELOPE-DATACHAN-1/RESULTS.md](../../runs/M3-ENVELOPE-DATACHAN-1/RESULTS.md)）——
  - **commit**：前端仓 `port/m3-envelope-datachan-1@75bdef38`（automation_client.gd 新文件+两探针）`+f461438`（真栈探针预初始化修复，首轮缺陷）；主仓 `port/m3-envelope-datachan-1-stack@3298d70d`（dev_agent_smoke.ps1 新参数 -AutomationDatachanGuiSmoke=SendRouteGuiSmoke 同模式+run 工件+RESULTS.md）；均已推远端。
  - **实现面**：三方法（rack_client track_send_* 同款 no-wait+signal_bus，rack_client.gd 零改 git 亲证）+payload 构造纯函数+回执解析（clear_region 回执实际无 mode/written/time_base——内核 :2247-2257 亲核，186 锚点一致停止条件未触发，解析器不虚构字段）+错误分类（三主类+四伴生类，内核文案逐字透传）+工作副本纯函数（apply_region_edit **[start,end) 半开镜像引擎 removePoints**+区域内新点 [start,end] 闭接收镜像内核拒收规则——两端语义差如实镜像；apply_full_replace=完整点集发包=裁定①案 A 零 CAS；单点 add/move 不造）。
  - **探针断言面**：独立探针 `probe_automation_datachan.gd`（headless 假传输零栈）**50/50 exit 0**（A 传输面 15/B 回执解析 9/C 错误分类 14 内核串逐字/D 纯函数 12 含 C4 组合链）；既有探针零回退 **7/7**（K3 313/popup 111/realinput 15/fader 35/shrink a+b/start_feed 244；shrink_c=179 域已知不稳定未跑）。
  - **真栈 run**：`datachan_gui_20261025_194233` **脚本 exit 0 stack probe 16/16 PASS**（L1 空曲线披露→L2 三点 mode=full 逐点回读→L3 **区域替换纯函数预测 vs 活内核逐点对账**→L4 全域清除回读空→L5 错误透传活栈；180 横幅双源在场；自启 kernel 9044=主检出 K3 722A79D7 只读+VIT_PROJECT_XML 隔离副本被写=隔离生效亲证）；第一轮 `193812` exit 1 留证=探针时序缺陷（已修 f461438）+**撞用户在飞栈事故**（RESULTS.md §五：占用检查-启动空窗内用户栈起来被 ps1"Start or reuse"静默复用，探针 clear_project 落在用户活内存工程；**盘上零损失**=repo default_project.xml 内容完整 stems 且末次写先于 clear；用户栈非本 owner 手拆；根因+候选整改上交待裁）。§2.2 登记 54765bac/释放 149f6e12（事故+两轮全披露在卷）。
  - **delta_update 核定结论（卡面⑤）**：**不携带 plugin state/曲线变更**——ops 仅 project/track core/clip 三类路径（VspKernelReference.cpp:924-980）；track 行 plugin_param_digest 只折叠参数**当前值**（CommandDispatcher.cpp:3662-3706）不含曲线、rack 只含结构图（:571-601）；即便偶发 digest 变化→replace op 载荷也无曲线数据。裁定①「按需拉取+写后回读」形态成立；接事件通知须内核扩面（另开卡，本卡未扩）。
  - **端测边界声明**：①独立探针=注入假传输零栈（非 180 警告域）；②真栈腿=生产 GUI 传输链全链真实往返（UDP 4445→agent→内核），未覆盖=C2 呈现面（本卡零 UI 改动）/渲染面与用户旅程（不适用）/beats 活栈反例（内核串逐字断言+源码锚点在卷，造 beats 曲线超本卡域）；③L3 对账腿证纯函数↔活内核区域替换逐点一致。
- 验收：（裁定文件 / 验收 commit）
