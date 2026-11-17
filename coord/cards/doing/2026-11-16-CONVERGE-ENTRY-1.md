# CONVERGE-ENTRY-1：工程管理层挂载+进出语义（收敛阶段 1 / 卡 4）

- 发卡：主管 2026-11-16 晚（权威=[368 施工图](../../runs/UI-CONVERGE-RECON-1/REPORT.md) 阶段 1 表+节 7 双轨）
- 池序 388；P1；依赖=**387 合入后**；真栈烟测腿
- 目标：recents 抽屉/新建/打开进新壳（387 未含全量则本卡收尾）+「回到启动页」交互（setSurface 内部切换）+宿主 vit_surface 双向对齐登记。
- **补充（2026-11-17 凌晨裁定档 §三）**：+settings 按钮**左下角**（启动台+工程面两态都放；入口即可，设置面板本体如无既有面则占位跳转/最小面板——用户裁定「主流 agent 窗布局」）。
- 文件域：`src/launcher/`+App.tsx（分发逻辑不改，必要时白名单）；Godot 侧收编点 A（`_AGENT_WEBUI_URL`→`_AGENT_WEBUI_URL_LAUNCHER` 1 行）**另微改卡**（前端仓域，本卡只登记）。
- 验收：桥协议五 type 零破坏断言（launcherBridge.test.ts 平移不删）+`ui_entry2_smoke.ps1` exit 0+vit_surface→project→launcher 往返烟测。
- 备注：383（列表可点击加载）若已在本卡前合入，其断言随迁新壳（领取时核对 383 状态防双做）。

## 领取/回执/验收：待回填

## 2026-11-17 执行授权与领取

- 主管/验收：Codex PC `/root`；派发已确认（用户批准今日顺序并明确开工）。
- owner：`/root/entry388`（GPT-6.1 Sol/high，PC）；分支`codex/converge-entry-388`；worktree=`C:/Users/timoz/.codex/worktrees/converge-entry-388/Vit_DAW`。
- 领取基线：HEAD/origin-main=`116f22155b1ba52c5c6e37076799791a3d1ee1a7`；status与diff均为空。正式实现以本领取提交成功push共享main并核对远端owner为准。
- 文件域补齐（主管授权）：原launcher/App域外，允许`src/project/ProjectSurface.tsx`及`project.css`完成两态settings/返回入口；最多5生产实现文件，必要测试与现有render-smoke断言可改。禁止改agent后端、VitShell原生层、Godot场景、全局styles.css和389退役面；需扩域先报告。
- 执行步骤：先锚定现有桥与返回语义（不能仅切React状态假装工程已退出），列文件清单→实现→局部/全量前端测试build/render→写真实栈RUNBOOK和所需ps1扩展建议→报告主管。零栈段不启动VitApp/agent/Godot/VitShell；依赖node_modules自行npm ci，不与其他worktree共享可写目录。
- 真栈：主管另行放行并统一登记owner，和394/396错峰；现有ui_entry2/vitshell体系扩展，记录退出码/run/构建哈希/recents还原，覆盖project→launcher往返。271验证另独立场景，不用本卡UI腿冒充。
- 既有视觉定稿优先；遵守frontend-design技能的可访问性/状态诚实/动效克制，不重做品牌样式。禁止computer-use/主机GUI控制，浏览器级headless DOM断言按既有脚本。
- 停止：无可靠返回宿主语义/要求关闭工程但内核无命令/越域缺陷→上交证据；不得借更改治理判据凑通过。

### 返回启动页语义裁定（2026-11-17）
主管核对原施工图224与生产桥：VitShell仅在Godot退出后通知launcher，没有React主动关闭工程入口。本卡按原目标setSurface内部切换=查看启动页/工程管理视图，当前工程保持打开，不宣称退出或解绑；有活跃工程时明确显示并可返回工程；新建/打开继续原治理。通过既有postEntrySurfaceAck登记视图，宿主后续vit_surface仍权威。此裁定满足不假装工程退出的约束，零内核/宿主新关闭能力。

### 管理页数据接缝（2026-11-17）
401 LAUNCHER-RECENTS-READ-1为本卡必要依赖，GET全量列表不经LLM。当前工程管理视图禁用自由composer/新问候误称独立启动卷，提供返回工程对话；recents真实读取不退化为快照空态。401同owner先独立提交（3生产文件），388后提交（原5文件），依赖序验收。401前388已有5文件骨架不丢弃。
