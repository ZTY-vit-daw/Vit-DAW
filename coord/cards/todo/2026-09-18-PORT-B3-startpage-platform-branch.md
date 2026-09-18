# PORT-B3：start_page.gd 平台分支 + python 桥 mac 剔除（B6 裁定落地）

- 优先级 / 预估 / 依赖：P1 / 1 天 / 依赖 B6 已采纳（退役结论 + R1 前置检查）、B1 已验收（mac 二进制命名事实：内核=无后缀 `VitApp`、agent=无后缀 `vitagent`、扩展=lib 前缀 .dylib）
- 模型分级：L2 / GLM-5.3（Godot 启动链语义 + 平台分支设计）
- 目标：Godot 工程（`D:\Godot\project\vit-daw-frontend`，仓外）`app/startup/start_page.gd` 的 mac 平台分支：
  - **前置检查（B6 R1，先做）**：现场读桥拉起逻辑（审计锚点 :355-380，行号可能漂移以现场为准），判定现状=无条件并行拉起 or vitagent 缺失才 fallback；若为无条件并行 → **先改条件分支再剔 mac 臂**（python 桥与 vitagent 同绑 UDP 4445 不能并存，双拉=双绑失败+看护重启循环）
  - **平台分支**：`OS.get_name()`（或等价）分 windows/darwin 臂——内核候选路径（mac：无 `.exe` 的 `VitApp`，路径按 mac 布局：dev 构建目录/运行时目录候选）、agent 候选（`vitagent`）、:116 状态文案（`VspHub.exe` 提法 mac 臂调整）、python 桥段 **mac 臂整段剔除**（B6 裁定：mac 无 python 运行时 mac 化子项，vitagent 为唯一桥接）
  - **PC 臂行为保持不变**（回归验证）
- 文件域：Godot 工程 `app/startup/start_page.gd`（仓外）+ 如需配套小改（常量/路径配置）逐项列明；不改主仓任何文件
- **证据携带（决策侧规定，仓外工程专用）**：Godot 工程若是 git 仓 → 建分支 `port/b3-start-page` 提交，回执报 repo 路径+分支+commit hash，并将 `git format-patch` 产物投 transfer 仓 `inbox-pc/`（带日期）；若非 git 仓 → 改动文件 unified diff 投 transfer。两种方式都必须附 `godot --headless --check-only`（或等价 GDScript 解析检查）输出。**回执同时报告工程是否 git 版控**（决策侧规划 B4/前端登 mac 用）
- 验收标准：①前置检查结论（拉起条件现状 + 处置动作）；②mac 臂 diff（平台分支 + 桥剔除 + 文案）；③PC 回归证据（Windows 臂路径解析/启动行为不变的脚本级验证）；④语法/解析检查 exit 0；⑤证据（patch/diff + 检查输出）已投 transfer
- 停止条件：start_page.gd 现状与 B6/审计锚点严重不符（结构变了/桥逻辑不在预期位置）→ 上交现场证据由决策侧重定卡面；发现桥剔除会破坏 PC 发布链（PC 臂也被迫改动）→ 停止上交（本卡只动 mac 臂）；需改 vitagent/内核侧配合 → 域外上报
- 领取：
- 回执：
- 验收：
