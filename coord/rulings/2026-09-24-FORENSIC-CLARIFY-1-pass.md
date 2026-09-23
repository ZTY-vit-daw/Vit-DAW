# Ruling：FORENSIC-MAC-CLARIFY-CHAIN-1 pass（2026-09-24）

- 卡：`coord/cards/done/2026-09-23-FORENSIC-MAC-CLARIFY-CHAIN-1.md`（PC 决策侧开卡，mac 执行侧只读取证，Mac 决策会话验收）
- 提交：`6234d2e`（领取）+ `bda8f51`（回执）+ `ad687d0`（状态机删除面补齐——mv 暂存时序事故第四次出现，执行侧自查自纠）；零代码改动（coord-only）
- 裁定：**pass**——⑤ R3 clarify 链失败黑箱全解，定责④（双方差）+⑤弱协因，①F2 回归铁证排除

## 决策侧核验（独立复算）

1. **11 秒黑箱还原**：turn=3 原始输出=形态良好 needs_experiment 提案（+1.5dB@3kHz）但**缺一个闭括号**（8591 token 异常长推理，绿轮同段 ≤2297）→ `parse_failed` → **10.3s `message_loop_repair` 调用**（闭包变体 prompt，`message_loop.go:4322-4327` 明令 *"never invent a clarification … Do not add … a needs_clarification field"*）→ **repair 违令返回 needs_clarification 问句**（「需要先确认哪条是主唱轨…」）→ `audio_closure_controller.go:557`（`repairCount>0 && res.NeedsClarification`）判「model protocol failure」→ turn.failed。问句逐字落在 durable settlement——链路五步各有秒级时间戳+证据指针。
2. **①排除铁证**：判死策略入库=`ee0fac4`（2026-09-16，仓库根提交）——**决策侧 `git log -S "repairCount > 0 && res.NeedsClarification"` 独立验证，早 F2 修复五天**；失败分类发生在投递面之前且投递面如实工作；F2 后 PC 同句问句直接路径 park exit 0；bbd0dbf↔HEAD 关键文件零 diff。
3. **定责④双方差成立**（缺一不失败）：方差 A=原始输出畸形（长推理截断形）；方差 B=repair 违令选 clarify 形态。②竞态排除（确定性序列）、③平台排除（代码同源+引擎一致+「PC 零同形态」含夹具混淆）。
4. **⑤弱协因采信**：mac 夹具无主唱轨标注给 repair 提供问句动机（PC bx_hybrid V2 7/7 零此形态）——素材面反馈：真实素材轨名含 vocals 标注，天然免疫此动机（REALSTEMS 各轮零此形态旁证）。
5. **证据边界诚实**：repair 原文未落 durable（形态由 telemetry out=2412+无第二诊断+trace planner_repair+durable 逐字问句反推，高置信非直证）——如实声明；修复建议含"repair 成功分支补 raw 落诊断"正对此界。

## 归族与移交

- **新失败族**：「repair-clarify 协议违约判死」（触发要件=parse_failed+闭包在案+repair 返回 clarify；10 轮 1 例；与 F4 并列非 F2 呈现面族）——登记入失败分类账。
- **开卡 FIX-REPAIR-CLARIFY-DEATH-1**（todo）：557 判死策略软化——repair 解析成功但返回 needs_clarification 且闭包在案时，先追加一次禁-clarify 强化 repair（或剥离字段按不可恢复降级），仍违令才判死；repair 成功分支补 raw 诊断落盘（顺带补齐本次取证的证据边界）。共享 chat 代码，单边开发两端合入。
- **流程备忘**：git mv 暂存时序事故**第四次**出现（本次执行侧自查自纠 ad687d0）——已在 FIX-FE 验收备忘"git mv 后必须 re-add"；执行侧本次自查值得肯定，但该备忘应写入 PROTOCOL §3 推送纪律条款（决策侧择机）。
