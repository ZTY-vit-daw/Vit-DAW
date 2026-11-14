# UI-BRIDGE-1 §8 跑前口径（冻结于任何真栈运行之前；AGENTS §8）

- 卡：coord/cards/doing/2026-11-14-UI-BRIDGE-1.md（池序 348）
- 执行：freeflash 免费车道子代理 / PC / worktree D:/Vit_DAW_wt_uibridge1（主仓）+ D:/Godot/vit-daw-frontend_wt_uibridge1（前端仓）/ 分支 port/ui-bridge-1
- 被测：主仓 worktree 实现（webui 桥客户端）+ 前端 worktree 实现（start_page/dock/nav_bridge，extension 零改动则 dll 不重编）；内核 D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe 零内核改动只读复用；agent 现场构建隔离二进制（go build 到 run 目录）
- 栈协议：coord/resources/PC-RUNTIME-STACK.md 两条登记（占用+释放），登记推送后 ≤60s 复核（PROTOCOL §2.2 2026-10-25 增补）；「already listening」即停

## 腿结构（一次运行 = 内核/agent 单次启动 + 4 次 Godot boot）

| 腿 | boot | 前置 kernel 态 | React 桥消息 | 治理钩子 | 期待 |
|---|---|---|---|---|---|
| L1 零摩擦首开 | boot1 | 无工程 | 打开烟测桥甲 | （无对话框） | 甲 open→VitDockRoot，result=opened，governance=none |
| L2a 三键·直接切换 | boot2 | 甲在开 | 打开烟测桥乙 | switch | 治理对话框出现，乙 open→VitDockRoot，result=opened，governance=switch |
| L2b 三键·取消 | boot3 | 乙在开 | 打开烟测桥甲 | cancel | 无场景切换，/agent/state 仍乙，result=canceled，governance=cancel |
| L2c 三键·保存并切换+降级 | boot4 | 乙在开 | 打开烟测桥甲 ‖（+8s）打开烟测桥乙 | save | save 乙真实落盘→甲 open→VitDockRoot；followup 在 dock 阶段→unsupported 降级回执 |

## 成功条件（全部满足=exit 0）

- 每腿 [LauncherBridge] 日志链完整：surface_ready → directive.received → governance.*（L2 各腿 dialog_shown+choice）→ open.begin → result.posted
- L1/L2a/L2c：scene_added VitDockRoot + boot 内同一 child_hwnd（343 不变量，smoke navigate 显式记账不计入 navigate_count 断言）+ result_ack(opened)（React 真收回执）
- L2b：无 scene_added + /agent/state current_project.path==乙 + result_ack(canceled)
- L2c 降级段：dock 侧 unsupported 回执 + result_ack(unsupported) + [LauncherBridge] unsupported 日志
- fixture 源（912.vit）sha256 运行前后一致；recents global_project_config.json 还原快照（净差零）；零 LLM telemetry
- 脚本退出码 0

## 运行次数与止损

- 每腿 1 次，全 run 1 次运行（4 boots 连续）；无预演。
- 失败分类（分开记录不合并）：exit 2=环境中断（内核起不来/端口被占/Godot 崩/WebView 未起）；exit 3=桥链路确定性断言失败（directive/governance/open/result/降级任一步）——取证上交不重跑；exit 4=烟测脚本装置缺陷（脚本自身解析/驱动问题）——修复后整 run 重跑 1 次记 attempt。
- 同一确定性断点两连败（同代码版本同断言）→ 停手上交（AGENTS §8）。
- LLM 预算：0 调用（launcher greeting/list/directive 全零 LLM 确定性路由）；telemetry 行数对账非零即 budget_exceeded 停。

## 隔离与防污染（§10）

- fixture：912.vit 复制两份到 run 目录（烟测桥甲.vit/烟测桥乙.vit，不同文件名同内容——340 先例）；源哈希前后核对。
- recents：%APPDATA%\Vit-DAW\global_project_config.json snapshot→prepend 两 fixture→全部腿结束+拆栈后还原快照（BOM-less UTF-8 手工数组构造，dev_agent_smoke 同款）。
- agent 草稿根 VIT_HISTORY_DRAFT_ROOT 隔离到 run 目录；VIT_PROJECT_XML 指向 run 目录 kernel_workspace；真实用户工程/sealed fixture 零触碰。
- React 烟测钩子经 URL 参数门控（launcher_smoke=1），生产 URL 无参零行为；Godot 烟测钩子经 VIT_SMOKE_LAUNCHER_* 环境变量门控，生产零行为。

## 工件

- coord/runs/UI-BRIDGE-1/run_<stamp>/：logs（godot_boot{1..4}.log/agent_last.log/kernel log 副本）、artifacts（ui_bridge_smoke_report.json+recents before/seeded/after）、console 全文。
- 随主仓 port/ui-bridge-1 入库；卡面回执记录 run ID/退出码/HEAD。
