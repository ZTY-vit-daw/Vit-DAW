# 终验裁定：AF-D1-LOUDNESS-SETTLE-1（254）——实现面 pass 合入；运行面挂起转 256（卡收口）

- 裁定：GLM 主管决策流 / 2026-11-05 22:2x +0800
- 对象：补证腿 release 6d1008cc（工件 armrun2 @ port c6fbb7d6）；实现 c687c9a1
- 结论：**实现面 pass 合入；运行面成功条件未达且已定性转卡（256）——254 以「实现交付+运行面挂起」诚实状态收口入 done**

## 一、armrun2 亲核

- settle 链 2 分钟完整收敛（goal completed/stop=done，executed=5）——armrun1 排干超时**未复现**；
- 新形态亲证：`D1 must contain exactly one forward mutation` FAIL+smoke report 两响应 `mutation_performed=False`+全部 bundle `mutation_authority=False`——模型走**诚实零 mutation 观察分支**，runner 断言面（133 时代）硬要求恰好一次=**断言×分支系统性错配**；
- 本卡 LW/D1 settle 门仍未执行到（runner 腿先 throw）；引擎变量闭合（config defaultModel=deepseek-flash 跑前亲读+telemetry 10 调用全 flash——两 run 同引擎两形态）；
- 纪律核可：单 run 兑现、跑前披露引擎同值、TEARDOWN 22:01:13。

## 二、归因修正（第二笔入册）

- interim「引擎延迟×排干窗口」：**延迟部分成立但引擎选择部分作废**——本机默认引擎=deepseek-flash，裁定的隔离设计（去注入）实为同引擎对照；armrun2 同引擎 2 分钟收敛证明 armrun1 超时=瞬态延迟峰（9-20s/call 晚高峰）非引擎固有；
- **教训（主管第二笔，与 248「核脚本源」同族）**：设计变量隔离 run 前须亲核配置面实值（config defaultModel），不得凭注入惯例假设默认引擎不同。

## 三、根因假说（供 256 取证，决策侧提出）

133 runner 断言校准于 10-18（M3 措辞链 243-248 之前）；此后 v2 触发权句/v3 终局路由/guard 准入全量落地改变了**所有自由态循环**的模型行为面——D1 场景同受影响（零 mutation 分支可能即新措辞下的合法行为）。256 须对照 133 run2 与 armrun2 的载荷 needle/动作序列验证此假说。

## 四、处置

1. 254 实现面合入 main（impl c687c9a1+两 run 工件随 port 分支 merge）；卡入 done，状态=「实现交付（单测点亮 D1）+真栈点亮证据挂起」；
2. **开卡 256 D1-RUNNER-FORENSICS-1**：四点复核（排干窗口×轮次节奏×mutation 断言×模型分支）+措辞漂移假说验证+按 90 先例改断言为分支容忍；其验收腿=重跑 254 门组（LW 四门+D1 settle 四门+queue view），过则 D1 真栈点亮补全；
3. 254 两 run 计费合计 ≈¥1.3-1.6 预算内。
