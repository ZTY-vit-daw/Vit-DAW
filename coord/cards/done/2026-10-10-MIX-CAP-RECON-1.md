# MIX-CAP-RECON-1：混音能力层现状盘点（PC 线启动勘察）

- 发卡：GLM 主管决策侧 / 2026-10-10 夜（依据：用户裁定三线放行+PC=混音线）
- 派发确认：已确认（用户 2026-10-10 裁定）
- 验收负责人：GLM 主管决策流
- 池序 55；目标仓库=D:\Vit_DAW（+前端仓只读）；只读勘察零代码
- 优先级 / 预估 / 依赖：P1 / 1-2h / 无
- 模型分级：L0 / 任意引擎（含闲时）

## 目标（报告四节）

1. **固定 runtime 能力清单**：chat 包混音固定编排面全量盘点（c1_frequency_cleanup / c2_dynamic_batch / c2_dynamic_control / b4_eq / semantic_eq / static_balance / pan_layout / low_end_relation / strip_silence / clip_fade_gain / gain_staging 等）——每个一张：功能/入口（话术或工具）/接回自由态现状（自由态循环内可达 or 仅编排入口）/退役候选标记。
2. **能力底盘面**：插件控制链（PCA/attestation/certification）、时域效果器接入面现状（FXM 差分载体就位度）、自动化曲线面现状（B2 相关：clip/track 参数曲线是否存在、FREESTATE-REGION-IMPL-1 冻存卡的前置）。
3. **A-F 序列对照（主管补注 2026-10-10 夜：官方文档已由用户指定）**：权威定义=docs/agent_action_workflow_v1_master_plan.md §6.1（A=project_prep A1-A5/B=static_mix B1-B4 并列/C=细混 C1-C5/D=母带前 D1-D4/E=母带 E1-E4/F=审查 F1）——本节改为现状对照表（逐节点 已完成/部分/未动+锚点+下一批排序建议），不再推测。待核点：①用户"B2 自动化曲线批次"（2026-10-04）与文档 B2（静态主次平衡）不同名——文档中自动化=C4+DAW 缺口清单"线性包络线/automation"；②文档 07-13 规范注记：authority 以 PROJECT_AWARE_CAPABILITY_ORCHESTRATION_ARCHITECTURE_V1 为准，A-F 族结构按用户指定为权威。原措辞（推测映射）作废：
   > 3. **A-F 序列映射问题清单**：用户口中的 A-F 开发序列未见书面定义——本节列出"按现状推测的六段划分候选"（如 A=时域效果器/B=自动化曲线/…）+每段现有基础与缺口，**标注为讨论输入，供用户在讨论中确认或给出权威映射**。
4. **超大工程基材衔接**：三工程（sattelites/长河分轨/Weekend Lover）在混音线中的使用面（导入链就绪度=L2-2+SMOKE 已证；A/B 试听链就位度）。

## 文件域

零代码；报告+grep 附录落 coord/runs/MIX-CAP-RECON-1/。

## 验收标准

四节齐+逐条锚点；runtime 清单与 fastpath/A4 五标记交叉核对零遗漏。

- 领取：2026-10-10 21:05 / origin/main=1c1a447a4315d8b5800c5dd3c4df88dfdc62f915 / owner=GLM-5.3 flash（ZCode PC 会话，PC 执行流）/ 分支=main（coord-only 提交直推）/ worktree=主工作树 D:\Vit_DAW（本卡零代码只读，仅 coord/cards 状态提交）
- 回执：报告=[coord/runs/MIX-CAP-RECON-1/RECON.md](../../runs/MIX-CAP-RECON-1/RECON.md)+[GREP-APPENDIX.md](../../runs/MIX-CAP-RECON-1/GREP-APPENDIX.md)。四节全齐：§1 六固定编排 runtime+7 退役项+10 fastpath+工具包+legacy mix session（休眠）+D1 域表，逐项功能/入口/自由态可达性/退役标记；§1-A 交叉核对=A4 文本标记实为 6 枚（原始 5+review A note 1 增补 c2.dynamic_plugin_load.governed）+8 遥测 source 标，全部映射到 B4/C1/C2 生产者锚点，零遗漏成立；§2 底盘面=PCA 链已收口（内核无感知）/时域写回已打通而 FXM 差分采集缺生产者（仅压缩器双 tap 探针样板）/自动化曲线面不存在（引擎原生未暴露，agent 观测恒 0）；§3 按主管补注改为现状对照表（基线=MIXLAYER-RECON-1 已验收矩阵+六日增量：C3 仅路由映射无 runtime/C4 真 dev/F=固定编排淘汰判据=A4 零命中+AB 腿）；§4 三工程基材=sattelites 导入链已证、Weekend Lover 仅 preflight、长河零验证，A/B 试听 harness 端端到端可用但大工程上 A3 腿零触发（7 run audition_events 全 0）。端测边界声明：纯只读勘察零代码零栈启动，不适用端侧烟测门槛；结论以源码锚点+grep+已验收 run 工件为据。⚠️ 领取提交 076ce106 事故如实记录：扫入共享索引中并行会话已暂存的 ARR/GEN-CAP-RECON 两卡 todo→doing 纯改名（100% rename 内容零改动，coord/cards 允许面内），无内容干扰；共享索引并发 mv 冲突面建议主管协议层留意（§2.1 协调分支模式可规避）。另有记录：勘察期间卡面被主管补注（§3 改对照表），执行侧按补注后口径交付。
- 验收：（裁定文件）
