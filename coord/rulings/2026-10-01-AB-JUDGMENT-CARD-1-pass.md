# Ruling：AB-JUDGMENT-CARD-1 — pass（2026-10-01 决策会话；终验留合并后手测）

- 实现：a441224b@port/ab-judgment-card-1 → 合并 main fb1c3771（决策侧 cherry-pick，卡路径按终态 done/ 解冲突）。
- 亲核项：
  1. **取证结论采信（三轮现场闭环）**：点击无效=三层复合——①judgment POST 的 turn_id 用了挂靠域 run 级键（trajectory.user_judgment.requested 的 source_turn_id）而非会话原生实验域 → 服务端身份校验必拒 409；②App.tsx 无 catch → 409 静默吞（两轮取证「零事件」的真相：POST 发了、被拒了、没显示）；③**顺带真缺陷（E2E 实测）**：在飞轮询成功分支无条件 setError("") 抹掉刚显形的错误——banner +26ms 渲染、+706ms 被清，所有权语义修复（成功只清运行时自写错误）。
  2. **diff 直读**：audition.ts 双域分账（turnID 挂靠域零回退=WEBUI-MSG-ORDER-2 三级挂靠不动；experimentTurnID 判定契约域，run 级键不入）+TrajectoryAuditionPanel 三个判定席位+mix-tick 席位同治+409 显形+chat 两拒绝路径 Warn 日志（取证盲区收口，扩域在卡面「实锚后申报」授权内）。
  3. **我方独立复跑**：webui **399/399**（34 文件）+tsc 0 错+go chat 包 ok（100s）。
  4. **E2E 工件亲读**：20261001_213539 verdict=pass **26 组零失败**——新组 judgment-identity-J1=真实浏览器点击「A 更好」→网络层截获 POST 断言实验域 turn_id+409 显形黄条+吞咽竞态修复；FIX-CONFIRM K1/K2 与 settleTerminatedTurnInteractions 面零回归。红绿=撤源复跑 6/6 红（3 钉断言级）。
  5. **泊位合规**：隔离 agent+文档化环境变量错峰（用户活栈占默认桥接口 21:08 实测在跑——ZMQ/UDP 改道+VSP 注册关闭，首次未错峰 bind 失败如实记 env_failure）。
- 终验边界（卡预置条件，如实保留）：真实三件套真人点击链（真实内核试听声+park 释放端到端）未跑——**FS-ADOPT-CLOSURE-1 已随本日验收合入（2d6c57a5），前置已齐**；用户手测复验建议与 FS-ADOPT 收尾项合并一场（park→点 A/B 卡→settle→新输入全链）。
- 服务端 POST→park settle 语义由 Go 测试覆盖（judgment_park_continuation），E2E J1 覆盖点击→POST 契约——分层覆盖声明采纳。
