# GENSHELL-AUDITION-1：缓冲播放+打印腿（实施⑥）

- 发卡：Mac 决策侧 / 2026-10-12（依据=件五 卡6）
- 派发确认：已确认（用户 2026-10-12「可以发卡」）
- 验收负责人：Mac 决策侧先行裁定+PC 主管复核留档
- 池序 100；**规格全文=[件五 卡6](../../runs/GEN-PLUGIN-SHELL-DESIGN-1/05-impl-card-specs.md)（权威）**
- 归属：Mac 生成线；P2 / 1-2 天 / GENSHELL-AGENT-1
- 模型分级：L2 / GLM 执行会话

## 要点摘录（以规格为准）

壳 instrument 实播（audition_ready→解码+线性重采样+走带同步流播+Bypass 静音）+print 路径（壳图内输出就绪→start_render→WAV→ingest，件二 §6）+非 WAV 可解码性负探针。验收=print 全链烟测（render_done→RIFF 断言→ingest，GEN-MAC render 腿 A 同模式）+fast/print 双路径等价+图内流播 RMS 非静音断言+采样率不匹配单测。停止条件=render 期白名单互斥面变化（CommandDispatcher.cpp:2242-2276）→锚点上交。端测边界：播放听感=手测清单（[等待用户]），自动化到 WAV 数据级。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / run ID / 手测清单 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
