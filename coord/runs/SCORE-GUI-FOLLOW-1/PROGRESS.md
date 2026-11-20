# SCORE-GUI-FOLLOW-1 PROGRESS (scorefollow398)

- [S0] 入场检查完成，无前序断点（全新开工）。下一步：读 397 产物 + scoreworkbench 域 + 441 参考卡。
- [S1] 入场阅读完成：397卡(done/2026-11-17-SCORE-BINDING-SAFE-1.md, PASS c6991e18→aa3e9b47)、ScoreTrackWorkbench/binding/kernel/panel、397测试模式、render-smoke、DERIVATION_RULES、select_track(harness local, RiskDirect, tool id=select_track, 回执ui_action=focus_track+track_id)、selected_track读链(uiSelectedTrack: selected_track_id→tracks解析)。397遗留边界测试"catalog refresh does not follow GUI selection (398 boundary)"=本卡要替换的行为。
- [S2] 领取完成：卡面领取节已提交 main 36a0bc1d（仅本卡coord文件）；worktree D:/Vit_DAW_wt_pc398follow 分支 pc398-follow @ 36a0bc1d，干净。下一步：S3 实现。
- 设计定案（S3 依据）：边沿触发跟随（GUI selected_track 变化才跟随，非电平轮询）——天然防轮询抢回；follow 路由走 choose() 守卫（busy 拒/defer、dirty→pending 对话框不旁路）；谱面选轨 commit 后 invoke select_track(track_id) source=ask_vit_webui_score，回执 track_id 核对+awaitConfirm 等 GUI 状态确认（3轮未确认显 notice）；跨工程重置 follow 簿记；clip 提示须 targetFor 守卫（属当前轨且 MIDI）。
- [S3] 生产改动完成（恰好3文件）：新 selection.ts（边沿触发跟随状态机：跨工程重置/空值保留边沿/awaitConfirm工程戳/busy defer不消费边沿）、binding.ts +followTargetFor（clip须属当前轨且MIDI守卫）、ScoreTrackWorkbench.tsx 装配（selectTrack 回写 select_track+回执核对+awaitConfirm、evaluateFollow 接入 readCatalog receipt-guard 内、commitChoice 拆分、data-score-gui-follow）。测试：ScoreTrackWorkbench.test.tsx 重写397边界测试为7条398用例、新 selection.test.ts。下一步：S4 跑 vitest。
- [长命令登记] S4前置：cd D:/Vit_DAW_wt_pc398follow/agent/webui && npm install（worktree 无 node_modules，环境依赖安装，非产品改动）。
- [长命令登记] S4：cd D:/Vit_DAW_wt_pc398follow/agent/webui && npm run test（全量 vitest；scoreworkbench 子集先跑已 101/101 PASS，含 7 条 398 组件用例 + 9 条状态机用例）。
- [S4] 全量 vitest PASS：68 文件 837 测试，exit 0（含 398 新反例：跟随一次/dirty 待切防抢回/busy defer/cross-project 重置/clip 守卫/select 失败诚实/超时 notice）。下一步：S5 build。
- [长命令登记] S5：cd D:/Vit_DAW_wt_pc398follow/agent/webui && npm run build
- [S5] build exit 0（tsc --noEmit + vite build；修复1处 TS 回调收窄）。scoreworkbench 子集复跑仍全绿。下一步：S6 render smoke 扩展+两门。
- [长命令登记] S6a：node src/scoreworkbench/render-smoke.mjs gui_follow_1（398 扩展 7 断言：select_track 回执链/GUI跟随一次+clip绑定/pending→confirmed/dirty 询问不强制/取消后不抢回/新边沿仍询问/截图）。经3轮修复（locator歧义、补 write-ready 等待、failB 复位）后 PASS 18断言 exit0。
- [长命令登记] S6b：npm run test:render（gallery 全面）exit0，全表面 PASS。
- [长命令登记] S4b/S5b 终态复跑：npm run test + npm run build（最终代码态取exit码）。
- [S6] render 两门 PASS：scoreworkbench render-smoke gui_follow_1 18断言 exit0（11 397 + 7 398 DOM 断言：select_track 回执链/GUI跟随一次+clip绑定/pending→confirmed/dirty询问不强制/取消后不抢回/新边沿仍询问）；gallery npm run test:render exit0 全表面 PASS。
- [S4b/S5b] 终态复跑：npm run test 68文件837/837 exit0；npm run build exit0。
- [S7] 分支 pc398-follow 提交 31eff44e（代码+测试）+ 5bd5e3c8（证据 gui_follow_1）→ 合入 main merge 56ea1732（442 e58e133a 并行推进无冲突）。卡面回执已写、卡移 done/。ROSTER scorefollow398 行更新。
- 终态：DONE（零栈交付；真栈 Godot 选择装配腿留 lead 排队）。
