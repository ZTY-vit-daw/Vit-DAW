# Ruling：GEN-CTX-CONSUME-1 pass（2026-10-12，Mac 决策侧先行裁定）

- **裁定：pass**（调度件授权+PC 复核留档）。合入 main = cherry-pick ccc67053（实现）+d3fc4ffe（done 移动）；领取 e89922ca 已直推（含上一轮外部派发无领取痕迹的如实记录与防撞自领）。
- **验收证据**（本会话执行+自验，证据链同链可溯）：
  - 实现与设计件三 §2 逐条对齐：gen.generate 全量经单一渲染引擎（profile=gen_api）；**协议转换不改自动块双证**（单测断言 provider 请求体含块+用户文本置尾；模拟器 sync_request_body trace 同断言）；scene 三枚举贯通+写前哨兵（free_creation+track_id=拒 ingest 且零内核写命令，单测断言 commands 面）；in_project 无工程真值 fail-closed 不发 API；空 prompt 显式守卫。
  - 门：go vet 0；gofmt 净；**全量 95 包 0 FAIL 终态复跑**；**真栈冒测 21/21 门 exit 0**（ADAPTER 原 17 门+consume 四门：注入证据/模拟器见块/豁免拒写/豁免零块；工件 coord/runs/GEN-CTX-CONSUME-1/ 含 p05b/p05c 回执与 sim_evidence）；MAC-RUNTIME-STACK 占用释放闭环。
  - 过程两笔修复如实入回执：①空 prompt 守卫真回归（注入块使 prompt 恒非空绕过旧检查——冒测首轮红门暴露，生产侧补守卫+单测钉住）；②sim 形态断言按注入契约更新（裸 prompt 相等→含块+用户文本）。冒测三轮迭代=1 功能修复轮+1 断言更新轮+权威轮。
  - 避让红线兑现：capabilitycontext/contextruntime/promptruntime 零触碰。
- **里程碑**：统一化四卡全闭环（DISCLOSE→KEYFIELD→INJECT→CONSUME）——生成链自此刻起所有 gen.generate 调用携带工程音乐上下文，豁免语义有哨兵强制。
- 端测边界：agent HTTP 面+内核只读 state+本地模拟器承载；真实外源验证仍待用户 API 配置（沿 ADAPTER 裁定留档）。
