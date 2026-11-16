# CONVERGE-SHELL-1：新壳组件+壳装修+磁带卡落位（收敛阶段 1 / 卡 3，含 366）

- 发卡：主管 2026-11-16 晚（用户裁定 b 案：磁带完整造型随本卡落位——「工程条目本身=磁带盒形态」写进验收标准；权威=[368 施工图](../../runs/UI-CONVERGE-RECON-1/REPORT.md) 节 4.2 锚 3–5+阶段 1 表）
- 池序 387；P1；依赖=**386 合入后**（MessageStream/Composer 可 import）；零栈+真栈烟测腿
- 目标：
  1. 新壳组件 ConvergedLauncher：LauncherSurface 桥面/数据流平移+对话流复用（MessageStream/Composer import，不复用 MainApp 实例）+v5 令牌接入（tokens/components/sprite+v5-design 根类）；
  2. **366 磁带卡落位（用户 2026-11-16 裁定 b 案承接）**：recents 工程条目本身做成磁带盒造型（旧启动页磁带卡血统，366 卡面设计权威），工程名/路径排布进卡结构，氛围色哈希沿用；TapeBadge 小尺寸位保留给场景清单；
  3. App.tsx 303–305 替换为新壳。
- 文件域：新建 `src/launcher/ConvergedLauncher.tsx`+`launcher/converged.css`+App.tsx 替换点；SPEC-V5 §7.3 补条款。
- 验收：test:render 新增 converged fixture 段（DOM 断言：recents/新建/打开/建议/对话流/治理卡+v5 令牌生效+**磁带卡结构三类节点壳/窗/带轮在每行在场**）+vitest 不减+`ui_entry_smoke.ps1` exit 0+**用户目检「条目本身就是磁带盒」**（366 验收口径）。
- 停止条件：磁带卡形态与信息密度冲突（366 卡面预置）→带方案上交。

## 领取/回执/验收（执行侧自验回执，2026-11-16 晚；决策验收待主管）

- **领取**：池序 387；领取基线 ec437bb5=origin/main（386 已合入，领取时工作树零未知 diff）；分支 `port/converge-shell-1`，worktree `D:/Vit_DAW_wt_convshell`；owner=GLM-5.3-Flash 执行子代理（ZCode PC 会话）。
- **执行的命令+退出码**（均在 worktree `agent/webui` 下，除烟测在仓根）：
  1. `npm test` → **698 passed（58 文件）= 基线 698 不变**，exit 0；
  2. `npm run build`（tsc --noEmit && vite build）→ 0 错，exit 0；
  3. `npm run test:render` → **10 段全 PASS，exit 0**（既有 9 段零回退：FOUNDATION/LAUNCHER/PROJECT/LOADING/FIDELITY/SHADOWBOX/REC-TAPE/WIDGETS/APPENTRY；新增第 10 段 CONVERGED，两轮复跑均绿）；
  4. 真栈腿：卡面原点名 `ui_entry_smoke.ps1` → **腿1 FAIL（exit 1）=脚本前提失效取证，非本卡缺陷**（boot/main_window.hidden/health.failed/unavailable 全正常，`main_window.restore(reason=unavailable)` 永不发生——生产前端 d05043e1 `_mark_unavailable`（agent_chat_window.gd L705-714，UI-ENTRY-2 注释在卷）已**退役 unavailable→restore 回退链**；腿2/3 未达；工件 coord/runs/UI-ENTRY-1/run_20261116_211933/）；改跑**现行代 `ui_entry2_smoke.ps1`**（腿1=零 restore 正向断言+React 窗保持；腿3=354 全链超集）→ **4 腿全 PASS，exit_code 0**（run_20261116_212418：failinject/webview2/**autoload 全链**/legacy）。
  5. 真栈栈位协议：占用登记 520d17c7→释放登记 fa65d06e（均推 main 重读确认）；净空/即时复核/TEARDOWN_ALL_ZERO 亲证（Vit 系进程=0+四端口=0）；recents 807226F4AF3F 还原哈希亲证；fixture F6CA36FA 不变。
- **产物路径**：`agent/webui/src/launcher/ConvergedLauncher.tsx`（收敛壳：桥面/数据流平移+segmentThread 文本段→MessageStream、Composer 挂载+TapeCardFace）、`agent/webui/src/launcher/converged.css`（磁带卡+chat 域组件作用域化）、`agent/webui/src/App.tsx`（launcher 分发换挂，其余零改动）、`agent/webui/design/main-window-v5/SPEC-V5.md`（§7.3 增 v5.1.2「启动台工程卡=磁带卡形态」条款+§7.3.1 表两行更新）、`agent/webui/src/gallery/main.tsx`（?surface=converged fixture 页+桥 mock live 样例）、`agent/webui/src/gallery/render-smoke.mjs`（新增 converged 段）；工件 coord/runs/CONVERGE-SHELL-1/{render-smoke-converged-report.json,screens/（整页+recents 特写）}。
- **commits**：cb4b569d（实现）→492adf3e（render 工件）→76bbdb86（HEAD 复验工件）→8b876c65（烟测工件），已推 `origin/port/converge-shell-1`；385/386 血统上 diff 零接触（领取基线后纯增量）。
- **366 落位核对**：recents 工程条目=磁带盒整卡（壳/窗/带轮三类结构节点每行在场——converged 段 DOM 断言 shell×1/window×1/reel×2/底标×1 逐行通过；氛围色=FNV-1a 哈希 computed bg 逐行核对 rgba(c1,0.62)）；工程名/路径入卡结构（底标主标 14/600+副标 13 mono）；**对话流建议卡工程位同形态核对=compact 磁带卡同形态通过**；greeting 位核对=纯文本回执（greetingDisplayText 只出 reply，无工程槽，无需改形）；TapeBadge 小尺寸档保留给 §7.3.1 清单其余位（旧 LauncherSurface 保留至 CLEANUP，其 fixture 面/单测/REC-TAPE 段全绿未动）。
- **桥协议核对**：launcherBridge.ts 零改动（五 type 零新增零改名）；converged 段以 webview mock 实证「打开…」点击经真实桥路径发出 launcher_choose_file（request_id 在卷）；真栈腿3 surface.ack=project（entrySurfaceBridge 链零回归）。
- **渲染面覆盖边界声明（AGENTS §5）**：浏览器级 DOM/对比度/裁切断言+真栈 autoload 全链已在卷；**「条目本身就是磁带盒」的用户目检（366 验收口径）未做**——截图证据 coord/runs/CONVERGE-SHELL-1/screens/，须决策侧安排用户按 AGENTS §5 从 Godot 前端拉起手测；磁带卡视觉血统相似度（带量盘径观感）仅截图在卷，无像素级断言。
- **已知边界（供 CLEANUP 卡）**：render-smoke §12 guard 的 `data-tape-badge` 零残留断言在收敛壳挂载后变平凡通过（收敛壳用 data-tape-card 锚点）；CONVERGED 段已带等价磁带卡断言，CLEANUP 迁移 guard 时对齐。gallery fixture 页不加载 styles.css（345 全局反污染防线不变）→ chat 域组件消费面规则在 converged.css 作用域化补齐（styles.css 零改动）；launcher.css 三处 12px 字（治理注/桥注/已答注）在收敛壳作用域升至 13px（launcher.css 零改动，13px 红线扫描过）。
- **停止条件核对**：磁带卡形态与信息密度无冲突（fixture 3 行名/路径全放下）；桥协议五 type 零破坏。**遗留上交**：ui_entry_smoke.ps1 腿1 旧语义回填或退役 → 请决策侧另开卡裁定（本卡文件域外，未动脚本）。

