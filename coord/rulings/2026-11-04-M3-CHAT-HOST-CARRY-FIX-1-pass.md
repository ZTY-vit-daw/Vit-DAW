# Ruling 2026-11-04：M3-CHAT-HOST-CARRY-FIX-1（245）——pass（实现合入 b9e14639）

## 裁定

**实现 pass 合入**（merge b9e14639，含实现 fae86236+工件 14c3e173）。三面确定性修复全部达成，文件域零越域（恰三文件：loop+goalrunner+新测试 281 行）。

## 主管亲核清单

1. **面一 diff 亲读**：struct `AutomationSurfaces` 字段（注释锚定 244 报告 §1.2 步骤 3）+`mergeFreeStateAutomationSurfaceRows` 四 helper（指纹刷新/严格前进过期/等值保留 fail-closed 到 pin/上限 8 FIFO），语义逐条沿用 agentloop 写侧 ✓
2. **chat 包亲跑**：ok 122.1s exit 0 ✓（执行侧另报全仓 ./... exit 0）
3. **needle 亲证（终局轮定位法）**：r1 终局决策轮（TERMINAL TURN 在场，conv 行 31）1011/0.740818/automation_surfaces 键全在场；r2 终局轮（行 35）同 ✓——**244 定案的 run 边界丢弃形态真栈消失**。r3 终局轮无披露=全轮零 get_parameters 调用（tools 仅 observation_request）=无源可携，正确语义、只记录 ✓。初查「末行缺披露」为终局后附加行，非 needle 失败——定位方法修正记录在案
4. **T1a 亲证**：三轮 ex1 吞回执 WorkflowData 均携带 free_state_summary ✓
5. **T1b 证据等级注**：代码面（emit 通道+sanitize 护栏在 diff）+单测④+回执具体引用（goal_id+时间戳）；**原始 agent log 未随工件入库**——工件卫生小瑕疵记档，多源一致不翻案，下卡工件清单应含 agent log
6. **栈纪律**：占用 7d5bf94c/释放 e089cbfe 全流程+TEARDOWN_ALL_ZERO（21:08:59）✓；错峰解锁 246 已生效
7. **计费双口径**：in+out=¥1.74（线内）；含 cache 粗上界=¥2.53（微超 ¥2 上界，如实披露；与前卡口径差异=cache 计入否，本裁定两口径并列入册）

## 附带观察处置（主管）

r2 终局 run 无 [continuation.arm] 行=recordGoalResult 未执行证据，durable 终态=cancelled 而 goal=completed，与理论必经路径矛盾——**既有路由面缺口，非本卡引入（改动前同在），终局结果未丢失（经 gate 通道投递）**。不阻塞本卡；挂账：若 246 后组 1 绿则并入链收口前的清理卡，若仍红则并入下次取证包（recordGoalResult/durable 记账面）。

## 链账本

229→245 十七卡：三轮 verdict 仍 no_route=预期（246 措辞未落地）；基础设施面两洞（终局驱逐+前台零状态）已闭合，剩余主阻塞=面二措辞（246 已解锁可领取）。距首绿=246 落地+一次定向重跑。
