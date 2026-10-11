# VIS-FILM-DESIGN-1 设计件一：film 契约（VitReel v1）

- 归属：VIS-FILM-DESIGN-1 设计基线四件之一；产品名 VitReel（用户 2026-10-10 裁定），中文 UI 落"成片"
- 吸收来源：openfilm 三原则（纯函数帧契约/确定性校验替代模型自证/用户手改落回文件），不复制其格式细节（MIT 协议无法律障碍）
- 证据底座：coord/runs/VIS-FILM-OPENFILM-RECON-1/README.md §2 §5；本仓库锚点见文内

## 1. 核心契约：frame(t) 纯函数页

**每个镜头 = 一个自包含网页**。页面唯一对外契约：

```js
window.film.frame(t)   // t: 本镜头内的本地时间（秒，t=0 为镜头入点）
```

铁律（与 openfilm 同源）：**同一个 t、同一份内嵌数据，永远画出同一张图**——与调用顺序、播放历史、页面存活时长无关。

### 1.1 纯度规则（生成器必须遵守，校验器负责抓违例）

| 规则 | 内容 | 违例检测 |
|---|---|---|
| P1 无时钟 | 禁 `Date.now()`/`performance.now()`/`new Date()` 影响画面 | 双打乱序重画像素比对 |
| P2 无随机 | 禁未播种 `Math.random()`；播种随机（固定种子的 LCG 等）合法 | 同上 |
| P3 无网络 | 禁一切运行时网络请求（fetch/XHR/WebSocket/字体 CDN）；素材与数据在生成期内联 | CSP 违规报告+运行时封禁断言 |
| P4 无外部状态 | 禁读 localStorage/cookie/父页面 DOM；唯一输入=t 与页内数据 | 沙箱属性+校验器断言 |
| P5 幂等重入 | `frame(t)` 可任意次、任意序调用；页面不得在 frame 调用间累积可观察状态 | 校验器第二遍乱序全量重画 |

### 1.2 时间模型

- film 总时间轴 = 镜头表串接；`film.json` 记录每镜头 `[t_in, t_out)`；页面收到的 t 已换算为**镜头本地时间**（播放器/导出器负责换算，页面不感知全局时间轴）。
- 帧率解耦：页面不假设帧率；驱动方（播放器 rAF / 导出器逐帧）以任意频率采样 `frame(t)`，t 为连续实数。导出固定 30fps（v1 不做 openfilm 的快门运动模糊，列 v2 打磨项）。

### 1.3 自包含单文件（v1 约束）

每个 shot 页 = 单 HTML 文件，CSS/JS/数据全部内联（数据经生成期内联为 `<script type="application/json">` 块，页内解析）。理由：
- 复用现有 artifact 文件服务面（`/agent/artifact/file?id=`，`agent/internal/chat/server.go:1365` `http.ServeFile` 无目录路由，见设计件三 §1）；
- 纯度更硬：页面+数据是一个不可变整体，"同一 t+同一特征数据→同一画面"由文件不可变性保证；
- 与 openfilm 镜头页形态一致（其 Studio/多文件面我们不需要）。

代价：混音状态变化→特征数据变化→镜头页需再生成。这是能力循环（agent 重渲染包络→再生成页面→再过校验）的本职，v1 接受。

## 2. 工程内 film 文件夹格式

```
<工程根>/film/
  film.json                 # manifest：schema、片名、时长、帧率、镜头表、数据指纹
  shots/
    shot_001.html           # 自包含镜头页（实现 window.film.frame）
    shot_002.html           # 命名：<序号>_<slug>.html，序号即时间轴顺序
  exports/                  # 导出产物（MP4 等），gitignore 候选
```

### 2.1 film.json schema 草案（`vit.film.manifest.v1`）

```json
{
  "schema": "vit.film.manifest.v1",
  "title": "demo_noir_rain",
  "duration_seconds": 183.4,
  "export_fps": 30,
  "audio": {
    "source_render_sha256": "…",
    "waveform_artifact_id": "…",
    "duration_seconds": 183.4
  },
  "shots": [
    {"id": "shot_001", "file": "shots/shot_001.html", "t_in": 0.0, "t_out": 12.5,
     "evidence": {"kind": "segment", "label": "intro", "refs": ["mom://…"]}}
  ],
  "data_fingerprint": "sha256(全部镜头页文件哈希的有序串接)"
}
```

- `evidence` 字段是 v1 可选、v2 音画对位 verifier（设计件二 §4）的输入面：切镜决策的证据回指（DOM 动态事件/MOM 关系/RLM 电平的证据 ref）。
- `data_fingerprint` 供确定性校验防"两遍之间数据被换"的护栏（校验开始与结束各算一次，不一致则中止并报 `data_changed`）。

### 2.2 目录位置裁定

`<工程根>/film/` 为一等工程子目录（先例：工程根旁 `<base>_Media` 媒体目录，`agent/internal/workspace/workspace.go:87-92`；派生数据用 `.vit_derived/<uuid>/`，`agent/internal/projectworkspace/derived.go:29-66`）。选工程根可见位置而非 `.vit_agent` 深处的理由：v2 用户手改落回需要用户可见可编辑；demo 工程可直接入 git 展示；与"成片=工程呈现产物"的语义一致。写入路径**只能经 film capability**（见设计件二 §3 封堵条款）。

## 3. 音频特征数据面

### 3.1 选型结论（方案 A：全 Go，零内核改动）

| | 方案 A：start_render+Go 分析（**选定**） | 方案 B：复用 l2_render_probe |
|---|---|---|
| 链路 | `start_render` 整曲 master 渲染（WAV 保留）→ Go 读 WAV 算包络 → 写 JSON | 抽 `l2_render_probe` 事件的 masking_frames 写 JSON |
| 分辨率 | 任意（hop 自定，v1=50ms） | 上限 256 帧（长曲约 1-2s/帧，偏粗） |
| 内核改动 | **零** | 提密度需改 `maxMaskingFrames`+重编译 VitApp |
| 产物留存 | WAV 保留（可作播放器音轨+导出音轨） | probe WAV 即删（VitProductionCoordinator.cpp:782-784） |
| 复用锚点 | `agent/internal/chat/free_state_d1_render.go:99`（start_render→WaitRender→sha256 溯源的标准等待模式）；`agent/internal/probeaudio/suite.go:55`（Go 自写分析资产+manifest 模板）；`agent/internal/acousticpackage/status.go:136-157`（`.vit_derived/<uuid>/` JSON 落盘模式） | `VitApp/Source/Service/VitProductionCoordinator.cpp:265` `analyseL2ProbeFile`（RMS/peak/6 频段，**无 LUFS**）；maskingFrames 256 帧上限 :296-298 |

方案 A 链路（音频特征面实施卡的实现规格）：
1. agent 发 `start_render{file_path: <工程>/.vit_agent/<uuid>/film/render/<ts>.wav, bit_depth: 24, use_master_plugins: true}` → `harness.WaitRender` → sha256 记入 manifest；
2. Go 侧 WAV 解码（24bit/32f，48k）→ 能量包络：hop 50ms 窗 100ms，逐窗 RMS(dBFS)/peak(dB)；6 频段能量照搬 `analyseL2ProbeFile` 的分频点（sub/bass/lowmid/mid/highmid/air，VitProductionCoordinator.cpp:170-178）以保证与证据层频段口径一致；
3. 段落切分 v1 用能量法（双阈值+最小段长），边界落 `segments`；v2 换 DOM/MOM/RLM 证据驱动；
4. 输出 `energy_envelope.json` 内联进镜头页（§1.3），源 JSON 存 `.vit_derived/<uuid>/film/` 留审计。

### 3.2 energy_envelope.json schema 草案（`vit.film.energy_envelope.v1`）

```json
{
  "schema": "vit.film.energy_envelope.v1",
  "source_render": {"sha256": "…", "duration_seconds": 183.4, "sample_rate": 48000},
  "hop_seconds": 0.05, "window_seconds": 0.1,
  "bands": ["sub","bass","lowmid","mid","highmid","air"],
  "frames": [{"t": 0.0, "rms_db": -23.1, "peak_db": -12.9, "band_db": [-30,-24,-26,-28,-33,-38]}],
  "segments": [{"t_start": 0.0, "t_end": 21.3, "label": "intro", "energy": "low"}]
}
```

### 3.3 与生成线的咬合

成片数据面永远取自**最终混音**（master 整曲渲染），生成链素材（GeneratedAssetService/MIDI/stem）先进工程、进混音，再经渲染进入 film 数据面——"ask vit 单入口生成成片"的产品语义=生成→混音→成片一条链，成片反映最终混音状态。v1 画面=代码绘制（canvas/CSS/SVG/文字动画），不依赖图像生成模型（质量可控+`image_generation` 路由亦处关闭态）；生成模型产画面列 v2 议题。

## 4. 封面形态（卡面裁定落档）

- **封面=VitReel 预播短视频片段**：如贴在磁带壳上的 J-card 标签从静态图升级为活动画面。
- 技术红利：封面形态是 frame(t) 纯函数页的直接推论——feed/列表位预播=播放器在列表位实时循环调 `frame(t)`，**零预编码视频文件、永远与当前混音状态同步**（混音变→能力循环再生成页面→列表位下一次循环即新画面）。
- 挂载归属：工程列表"磁带壳陈列"（带架式列表、播放时带轮转动）是 V5 工程流首页列表视图的未来延展方向，**归属下一轮前端设计会话裁定**（FE-GUI-SHRINK 第一轮基线已存档不动，本节为其后续轮输入）；v1 不实现封面挂载，仅固化本契约。
- 文化血统注记（论文/叙事可选素材，用户 2026-10-10 裁定不优先）：磁带读取头在位置 t 的读出只依赖位置不依赖历史——frame(t) 确定性契约的介质祖师爷就是磁带；DAW 的 "cut" 词源=磁带剃刀剪辑；AMV 文化源自双录像机对录剪接，VitReel=该手工流程的证据驱动自动化。

## 5. v2 预留：用户手改落回文件（overrides）

openfilm 原则③。v1 不实现，契约预留：播放器侧手改（镜头时长微调/文字替换/颜色覆盖）落 `<工程>/film/overrides.json`（键=镜头 id+参数路径，值=用户值+时间戳）；agent 再生成时必须读 overrides 并保留用户值（agent 可见且不可覆盖）；确定性校验对 overrides 应用后的最终画面做（同一 t+同一数据+同一 overrides→同一画面）。这与 FE-GUI-SHRINK 后续轮的 Studio 式编辑面咬合，设计细节届时另出。
