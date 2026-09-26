# PORT-FE-ALIGN-1：PC 前端仓治理与双端对齐——基线厘清 + leg5 并入 +（待批）远程建立

- 优先级 / 预估 / 依赖：P1 / 0.5 天 / rulings/2026-09-26-PORT-CLOSEOUT-LEGS-1-pass.md 遗留②；**D-FE-REMOTE 已批准（用户 2026-09-26"可以"）——目标 3 转正式执行项**
- 模型分级：L1（分支/提交整理需谨慎，flash 可接但验收从严）/ 建议 GLM 亲自或强督导
- **执行侧（PC 侧卡，文件域=D:\Godot\project\vit-daw-frontend，仓库外）**
- 现状快照（2026-09-26 决策侧备份时点）：分支 `codex/auto-mix-session-entry`（HEAD 4f631a8，GD-TELEMETRY-BELL-1）；32 项未提交改动（M 含 telemetry_manager.gd/GlobalSettingsModal/start_page 等 + D 一批 archive 大文件）；15 项 untracked（12 个手测工程目录 + 3 个新 probe 脚本）；未提交面已备份至 `D:\Vit_DAW_backup\`（patch+status+probes）
- 目标：
  1. **基线厘清**：逐项判定 32 项未提交改动的归属（哪张卡/哪次手测的产物；archive 大文件删除是否=有意的剥离动作）——产出归属清单；该提交的按代码线分批提交（提交消息带卡 ID），运行时/临时面不动不提交
  2. **双端对齐**：mac 前端仓 `port/leg5-telemetry-probe`（95e362f，telemetry_manager.gd +83 纯增量：BELL 键集常量+标注调用+三段语义函数）并入 PC 前端主线。**注意**：PC 仓 telemetry_manager.gd 本身在未提交 M 清单里（+18 行 WIP），先完成第 1 步归属厘清再叠加，避免混层；mac 侧另有 PC 剥离的两块已提交功能（L3READY 对账+B4A same_material）——对齐目标是**两端功能并集**，禁止任一侧整覆盖
  3. **（D-FE-REMOTE 批准后）**：github ZTY-vit-daw org 下建前端私有仓，推送全部已提交历史+整理后主线；此后前端仓与主仓同等的 push 纪律
- 约束：前端仓工作树已有改动是权威输入禁止丢弃；PC/mac 两端前端仓的差异以"功能并集"为对齐目标，逐块判定不整覆盖；token/key 零入提交
- 验收：归属清单 + 分批提交（git show --stat 核对）+ leg5 增量落地加载零错（godot headless check）+（如批准）远程建立并首推成功
- 停止条件：未提交改动归属无法判定（找不到对应卡/记录）→ 该项挂起列表上交决策侧，其余照常推进
