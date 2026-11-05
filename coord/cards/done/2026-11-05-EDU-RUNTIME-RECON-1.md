# EDU-RUNTIME-RECON-1：乐局运行时面勘察（关卡运行时挂载 / 前端复用 / 挂起定级）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=用户裁定 [2026-11-05-EDU-DEMO-SCOPE-RULINGS-1.md](../../rulings/2026-11-05-EDU-DEMO-SCOPE-RULINGS-1.md) §五 + 本日口头"勘察卡可出"；取代已撤的原 EDU-RECON-1 运行时面各点。注：轨迹 HTML 导出已按用户裁定从 12 月链删除，原"展演级素材源盘点"点随之取消）
- 派发确认：已确认（用户 2026-11-05 口头开工；执行转交由用户触发）
- 验收负责人：GLM 主管决策流
- 池序 257；目标=只读勘察，报告落 coord/reports/；P1 / 0.5 天 / 依赖=无（与 256 并行）；L1 / PC 执行会话；**不占真实栈、零 LLM 调用、零生产代码改动**

## 背景

乐局=工程+关卡（关键交互节点，带 done_when 完成判定+教学目标/检查挂接）+最小关卡运行时（节点状态机+完成判定+拉回）。本卡为构建块 3（关卡运行时）与乐局内容设计前置勘察。

## 勘察点（每点产出 = 代码锚点 + 结论）

1. **关卡运行时挂载面**：chat 主循环消息流（message_loop）+ `chat/capability_routing.go` + `internal/policy` 三处，乐局状态机（节点推进 / done_when 判定 / 拉回 / 教学目标检查挂接 / 工程完成判定）挂在哪一层最小而正；与 free_state / mixing_capability_layer 两个既有 capability 的共存方式。产出=**挂载方案草案**（动哪些文件、每处预计面，不写代码）。
   **【2026-11-05 派发后补（用户裁定，rulings §五-16）】评估前提：乐局状态=叠加引导框架层，与能力域路由（free_state/mixing…）正交、可进出/挂起，非并列隔离模式**——挂载方案须按此形态评估，并给 enter/exit/suspend 接口注意点（12 月只实现开局→通关单向，切换只定接口不做实现）。
2. **前端复用面**：Godot 前端（`D:\Godot\project\vit-daw-frontend`）现有 marker 呈现 / 循环播放 / A-B 呈现能力；学生卷帘编辑后 agent 的读取通路（交互已定=对话触发读取，查读取命令的数据面是否覆盖"学生写了什么"）；音符着色（有余力项）可行性一句话。
3. **park/挂起定级（轻量）**：12 月单会话一局完成已定，挂起非演示必需；本点只回答一句：状态机接口怎样设计才不封死答辩后并入 `level` 包与 park/continuation 复用（给接口注意点，不展开）。

## 验收标准

- 报告 `coord/reports/2026-11-05-edu-runtime-recon-1.md`（执行日顺延则用执行日）：每点 `file:line` 锚点 + 结论；
- 点 1 含挂载方案草案（文件清单+每处预计改动面+与既有 capability 的共存说明）；
- 对构建块 3（关卡运行时）与构建块 2（乐局卡格式定稿）给估时/格式修订意见。

## 停止条件

- 挂载面发现结构性障碍（如 routing 无法接纳新 capability owner）→ 停在该点记录证据上交决策侧；
- 全程只读；Godot 侧只读勘察，不改前端工程。

## 领取

- 会话：ZCode 执行流（GLM-5.3 flash，PC）/ 2026-11-05
- worktree：主检出直读（只读勘察，不建 worktree、不占真实栈）
- 分支：main
- 领取基线：HEAD = `b46a9096406fcde6fcfd29013bccd724ae938fb5`（b46a9096 dispatch(A-F 主线开线,254+255)）
- `git status --short`：既有其他卡未提交改动（VitApp/Workspace 两件 M、coord/cards 三件 M+一件 D、coord/runs/UX-AB-AUDITION-IMPL-1 四件 M、若干 ?? 含本卡与 256 卡文件）——一律不碰不整理
- `git diff --stat`（跟踪文件部分）：9 files changed, 4551 insertions(+), 4479 deletions(-)（均为领取前已有）

## 回执

- 报告：[coord/reports/2026-11-05-edu-runtime-recon-1.md](../../reports/2026-11-05-edu-runtime-recon-1.md)（2026-11-05，已按派发后补前提 §五-16 对齐）
- **点 1（挂载面）**：结论=乐局状态机不做第五 controller Kind，取**横切会话状态机**——与 §五-16"叠加引导框架层、正交非隔离"前提同向互证：freeStateLoop 同型会话级持久纯数据状态 + 三挂载点（确定性拦截面 handleEduJamChat / 判定器注册为工具目录命令（§五-9 倾向）/ 节点推进检查）。依据=controllerOwners 是排他执行权（orchestrationcontroller/types.go:159），乐局占位会拒所有普通轮次，与 §五-10"游戏即教学无二分"和 §五-16 正交性双冲突；ProjectMixWorkflow 先例（路由已接、runtime 未注册 fail-closed，orchestration_controller_host.go:127-129）证明 controller=排他会话权非乐局形状。共存=正交叠加零路由改动（不改 capability_routing.go 三处白名单 :894/:911/:919，乐局轮次照常走语义入口与 free_state/mixing 路由）；对既有文件侵入 ≈12 行插点 + 新建 internal/edu/ 包 + chat/edu_jam_chat.go。无结构性障碍。enter/exit/suspend 接口注意点已按 §五-16 给出（12 月只实现开局→通关单向：enter=拦截面建 state 幂等、exit=置终态+投影+零阻面沉淀钩子、suspend/resume=12 月零实现零字段预留，仅 status 枚举留 suspended 值，显式 park 将来走 DurableContinuation PendingInteraction）。
- **点 2（前端复用）**：marker（位置+range，project_marker_lane.gd:3-9）与循环播放（时间选区 Loop，top_transport_bar.gd:324）与 A-B 呈现（Godot vitnote affordance vit_face_supplier_timeline.gd:45 + webui audition 卡）全部现成；学生卷帘编辑读取=同源数据面闭环（piano_roll_midi_rules.gd:5 命令族 = agent 读面；内核 midiNoteToVar MidiService.cpp:310-320 返回 id/start/length/pitch/velocity/channel，beats 坐标、音符可寻址）——"学生写了什么"零新增通路；音符着色可行（PianoRollView.gd:1389 双态分派先例，加第三态即可）。
- **点 3（挂起）**：P3/答辩后非演示必需；接口三注意点=判定器纯函数化（输入纯数据无会话引用）、状态双绑定但不绑 controller Kind（显式挂起将来走 DurableContinuation PendingInteraction，continuation_scheduler.go:209 现成）、schema_version+restore fail-closed（mix queue restore:670 模式）+旧状态缺省语义先写进卡；enter/exit/suspend 共通红线=三接口不得触碰路由白名单与 owner 注册表（正交的机制含义，也是并入 level 包的接口不变式）。
- **估时/格式**：构建块 3 估 1.5-2 天建议拆 2 卡（edu 包 0.5-1 天 + 挂载/烟测 0.5+0.5 天）；构建块 2 修订 3 条=判定器枚举小而前置（首版 3 个）、小节坐标显式化（bpm+起拍进卡，beats 统一）、写入放行边界写进卡（RiskConfirm 乐局内降级=独立产品裁定）。
- 勘衡量：chat 4 核心文件全文 + goalrunner 主流程段 + orchestrationcontroller/tools/内核 MidiService 读取链 + Godot 前端 6 文件；未展开（实现卡阅读项，不影响结论）=continuation_scheduler 全文、message_loop preflights 全文、webui audition 组件内部。上交两件事：①"不做第五 Kind"（=叠加框架层形态的机制落地）请主管发实现卡时确认；②写入放行降级边界建议随构建块 2 定稿一并裁定。
- 边界声明：全程只读（agent/VitApp/scripts/Godot 前端零生产改动）；未占真实栈；零 LLM 调用；临时产物零落盘；工作树既有其他卡改动未触碰；未做 git 提交。

## 验收

- **结论：pass**（GLM 主管决策流 / 2026-11-05）
- 亲核记录：Acquire 排他语义逐字（orchestrationcontroller/types.go:155-164 "conversation %s is owned by…"）；三处路由白名单（validCapacityAssessment 锁 free_state/mixing 两值 + restore 校验）；projectMixWorkflowV1Available 恒 false fail-closed 先例（orchestration_controller_host.go:127-129）；确定性拦截面在位（goalrunner_chat.go:180-191，mix queue + send topology 双先例）；Godot 三锚点（top_transport_bar.gd:324 Loop 文案、piano_roll_midi_rules.gd:3-13 IPC 同族命令、PianoRollView.gd:1389 `_apply_note_polymorphic_visual`）。只读纪律：git status 跟踪面与领取基线一致，零生产改动（agent/VitApp/Godot 全零）。
- 叠加层前提对齐确认：报告按派发后补（§五-16）评估且同向互证——横切会话状态机即"叠加引导框架层"的机制落地。
- **上交两项处置**：①**主管确认采纳"不做第五 controller Kind"**（排他 owner 语义与 §五-10"游戏即教学无二分"、§五-16 正交性双冲突的论据成立；ProjectMixWorkflow fail-closed 先例佐证 controller=排他会话权非乐局形状）；②12 月课堂写入放行降级边界（乐局上下文 RiskConfirm 是否降级）=**交用户产品裁定**，随构建块 2 乐局卡格式定稿一并定；主管倾向首版不降级（写入保持确认，演示节奏可控且"可拒绝的提议"符合 doc 1 §1.5 原则）。
- 采纳要点：挂载方案（三挂载点≈12 行插点+edu 包三文件）、估时（块 3 拆 2 卡 1.5-2 天）、块 2 格式修订三条（判定器枚举小而前置/小节坐标显式化/写入边界进卡）作为实现卡输入。
