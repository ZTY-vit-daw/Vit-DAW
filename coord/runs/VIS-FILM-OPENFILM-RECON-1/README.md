# VIS-FILM-OPENFILM-RECON-1：openfilm/openfilm 调研记录（VIS-FILM-DESIGN-1 证据底座）

- 调研会话：GLM-5.3 ZCode 主会话（用户发起）/ 2026-10-10
- 性质：只读外部调研，零本地代码；来源=仓库原文抓取（README/MANUAL/SPEC/SKILL/agent-roster.mjs/render.mjs/package.json + 文件树 598 项 + 逐文件行数）
- 用途：VIS-FILM-DESIGN-1 设计卡的证据底座；GEN-CAP-RECON-1 报告第 4 节（启动建议）合并复核输入

## 1. openfilm 是什么（一句话）

把"AMV/手书制作"工程化的开源 AI 视频 agent：LLM（任意编程 agent）把电影写成网页，确定性代码负责播放/渲染/校验。口号"turns any coding agent into a video agent"。

## 2. 核心架构（三件套）

1. **格式**：电影=文件夹。`film.html` 是时间轴（`<section>`=轨道，剪辑=HTML 元素，`#t=in,out` 媒体区间、`at` 入点、CSS 定位）；每个镜头=一个网页，唯一契约是纯函数 `window.film.frame(t)`。铁律："同一个 t 永远画同一张图，与顺序/历史无关"→帧可乱序、并行、跨机一致渲染。
2. **执行**：全确定性——播放器/headless Chromium 逐帧调 `frame(t)`+ffmpeg 编码（并行页面实例、快门采样运动模糊、BT.709 标记、与混音合流）；Studio 编辑器（用户手改落回文件 `overrides`，agent 可见且保留）。
3. **校验**（设计核心，README 原话 "OpenFilm spends its effort where models fail: checking the film"）：`look` 命令每帧按**两种打乱顺序各画一遍**，自动报：非确定性帧/页面报错/字体加载失败/音频缺失/文字被边缘裁切。

## 3. 工程量实测（含金量定位）

- 核心引擎约 **3900 行纯 Node .mjs**，运行时依赖仅 `playwright-core 1.61.1` + 外部 ffmpeg。分布：film-doc.mjs 937 / timeline.mjs 819 / host.mjs 361 / look.mjs 357 / shared.mjs 330 / render.mjs 157（+测试）。
- 仓库其余 ~450 文件是产品壳：Studio 编辑器（327 文件，React）+ 桌面聊天应用（121 文件，Electron）。**无 Python 工具包、无 ML 组件、无自建 agent 编排引擎**——agent 是外部的（Claude Code/Codex/…），openfilm 只用约一千词 MANUAL 教格式 + 四个确定性 CLI（open/look/render/get）。
- 结论：含金量=**契约设计 + 校验哲学**，不是重度系统工程；vit 复刻所需子集（契约/播放器/校验器/导出）估 1500-2500 行，openfilm 八成体量（编辑器/桌面端/服务商接入）不需要。

## 4. 模型要求（"必须 Opus"是误传）

README/MANUAL/SPEC/SKILL 全文零模型指定；桌面端 agent-roster.mjs 支持 Codex/Claude Code/Gemini CLI/Copilot/Cursor/OpenCode/CodeBuddy/Qwen/Kimi 九种 + 自带 key 任意模型。唯一硬要求：**模型能读图**（probeModel 发 16×16 测试图，不收图模型被拒——agent 要看自己生成的帧）。强模型=质量差异，非架构要求。

## 5. vit 吸收方案（VIS-FILM-DESIGN-1 的输入）

**复制三原则，不复制格式细节**（MIT 协议无法律障碍）：①纯函数帧契约；②确定性校验替代模型自证；③用户手改落回文件。

管线重归属三处（概念零偏差）：
- **创作主体**：外部 agent 自由写文件 → vit chat agent 走 capabilityruntime（"film 写作"能力，写入限定工程 film 目录，过 look 式校验才能挂载——比 openfilm 更严，它无准入）。
- **数据源**：哑 mp3 素材 → 真实混音。vit 已有底子：FXM render probe + acousticpackage post-FX meter 已产出渲染后表计（agent/internal/acousticpackage/status.go:916、internal/fxm/projection.go）；其上加能量/段落包络导出。纯度不破坏：同 t+同特征数据→同画面。
- **切镜决策**（结构性超越点）：openfilm 的 agent 对音乐盲切；vit 的 agent 拿 DOM 动态/MOM 关系/RLM 电平证据切镜，可做第二 verifier 验"音画对位"。

## 6. 预答辩时间预算（2026-10-10 → 12 月底预答辩，约 11 周）

设计卡 2-3 天；音频侧（离线渲染+包络导出，大半复用 probe）~1 周；webui 播放器 ~1 周；agent 能力面+校验器 1.5-2 周；MP4 导出（playwright-core+ffmpeg，Windows 栈验证）~1 周；集成烟测+展演素材 ~1 周。**单线 5-7 周**，赶得上 11 月中评估点（DEFENSE-SHOW-1 升级段）出可判定 v1。

## 7. 风险边界

- LLM 画面质量概率性：校验器只保"不断/不糊/不裁切"不保"好看"；展演对策=排练最佳轮次实录兜底（与 DEFENSE-SHOW-1 保底逻辑同构，live/录播明示）。
- 沙箱：film 页=任意 web 代码，iframe 须禁网络/只读本地素材。
- playwright-core 新依赖须入 CONFIG-BOM；FE-GUI-SHRINK 四红线（遥测写者链/左库/右键菜单/拖拽双落点）零触碰；真栈串行约束不变。

## 来源

- https://github.com/openfilm/openfilm（README.md / README.zh-CN.md / packages/openfilm/{MANUAL.md,SPEC.md} / skills/openfilm/SKILL.md / apps/desktop/src/agent-roster.mjs / packages/openfilm/src/*.mjs / package.json，2026-10-10 抓取）
