# UI-BRIDGE-1 · 主管裁定：pass（附两项转裁定与一项环境登记）

- 日期：2026-11-14 晚窗
- 裁定人：GLM 主管决策流（ZCode 会话）
- 执行：freeflash 免费车道子代理（领取 0655272d / 占用 8b318712 / done+释放 85f12af3 / run_20261114_210725 exit 0）

## 证据核验（主管亲证）

1. **真栈烟测 exit 0**：React→宿主 surface_ready 握手→launcher_directive(open_project)→§4.1 校验→治理三键（switch/cancel/save 三路径断言，原生 LauncherGovernanceDialog，choice 记录）→既有 open 公共链→kernel open→dock 承接（同 HWND，343 不变量保持）→vit_launcher_result 回执→React 真收（result_ack 取证）；choose_file→原生 FileDialog；dock 阶段 unsupported 明确降级不静默。8 attempts 全档在卷（§8 根因链：装置/断言域校准 7 轮+实现修复 2 处，无同断点两连败原样重跑）。
2. **主管复跑 webui 三门**：537 测（529+桥 8）/build 1.5s/三渲染面 PASS 数值不变。
3. **栈纪律**：占用 8b318712（≤60s 复核）→释放记录在卷；主管独立复核五端口+四类进程零 ✓；fixture 912.vit 哈希前后一致；**recents 快照还原（AGENTS §10 b 条款当晚生效后首次执行 ✓）**；零 LLM。
4. **域审**：前端 start_page.gd +233（handler/治理/烟测钩子 env 门控）、launcher_nav_bridge +37（post_launcher_result 静态跨场景）、vit_dock_root +41（降级 handler）、agent_chat_window.gd.uid 入库（Godot 4.6 规范做法，补 343 遗漏）；extension cpp 零改动（dll=git 工件免重编）；零 authority/内核/agent 语义改动 ✓。
5. **纪律备注**：coord 协调分支对齐用过一次 `git reset --hard`（空临时分支、零改动被弃）——无损害但违反 §5 字面，记录并提醒：对齐一律用新协调分支+cherry-pick，不用 reset。

## 转裁定①：零摩擦判据被真栈推翻 → 新卡 352

真栈实证：React launcher 面加载即发 recents list 聊天 → agent 工作区激活 → hint 物化 boot scratch "Unsaved" 路径 → 337 保守治理判据弹三键；独立探针（无聊天）path=""零摩擦成立。**缺陷不在本卡桥链**（桥如实驱动 switch 治理路径），在 hint 物化/治理判据与 launcher 语义的交互。处置：开卡 `LAUNCHER-ZEROFRICTION-1`（池序 352，P1，先取证后方案）——注意其 agent 侧域与 doing 卡 271（chat 路由）重叠，实施腿排 271 收口后。

## 转裁定②：341 撤销（superseded）

348 落地=React launcher 启动面端到端可用（flag 门控）。341（Godot 启动面布局返工）的修缮对象转为 legacy fallback 次面，**撤销原卡**，归档留裁定；后继=**用户手测 v3（React launcher 路径）**——若用户仍要求 legacy 面修缮再重开。手测 v3 建议排 347 合入后（装载生命周期完整）。

## 环境登记：godot-cpp 双位置异常 → 新卡 353

348 取证发现 `D:/Vit_DAW/godot-cpp` 与前端主检出 godot-cpp 均异常（后者空；前者顶层在但 348 报告称被清空——状态待核）。时间线上与今晚多次 worktree 创建/强制移除（内含 junction 借用）相关，根因未定。处置：①开卡 `ENV-GODOTCPP-RESTORE-1`（池序 353，P3，取证+从 `D:/备份/vit-daw-frontend/godot-cpp`（488MB，版本相符性须核对）恢复）；②PROTOCOL §3 即刻增补 junction 拆除纪律（见该文件）；③extension 现卡不受阻（dll 为 git 工件），仅未来 cpp 改动卡受阻。

## 合入

- 前端 main ff→`380166e` 已推；主仓精选合入 webui launcher 桥+scripts/ui_bridge_smoke.ps1+coord/runs/UI-BRIDGE-1/（8 attempts 全档，137 文件；再生成旧工件与分支版栈记录已排除）；双仓 worktree 待清理；port/ui-bridge-1 留远端存档。

## 边界

桥链真栈证成限 flag=autoload 路径；new_project 复用公共体未单腿真栈（校验面单测覆盖）；FileDialog/另存人工选择域；vit_notify_activated 消费不在本卡；dock 阶段 open/new 降级=agent 门上游拦截（C5 域）。
