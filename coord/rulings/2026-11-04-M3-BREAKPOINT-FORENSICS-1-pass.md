# Ruling 2026-11-04：M3-BREAKPOINT-FORENSICS-1（244）——pass（报告合入 319e3444）

## 裁定

勘察卡 **pass**。零实现面（port 分支 diff 仅报告 145 行）+零 LLM 调用 ✓；三面各有锚点+载荷证据+分级 ✓；未定位六处如实标注 ✓。

## 主管亲核（五承重锚点全属实）

1. `chat/free_state_reasoning_loop.go` struct **无 AutomationSurfaces 字段**（全文件 grep 零命中）；
2. `freeStateLoopFromAny`（:3401）Marshal→typed Unmarshal 标准丢弃未知键——run 边界丢弃点成立；
3. `agentloop/free_state_reasoning.go:2248-2256` 钉住写入 loop["automation_surfaces"]+ledger 镜像；
4. `chat/goalrunner_chat.go:194-204` 吞回执文本与三轮载荷逐字一致；
5. `agentloop/runner.go:589` 单槽覆写+TERMINAL TURN 渲染格式（ccb_model_prompt.go:420+）与载荷亲合。

## 假说修正（勘察卡价值兑现）

243 裁定修正②「上下文驱逐」方向正确、机制层修正：**非 compaction 字节裁剪**（hot 10K/warm 5K/full 24K 机器独立存在且未触发，msg[2] 恒 ~40KB 节区每轮重组），而是 **chat 宿主 run 边界 typed round-trip 键丢失**（相位触发：limit_reached→checkpoint→续跑重建）。r2 幸存=单槽时序运气（终局前最后回执恰为 get_parameters 停在永不降级阻塞节区）。

## 设计裁决（主管，用户可推翻）

**时变目标维持「直调正道」，不进 admitted 提案词表**。依据：2026-10-31 vocabulary-full-retirement 裁定（语法归确定性代码、语义归模型）+automation 工具链设立初衷（时变走确定性工具链正是为绕开提案词表）。落地=246 的目标级规则句（#3）加时变豁免，不新增 automation 提案域。

## 产出转卡

- **245 M3-CHAT-HOST-CARRY-FIX-1**：面一 T1（struct 字段+merge 分支）+面三 T1（吞回执升级+终局补投递）——chat 侧确定性双修复，判据=终局轮载荷含披露 needle（模型无关）。
- **246 M3-TERMINAL-ROUTE-WORDING-1**：面二措辞四项（终局三选补直调形态/时变 final:true+set_points 示例/确认语义桥接句/目标级规则句时变豁免）——五面 parity 同步按 243 先例。
- 两卡文件域不相交可并行；真栈腿错峰（卡面已互注）；预算各 ¥1.5-3。
- 观察回执累积面（T2）挂证据触发器沿用 2026-11-03 用户裁定；预算句错位留 T2 不动。
