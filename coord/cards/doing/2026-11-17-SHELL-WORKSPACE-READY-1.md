# SHELL-WORKSPACE-READY-1：装配与首份工程快照的真实完成回执

- 池序402；P1；391/395必要接缝；PC；主管发卡/验收`/root`；用户批准真实装载反馈与双窗行为，本卡按394取证拆出，不新增产品方向。
- 证据：start_page.gd成功回执在change_scene_to_packed返回OK后，早于dock `_ready`/deferred refresh；dock `_apply_kernel_project_reply_and_emit`成功apply后project_loaded为真实数据装配点。旧SHELL-2卷已承认“dock内失败宿主不感知”。
- 目标：仅VIT_SHELL_DIRECT新工作台路径，宿主launch-result成功改为生产dock已装配且首份kernel project snapshot成功写入repo后发送，stage=`workspace_ready`，不声称插件/分析后台全部就绪。场景加载失败/首份快照失败/有界超时回failed，不发假opened。旧legacy路径保留明确scene_accepted边界。
- 实现域≤3：前端仓`app/startup/start_page.gd`、`vit_dock/scenes/vit_dock_root.gd`、必要新`app/shell/vitshell_launch_readiness.gd`（跨场景root托管一次性回执/有界timer）；不动394CPP。401/388主仓域不交；396 agent_chat_window域不交。
- 协议：现有launch-result add可选stage；394已准备workspace_ready/无字段scene_accepted兼容；真实线程/进程观察字段不改。幂等与关联launch在设计时锚定，不能复用上轮延迟回执当成功。
- 验收：Godot解析/装配单元探针+成功/场景失败/快照失败/超时/legacy语义配对；真正vitshell_full_smoke三件套断言生产ready日志先于opened且stage吻合、失败不opened+焦点锁释放，exit0；真栈与388/394/396串行，未登记不得启栈。
- 测试卫生：原fixture复制，recents启动前快照、finally还原且哈希亲证，独立run目录；跑前写次数/失败分类/预算/止损。
- 停止：需内核改动/大规模场景重构/无法可靠关联本次launch则先报锚点；无GUI/computer-use。
- 领取/回执/验收待回填。

## 领取（2026-11-17）

- owner=/root/window391（Sol/xhigh，PC），394零栈完成后接力；不承担自身最终验收。
- 分支codex/shell-workspace-ready-402；worktree=C:/Users/timoz/.codex/worktrees/godot-entry-396/vit-daw-frontend；HEAD=e9006de7（394A/B待真栈，不当最终PASS），初始status/diff空。
- 主仓基线e416de6e；本领取提交成功推main核对owner后可实现；仅gd≤3生产文件，CPP不动，真实栈未放行。

## 402-B 真栈失败注入接缝交接（2026-11-17）

- 原owner/root/window391已提交402 b1f94066与394-C/D并保留干净组合树；主管核HEAD0017bba5、status空（396另已cherry-pick）。本卡后续生产/验收owner显式转/root/entry388（Sol/high，PC），原owner继续395，不再改402域。
- worktree=C:/Users/timoz/.codex/worktrees/godot-entry-396/vit-daw-frontend，分支codex/shell-workspace-ready-402；只现3gd领域加白名单VIT_SMOKE_SHELL_READINESS_SCENARIO门控：scene_failed真实场景加载失败、snapshot_failed真实首快照拒绝、delay_snapshot延迟真实应用供ready/timeout、exit_before_ready实际退出。默认空零行为，子进程env注入/清理，真实触发日志含launch_id/instance，禁止主仓伪造POST失败替代真实Godot链。
- 402-B独立commit；391验收装置在既有vitshell_full_smoke.ps1/cdp.mjs/run_entry_management_acceptance.ps1三脚本独立commit，原owner/main已无未提交实现不叠未知diff。
- 矩阵跑前成文：成功/场景失败/快照失败/确定超时/退出各一腿；错误/上轮nonce在真实延迟成功窗配对（rejected、不结束本轮，后实际ready）；无百分比/假文件音频步骤。最终组合主旅程、395真实消费、只读焦点/z/guard取证，后续Godot用户手测仍单列。
- 当前栈271 owner，402-B只能静态准备，禁止Godot解析/import/服务/端口操作；主管后续统一放行组合runtime。
