# FE-FX-WETCHAIN-BADGE-1：FX 轨卡内部湿链徽标化呈现（B 方案）

- 发卡：GLM 主管决策流 / 2026-10-24 晚窗（依据=[decisions/2026-10-24-fx-badge-and-vittape-rename.md](../../decisions/2026-10-24-fx-badge-and-vittape-rename.md) §一——用户 B 方案裁定，关闭 176 勘察上交挂账）
- 派发确认：已确认（用户现场裁定：传统 DAW 无 A 形态/画布连线臃肿/FX 效果器不该显示在发送画布）
- 验收负责人：GLM 主管决策流
- 池序 185；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；P3（体验补全，非差口）/ 0.25-0.5 天 / 依赖=176 已合入（787150f）；模型分级：L1 / PC 前端执行会话
- 并行：与 180（前端 tools/ 探针头）文件域不相交可并行；真栈腿按 §2.2 错峰。

## 目标

FX 轨卡（SendIn/总线对卡的 FX 侧）上以**徽标/摘要**呈现内部湿处理链：效果器名（或类型 icon+数量），随 rack 变更经既有拓扑回执同步面刷新。**红线：零画布连线、零锚点语义改动、零 `can_connect_rack_topology` 门触碰**（B 方案的定义性边界）。

## 范围

①数据派生：徽标内容从同源 pluginList+rack 成员序派生（176 勘察锚点：内核 `normaliseFxChain` 反环门 2 规整形态=receiver(AuxReturn)@头+rack 成员+wetSend(AuxSend)@rack 后，SendRouteService.cpp:365-408——前端只消费既有 repository 拓扑回执，不加新请求面）；②呈现：FX 轨卡头/尾部徽标行（仓内既有徽标先例形态勘察择型——如 166 的路由徽标/调色板邻域），空 rack=空徽标或「（空）」，侧链/悬空形态 fail-closed；③刷新：随 `_apply_send_route_node_title` 既有回执同步面刷新（183 同位），拖动/弹窗操作不互抢；④探针：独立探针腿断言（徽标在场且内容与拓扑一致/rack 装载与卸载后刷新/普通轨无徽标/空 FX 轨形态）+K3 等既有探针零回退；⑤真栈腿（SendRouteGuiSmoke 模式，§2.2 登记）徽标断言入栈探针 L 腿（装载→徽标出现→卸载→徽标消失）。

## 文件域

以勘察固定（预期 `app/rack/Vit_Graph_Rack.gd` 对卡呈现邻域+`rack_send_route_rules.gd` 派生纯函数+探针件+stack_probe L 腿）。越域即停。

## 验收标准

独立探针全绿+既有探针零回退（K3/popup/realinput/shrink abc/start_feed/fader 35）+真栈腿 exit 0（徽标装载/卸载断言）；工件入主仓 coord/runs/FE-FX-WETCHAIN-BADGE-1/。

## 停止条件

pluginList 回执面不含 FX 轨 rack 成员（数据源与勘察假设不符）→ 锚点上交再裁。

- 领取：（时间 / origin hash / owner / 分支 / worktree / 领取提交）
- 回执：（择型结论 / commit / 探针断言面 / 真栈 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
