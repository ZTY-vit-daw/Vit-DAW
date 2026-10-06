# Ruling：KERNEL-RENDER-FREEZE-1 — pass（2026-10-06 决策侧验收）

- 卡：[cards/done/2026-10-06-KERNEL-RENDER-FREEZE-1.md](../cards/done/2026-10-06-KERNEL-RENDER-FREEZE-1.md)；报告=[reports/2026-10-06-KERNEL-RENDER-FREEZE-1.md](../reports/2026-10-06-KERNEL-RENDER-FREEZE-1.md)；run `coord/runs/KERNEL-RENDER-FREEZE-1/20261006_210431`（主）+`20261006_203747`（首试，栈未取得已如实申报）
- 判定：**pass（取证卡口径，且为教科书级取证）**——三假设全裁定、根因闭合、修复方案明确上交。零代码改动、诊断日志 0 行、复用 Sep-06 符号化战役增量闭合。

## 决策侧亲核证据

1. **代码锚点逐条亲核**（main 工作树）：`~Handle`= `cancel(); renderThread.join()`（tracktion_Renderer.cpp:663-667）✓；完成 lambda `renderHandle.reset()`（VPC:696）✓；迟到回调 `releaseWedgedRenderHandle` erase（VPC:979-985）✓；`armRenderWatchdog` 仅写原子时间戳（VPC:972-977）✓；设备查询=廉价 getter+48k 回落（VPC:668-671，H1 否定锚）✓；渲染线程 lambda 捕获 task 析构→`~NodeRenderContext:168 callBlocking` ✓；**失败路径（status fail）`return true` 不 reset nodeRenderContext（Renderer:291-295）vs 成功路径 :301 先 reset**——"失败路径必冻/成功不冻"机制成立 ✓；NRC:92-96 `checkNodesForAudio&&!hasAudio` 在 writer 创建（:104）前早退——产物缺失解释 ✓。
2. **工件亲读**：stacks_frozen.json 46 线程——tid 7188 栈 `NtWaitForSingleObject←WaitForSingleObjectEx←Thrd_join←VitApp+0x6efb43`（join）与 tid 34304 栈 `NtWaitForAlertByThreadId←SleepConditionVariableSRW←Cnd_wait←VitApp+0xca62ff`（条件变量等待），与报告逐帧一致；wct_frozen.json 等待链 tid 7188 Blocked→ThreadWait(Owned)→tid 34304 Blocked 直连两冻结栈；probe_events.jsonl 命令序列+冻结判定（render.start rtt 0→ping 5s 整超时回包）齐。
3. **ABBA 机制逻辑核**：消息队列 FIFO（M1 完成消息先于 M2 析构 callBlocking）保证失败路径确定性互等，非竞态——与 n≥5 同型复现（MIDI-RECON 两轮+Sep-06 多轮+本卡两轮）自洽。
4. Sep-06 战役证据复用（artifacts/render_wedge_evidence 符号化帧与当前构建结构同构+WCT+代码路径唯一性）——合理闭合。

## 根因采信与修复裁定（本 ruling 的决策产出）

**采信**：ABBA 死锁（消息线程 join 渲染线程 vs 渲染线程 callBlocking 等消息线程），看门狗与消息线程共死。**触发面**：任何"启动后失败/取消"的离线渲染（MIDI-only 无 instrument 工程必现）。

**修复裁定（按 2026-10-06 刹车决策例外条款——P0 产品缺陷独立于节目线评估）**：立 **KERNEL-RENDER-FREEZE-FIX-1**，范围=方案 A（Handle 析构搬离消息线程，两违规点 :696/:691→:979-985，≤15 行，parking 机制 :1006-1010 现成先例）+ 看门狗线程化加固（≤20 行）+ 方案 B 无可发声内容前置短路（必须与 A 同卡，B 单独不修死锁不可作为修复验收）。方案 C 记录为技术债方向不实施。third_party 不碰（报告证实修复面全在我方代码）。

**连带收益**：render_failed 遥测恢复、cancel_render（CommandDispatcher.cpp:3155）恢复可用、JOURNEY-1 旅程渲染环节解锁。

## 处置

- 卡面回填验收行；空分支 port/kernel-render-freeze-1 清理（本地+远端）。
- render_wedge_evidence 沉淀（253MB PDB 归档策略+结论入现行文档）：上交用户，本 ruling 不动。
- 修复卡入池后执行侧接力（对机制最熟）；JOURNEY-1 顺延一位。
