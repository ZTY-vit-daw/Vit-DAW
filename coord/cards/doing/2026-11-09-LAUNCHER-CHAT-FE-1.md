# LAUNCHER-CHAT-FE-1：最简对话窗+启动台挂载+磁带卡（332 拆卡 C1，Godot 域）

- 发卡：GLM 主管决策流 / 2026-11-09（依据=332 设计稿 §7 C1+v2 终选裁定；用户确认开工）
- 派发确认：已确认（用户 2026-11-09「可以按这个方案重做 UI 和交互」）
- 验收负责人：GLM 主管决策流+用户 GUI 终审
- 池序 335；P1 / 2-2.5 天 / 依赖=零（与 337 并行，契约 JSON 先冻结）；L2 / PC（Godot 前端仓）；零栈单测（headless 探针）+真栈腿并入 339

## 目标

1. 新组件 `app/startup/launcher_chat_panel.gd+.tscn`（消息流/输入框/agent 事件卡/流徽章/未就绪禁用态/降级引导卡）——视觉规格权威=`D:/Vit_DAW/agent/webui/design/launcher-route-mondrian-v2.html`（令牌表+磁带卡形态+开场白布局照此实现，Godot 控件化）；
2. start_page 挂载：视觉主位+agent 就绪态接线（就绪前输入禁用+「正在唤醒搭档…」状态行）+磁带流收为背景层/一键展开抽屉（把手放磁带 icon——TapeIconArt 既有类直接复用，规格已在案）；
3. 工程结果卡=磁带卡：复用 tape_atmosphere.gd 纯函数（气氛渐变+带轮量）+TapeIconArt——既有零件零新造；
4. 降级态：agent 探测超时→引导卡形态（「搭档暂时不在。可以直接选工程…」+重试钮）。

## 文件域

前端仓 `app/startup/`（新文件）+`start_page.gd`（挂载+就绪接线 ~30 行插点）；**红线零触碰** LLM_Chat_Controller.gd/LLM_Chat_Window.tscn。

## 验收标准

headless 探针断言（窗节点在场/就绪前禁用后解禁/磁带卡数据绑定）+渲染面边界显式声明（AGENTS §5）+真栈腿并入 339。

## 领取/回执/验收

- **领取**（2026-11-09）：origin/main=987455063a48f54193621f7060b4e1d8927833b8；owner=GLM-5.3 L2 执行子代理（335exec，PC）；实现分支=前端仓 `port/launcher-chat-fe-1` @基线 f805bde0d69365b27a5747292ead36e34d04d000（领取基线亲证：前端仓 main 干净，仅未跟踪运行时目录 `.vit_agent/` 等既有项，与本卡文件域无涉）；worktree=`D:/Godot/project/vit-daw-frontend-lchat1`；领取提交=本提交（仅 coord/ 变更）。doing 三卡（273/275 族 agent 域+WAVESHELL 取证）文件域零冲突已核对。
- 回执：（执行侧回填）
- 验收：（主管回填）
