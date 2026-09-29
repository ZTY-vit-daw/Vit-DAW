# 终版排程（2026-09-29 定稿，D 阶段汇总）

> 今日全部裁定的合成：定位定案（decisions/2026-09-29-thesis-positioning-decision.md，含 68 号章节结构）/ 质量第一（dev-quality-first-ruling）/ 宏组件设计收敛+越过裁定 / 编曲生成方向+集合商+Suno 会话适配器 / harness 线梳理（AGENTIC-OBSERVATION 蓝图为骨架）/ 用户三委托（编曲通路层先行按合理排、L1-4/5 落实结果导向、Mac 正常分工）。
> 排程原则：目标为主时间为输出（用户裁定）；质量门永不因日期弱化；freeze 管论文不管开发。

## 1. 分线目标与归属

| 线 | 内容 | 引擎/端 |
|---|---|---|
| **L1 地基**（蓝图关键路径） | L1-3 收段（在飞）→ L1-4 浮动窗口（四层前缀+退场）→ L1-5 新 harness+G3 A/B → 汇合（编曲核心上新 harness） | PC：GLM 亲自（关键机制）/flash（机制性外围）；L1-4 卡 L1-3 收段后由决策侧写 |
| **L2 独立组件** | L2-1 交付 profile 收尾 → L2-2 段落 DSP 收尾 → L2-3 质询协议实现 → L2-4 时域效果器接入 | 逐卡定端（2026-09-29 修订，decisions/2026-09-29-endpoint-allocation-principle.md）：L2-4=**PC**（processor 体系深耦合，用户裁定）；L2-2/L2-3 写卡时核实耦合度（预计偏 PC）；新开面子项 Mac 可接（真栈门归 PC 验收面） |
| **宏组件** | MACRO-RECON-1（在池）→ DESIGN（决策侧亲自）→ IMPL-A agent 侧 → IMPL-B webui+E2E-WEBUI-1 最小渲染烟测 → 端测收口 | PC：GLM L1 引导+flash 执行；与蓝图正交并行 |
| **编曲/生成（通路层先行）** | ARRANGE-RECON-1（在池）→ DESIGN → 通路层 IMPL（工程统一性底座/资料库/生成驱动 CLI+适配器/外部导入/Basic Pitch 音频转 MIDI）→ **核心编曲能力（NL 写入轨道等）等 L1-5 汇合后上新 harness** | 子项拆分定端（同上修订）：深耦面（tempo 披露键/观察装配）=PC；新开面（生成驱动 CLI 底座/Basic Pitch 集成）Mac 可接+真栈场景归 PC 验收；Suno 会话适配器随通路层后 |
| **实验+论文** | 重锚 tag（PILOT 前切）→ PAPER-EXP-ADAPT-1（本日入池）→ PILOT → K1-K7 拍板会（10 月中，决策侧先出预消化材料）→ 56b 定稿 → MAIN 84 全量 → JUDGE → 42 号回填；写作侧：用户三章审稿+轮 6/7 | PC 真栈跑实验；写作=用户+材料侧会话 |
| **展演** | DEFENSE-SHOW-1：11 月上分镜、12 月上素材、12 月中版本冻结+排练 ≥2、12 月底预答辩 | 决策侧设计+用户 |
| **vit note**（2026-09-29 晚增设，decisions/2026-09-29-vit-note-concept.md） | VITNOTE-RECON-1（在池，零代码可并行）→ DESIGN（设计队列最前排，RECON 回报后先落）→ **v1 IMPL（便签+每 note 一对话流+RiskCeiling 权限分级）——排在展演分镜设计之前完成可演示形态（M8 入场）** → v2 orchestration Worker 实例化（L1-5 汇合后，与编曲核心同期） | v1=PC（Godot 前端+chat 接线+webui 徽章小卡）；v2 耦合 orchestration 面 PC；答辩招牌素材 |

## 2. 周历

| 周 | L1/L2 | 宏/编曲 | 实验/论文 |
|---|---|---|---|
| 本周 09-29–10-05 | IMPL-B→C→D（GLM）；CCB-PARAM（flash 晚窗）；Mac 领 L2-1 | MACRO-RECON-1、ARRANGE-RECON-1（只读并行） | 用户启动三章审稿 |
| 下周 10-06–10-12 | L1-3 验收；L1-4 卡入池开工 | MACRO-DESIGN+ARRANGE-DESIGN（决策侧） | ADAPT 执行；重锚 tag 预备 |
| 10 月中 10-13–10-19 | L1-4 推进；Mac L2-2/2-3 | 宏 IMPL-A | **K1-K7 拍板会+切重锚 tag→PILOT** |
| 10 月下 10-20–10-26 | L1-4 收口；L1-5 开卡 | 宏 IMPL-B（渲染烟测）；编曲通路层 IMPL | 56b 定稿→MAIN 开跑；轮 6 |
| 11 月上 10-27–11-16 | L1-5 G3 A/B | 编曲通路层收口+端测 | MAIN 完成→JUDGE→42 号回填；DEFENSE-SHOW 分镜启动 |
| **11-17 论文 freeze** | — | — | 论文内容冻结，1.5 个月改稿启动 |
| 11-17–12 月中 | L1-5 收口→**汇合：编曲核心上新 harness** | Suno 会话适配器；展演集成 | 三轮改稿；第五章；轮 7 |
| 12 月中–底 | 汇合编曲演示化 | 展演版本冻结+排练 ≥2 | **12 月底预答辩** |

跨 freeze 后续（1 月+）：memory 收尾（D14，答辩压轴素材）；L2-4；蓝图全线推进。

## 3. 兜底链（质量第一的出口序，只缩范围不降质量）

1. 编曲演示分级：核心编曲（新 harness）赶不上 12 月中 → **通路层演示保底**（外部导入 Suno+Basic Pitch 音频转 MIDI+统一性底座——10 月下已就绪，本身可演示）；再退=DEFENSE-SHOW-1 既有零开发依赖保底节目。
2. 时间再紧：砍展演编曲段规模 → 砍实验规模（56b §6 压缩序）→ 论文覆盖缩（插槽标研究计划）→ 顺延。**永不砍质量门与实验纪律。**

## 4. 池动作（本日）

- 入池：PAPER-EXP-ADAPT-1（实验线首张执行卡）。
- 待写：L1-4 卡（L1-3 验收后，决策侧）；MACRO-DESIGN/ARRANGE-DESIGN（各自 RECON 回报后）。
- 常备：todo 维持 ≥2 张可执行余卡。

## 5. 纪律回执

排程产出链：本文件+今日 5 份 decisions+3 份 reports+池内 7 卡，全部已推 main（HEAD 见提交链）。开工时点由用户掌握——本排程不含自动开工。
