# SOFTGUIDE-VERIFY-ROUND2-1 §8 预声明（跑前锁定，跑后不得改）

- 卡：coord/cards/todo/2026-10-22-SOFTGUIDE-VERIFY-ROUND2-1.md（池序 175）；本件落笔于任何产品轮之前（2026-10-22 22:0x +0800）
- 协议权威：[146 实验工件 §E 原协议](../../runs/146-SOFT-GUIDE-EXP/20261020_b2arm/RESULTS.md) + [159 复用面](../../runs/SOFTGUIDE-RESCHEDULE-1/)（PREDECLARATION 0078e2ec / RUNBOOK / bringup / run_20261022_1846 中断面）
- 领取基线：origin/main=**9b79ceb3**（与本地 main 一致，159 分叉已合齐）；分支=port/softguide-verify-round2-1；**生产代码零改动**（工作树 VitApp/Workspace/default_project.xml 的 M 态为领取前已有改动，本卡不触碰）

## 0. 被测与环境（§9 契约）

- **agent=主检出 `D:/Vit_DAW/agent/bin/VitAgent.exe`** SHA256 前 16=**6CE34E541E77270F**（2026-10-22 21:40 构建，与 170 登记同一二进制；含 169 别名扩展+170 披露确定性供应+162 域漂移修复——159 域漂移注记「基线不含 162」的收敛观察条件本轮成立）
- vsphub=主检出 `VspHub.exe` SHA256 前 16=**B10CC41295066C48**（与 170 同）；kernel=主检出 K3 Release `VitApp/build/VitApp_artefacts/Release/VitApp.exe` SHA256 前 16=**722A79D7A3F53797**（跑前亲核，bringup 硬校验；只读复用，VIT_PROJECT_XML 指 run 目录 kernel_workspace 副本，主检出/staging workspace 零触碰）
- 手动起栈三进程（bringup_sv2.ps1，复用 170 脚本面）：kernel ZMQ 5555/5556 + vsphub 8787 + agent HTTP 7878；**Godot UI 不启**（chat 链判定面不涉 GUI，159/170 先例同形）；驱动=主检出 `scripts/softguide_exp.py`（快照副本随本目录入库）；fixture 由驱动在 run 目录自建（确定性 stems，0 LLM）
- 栈占用：错峰序 **173（VSP-HUB-PUBLISH-RESILIENCE-1）之后**——登记时点 173 未领取在飞、PC-RUNTIME-STACK 当前占用=空闲（170 已 21:49 释放）；登记走本卡分支 push 为可见面（通道偏差同 159/166/170 形态，登记行声明）

## 1. 臂与次数（§E 原文沿用，159 未完成面补全）

- **B2 臂（惯例 ON=生产 carriers，零补丁）**：10 变体 ×1 run（A1–A5、B1–B5，§E §一.C 原话术；159 止损停于 6/10——A1–A5/B1 已跑形态本轮照跑，B2–B5 补全）。每轮 prompt_render 快照亲核惯例段在场。
- **B1 对照臂（现状提示词=同生产 agent，159 未跑面）**：5 run（A1、B1、B2、B3、B4，§E 原名单）。职能收窄：仅作 F-gate 计数与到达率对照采样，**不做惯例差分判读**（146 §0 硬规则分支已锁）。
- 每变体=新会话（新 conversation/goal）+观察前置话术+full_project_access（驱动亲设+/smoke authority 探针）。
- 驱动批次：B2 逐批 2 变体串行（[A1,A2]→[A3,A4]→[A5,B1]→[B2,B3]→[B4,B5]，批间机械计数），B1 同（[A1,B1]→[B2,B3]→[B4]）。fixture 首批自建后复用；每变体 project.open 复位副本。

## 2. 判据（机械计数，不设通过线；判读交主管）

- **主判据（机械计数）**：①**F-gate 计数**（159 主期望=0，本轮如实计数——F-gate 再现=新缺陷证据保存上交）；②**提案面到达率**=到达提案（提案卡/typed controller/终答计划文本）的 run 数 ÷ 有效 run 数，本轮为 **15 run 完成面统计**（对照 159 的 6/10 中断面 4/6）；③**0 幻觉拓扑**（硬判据 §E 逐字：提案引用轨/send 目标 ∈ 观察披露 ∪ 本提案新建；track.send.list 读回亲核）；④**域漂移收敛观察**——159 形态 A3 源轨 other/1022（应鼓组）、A4 源轨 piano/1027（应吉他）在 169 别名扩展+162 域漂移修复后是否归零（A3 期望形态=drums/1012、A4=guitar/1017；只观察记录，归因与判读交主管）。
- F-inline / F-notopo：只如实分类计数，**不构成失败**（B2 拓扑判读已预绑定锁硬规则分支，惯例路由效应不在重判域）。
- Plates decay 轴：159 挂账顺延本轮——本轮产出惯例路由到达面，判读交主管。

## 3. 失败分类（§E ×5 逐字）

F-inline（inline 装载源轨）/ F-notopo（提案无位置主张）/ F-halluc（幻觉拓扑，硬失败）/ **F-gate**（未达提案：门拒或自报边界——含 target_identity 类）/ F-env（环境中断，须原始日志或退出证据，AGENTS §8）。
驱动形态无效轮单列，不占产品轮。

## 4. 止损线（159 同款）

1. **F-gate 任何一次出现**：捕获该轮原始工件后，主线暂停计数，另跑 1 枚新变体复核签名；**第 2 次 F-gate（或同签名重现）→ 全停**，保存原始工件上交=169/170 修复失效或新形态证据候选（是否失效交主管判读）。原样重跑同一变体 ≤2 轮（卡面停止条件）。
2. **F-halluc 任一出现**：该 run 硬失败并全停上交（§E 硬判据面）。
3. F-env：按 AGENTS §8 单列不计有效轮；环境修复后重跑不占产品轮。
4. 驱动自身缺陷轮：修复尝试 ≤2 次后停止，已捕获工件如实上交。

## 5. LLM 记账（§8）

- 模型=deepseek-flash（栈 config 不动，与 146/159 同款）；预算 **≤135 次调用**（15 run × 每轮约 6–8 调用实测面[159 实测 42/6≈7]+authority 探针；fixture 构建 0 LLM）；0 error 期望如实记录，超预算即停。
- 驱动缺陷轮/环境轮不占产品轮，不消耗预算判读面。

## 6. 工件契约

per-case（prompt.txt / chat_*.json / hop_*.json / walkhop_*.json / events.json / send_list_{before,after}.json / project_state_after.json / runtime_status_final.json / variant_summary.json / **prompt_render_snapshot.jsonl**）+ session_summary/session_log + 每批 console 留存 + 跑后 agent_last.log / agent_llm_telemetry.jsonl / agent_prompt_render.jsonl / vsphub_last.log / pid 三件 / kernel_sha256.txt 快照。工件根=本目录之下新建 run 目录（AGENTS §9，不覆盖旧 run）；重件（fixture wav/工程副本/.vit_history/.vit_derived）留盘不入库可回指（170 工件瘦身先例）。判读面回指原始工件。

## 7. 端测边界声明

本腿=agent HTTP/ZMQ 面 chat 链真栈（真实内核+真实 deepseek 链+真实工程副本隔离）；**不含** Godot GUI 渲染面与 webui 渲染面断言（146/159/170 先例同界）。提案面到达的判定以 chat 响应/events/内核回执工件为据，不以 GUI 呈现为据。
