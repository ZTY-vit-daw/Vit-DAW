# D2-STEREO-THIN-1：D2 全局立体声关系薄层落地（判据面先行）

- 发卡：GLM 主管决策流 / 2026-11-06 晚窗（依据=[终局裁定](../../decisions/2026-11-05-af-scaffolding-endstate.md) §二-3 薄固定层原则+§三 D/E/F 薄层方案+[257 矩阵](../../reports/2026-11-05-af-family-health.md) §五 C5/D2-D4 not_built 行+MIXLAYER-M1-4 已建观察面（e0e2f4ac：pre_master.observation.v0 含 D2 全局 stereo_relation+mix.read 逐轨 stereo summary，mono 风险阈值 corr<0.25||neg>0.1 已入库））
- 派发确认：已确认（主管发卡；gate §五-2 预定产物，判据面先行、指引面等校准）
- 验收负责人：GLM 主管决策流
- 池序 265；目标仓库=主仓 agent；P2 / 1 天 / 依赖=M1-4 观察面已在 main；L1-L2 / PC 执行会话；验证腿需真栈一次

## 背景

D2=全局立体声关系（not_built，capability IDs=nil）：观察面已由 MIXLAYER-M1-4 建成（只读 pack 三面之一），runtime 编排未建。D1 已点亮（254 实现+258 修复+runner 面真栈亲证）——D2 runtime **复用 D1 模式**（`agent/internal/chat/d1_loudness_settle_runtime.go` 的判据→verdict 落 store→建议挂 mix_tick 确认链卡零裸写→queue view 点亮）。

## 目标

1. **判据+验证器（薄层核心，零 LLM 单测先行）**：全局 stereo_relation 观察→mono 风险轨识别（阈值复用 M1-4 corr<0.25||neg>0.1，纯函数）→verdict 三态（risk_found/no_risk/unmeasurable 同 LW verdict 模式：行缺失/无生产者=unmeasurable 一等公民）；
2. **薄 runtime**：D2 话术→判据消费→verdict 落 store→有风险时建议挂既有确认链卡（零裸写）→queue view D2 节点点亮（not_started→in_progress/needs_confirmation，模式同 D1 的 WriteMixWorkflowQueueNodeState）；**入口识别先测裸话术能否过 semantic_entry**（263 教训：能过就不建词表，需词表则按「入口识别薄层」定位最小建）；
3. **验证腿**（真栈单 run）：D2 场景冒测（观察→判据→挂卡/无卡两分支诚实断言；判据腿零 LLM 预期，话术腿 LLM ≈¥1-3）；§8 跑前口径成文；脚本按 243/263 加参先例扩展现有冒测体系；
4. **指引面不在本卡**（等 B4 ablation 校准后另定，gate §五-2 原则）。

## 约束

- 文件域=chat D2 runtime 新文件+判据纯函数+queue view 挂点+测试+冒测脚本加参；禁改 D1/LW 既有语义（复用不改写）；禁重型 stage 机（终局裁定 §二-3）；治理红线（确认链/零裸写）不变；
- 持久化新字段按 §11 定义旧态缺省（omitempty/缺省关）。

## 停止条件

- 判据面发现观察面数据不支撑（M1-4 pack 字段缺失/口径不符）→取证上交转 M1 系修补卡；验证腿断言失败取证不重跑。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）
