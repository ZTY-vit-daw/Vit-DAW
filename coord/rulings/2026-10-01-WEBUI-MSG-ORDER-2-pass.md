# Ruling：WEBUI-MSG-ORDER-2 — pass（2026-10-01 决策会话）

- 实现：d313eb72@port/webui-msg-order-2 → 合并 main ef8de3d4（cherry-pick）；卡片状态变更 cb695fb4 已由执行侧合规直推 main。
- 亲核项：
  1. **机制取证采信**：A/B 判定卡钉底=独立 pinned lane（App.tsx MessageStream `unanchoredSessions`）——卡 turnID 取自 audition.ts 而**内核 audition::Session 无 turn 字段**（用 M1-RETEST 固化证据：八条 audition.* 事件 turn 域全空实证）→ turnID 恒空→永久流尾；第二通道=renderPlan orphanTurnIds 尾部追加。停止条件核验正确（free_state 事件带原生 turn 域+session_id 内嵌原生域，webui 可独活排序）。
  2. **diff 直读**（核心 9 文件 +375/-30）：①孤儿块时序插列——有 turnEventMeta.startedAt 证据才插列组边界（容差复用 TURN_SLOT_ANCHOR_TOLERANCE_MS），无证据不猜保持流尾，旧调用点行为逐字不变；②判定卡三级挂靠（组尾→轨迹块跟随→流尾兜底）+session_id 原生域解析（`audition:turn:<native>:round-*` 规范编码，trajectory.nativeTurnIds 新账映射回 B9 轮次）；③audition.session.startedAt 增量归约取最小（解析失败保持 NaN 不虚造）。全部「无证据不猜」纪律，transient 态零持久化兼容面。
  3. **我方独立复跑**（执行侧 worktree 态）：npm run test **393/393 全绿**（main 基线 383+新 10）+build 过。
  4. **E2E 工件亲读**：red run verdict=fail 且**唯一败组=msg-order-M2**（修复前 bundle 复现用户目视形态：卡 flowIndex 7 在 u2(4)/报错(6) 之下）；green run verdict=pass **24 组零失败**。msg-order-M2=**活态形态组**（composer 驱动+事件缓冲中段追加，非纯重放）——上卡声明的「活态无真栈断言」缺口正式补面，本卡验收要求达成。
  5. **泊位声明合规**：red/green 独立 run root 不覆盖+隔离端口（7899/桥接改道）+agent 进程运行后即停（pid 在案）+与并行卡①真栈零共享。
  6. **行为语义变化核可**（设计内）：用户越过待裁卡继续对话后，卡按既有 supersedes 语义沉淀「卡面选项未采用」（原钉尾逃逸了 supersedes 判据）——E2E 已断言该沉淀；与 FS-PARK adoption 语义同向（继续=默认采纳），两卡口径一致。
- 已知边界（回执如实申报，不返工）：轮内细粒度次序（卡挂开启消息后的槽位语义，轮间边界才是本卡承重面）；TRAJ 水合收据行仍流尾兜底（同款形态出现再按本卡模式扩展，候选后续卡）。
- 收尾项：用户手测复验（顺序正常）留待下次手测轮——与 FS-STOP-APPLY-1 修复合入后的 M1 复验第四轮同场销项。
- 关联：webui dist 将随本 ruling 后由决策侧重建（供 Godot 拉栈）。
