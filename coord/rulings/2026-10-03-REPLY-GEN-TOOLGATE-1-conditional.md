# Ruling：REPLY-GEN-TOOLGATE-1 — conditional pass（2026-10-03 决策会话）

## 验收

1. **根因实锚采信**（debug 日志+FORENSIC 双读）：repair LLM 返回 OpenAI 风格 `name`/`arguments` 字段名，解码器只认 `tool`/`args`→名字丢失成空名调用→准入连拒 2 次（MaxConsecutiveErrors=2）→fail("未知或不允许的工具："+空)——**协议字段名不匹配（解码层）**，非 LLM 裸空名，未触发停止条件。
2. **diff 亲核**（28da5db1，5 文件）：①planner.ToolCall 别名单点回退（三解码路径全覆盖）；②toolAdmissionError 可诊断化（5 处准入点统一，空名明示缺名原因）；③真·空名耗尽收口（只读观察在场→r.complete 物化观察摘要+含因诊断，不静默吞不中断链）——修复面与判定/结算链零交集（chat 包零改动）✓。
3. **我方独立复跑**（合并态 main 亲跑）：三钉（别名解码/空名收口/文案可诊断）PASS；planner+chat 全包（94.9s）PASS；**全仓 -count=1 EXIT=0**。
4. **RED 抽验（我方亲做）**：仅回退 planner.go 别名修复→钉① FAIL 复现——测试真钉。
5. **烟测工件**：三轮 exit-0 未达成（run1 判定→结算→落库全通、脚本断言面误用 /agent/state 同族缺陷已改会话图面；run2/3 判定席前模型随机分支同型连败→§8 止损触发，工件保全）——**分裂证据+用户手测复验**转正，与 SETTLE-DELIVER-1 同先例。
6. 过程事故（共享工作树暂存残留被并行领取误携→coord(fix) 归位+原子重领）如实申报，教训记 gate。

## 裁定

- **conditional pass**：转正条件=**用户手测复验**（同会话判定结算后发观察问句→收到正常观察回复而非报错）；单场 exit-0 烟测可由该场替代。
- 实现 28da5db1 随验收合 main 已推送。
