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
