# FE-SEND-LEVEL-FADER-1：发送量推子直调（用户裁定形态——滑动推子替代点击弹窗）

- 发卡：GLM 主管决策流 / 2026-10-24（依据=用户 182 复验反馈：弹窗输入已生效且听感确认，但期望形态=可直接拖动的推子控件）
- 派发确认：已确认（用户现场表达）
- 验收负责人：GLM 主管决策流
- 池序 183；目标仓库=**前端仓 D:\Godot/project/vit-daw-frontend**；P1（主旅程可感面·用户裁定形态） / 0.5 天 / 依赖=182 已合入（ee8fa0e）；模型分级：L1/L2 / PC 前端执行流

## 现状与目标

- 现状（182 后）：SendOut 对卡「发送量」口行平钮→点击弹 PanelContainer 叠层（SpinBox 步进/键入）→实时 track.send.set_amount。功能闭环，用户复验通过。
- 目标（用户裁定）：**卡上直接可拖的推子**（slider/fader）调节发送量——拖动即时回显、无需先点开弹窗。

## 范围

①交互实现：SendOut 对卡发送量行加横向推子（HSlider 或仓内既有 slider 先例形态，勘察 K3 控件族十词表 slider 件与 167 混音台邻域后择型）；范围 -96..+6 dB；映射设计注意宽跨度下常用区（约 -30..0）可拖精度（线性步进 vs taper 由勘察定，拖动粒度≥0.5dB 对齐 SpinBox 步进）；口行平钮弹窗保留为精调键入面（双入口并存，弹窗关闭语义沿用 182）。②发包节流：拖动中间帧本地回显、释放/防抖才落 track.send.set_amount——**复用仓内控件值提交节流先例**（Vit_Graph_Rack.gd:335 邻域「drag 中间帧本地回显，释放才落」机制），不得每帧直发 payload。③混音台 SEND 槽位（167）是否同升推子=勘察项写结论（改/不改给理由，改则一并实现，用户裁定面）。④探针断言：真实事件管线（push_input 拖动序列 motion→press→motion…→release）→payload 节奏断言（拖动中零发包/释放后恰一笔+值正确）+repository 读回收敛；禁止 emit 直调（182 通则）。⑤窗口态手感=用户手测复验。

## 文件域

以勘察固定（预期 `app/rack/Vit_Graph_Rack.gd` 发送量行+节流复用件+探针件；若 167 同升则含混音台邻域）。越域即停。

## 验收标准

真实拖动管线断言（拖中零发包/释放落值/读回收敛）+既有探针零回退（K3 313/popup 111/realinput 15/shrink abc/start_feed 244）+真栈腿（SendRouteGuiSmoke 模式，§2.2 登记）；工件入主仓 coord/runs/FE-SEND-LEVEL-FADER-1/；用户手测手感复验。

## 停止条件

推子与对卡尺寸/GraphEdit 拖拽手势冲突不可调和（卡内拖动被画布手势抢）→ 锚点上交再裁定。

- 领取：（2026-10-24 18:09 / 主仓 origin/main=4c7736c9 / 前端仓 main=ee8fa0e / owner=ZCode GLM-5.3 PC 前端执行会话（FE-SEND-LEVEL-FADER-1 专属） / 分支 port/fe-send-level-fader-1 / worktree D:/Godot/project/vit-daw-frontend-lvlfader1 / 领取提交=本提交）
- 回执：（择型结论 / 167 勘察结论 / commit / 探针断言面 / 真栈 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
