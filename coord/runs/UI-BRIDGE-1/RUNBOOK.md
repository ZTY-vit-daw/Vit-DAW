# UI-BRIDGE-1 §8 跑前口径（冻结于任何真栈运行之前；AGENTS §8）

- 卡：coord/cards/doing/2026-11-14-UI-BRIDGE-1.md（池序 348）
- 执行：freeflash 免费车道子代理 / PC / worktree D:/Vit_DAW_wt_uibridge1（主仓）+ D:/Godot/vit-daw-frontend_wt_uibridge1（前端仓）/ 分支 port/ui-bridge-1
- 被测：主仓 worktree 实现（webui 桥客户端）+ 前端 worktree 实现（start_page/dock/nav_bridge，extension 零改动则 dll 不重编）；内核 D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe 零内核改动只读复用；agent 现场构建隔离二进制（go build 到 run 目录）
- 栈协议：coord/resources/PC-RUNTIME-STACK.md 两条登记（占用+释放），登记推送后 ≤60s 复核（PROTOCOL §2.2 2026-10-25 增补）；「already listening」即停

## 腿结构（一次运行 = 内核单次启动 + 4 组「agent 重启 + Godot boot」）

> 设计更正（M5 期发现）：launcher 门副门 user_entered_project 是 **agent 进程级不重置**
> （launcher_flow.go 副门语义，340 落地）——共享 agent 会让 boot2-4 门关闭、directive 不再
> 路由。故每 boot 前重启 agent（340 `-RestartAgent` 先例），内核不重启（在开工程跨 agent
> 存活；L2 治理前提=新 agent shadow 从内核态同步 project_path，337 双面同源契约）。

| 腿 | boot | 前置 kernel 态 | React 桥消息 | 治理钩子 | 期待 |
|---|---|---|---|---|---|
| L1 零摩擦首开 | boot1 | 无工程（boot XML scratch，path 空） | 打开烟测桥甲 | （无对话框） | 甲 open→VitDockRoot，result=opened，governance=none |
| L2a 三键·直接切换 | boot2 | 甲在开 | 打开烟测桥乙 | switch | 治理对话框出现，乙 open→VitDockRoot，result=opened，governance=switch |
| L2b 三键·取消 | boot3 | 乙在开 | 打开烟测桥甲 | cancel | 无场景切换，/agent/state 仍乙，result=canceled，governance=cancel |
| L2c 三键·保存并切换+降级 | boot4 | 乙在开 | 打开烟测桥甲 ‖（+8s）打开烟测桥乙 | save | save 乙真实落盘→甲 open→VitDockRoot；followup 在 dock 阶段→unsupported 降级回执 |

L1 零摩擦前提的依据：boot 态 /agent/state current_project.path 依赖 ProjectPathHint
（339 取证「path=草稿回退」=hint 面）——L1 全新 agent 无 hint → path 空 → 治理直行；
L2 腿 kernel 有实开工程 → shadow 携带路径 → 对话框出现。若 L1 意外出对话框=前提被
真栈证据推翻 → 按 §8 失败分类 3 停手上交（不原样重跑）。

## 成功条件（全部满足=exit 0；attempt-4 后校准——§8 修复轮记录见 PROGRESS）

- 每腿 [LauncherBridge] 日志链完整：surface_ready → directive.received → governance.*（各腿 dialog_shown+choice）→ open.begin → result.posted（行面含 governance/action）
- L1（boot scratch 态，治理 switch 驱动——零摩擦前提被 attempt-1 真栈证据推翻转 finding）：dialog_shown(current_project=Unsaved scratch) → switch → open → scene_added VitDockRoot + 同 HWND + result(opened,governance=switch) + result_ack(opened)；/agent/state 仅记录（工作区绑定语义=340 桥播域）
- L2a：dialog_shown 且 **current_project 含烟测桥甲**（内核在开工程证明）→ switch → result(opened,switch) + scene_added + 同 HWND
- L2b：dialog_shown → cancel → result(canceled,cancel) + result_ack(canceled) + 无 scene_added + /agent/state==乙（激活先于指令，绑定有效）
- L2c：dialog_shown → save → result(opened,save) + scene_added + 同 HWND；followup 在 dock → dock.received + result(unsupported) + result_ack(unsupported)；/agent/state 仅记录
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
