# WEBUI-MSG-ORDER-2：活态路径钉尾残留——A/B 判定卡/park 消息族钉流底，新输入与报错插其上（M1 复验实测，P1）

- 池序 23（P1 关键呈现缺陷）；目标仓库=D:\Vit_DAW（PC 执行侧，webui 域）；来源=M1 复验第三轮（[runs/M1-RETEST-20261001/FORENSIC-NOTE.md](../../runs/M1-RETEST-20261001/FORENSIC-NOTE.md) 症状 4+evidence/）
- 优先级 / 预估 / 依赖：P1 / 取证 0.5 天+修复 0.5 天 / WEBUI-MSG-ORDER-1 已合入（0e7ba86d/8e21258e——本卡是其声明的活态覆盖缺口的补面，非回退）
- 模型分级：L1 / GLM 首选（渲染序机制取证+活态断言面设计）；flash 可接若取证路径清晰
- 已核实事实：
  1. 用户目视形态（2026-10-01 18:3x，修复后二进制）：**第二句用户输入与 turn.failed 报错都显示在第一轮输出的上面**（第一轮链内容——轨迹块/A-B 判定卡/park 系列消息——钉在流底）。
  2. **服务端序正确**：会话图节点线性（ask→回执→ask→错误，evidence/conversation-graph.json 逐节点时间戳核对）——错位纯在 webui 活态渲染层。
  3. WEBUI-MSG-ORDER-1 修的两个机制（renderPlan 链终局钉尾=isChainResultChatMessage 族+turnGroups 同 run 用户行回吸）与本症状**不同族**：本轮第一轮内容里钉底的是 A/B 判定卡/park 交互消息（audition/pending-interaction 投递通道），不在 isChainResultChatMessage 谓词覆盖内（该卡回执已声明「活态 composer 驱动时序未真栈断言」——本卡即被打中的缺口）。
- 目标：
  1. **取证**：钉出 A/B 判定卡/park 消息族在活态路径的渲染与排序通道（哪个组件/哪条 pinning 逻辑把它们钉在流底；与 renderPlan/turnGroups 的关系——是独立 pinned lane 还是消息族谓词漏网）；领取时实锚（预期 App.tsx 消息流组装+audition/interaction 卡渲染段）。
  2. **修复**：该消息族按 createdAt 归位时序槽位（与既有两机制同语义：新消息在其后即插其上/旧内容回流入序）；不破坏 A/B 卡的交互可用性（点击/试听/judgment POST 不受排序影响）与 FIX-CONFIRM 的 settleTerminatedTurnInteractions。
  3. **活态断言补面**：E2E-WEBUI-1 加活态形态组（模拟 composer 驱动的新输入接在 park/判定卡之后——非纯事件重放；参照既有 driven-turn 组模式）——这是 WEBUI-MSG-ORDER-1 遗留的「活态无真栈断言」缺口的正式补面。
  4. **回归**：水合/重放组零回退（上卡 23 组含 msg-order-M1 必须仍绿）+新活态组红绿。
- 文件域：agent/webui/src/（消息流组装/park 判定卡渲染段+测试）+ scripts/webui_rendered_dom_smoke.mjs（加组）；如取证指向服务端事件缺回合域（audition 族无 turn 归属），按停止条件上交不越域。
- 约束：E2E 走隔离泊位先例（与并行真栈卡不冲突）；npm run test 全绿+build+E2E exit 0。
- 验收标准：新活态组红绿+既有 23 组零回退+用户手测复验（顺序正常）。
- 停止条件：取证发现钉底源于服务端事件缺 turn_id/回合域（webui 无法独活排序）→ 实证锚点上交，转 agent 侧卡。
- 领取：（时间 / origin/main hash / 分支名）
- 回执：（commit hash / 机制锚点 / 红绿 / E2E run ID）
- 验收：（裁定文件 / 验收 commit）
