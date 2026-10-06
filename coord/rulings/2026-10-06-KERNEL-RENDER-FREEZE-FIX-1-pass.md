# Ruling：KERNEL-RENDER-FREEZE-FIX-1 — pass（2026-10-06 决策侧验收）

- 卡：[cards/done/2026-10-06-KERNEL-RENDER-FREEZE-FIX-1.md](../cards/done/2026-10-06-KERNEL-RENDER-FREEZE-FIX-1.md)；实现 commit `4984ef66`（port/kernel-render-freeze-fix-1，基线 97af10a9）
- 判定：**pass**，合入 main（cherry-pick，见卡面验收行）。KERNEL-RENDER-FREEZE-1 定位的 ABBA 死锁（K1）就此修复关闭。

## 决策侧亲核证据

1. **diff 审查**（97af10a9..4984ef66，4 文件 +616/−17）：
   - 腿 A `retireRenderHandleOffThread`（detached 线程持 Handle 析构，join 离消息线程）——完成回调 4 处 reset 全改 retire（:696 原位 + **dual-tap :831/:871/:909 三处同型违规一并修**，属卡面"消息线程零 Handle 析构"不变式内的合理扩展，申报合规）；`releaseWedgedRenderHandle` erase 改 move-out+retire（迟到回调路径）。
   - 腿 A' 看门狗 detached 定时线程：`renderWatchdogSignalled.exchange` exactly-once 门 + superseded 竞态检查（deadline 不匹配/rendering false 退出），timer 路径 force-clear 职责保留兼容——设计正确。
   - 腿 B `editHasRenderableAudioContent` 预检偏向"有内容"（false negative 阻塞可渲染工程比 false positive 更糟，后者已被腿 A 变成干净异步失败）——保守权衡正确；ASCII 消息（CP936 编码约束）申报合理。
2. **烟测工件亲读**（run smoke_render_freeze_1/2 双轮 exit 0）：腿 B 同步拒绝回执（kernel_error+no_renderable_audio_content 消息）亲读；腿 A 死锁反转主断言亲读——不可写目标盘同步 "Render started" → render bind 探针报 `status "failed"` 终态 + 命令面活（旧内核此序列物理上只能冻结超时，不可能产生这些回执=功能自证）；live-cancel ok+任一终态（竞态边界不设门，诚实）；空范围 render_done 健康回归；**验证方法学认可**：发现空范围不能触发失败（checkNodesForAudio 全工程判定，附加发现已记 manifest）后改用不可写目标盘做零竞态确定性失败触发器——正确且聪明。
3. **被测产物核对**：内核 sha256 `b6565dcf…3123b` 与回执/manifest 逐字一致；烟测时 git_status.txt 含全部实现文件 M 态（构建自含改动工作树）。
4. **我复跑**：`go build + go test ./... -count=1` 88 包 0 FAIL exit 0；`run_ab_result_smoke.ps1` **exit 0**（mixboard/mom/chat 全 ok——渲染族回归亲验）。
5. 泊位：两轮自起自拆+端口复查净+探针 2 次 taskkill 已申报；先行探针 verify1 的方法学发现（含 cancel 竞态、看门狗零误报）均记 manifest。

## 边界注记

- live-cancel 竞态与看门狗真栈 122s 触发不进烟测门（由单测 VitRenderWatchdogTests exit 0 覆盖+边界声明）——认可：竞态面不设确定性门是诚实口径而非弱化。
- 连带收益兑现：render_failed 遥测恢复、cancel_render 可用、命令面在渲染失败后存活——JOURNEY-1 渲染环节解锁。
- 附加发现（checkNodesForAudio 全工程判定→范围外音频剪辑得静音 render_done）：语义性注记非缺陷，留渲染语义后续卡参考。

## 处置

- cherry-pick `4984ef66` → main；卡面回填验收行；本 ruling 为裁定文件。K1 关闭，MIDI-RECON-1 缺口清单仅剩 A1（instrument 通道，待用户定产品形态）。
- 后续按刹车决策：执行侧回主线池（首选 JOURNEY-1）；决策侧下个工作窗首要=L1-1-REFSCHEMA-1 定版评审。
