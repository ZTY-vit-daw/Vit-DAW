# FIX-GOALSTATUS-FORGERY-SWEEP-1：goal_status 伪造面全树普查+同型修（327/330 缺陷族收尾）

- 发卡：GLM 主管决策流 / 2026-11-09（依据=327+330 两卡域外登记累积：interaction 面 catch-all[327 已修]/plan 面[330 已修]/chat 投影空回填[330 已修]/interaction 面三个 workflow 专属恢复兜底 server.go:4834/4885/4956 仍伪造——同族第四批）
- 派发确认：已确认（主管发卡；P3 不占展演前关键路径，随手清型）
- 验收负责人：GLM 主管决策流
- 池序 334；P3 / 0.5-1 天 / 依赖=327/330 已合入（语义先例与台账机制均在 main）；L1 / PC；零栈单测为主

## 目标

1. **全树普查**：grep 锚定全部「罐头文案+goal_status 回填 completed/failed」形态（关键词：`已处理或已过期`/`GoalStatus` 空回填/`completed` 兜底赋值），产出普查表（位置+触发路径+是否伪造）；
2. **同型修**：确认伪造的站点对齐 327/330 两态回执语义（retired/not_found，goal_status 空串，WARN 留痕）；复用既有 retirement 台账或按面建最小对应物；
3. **单测**：每修站点镜像三态形态；红转绿记录；
4. **负结论如实**：普查后判定某站点回填是「合法语义」而非伪造（如真实 completed 路径）→ 登记不修，说明理由。

## 文件域

agent/internal/chat/（以普查锚点为准）+测试；工件 coord/runs/FIX-GOALSTATUS-FORGERY-SWEEP-1/。

## 验收标准

普查表+（如修）单测红转绿+chat 全包不回退+回执附 diff。

## 停止条件

某站点语义牵涉 workflow 状态机重构 → 登记候裁定不硬改。

## 领取/回执/验收

### 领取（334）

- 时间：2026-11-20
- main 基线：`948d7b1c07734e89af93d857687bbaad1939ae37`
- owner：GLM-5.3 flash 执行子代理 forge334 / PC
- 分支：`port/goalstatus-forgery-sweep-1`；worktree：`D:/Vit_DAW_wt_pc334forge`
- 领取时工作树状态：主树存在他卡/人工既有改动（CURRENT-STATE.md、VitApp/Workspace/*.xml、tracktion_engine、YUE 卡删除等）——记录不触碰，与本卡文件域（agent/internal/chat + coord 工件）不相交
- 前置核对：卡仍在 todo ✓ 派发已确认 ✓ 依赖 327/330 已合入 main ✓；并行卡 440（genapi）/441（webui）文件域不相交 ✓
- 普查先行结论（survey.md 已落盘工件目录）：真伪造 4 处（三个 workflow 恢复兜底 4961/5012/5083 + finalize 空回填与 CompleteGoal default 组成的 interaction 面第二伪造面 F4）；合法/存疑/域外分类见 survey.md
- 零栈单测卡：不起真实栈、不占 PC-RUNTIME-STACK
