# PAPER-EXP-RECON-1：论文实验设计对基线版本校准（论文线首卡，不能让路）

- 优先级 / 预估 / 依赖：P0 / 0.4 天 / 战略决策 2026-09-26（论文实验基线=midterm-checkpoint-2026-09-26 tag）；实验设计文档在用户毕业设计目录（`C:\Users\timoz\Documents\毕业设计\` 下的实验相关文档——若在 mac 侧同样路径风格查找；找不到则从论文草稿章节提取实验节）
- 模型分级：L1 / flash 可接（只读校准+报告）
- **执行侧（mac 夜池；注意实验设计可能在 PC 侧目录——mac 找不到时报告实际找到的路径清单上交，PC 侧续做）**
- 目标：
  1. **实验设计清单提取**：从毕业设计目录的论文/实验文档中提取全部已设计的实验（每个实验：主张/变量/素材/测量指标/成功判据——原样摘录，注明文档锚点）。
  2. **对基线版本可执行性校准**：每个实验对照基线版本（tag a19ba1d8 的能力面——全链常态/候选面 83/84 包/EQ+压缩语义链）判定：直接可跑 / 需适配（缺什么：素材/断言工具/数据记录面）/ 不可行（主张超出基线能力）。素材源=REALSTEMS 素材库+journey 断言工具族。
  3. **实验运行方案草案**：可跑实验的执行方案（跑法/轮次按 §8 纪律给建议/工件目录规范=coord/runs/paper-exp/<exp-id>/）。
  4. 报告落 `coord/runs/PAPER-EXP-RECON-1/EXPERIMENT_CALIBRATION.md`。
- 约束：零代码改动；不实际跑实验（校准卡）；实验主张摘录必须带原文锚点不转述。
- 验收：①实验清单 ≥N（实际数）全摘录 ②可执行性三分类判定逐项 ③运行方案草案 ④报告入库
- 停止条件：实验设计文档不可定位 → 目录清单上交（不猜测编造）
- 领取：2026-09-28 10:27 / origin/main=55c9d8b9 / main（PC 执行侧，coord-only 直推 main；实验设计文档已定位 PC 侧 `C:\Users\timoz\Documents\毕业设计\`，停止条件未触发）
- 领取时工作树：HEAD=55c9d8b9，`git status --short` 仅 ` M VitApp/Workspace/default_project.xml`（运行时工程状态，非本卡域，不触碰）
- 回执：（执行侧 2026-09-28，PC 会话，零代码改动）
  - **报告**：`coord/runs/PAPER-EXP-RECON-1/EXPERIMENT_CALIBRATION.md`（实验清单全摘录+三分类逐项+运行方案草案+缺口与建议卡）
  - **验收四项对齐**：①实验清单——现行完整设计 1 个（56b 四配对对照实验 84 次，主张/变量/素材/测量/判据逐字摘录带锚点）+运行单元分解 6 项+已完成附属活动 3 项+已废弃旧矩阵 2 项（56 号/55 §2，防误用）②三分类逐项——Vit 臂=需适配（pv1 无 .vit 工程：runner 要求 case.project_path 而 suite_v1_lm 清单无此字段；manifest schema `vit_thesis_fixture_manifest.v1` ≠ runner 门 `..._public_manifest.v1`；封存 schema 异构；full access 脚本化未确认；三模型凭证缺）；原生臂 L1/L2=需适配（K2/K7+投喂机制，非基线面；L2 输入包已建成 runs/L2 逐例 sha256）；抽取脚本=需适配（56b §5.2 有规格无脚本）；复听/案例选例=直接可跑；**不可行 0**③运行方案草案——EXP-VIT-PILOT（试跑 4 轮不进论文）/EXP-VIT-MAIN（28 轮每格 1 次）/EXP-NATIVE-L1L2（56 轮含压缩顺序）/EXP-JUDGE，§8 纪律逐项（次数/成功条件/失败分类/连续 2 失败止损），工件 coord/runs/paper-exp/<exp-id>/ 与材料仓库 experiments/runs/ 分工建议④报告入库本提交
  - **两个上交裁定点**：(a) 剂量口径冲突——57 §6（±6 dB=DOSE-AUDIBLE-1 前置，基线未兑现：free_state_d1_runtime.go:139/203 ±2 dB、:252 声像 ±0.15）vs 56b/42 正文 v0.14（未列前置，4.4 已按窄上限口径写）——按 56b 口径零开发可跑，按 57 制度需开发卡；(b) 素材量级三处数值口径不一致（56b §2 / 57 §2.2 / dose_basis_lm.json 实测）——定稿时以 sealed+正式版本统一
  - **端测边界声明**：本卡为只读校准，未跑任何实验、未触真栈（卡面约束"不实际跑实验"）；所有基线证据=git show a19ba1d8 实核锚点；材料仓库侧仅读结构（封存真值只读 schema 键名，内容未进任何模型上下文）
  - **commit**：（见本卡提交；coord-only：卡 todo→doing→done + 报告，工作树其余改动未触碰）
