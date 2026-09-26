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
- 领取：2026-09-26 16:50 +0800 / 主仓 main=191aabff（决策会话领取并亲自执行，L2）/ 前端仓领取态：codex/auto-mix-session-entry@4f631a8 + 32 项未提交 + 15 untracked
- 回执（2026-09-26 决策会话执行+自验）：
  - **目标 1 归属厘清+归档**：32 项已跟踪改动逐一判定→**四笔主题归档提交**：da11611（PCA 认证入口 UI +331，契约 PROCESSOR_CERTIFICATION_ENTRY_V1 前端实现）/718e83c（Save As Project Folder +739，ProjectSavePhase 八态+保存状态栏+菜单项）/17e4304（启动链编排 +212，kernel 重试+vsp hub/bridge 拉起）/4aa6356（3 个诊断 probe 入库）；**挂起三项**：telemetry_manager.gd 18 行 WIP（BELL 卡回执声明的 Save As 主题留区，与 mac 已提交 B4A same_material 同主题——待两端字节核对后定提交形态，不擅动）、LLM_Chat_Controller.gd（--ignore-all-space 零差异=纯行尾，对齐过程中归一，无实质内容损失，patch 备份在案）、archive/ 9 文件删除（该目录全部 tracked 内容，归因不明——**挂起上交用户裁定：有意清理 or 误删**）；12 个 untracked 手测工程目录（≈1.9G）+ .vit_agent/.vit_derived 留工作区不动。
  - **目标 2 leg5 判定**：PC BELL 提交 4f631a8（+83：键集常量×2+_annotate 调用+标注函数族）与 mac leg5 95e362f 同构同 +83——**内容已覆盖无需再并入**（blob c1cdefc≠投递件 eb93e210 为基线形态差异，三块锚点结构逐项核对一致）；双端并集剩余两块（L3READY 入口对账+B4A same_material，mac 有 PC 无）为 mac→PC 搬运，**待 mac 侧推送本远程后拉取补齐**（转交腿）。
  - **目标 3 远程建立+首推**：`github.com/ZTY-vit-daw/vit-daw-frontend`（private）建仓（用户操作）→ 首推被 GitHub 拒：**textures/spectrum_data.exr 226.96MB 超 100MB 硬限**（唯一超限 blob；Vit_Mountain.gdshader.tscn 产品引用中、无内核生产者=不可简单再生）→ **git lfs migrate --everything**（本地干净副本 D:/Vit_DAW_backup/fe-migrate-tmp 执行，原仓工作树全程未动；migrate 直跑被 dirty 检查拒因 untracked 计入）：26 提交重写、**全部 commit hash 变更**（旧历史完整保全于 vit-daw-frontend-midterm-2026-09-26.bundle，与旧记录不可比——同主仓 2026-09-16 历史清洗先例）；**首推成功**：3 分支（main/codex/auto-mix-session-entry/port/b3-start-page）+1 tag，LFS 对象 238MB 上传，与 origin 同步 0/0；原仓对齐后 exr 经 git lfs checkout smudge 恢复 227M 实体（EXR 头核验）；main 分支正规化（旧 main 停在 5 月 eb59666，纯快进至 HEAD 零改写）。
  - §10 核对：原仓未提交改动全程保全（stash push/pop 往返+最终 status 与领取态一致除行尾归一项）；stash/pop 与 patch 双备份在案。
- 验收：**pass（2026-09-26 决策会话，PC 侧闭环；执行=验收同会话 L2）**——两遗留项不阻塞：①archive 删除归因待用户 ②双端并集两块待 mac 推送（建议 mac 侧下一会话将 mac 前端仓推同远程完成汇合）
