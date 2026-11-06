# 决定：生成线转向 MCP + 池内清理裁定

- 会话：GLM 主管决策流 / 2026-11-05（用户口头裁定）
- 性质：生成能力线方向裁定 + 卡池结案记录

## 一、生成线转向（用户裁定原文意旨）

1. **Suno 经 MCP 调用**：不再走官网订阅网页反代，也不再走自建 API 直连线；
2. **套壳按 MCP 思路开发**：Vit 侧生成集成层以 MCP（Model Context Protocol）模式接入外部生成源；
3. **网页反代与 API 线全部取消**：
   - GEN-SUNO-WEB-DESIGN-1（doing，10-16 起）→ 取消结案（无需开通 Suno 订阅）；
   - GENAPI-REALVERIFY-1（blocked）→ 取消结案；
   - 已合入的 GEN-API-ADAPTER 代码留存库中不追改，后续生成集成按 MCP 方向另立卡。

## 二、连带的池裁定（同日）

- FE-SPLIT-RESIZE-REGRESSION-1：用户确认已修好 → 结案（blocked→done）；
- VIS-FILM 线正名 **Vitape**：11-12 月来不及考虑 → 冻结；
- VMT-1：维持 2026-10-24 既有裁定（后置到编曲之后），今日用户重申"太激进暂不做"；
- 论文：移出本仓调度范围（毕业设计项目文件夹的任务，不在 coord 卡池排序）。

## 三、影响与后续

1. **MCP=生成线专属方案，不进编曲勘察**（2026-11-05 用户更正）：编曲=域内能力——编辑 MIDI、调用音源（VSTi/音色）、渲染，与 Suno/MCP 生成素材是两条线；编曲勘察卡范围不含 MCP。生成线 MCP 集成（agent 侧套壳按 MCP 思路）为独立方向，需要时另立卡；
2. GEN 卡群（AIGC-DRUM-GEN / GEN-VOCAL-SYNTH / ARTIST-AUDITION / GENSHELL×2）维持冻结，归属与解冻随编曲勘察结论及生成 MCP 集成设计分别定；
3. 生成线"堵在用户开通动作"的状态解除——不再依赖任何订阅开通；
4. FREESTATE-REGION-IMPL-1 同日取消结案（用户裁定：卡源于对自动化实现方案的误解；自动化包络线处理已实现，范围处理走包络单变更终态）。

## 四、生成线立即重勘 + 全 PC 化（2026-11-05 用户补裁）

1. **生成线是当前需要的**（非后置）：立即重勘（GEN-RECON-1，池序 259）；
2. **Mac/PC 拆线取消**：2026-10-24"当前 PC=混音、Mac=生成"作废——PC 单机即可承载并行开发（多执行会话/多 worktree，与 2026-10-09 并行裁定一致）；YuE2 已部署于 PC（YUE2-PC-DEPLOY-1 已满足），Suno 走 MCP 不挑机器；
3. **YuE2=展示使用的主要模型**（本地部署），Suno=MCP 补充源；
4. **人声不单独开发模型**（决策侧建议，勘察确认）：三层框架——整首歌人声=YuE2/Suno 直出；工程内人声轨=生成→分离两步（stems/demucs 类，勘察确认 Suno stems 与 YuE2 分轨面）；可编辑精细歌声合成=不做（远期）。加分事实：YuE2 ABC 引导已验证（corr 0.99 级）→ 生成线与符号域（编曲/乐局）天然可打通（ABC↔MIDI 转换为 music21 级）。
5. **排期裁定（2026-11-05 用户）：生成与编曲同为必须能力层，11 月内完成**——11 月生成面=GEN-YUE-DIALECT-1（agent 集成，genapi yue-local dialect 直连）+GEN-MCP-DIALECT-1（MCP dialect）；GEN-SUNO-MCP-VERIFY-1 凭据后置（用户取得 AceDataCloud key 后 0.5 天随时插，如实声明不阻塞）；12 月仅余 GEN-YUE-RENDER-1 渲染腿（素材）。
