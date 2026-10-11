# VIS-FILM-DESIGN-1：VitReel 成片能力设计基线（总览）

- 执行：GLM-5.3 ZCode 执行流会话（PC，用户点名指定）/ 2026-10-11；领取 origin/main=844c6524
- 性质：设计卡，零生产代码（唯一代码面动作=一次性读图探针 test 文件，跑完即删，agent 树净 diff 已核）；不占真栈
- 证据底座：`../VIS-FILM-OPENFILM-RECON-1/`（openfilm 全量调研）+ 本目录四路代码勘察锚点

## 0. 第零验证项结论：LLM 读图能力核验（可行性前提）——**通过，无需接线小卡，仅余一键配置**

**消息构造面锚点（完备，已在案）**：
- `agent/internal/llm/client.go:34` `ImageInput{MIME, DataBase64, URL, Detail}`；`:41` `VisionRequest{Prompt, Images}`；`:244` `CompleteImageUnderstanding`（完整客户端链路：路由解析→请求构造→SSE/JSON 解析→telemetry）
- 双格式构造：`:430` `visionRequestBody`（OpenAI Responses `input_image` / Chat Completions `image_url`）；`:471` `imageInputURL`（base64/URL 双通道）
- 路由解析：`:328` `imageUnderstandingRouteConfig`——`enabled:false` 或路由缺失即 `ErrImageUnderstandingRouteUnavailable`（`:102`）；provider=default 继承引擎 BaseURL/APIKey/Model
- 配置面：`agent/internal/config/config.go:102-105` `DefaultMultimodalRoutes()` 含 `image_understanding` 默认路由

**实测（probe/ 目录，证据可复现）**：
- 测试图：16×16 PNG（左红右蓝+(4,4)黄点），`probe/test_image_16x16.png`
- 实测输出：`probe/live_probe_output.txt`——真实配置（`~/.vit/config.json`：rightapi 中转 deepseek-v4-flash）下经 `llm.Client.CompleteImageUnderstanding` 全链路发送：
  - 路由现状（enabled:false）→ `image_understanding_route_unavailable`（如预期）
  - 路由开启（仅改 enabled）→ **端点收图且读图逐项正确**："The image shows a vertical split: left half red with a yellow glowing dot, right half blue."（往返 1.7s，in:230/out:41 tokens，responses 端点）
- telemetry 双记录归档：`probe/live_probe_telemetry.jsonl`（route_err 版+成功版各一）

**结论**：客户端链路与端点均支持读图，openfilm 唯一硬要求（agent 能看自己生成的帧）**成立**。唯一缺口=运行时配置 `image_understanding.enabled:false`——启用为**一行配置变更**（用户/主管裁定），非代码接线卡；落地与真实截图级验证归实施卡 VIS-FILM-P0（设计件五卡 1）。残余风险：deepseek-v4-flash 对 ≥720p 真实截图的读图质量未验（P0 卡覆盖）。

## 1. 设计基线四件（索引）

| 件 | 文件 | 内容 |
|---|---|---|
| 一 | [01-film-contract.md](01-film-contract.md) | frame(t) 纯函数契约（纯度规则 P1-P5）/工程 film 文件夹格式（manifest schema v1）/音频特征数据面（方案 A 全 Go 选定+schema）/封面形态节/overrides v2 预留 |
| 二 | [02-capability-admission.md](02-capability-admission.md) | film.writing.v1 能力挂载点（builtins/runtime/owner/film_runtime 四层锚点）/FilmFilePort（staging+verify-rename）/look 式校验器 C1-C5+执行器选型（Node playwright-core）/封堵绕行/音画对位第二 verifier 预留 |
| 三 | [03-playback-export.md](03-playback-export.md) | webui 播放器（?surface=film+artifact 端点+本地音频主时钟裁定）/沙箱三层/MP4 导出链路+真栈验收/CONFIG-BOM 勘误与 DEPENDENCY-BOM 新建 |
| 四 | [04-phasing-defense.md](04-phasing-defense.md) | v1 锁死范围/分期表（8 卡）/时间轴排期（W1-W5→11 月中评估点）/可判定 v1 判据/风险对策表 |
| 五 | [05-impl-card-specs.md](05-impl-card-specs.md) | 实施卡规格草案 8 张（可直接发卡程度）+互斥文件清单+发卡顺序建议 |

## 2. 红线核对表（卡面验收项逐条结论）

| # | 红线 | 结论 |
|---|---|---|
| R1 | FE-GUI-SHRINK 四红线（遥测写者链/左库/右键菜单/拖拽双落点）零触碰 | **通过（设计层面零触碰）**。四红线全部位于 Godot 前端仓（`telemetry_manager.gd:1963`、`mix_client.gd:39-42/:65/:71`、`LLM_Chat_Controller.gd` 数据面、`vsp_asset_adapter.gd`、`left_library_dock.gd`、机架/轨行右键、插件拖拽双落点——原文见 `coord/runs/FE-GUI-SHRINK-ROUND1-SCOPED-RECON/实施卡规格-第一轮.md:39-43`）。VitReel v1 文件域=agent Go+webui+scripts+工程 film/ 目录，**不打开前端仓任何文件**；磁带壳工程列表归 FE-GUI-SHRINK 后续轮（卡面裁定），本设计仅固化"封面形态"契约（01 §4）。v2 涉前端仓时另行核对。 |
| R2 | playwright-core 入 CONFIG-BOM | **勘误+新建登记面**。CONFIG-BOM-1 实为 UTF-8 BOM 字节序容错修复卡（`coord/runs/CONFIG-BOM-1/receipt.md`），仓库不存在依赖 BOM 清单。本设计裁定新建 `coord/resources/DEPENDENCY-BOM.md`（与 PC-RUNTIME-STACK.md 同级），首版三条目已拟（03 §2.3）；建档归实施卡 B2 验收项、C3 追加 ffmpeg 行。执行器选型裁定=Node playwright-core（与 E2E-WEBUI-1 门同栈，自发现机制复用 `webui_rendered_dom_smoke.mjs:200-227`；channel 回退链移植自 `e2e_chat_chain_layer_b.py:28-58`）。 |
| R3 | 真栈串行约束不变 | **通过**。VitReel 仅 4 个烟测腿占真栈（P0 读图验证/B4 能力端到端/C1 DOM 烟测/C3 导出烟测+D1 复验），全部按 `coord/resources/PC-RUNTIME-STACK.md` 先登记先占用；其余卡（A1/B1/B2 开发腿）纯单测/脚本自测。与 L1/L2 烟测腿冲突=排队不抢栈。 |
| R4 | 沙箱策略 | **三层设计+v1 全量落地**（03 §1.3）：iframe `sandbox="allow-scripts"`（无 allow-same-origin）/页内 CSP meta `default-src 'none'`（生成器模板强制+校验器 C3 断言）/校验器运行时网络封禁断言。现状缺口如实记录：全仓零 CSP、既有 iframe 先例（App.tsx:11406）无 sandbox 属性——**既有面不动**（其内容非 agent 任意代码），新增 film 面必须带；既有面是否补沙箱列为可选 hygiene 议题交主管。残余风险：环回环境内低风险（全栈 127.0.0.1，无外网暴露面）。 |

## 3. 关键设计裁定汇总（主管验收聚焦点）

1. **第零验证项通过**：读图链路完备+端点实测正确；仅余 `enabled:false` 一键（用户裁定项）。
2. **音频特征面选方案 A**（全 Go：start_render 整曲渲染+WAV 分析+JSON），弃方案 B（probe 256 帧上限+即删 WAV）——零内核改动是决定性理由。
3. **镜头页=自包含单文件**（数据内联），复用 artifact 文件服务面零新 HTTP 协议；代价=混音变需再生成（能力循环本职）。
4. **播放器=本地音频主时钟**（v1 与 DAW transport 解耦；工程内同步播放列 v2）。
5. **校验器/导出器执行栈=Node playwright-core**（与渲染面门同栈+openfilm 同语言直搬；备选 Python playwright 已记录不采纳理由）。
6. **film/ 目录=工程根一等子目录**（先例 `<base>_Media`；写入仅经 capability，`workspace_write_file` 封堵）。
7. **CONFIG-BOM 勘误**：依赖登记面新建为 DEPENDENCY-BOM.md（卡面原文语义按此兑现）。
8. **音画对位第二 verifier=v2 评分器预留**（非准入门），v1 只落 evidence 回指字段。

## 4. 与生成线/GEN-CAP-RECON-1 §4 的合并复核结论

本设计按卡面定位=生成线（GEN）下游呈现产物、第二批次主力卡（首批=GEN-MAC-BASELINE-1+AIGC-DRUM-GEN-1 迁移，维持领取回填口径）。咬合点：成片数据面永远取最终混音（master 渲染），生成链素材经工程→混音→渲染进入成片——"ask vit 单入口生成成片"=生成→混音→成片一条链。v1 画面=代码绘制，不依赖图像生成模型；A1 instrument 装载通道缺口（GEN-CAP-RECON 结论②）不影响 VitReel v1（数据面只需渲染，不需 instrument 实装）。VitReel v1 为 PC 线；Mac 对齐待 GEN-MAC-BASELINE-1 基线后再议。

## 5. 工件清单

```
coord/runs/VIS-FILM-DESIGN-1/
  README.md                     ← 本文件
  01-film-contract.md           设计件一
  02-capability-admission.md    设计件二
  03-playback-export.md         设计件三
  04-phasing-defense.md         设计件四
  05-impl-card-specs.md         实施卡规格草案（8 张）
  probe/test_image_16x16.png    第零验证项测试图
  probe/live_probe_output.txt   实测原始输出（go test -v）
  probe/live_probe_telemetry.jsonl  实测 telemetry 双记录
```
