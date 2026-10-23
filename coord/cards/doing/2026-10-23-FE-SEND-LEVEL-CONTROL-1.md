# FE-SEND-LEVEL-CONTROL-1：send 发送量参数口不可交互（场次⑤手测：紫色小点无法点击）

- 发卡：GLM 主管决策流 / 2026-10-23 凌晨（依据=场次⑤用户手测：send 对卡展开后「一边的紫色小点」=send_level 参数口渲染在场但**无法点击交互**——171 终验断言到 DOM 存在未断言可交互=覆盖缺口如实记录）
- 派发确认：已确认（用户手测现场反馈）
- 验收负责人：GLM 主管决策流
- 池序 178；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；P1（主旅程可感面：发送量调节是 send 路由的核心操作） / 0.25-0.5 天 / 依赖=无（与 176 文件域同族 rack 邻域——**串行约定：与 176 协调领取**，若同 worktree 顺序执行注明先后）；模型分级：L1/L2 / PC 前端执行流

## 现象

展开 send 合一 icon 后对卡上的 send_level 参数口（紫色小点=ParamPortRow/参数口形态）渲染在场但点击无响应——用户无法调节发送量。混音台 SEND 槽位（167 面）调量是否可用一并核对（K3 实现=行构建+调量 payload，若同样不可交互同修）。

## 范围

①取证：参数口控件的交互语义——K3 控件族（十词表 knob/slider）可调 vs 参数口=挂宏的口不可直调的既有设计边界；用户预期=**可直接调节发送量**（点击弹值/拖拽调值，对齐普通控件交互）；②实现可交互（或按取证结论给最小可感路径：点击弹参数面板/直接拖拽，给用户可用的调节面）；③探针补交互断言（模拟点击/值变更 payload 发出——补 171 断言只到 DOM 的缺口）；④窗口态手感=用户手测目检项。

## 文件域

以勘察固定（预期 `app/rack/` 参数口/控件族邻域+探针件）。越域即停。

## 验收标准

探针交互断言+真栈腿（调节 send_level→track.send.list 读回值变化）+既有探针零回退；端测边界声明；工件入主仓 coord/runs/FE-SEND-LEVEL-CONTROL-1/。

## 停止条件

参数口直调与 K3 控件语义模型冲突（需 UX 重设计）→ 锚点上交用户裁定。

- 领取：2026-10-23 10:2x 本会话机器钟 / origin/main=d0fb88d8（领取前 fetch 亲核=当前 main HEAD） / owner=GLM-5.3 L2 PC 前端执行会话（ZCode，176/178 卡会话，机器=PC） / 分支=port/fe-send-level-control-1（基=176 领取与完成态 7c82260——串行约定：与 176 同 worktree 顺序执行，176 先 178 后，K3 探针文件同域叠加） / worktree=D:/Godot/project/vit-daw-frontend-sendchain1（同一 worktree；协调分支 coord/fe-send-chain-position-1-claim@D:/Vit_DAW_wt_sendchain1_coord 续用） / 领取提交=本 commit
- 回执：（交互语义结论 / commit / 探针断言面 / 真栈 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
