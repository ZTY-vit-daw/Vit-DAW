# SCORE-GUI-FOLLOW-1：392-B 两入口选择同步

- 池序398；P1；父392；PC；发卡/验收Codex主管`/root`；用户批准目标，依397验收合入后领取。
- 目标：生产GUI真实选轨变化→谱面跟随；谱面选轨→复用track.select并核对GUI状态，保留dirty待切目标，避免轮询旧选择反复抢回；选clip须属于当前轨且MIDI，跨工程清失效引用。
- 文件域：397新容器/binding/必要selection.ts≤3；不动Godot/server，链路缺口先报告。
- 验收：选择变化/过期缓存/dirty/busy反例+渲染DOM+真实Godot选择装配到agent到谱面及反向回读exit0。POST/ui/context仅限局部消费测试，不能冒充GUI装配验证。
- 真栈须主管单owner排队；领取/回执/验收待回填。

## 领取（2026-11-20 scorefollow398）

- owner=`scorefollow398`（GLM-5.3-Flash 执行子代理，PC）；分支 `pc398-follow`；worktree `D:/Vit_DAW_wt_pc398follow`。
- 领取基线：主仓 main `883a0db9b76028665b4ea5726465d4a409c526b7`；领取时 `git status --short` 有他卡在途改动（CURRENT-STATE.md、VitApp Workspace 设置、各 coord/runs 渲染工件等，非本卡文件域，不碰）；`git diff --stat` 与本卡相关文件域（agent/webui/src/scoreworkbench/、coord/cards/todo/2026-11-17-SCORE-GUI-FOLLOW-1.md）为空，即本卡领取前无未提交叠加。
- 生产文件域承诺（≤3）：`agent/webui/src/scoreworkbench/ScoreTrackWorkbench.tsx`、`binding.ts`、新 `selection.ts`；测试可加（397 先例）：`ScoreTrackWorkbench.test.tsx`、新 `selection.test.ts`、`render-smoke.mjs`（渲染面 DOM 断言扩展，398 证据目录改写为本卡 runs 目录）。不动 Godot/server/kernel.ts/panel。
- 边界声明：选择同步读源=「/agent/ui/state 的 resolved selected_track 行（回退 ui_context.selected_track_id 读序，同 daw/parse 先例）」；ui_context 注入仅用于零栈局部消费测试，不冒充 GUI 装配验证。真栈 Godot 选择装配→agent→谱面及反向回读腿留 lead 排队（本卡不启真栈）。
- 零栈验收门：`cd agent/webui && npm run test` / `npm run build` / `npm run test:render` 三门 exit 0（本卡新增反例+DOM 断言在卷）。

## 执行侧回执（2026-11-21 scorefollow398，零栈交付，done 待验收）

- 合入 main：分支 pc398-follow 两提交（31eff44e 代码+测试 / 5bd5e3c8 证据）→ merge 56ea1732；领取基线 36a0bc1d，领取时本域无叠加 diff（442 e58e133a 并行推进 scripts 域无相交）。
- 三门（终态复跑，cd agent/webui）：`npm run test` 68 文件 837/837 exit 0；`npm run build` exit 0；`npm run test:render` exit 0（gallery 全表面 PASS）+ `node src/scoreworkbench/render-smoke.mjs gui_follow_1` PASS 18 断言（397 原 11 + 398 新 7）。证据 `coord/runs/SCORE-GUI-FOLLOW-1/gui_follow_1/`。
- 实现要点（生产恰 3 文件：ScoreTrackWorkbench.tsx / binding.ts / 新 selection.ts）：跟随=**边沿触发**（GUI selected_track 变化恰一次消费，lastSeenTrackID），dirty 待切目标走既有 pending 对话框不旁路、取消后同 GUI 值不重推=**防轮询抢回**；busy 时 defer 不丢边沿（结束后仍跟随）；谱面 commit 后经 /agent/invoke 复用 `select_track`（source=ask_vit_webui_score）+回执 track_id 核对+awaitConfirm 工程戳等 GUI 回声（连续 3 轮未确认显 notice）；跨工程整体重置簿记（await/lastSeen 全清=失效引用清理）；clip 提示经 followTargetFor 守卫（须属当前轨且 MIDI，否则回退单 MIDI clip 自动装载/多 clip 显式选择）。
- 测试新增：ScoreTrackWorkbench.test.tsx 重写 397「不跟随」边界为 7 条 398 用例（跟随一次+回写核对/dirty 防抢回/busy defer/跨工程重置/clip 守卫/select 失败诚实/超时 notice）+ selection.test.ts 9 条状态机用例；397 既有 11 反例未弱化全保留。
- 真栈腿边界：**零栈交付**；真实 Godot 选轨装配→agent→谱面及反向回读留 lead 单 owner 排队（本卡未启真栈）；ui_context 注入仅零栈局部消费测试，不冒充 GUI 装配验证。
- 遗留：①读序回退 ui_context.selected_track_id 的真实披露形态待真栈核对；②worktree 内 gallery run 副产物（UI-*/CONVERGE-* 工件刷新、CONVERGE-ENTRY-1 未跟踪新件）非本卡产物，未提交；③停卡报告：无链路缺口（select_track 已有 harness 生产缝，未新增协议/端点）。
