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
