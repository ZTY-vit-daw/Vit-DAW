# FIX-MASTERVOL-CALIBRATION-1：工程模板 master volume 默认值 +6dB（fader 位置 1.0）修复与存量工程策略

- 发卡：12:00-18:00 决策流 / 2026-10-18 午窗（依据=KERNEL-RENDERGAIN-1(132) 取证腿 pass + 决策侧 P1 候选升级裁定，[BATCH3-rulings](../rulings/2026-10-18-BATCH3-rulings.md) §一）
- 派发确认：**待用户确认**（P1 候选修复；建议下一可用窗口派发，可与 118/其它非栈卡并行实现，真栈验证腿排用户手测之后）
- 验收负责人：GLM 主管决策流（或当班决策流）
- 池序 134；目标仓库=D:\Vit_DAW；**P1** / 预估 0.5-1 天 / 无代码依赖（证据在 132 取证分支）
- 模型分级：L2 / GLM 执行会话（内核 C++ 模板/迁移 + 回归）
- 领取（2026-10-18 午窗）：2026-10-18 14:03 +0800 / origin/main=aec47ad8 / owner=**DSH PC 执行流 exec-mastervol**（用户 2026-10-18 午窗选择派发 FIX-MASTERVOL+MEM-SCHEMA）/ 分支 **port/fix-mastervol-calibration-1** / worktree **D:/Vit_DAW_wt_mastervol1** / 领取提交=（本提交）。**决策点代理裁定（决策流裁定，用户可覆写）**：①模板目标值=**-3dB（fader 位置 0.6376281516）**，对齐引擎 Edit::Options defaultMasterVolumedB=-3 与 scripts/gen_vit_project.py；②存量工程=**不静默迁移**（只修模板/新建工程；存量保持原值+文档登记差异，迁移候选另评）；③master UI/命令控制面不在本卡。执行口径=先落模板修正+回归测试（kernel Release 构建/ctest+比值/绝对电平断言；不占栈），真栈验证腿等用户 92 手测后（PC-RUNTIME-STACK 排队条）。协作模式（DSH 团队）：协调面由 12-18 决策流单写，实现面由执行流在独立 worktree 写。

## 缺陷（132 真栈取证，证据在卷）

- 注入点=**工程 master volume 节点**（Edit 级 VolumeAndPanPlugin）状态 fader 位置 `1.0`；pin 引擎 `tracktion_AudioUtilities.cpp:136-156`：`volumeFaderPositionToDB(1.0)=+6.000000 dB`，`0.7408182=0 dB`，`0.6376282=-3 dB`。
- 三处模板硬编码 `volume="1.0"`：`VitApp/Source/.../VitPaths.h:25-29`（默认模板）、`:73-77`（blank 模板，new_project 用）、仓库 `VitApp/Workspace/default_project.xml:14-18`（headless 启动加载）；与 `scripts/gen_vit_project.py:174` 的 `0.6376281380653381`（-3dB）期望不一致。历史 blame=初始提交 `991cffb6`（2026-04-22），非近期引入。
- 真栈实测（132 run_20261018_135710，10/10 腿）：E2 use_master_plugins A/B=1.995181 vs 0.999959；E3 MASTERVOLUME fixture 五值映射精确对应；E4 new_project +6；E5 16/24/32bit 恒定、16/24bit 削波 clamp vs 32bit float 不 clamp；E6 幅度扫描恒定；E7 44.1k/mono/左独占逐声道一致；E8 `list_tracks` master `volume_db=null`+无 master 命令（用户不可见不可控）。
- 影响：所有新建/存量工程 master 总线恒 +6dB 过 unity；16/24bit 导出削波提前 6dB；跨工程来源电平不一致（new_project +6 vs gen_vit_project -3 vs 无 MASTERVOLUME -3）。比值类烟测免疫；绝对幅度面（L2 render probe 阈值、绝对定标断言等）受偏移，清单见 132 RESULTS §0③。

## 决策点（执行前先上交建议，获裁定后落代码）

1. **模板目标值**：0 dB unity（`0.7408182206817179`）还是 -3 dB（`0.6376281516`，与 gen_vit_project.py 一致）？建议 0 dB unity 并把 gen_vit_project 对齐；-3dB 会改变既有生成脚本语义，需一并裁定。
2. **存量工程策略**：旧 XML `volume="1.0"` 是「陈旧默认」还是「用户有意 +6dB」不可区分——迁移（旧线性 1.0→fader 0dB？有误伤风险）or 保留现状+仅新模板修正+文档说明？建议：默认**只修模板/新工程**，存量迁移另裁（风险与收益写清），不得静默改写用户工程。
3. **UI/命令面**：master volume 不可见不可控属独立缺口——不在本卡（可另立候选卡）。

## 文件域

- `VitApp/Source/.../VitPaths.h`（模板字符串）；`VitApp/Workspace/default_project.xml`；`scripts/gen_vit_project.py`（若裁定对齐）；模板/kernel 相关单测；`coord/runs/FIX-MASTERVOL-CALIBRATION-1/` 工件。
- **不改** te::Renderer/写入器/PCA/typed executor/前端；迁移若需改 `OldEditConversion` 等引擎迁移面 → 先上报另裁；越域即停。

## 验收标准

1. 新工程（`new_project` blank 模板/默认模板）master 渲染比≈1.0（0dB）或按裁定目标值；16/24bit 削波边界断言更新；比值类烟测全绿。
2. 单测/构建：`go build ./...`（若涉 agent 面）+ kernel Release 构建 exit 0；`ctest -C Release` 相关组全过（若加 C++ 测试）。
3. 真栈（AGENTS §5）：`scripts/dev_agent_smoke.ps1 -StartKernel` 体系扩展现有场景或同模式新增腿，断言 new_project 渲染比≈1.0 + 16bit 不提前削波；run ID/工件/HEAD/开关入回执。
4. 文档：语义说明（fader 位置 vs dB）+ 存量工程策略 + 已知边界；回执显式声明端测边界。
5. 回执含：决策点建议与裁定引用、diff、构建/测试 exit、真栈 run ID/工件、`git status --short`。

## 停止条件

- 决策点未获裁定前不落代码（先上交建议）；
- 存量迁移需要改引擎语义/有改写用户工程风险 → 停，另裁；
- 不得用比值断言掩盖绝对电平问题；不得把 UI master 控制扩入本卡；
- 真栈环境中断按 §8 记录，不把下游断言失败当功能结论。

## 关联

- 证据卡：KERNEL-RENDERGAIN-1(132，done，分支 port/kernel-rendergain-1@e3b4ac32）
- 触发：KERNEL-SENDROUTE-IMPL-1 验收域外发现（2026-10-17）
- 决策点裁定后回填卡面。
