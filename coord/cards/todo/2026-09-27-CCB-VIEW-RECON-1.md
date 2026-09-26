# CCB-VIEW-RECON-1：CCB view 目录与参数化缺口勘察（L1-3 查询引擎前置）

- 优先级 / 预估 / 依赖：P2 / 0.4 天 / 路线图 D5+L1-3 段（AGENTIC-OBSERVATION 蓝图）；观察投影拓扑权威=docs/OBSERVATION_PROJECTION_MANIFEST.md
- 模型分级：L1 / flash 可接（纯只读勘察+报告，零代码改动，适合夜间托管）
- **执行侧（mac 会话，夜间托管卡池成员）**
- 背景：路线图缺口四件事之一="view 参数化（尺度）、即席查询谓词、跨 view 组合与自动披露"（CCB 已是 pull 模型雏形：按显式 view 请求披露、不做自动选择）。本卡把 view 现状与参数化缺口盘成事实。
- 目标：
  1. **view 目录全集**：contextruntime/capabilitycontext 的 view catalog——每个 view 的键名、披露内容、数据源投影、裁剪规则（文件:行）；free_state_observation 的 project.structure 等 view 的裁剪子集（TIM 设计卡 §1.3 已有一例：8 字段子集不含 llm_context/issues）逐一列出。
  2. **请求与披露协议现状**：view 是怎么被请求的（显式 view_ids？混在 prompt 里的指示？）、披露的粒度控制现状（有无任何参数：时间窗/轨道过滤/粒度——大概率无，确认为零参数化基线）、partial/stale 的表示与透传。
  3. **参数化缺口清单**：对照路线图需求（尺度参数 scope×time_window×粒度/topK/谓词过滤），逐 view 评估"参数化需要动哪里"（catalog 结构/请求协议/投影侧支持）——每条带锚点。
  4. **与 D1/D2 的衔接点**：view 请求里哪些字段未来会成为 ref 谓词的输入（scope_id/time_window 的表达现状——与 L1-1-RECON-1 盘出的不一致矩阵互证）。
  5. 报告落 `coord/runs/CCB-VIEW-RECON-1/CCB_VIEW_INVENTORY.md`。
- 约束：零代码改动；锚点带文件:行；与 L1-1-RECON-1 报告（coord/runs/L1-1-RECON-1/）的发现交叉引用处注明。
- 验收：①view 目录 ≥10 个 view 逐一列出（键名/内容/裁剪/锚点）②参数化基线结论（现状零参数化或列出例外）③缺口清单 ≥6 条带锚点 ④报告入库
- 停止条件：view 体系分多层入口盘点不完 → 成稿部分+剩余层列明上交
