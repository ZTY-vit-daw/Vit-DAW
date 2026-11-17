# SCORE-GUI-FOLLOW-1：392-B 两入口选择同步

- 池序398；P1；父392；PC；发卡/验收Codex主管`/root`；用户批准目标，依397验收合入后领取。
- 目标：生产GUI真实选轨变化→谱面跟随；谱面选轨→复用track.select并核对GUI状态，保留dirty待切目标，避免轮询旧选择反复抢回；选clip须属于当前轨且MIDI，跨工程清失效引用。
- 文件域：397新容器/binding/必要selection.ts≤3；不动Godot/server，链路缺口先报告。
- 验收：选择变化/过期缓存/dirty/busy反例+渲染DOM+真实Godot选择装配到agent到谱面及反向回读exit0。POST/ui/context仅限局部消费测试，不能冒充GUI装配验证。
- 真栈须主管单owner排队；领取/回执/验收待回填。
