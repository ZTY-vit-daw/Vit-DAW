# REFSCHEMA-M8：Godot 写者+PCA 层 refs 勘察补腿（只读勘察卡，零代码）

- 池序 33（[G1 终审 M8 行](../../runs/L1-1-REFSCHEMA-1/G1_FINAL_REVIEW.md) §4："Godot 写者+PCA 层勘察补腿，≤0.5h，机会卡或 M6 前"）；目标仓库=D:\Vit_DAW + Godot 前端 `D:\Godot\project\vit-daw-frontend`（仓库外，只读）
- 优先级 / 预估 / 依赖：P3 / 0.5h / 无；**只读勘察卡与一切在飞卡天然可并行**
- 模型分级：L0 / 任意引擎（flash/codex/闲时均可）
- 背景：G1 迁移 M6 行（G 类快照族 mixboard_/kernel_prepared_+Godot 第三写者）需要先有 M8 勘察——Godot 侧写者族的 ref 前缀形态与 agent 侧 PCA 层的 refs 面从未系统盘点。
- 目标（产出报告 `coord/runs/REFSCHEMA-M8/report.md`，零代码改动）：
  1. **Godot 写者族盘点**：`D:\Godot\project\vit-daw-frontend` 内 GDScript 写出的 ref/ID 前缀形态（mixboard_ / kernel_prepared_ / 第三写者=telemetry 等，rg 前缀关键词清点）——每族：写出处（文件:行）、形态样例、消费方、是否已有 vit:// 对应物。
  2. **PCA 层 refs 面**：agent 侧 processorattestation / processorauthority / processorregistry 的 evidence refs 生成点清单（同上四栏）。
  3. **迁移建议**：各族归入 G1 终审 M4-M7 哪一行（或新行），预计触碰面与风险——只建议不实现。
- 约束：**纯只读**（两个仓库零改动；Godot 目录仓库外，只 rg/read）；报告写入 coord/runs/（不入 docs）。
- 验收标准：报告含两盘点清单+迁移建议表；每条锚点可回查（文件:行）；Godot 侧 grep 命令入报告附录（可复跑）。
- 停止条件：Godot 前端目录不可达/形态大改 → 如实记录环境形态上交。
- 领取：2026-10-09 晚 / 45ae7591 / main（PC 执行侧 ZCode GLM-5.3 直推；工作树含他卡在飞改动未触碰，本卡仅 coord/）
- 回执：报告 `coord/runs/REFSCHEMA-M8/report.md` @ afddcea3 基线。清单①（Godot 前端）14 条（2 证据面写者族复核+行号漂移 / 7 IPC 信封前缀 agent 侧零消费实证 / 3 诊断工具 / 2 vit:// 字面量面）；清单②（PCA）3 包面+4 条域外衍生（processorregistry 零 ref 面、processorattestation 复核成立、**processorauthority 系生成者：`pcr1_` 内容寻址族 3 调用点**——旧闲时产出未覆盖面；衍生：mixboard_l2_render_probe_ harness.go:3609 第三 mixboard_ 写者、mix-report:/mixboard-project: 未注册响应面 ref、注册表 mixboard_ Anchor 漂移 6849→6923、ccbr_ 维持）。迁移建议 R1-R8 共 8 行（M6×3+M5×1+M4×1+新行 M9 登记观察×1+不动作×2）。零代码改动，两仓 clean（三处 tracked 修改均领取前已有）。附录 A1-A9 grep 可复跑。
- 验收：（裁定文件 / 验收 commit）
