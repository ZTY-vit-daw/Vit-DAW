# TIMEFX-BUS-WRITEGATE-1：时域总线写门校验+send.connect 执行接线

- 发卡：GLM 主管决策流 / 2026-10-20 深夜（依据=146 实验判读拆卡 C；写门厚度=全量——成功线未达，写门承担拓扑正确性主责）
- 派发确认：已确认（146 卡实验腿验收裁定）
- 验收负责人：GLM 主管决策流
- 池序 155；目标仓库=D:\Vit_DAW；P1 / 0.5-1 天 / **依赖=154 合入后领取（消费其 topology plan 对象）**
- 模型分级：L2 / GLM 执行会话
- **规格全文=[146 设计稿 §二.1 写门+§三 卡C](../../reports/2026-10-20-146-timefx-design-and-experiment.md)（权威）**

## 范围

确认/执行时拓扑校验：提案引用轨必须存在于观察披露或本提案新建清单；send 目标必须存在（幻觉拒载）；`track.send.connect` 执行接线（K1/K2 命令面，零内核改动）；154 烟测场景补「工程已有总线」复用面（fixture 无既有 FX 轨——设计稿诚实边界第 3 条遗留场景）。

## 文件域

`agent/internal/chat/send_route_workflow.go` 邻域+执行面（vsphub 派发处）+测试。越域即停；K2 既有显式 send 拦截面语义零改变（只增默认惯例执行路径）。

## 验收标准

真栈烟测 exit 0 四断言：建轨/连接/读回/幻觉拒载；`go build ./...`+chat 包+全量 0 FAIL+gofmt 净；端测边界声明。

## 停止条件

执行接线与 vsphub 派发面既有 CAS 语义冲突 → 锚点上交。

- 领取：（2026-10-21 晚 +0800 / origin/main=5b6bd8caa185（=171640d5+156 领取提交，代码基线同卡面 171640d5）/ GLM-5.3 L2 PC 执行会话 ZCode（本会话，用户口令指定领取） / 分支=port/timefx-bus-writegate-1（worktree D:/Vit_DAW_wt_writegate155）+ 协调 worktree D:/Vit_DAW_wt_writegate155_coord（coord/writegate155-claim）/ 领取提交=本提交；领取核对：卡在 todo 派发已确认、154 已合入（93df0fcd，拓扑计划对象在位）、doing 两卡（GEN-SUNO 纯设计件 / 156 取证只读零生产代码）文件域零冲突；真栈腿时窗=实现+单测全绿后（预计本日晚窗），领取时不占栈，启栈前按 §2.2 只读核对进程/端口再登记——另发现栈记录 93df0fcd 把 154/151 两条释放记录覆盖回 151 陈旧占用态（历史 bb4b4db2/318805ec 均已释放），登记占用时从历史恢复并注明；153 腿未领取无在飞冲突，错峰义务以启栈前记录核对为准）
- 回执：（commit / 真栈 run ID 四断言面 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
