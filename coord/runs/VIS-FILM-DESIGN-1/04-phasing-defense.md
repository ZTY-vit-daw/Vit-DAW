# VIS-FILM-DESIGN-1 设计件四：分期与展演咬合（v1/v2 分期表 + DEFENSE-SHOW-1 合流）

- 归属：VIS-FILM-DESIGN-1 设计基线四件之四
- 输入：卡面分期口径 + DEFENSE-SHOW-1 卡面原文 + VIS-FILM-OPENFILM-RECON-1 §6 时间预算 + 与 GEN-CAP-RECON-1 §4 的合并定位（生成线第二批次主力卡，首批=GEN-MAC-BASELINE-1+AIGC-DRUM-GEN-1 迁移）

## 1. v1 锁死范围（卡面原文，不扩）

- 单 demo 工程（PC）
- 3-5 镜头
- PC webui 播放（?surface=film，本地音频时钟）
- MP4 导出（playwright-core+ffmpeg，Windows 真栈验证）
- 准入校验全绿（C1-C5）

**不在 v1**：移动端/feed 封面挂载、用户手改 overrides、音画对位第二 verifier（仅预留接口）、工程内 transport 同步播放、快门运动模糊/BT.709、转场词汇、Studio 式编辑器、桌面端。

## 2. 分期表（实施线拆卡视图，卡规格见设计件五）

| 批次 | 卡 | 内容 | 预估 | 真栈占用 | 依赖 |
|---|---|---|---|---|---|
| 前置 | VIS-FILM-P0 | image_understanding 路由启用+agent 内读图回路验证 | 0.5 天 | 烟测腿 1 次 | 用户/主管裁定启用 |
| M-A 音频特征面 | VIS-FILM-A1 | start_render 整曲渲染封装+WAV 能量包络分析器+energy_envelope.json | ~1 周 | 否（单测） | 无 |
| M-B 能力面+校验 | VIS-FILM-B1 | film capability 声明/路由/FilmFilePort（staging+verify-rename）+write 封堵 | ~1 周 | 否（单测） | 设计件一/二 |
| | VIS-FILM-B2 | film_verify.mjs（C1-C5）+Go 接线+DEPENDENCY-BOM 建档 | ~1 周 | 否（脚本自测用本地 fixture 页） | B1（接线）；可与 B1 并行开发 |
| | VIS-FILM-B4 | 能力端到端：chat 写测试镜头页→过校验→挂载（含 P0 读图迭代演示） | 2-3 天 | 烟测腿 1 次 | A1+B1+B2+P0 |
| M-C 播放与导出 | VIS-FILM-C1 | FilmPlayerView（artifact 加载+audio 时钟+frame 驱动+沙箱三层） | ~1 周 | DOM 烟测腿 | 设计件一/三；可与 B 线并行（fixture 页开发） |
| | VIS-FILM-C3 | film_export.mjs+ffmpeg 接线+run_film_export_smoke.ps1 | ~1 周 | 导出烟测腿 | C1 的 film.json 契约（设计已锁，可并行开发） |
| M-D demo+展演 | VIS-FILM-D1 | demo 工程成片：3-5 镜头能量驱动切镜，能力链全走 | ~3 天 | 烟测腿 1 次 | 全部 |
| | VIS-FILM-D2 | 展演素材：最佳轮次实录+导出 MP4 兜底（demo-capture SOP 咬合） | ~2 天 | 录制在位 | D1 |

（B3 封堵并入 B1；A3 段落切分并入 A1 v1 能量法；C4 播放器 DOM 断言并入 C1 验收；C2 沙箱并入 C1。）

## 3. 时间轴与 DEFENSE-SHOW-1 合流

DEFENSE-SHOW-1 卡面锚点（2026-09-26）：
- 用户裁定：设计工作推迟到 **11 月中评估点**，届时按 L1/L2 进度与剩余时间裁定节目编排；保底节目零开发依赖性质不变。
- 升级段（编曲 AIGC 起点）：展演级简化版，11 月中评估点裁定做否。
- 时间轴：11 月上分镜设计启动（与论文 freeze 并行）→**11 月中升级段评估点**→12 月上素材采集（demo-capture SOP）→**12 月中展演版本冻结**+带妆排练 ≥2 次→12 月底预答辩。
- 开场钩子（30s）：播成品 demo——"这段音乐，等下当场做出来"（开场版=排练最佳轮次实录，明示）。

VitReel 合流口径（卡面原文）："11 月中评估点出可判定 v1，12 月中展演冻结（开场钩子升级为'带画面的成品 demo'）"。

排期映射（今天 2026-10-11；单线 5-7 周、并行可压至 ~4-5 周，与 RECON §6 一致）：

```
W1 (10/12–10/18)：P0 + A1 启动（生成线首批 GEN-MAC-BASELINE-1 同窗并行，文件域不相交）
W2 (10/19–10/25)：A1 收尾 + B1/B2 并行 + C1 启动
W3 (10/26–11/01)：B4 端到端 + C1 收尾 + C3 启动
W4 (11/02–11/08)：C3 收尾 + 真栈烟测腿排队（与 L1/L2 烟测串行）
W5 (11/09–11/15)：D1 demo 成片 + D2 展演素材 → 11 月中评估点：可判定 v1
11 月中–12 月中：评估点裁定后——v2（feed 封面/overrides）或 v1 打磨/展演排练；12 月中展演冻结
```

## 4. 评估点"可判定 v1"的判据（给 DEFENSE-SHOW-1 评估点用）

- 硬判据：demo 工程 3-5 镜头成片，在 PC webui 播放器全片播放无中断；MP4 导出真栈烟测退出码 0；该成片由 film capability 产出且 verification_report 全绿（有 run 工件可查）；agent 读图迭代回路至少跑通一轮（读自己生成的帧截图并据此修改页面）。
- 软判据（评估点参考）：镜头切换与段落边界的对位观感（人工评审）；画面质量达到"可公开放映不丢人"线（排练实录可兜底）。
- 未达标处置：v1 判定延期不影响 DEFENSE-SHOW-1 保底节目（零开发依赖）；VitReel 降级为 12 月展演的"已有素材播放"（若 MP4 已可导出）或完全移出展演（明示）。

## 5. 风险与对策（卡面 §风险的设计化展开）

| 风险 | 对策 | 残余 |
|---|---|---|
| LLM 画面质量概率性 | 校验器只保底线；读图迭代回路（P0）提质；排练最佳轮次实录兜底，live/录播明示 | 画面审美上限取决于模型，展演用实录对冲 |
| 时间零和（与 L1/L2 主线、论文 freeze 抢会话与真栈） | 文件域不相交天然并行（A=Go agent 新包、B1=orchestration/executionports、B2/C3=scripts、C1=webui）；真栈串行只影响烟测腿，按 PC-RUNTIME-STACK 先登记先占用；11 月中评估点设"可判定/延期"双向出口 | 真栈烟测腿窗口仍需排队 |
| 沙箱逃逸面（film 页=任意 web 代码） | 三层沙箱（设计件三 §1.3）+封堵绕行（设计件二 §3）+校验器网络封禁断言 | v1 接受"环回环境内低风险"残余（无外网暴露面：全栈 127.0.0.1） |
| deepseek-v4-flash 读图质量未验（仅 16×16 实测） | P0 卡含真实截图（≥1280×720）读图验证；质量不足则路由指视觉强模型（配置层换，零代码） | 模型选择是用户/主管决策 |
| Windows ffmpeg 路径漂移（WinGet 版本目录含哈希） | DEPENDENCY-BOM 登记 PATH 优先+候选探测+版本断言；烟测腿 ffprobe 探测失败即环境中断记录，不算功能失败 | 环境维护成本常驻 |
