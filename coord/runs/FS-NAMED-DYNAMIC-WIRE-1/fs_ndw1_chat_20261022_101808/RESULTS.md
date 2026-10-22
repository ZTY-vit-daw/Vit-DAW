# FS-NAMED-DYNAMIC-WIRE-1 真栈 chat 腿收口（run=fs_ndw1_chat_20261022_101808）

- 日期：2026-10-22 10:18–10:2x +0800；owner=GLM-5.3 L2 PC ZCode 执行会话（162 卡会话）
- 被测：agent=worktree D:/Vit_DAW_wt_ndw1 构建（port/fs-named-dynamic-wire-1@impl a6702c56，agent/bin/VitAgent.exe SHA256 前 16=B37075B3CACEE17F，路径亲核）；内核=主检出 K3 build-feguik3 Release 722A79D7… 只读复用（10:2x 哈希亲核一致，本卡零内核改动）；不启 UI
- 占用登记：bee74f6f（10:3x 前置预检七端口 tcp+udp=0、四类进程=0、Godot=0）；拆栈 10:2x 复核同口径全 0（只杀本腿 pid agent 1364/kernel 31788）

## 驱动与判定

- `scripts/fs_named_dynamic_wire_chat.py`（本卡新增，fs_ledger_carry_chat 同族）：M2 六 stem fixture（verbatim 配方）+ full_project_access（/smoke authority 探针 exit 形态 ok）+ 场次③话术逐字复刻「给人声加点空间感」单轮（1 chat+0 nudge+0 hop，budget 内）
- 脚本 exit 0（classification=pass）：F1 `track.send.list` send 边 source_track_id=**1032（vocals）**→ fx_track_id=1037 @-6.0 dB；F2 改写-披露耦合=空集无违反（见判读）

## 旅程判读（判别性归属）

- agent 日志在卷：10:18:21 `[free-state.seed] booked revision=6 scan_satisfied=true`（158 生效）+ `user_named_target_resolution` shadow 刷新 ×3（157 生效）；10:19:04 `track.send.connect`+`rack_add_node` 双派发
- chat_1.json 判别性证据：`free_state_reasoning_loop.user_named_target="1032"` 且 **`loop.target_ref=null`**（本轮模型未绑任何轨，needs_observation 形态，链经调度续跑直达 completed）→ `request_context.selected_track_id="1032"`＝**指名指针供目标**分支的真实栈实证——main 上同形态 selected_track_id 为空、bus 拓扑计划无法成形（attachSemanticTopologyPlan 无源 no-op），本腿 connect source=1032 只能由本卡改写产生
- 「模型另选→确定性改写+披露」面本轮未出现（无 rewrite marker/披露句，属模型随机分支缺席非缺陷）：该面由单测四面钉死（TestFSNamedDynamicWireBindRewritesPlanTarget：模型绑 9006+指名 9001→改写+标记+陈旧插件绑定清除；TestFSNamedDynamicWireDisclosureOnResponseFace：披露句+双包装去重；TestFSNamedDynamicWireBindFailsOpenWithoutPointer：fail-open 现状；TestFSNamedDynamicWireWriteGateAcceptsNamedSource：155 写门联动零冲突）
- 端测边界声明：本腿覆盖 agent HTTP 面（chat/invoke/events/send.list 读回）+内核 ZMQ 真实派发；**不覆盖** webui 渲染面与用户旅程（无 UI 启动，E2E-WEBUI-1/JOURNEY-1 层级缺位同既往）；模型另选改写面的真栈形态未在本轮观测（见上）

## 工件

- 本目录：report.json（判定面）/ round1/（chat_1+events+runtime_status_final+prompt）/ send_list_final.json / authority_{switch,probe}.json；重件 fixture/（5.3MB stems+.vit）留盘不入库
- agent 原始日志：worktree `VitApp/Workspace/Logs/agent_last.log`（运行时段 10:18–10:19 可回指）
