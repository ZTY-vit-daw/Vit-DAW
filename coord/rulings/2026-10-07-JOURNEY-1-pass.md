# Ruling：JOURNEY-1 验收 pass（2026-10-07，决策侧）

- 对象：用户旅程烟测建设首交付——旅程清单设计+首旅程场景 journey_first（实现 **f3d8aaad**）
- 裁定：**pass**（设计段+首旅程双达标；J2-J6 后续逐旅程立卡）。

## 证据链（决策侧亲核）

1. **设计交付亲读**（coord/runs/JOURNEY-1/JOURNEYS.md）：六站点→六旅程切分总表（起点态/断言面/确定性口径/LLM 参与面四列齐）；确定性纪律正确落在服务端/内核拥有面（invoke 状态/authority/canary_stage/确认卡结构/事件/投影/文件系统），J3 NL 面按 §8 概率性声明（N=3/成功条件/失败分类/止损线=连续 2 轮同形失败）且 exit 0 门只含确定性子面；泊位隔离契约四件（run 目录 fixture/VIT_PROJECT_XML 重定向/VIT_HISTORY_DRAFT_ROOT 重定向/finally 必拆）沿 journey1+SMOKE-SCEN-RANGE-1 先例。J1 详设断言表 12 项逐腿可查。
2. **实现 diff 核**（f3d8aaad：scripts/dev_agent_smoke.ps1 +432 行+run 工件；agent 代码零改动属实——diff 无 agent/ 源文件）：journey_first 场景接线（不入 all、强制 -StartKernel、berth 隔离重定向）。
3. **执行侧证据**：真栈 2 轮 exit 0（20261007_095343/20261007_095458，propose 均首探即中）+midi_register 共享泊位回归 exit 0（20261007_095715）；工件亲读（summary.json outcome=pass、逐腿 probe JSON、git_status/head 留档）。
4. **我方复跑（关键）**：`-Scenario journey_first -StartKernel` **exit 0**（run **20261007_101641**，完整 HEAD=e26a282c+00f96200）；journey_summary 亲读逐项坐实——track_names=[Bass, Lead Vocal] 恰 2/project_uuid 在位/2 会话目录/authority_probe=authority_smoke_ok/propose_probes=1 首探即中/approve_stage=executed_verified；泊位自拆净（端口仅 TIME_WAIT 无 LISTENING）；源码树零运行态污染（git status 仅运行时 XML+决策侧文档编辑）。

## 附加裁定

1. **烟测归属修正（流程）**：执行侧 09:53–09:57 两轮烟测建于并行流 L1-4-IMPL-A 主树半成品编辑期（IMPL-A 09:48 领取 10:13 提交），二进制不对应任何已提交状态——按 §9 归属失效，**以决策侧 101641 复跑为验收依据**，执行侧两轮降级为过程证据。流程修正记 L1-4-IMPL-A ruling 边界条件 2（烟测与主树编辑会话不得同窗共用主树）。
2. **设计开放点**：J4 A/B 装卡入口待核（设计 §1 已申报）——J4 立卡时按停止条件处理，若装卡无确定性入口则上交锚点清单。
3. **后续排程**：J2-J6 每旅程一卡（设计 §2 清单即取材面）；M1 harness 更新批的旅程门槛层自此成型——涉"前端→agent→内核"活链路的卡可以 journey 场景回归兜底，不再以用户手测当第一道验证（AGENTS §5 旅程层兑现第一步）。
