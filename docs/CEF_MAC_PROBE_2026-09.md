# CEF mac 集成探针实测与 B2 集成手册（草稿）

- 日期：2026-09-18。任务卡：PORT-B2P（coord/cards/done/2026-09-18-PORT-B2P-cef-mac-probe.md）。
- 性质：**消 R3 的真机实测记录** + 给 PORT-B2（godot_cef 正式集成进 vit-daw-frontend）的步骤清单与坑位手册。
- 实测环境：Apple M3（Apple9）/ macOS 27.0（26A428）/ Godot 4.6.stable.official.89cea1439 / 上游 dsh0416/godot-cef **v1.15.4**（CEF 152.0.5 pin）。
- 工件：`~/Documents/vit-b2p-probe/`（仓外；总结见其 `PROBE_SUMMARY.md`，逐轮证据在 `runs/run-*/`）。

## 1. 结论（R3 消除）

**R3 兑现为"已实测可用"，未成为阻塞。** 三轮有效运行（A3/C2/B2）全部 exit 0：

| 轮 | 驱动 | enable_accelerated_osr | 浏览器实际模式 | 出图证据 | 退出残留 |
|---|---|---|---|---|---|
| A3 | `--rendering-driver metal` | true | **加速（Metal）** | viewport.png 1280x800，非背景比 0.975 / 810 色 | 无 |
| B2 | 默认 | true | **加速（Metal）** | viewport.png 1280x800，非背景比 0.975 / 826 色 | 无 |
| C2 | 默认 | false | **软件** | browser_texture.png 直读 1280x800（755 色） | 无 |

完整链路实证：CEF 实例化 → `res://` 本地 HTML 加载（`load_finished status=200`，冷启 3.46s / 暖启 0.2s）→ JS 执行（console_message 回传 6 条）→ canvas 动画（~55fps，帧计数经 IPC 回传）→ OSR 纹理出图（截图目检：渐变/轨道方块/脉冲圆环/文字全渲染）→ helper 进程 5 个（GPU ~12% CPU / network / renderer 等，PPID=godot）→ 干净退出（三轮退出后 ps 均 `NO_RESIDUAL_HELPER`）。

### 对审计前提的一处证据更新

审计 R3 原假设"Godot Forward+（MoltenVK）"。实测 **Godot 4.6.stable 在 macOS 默认 Forward+ 即走原生 Metal**（无旗标也打印 `Metal 4.0 - Forward+ - Apple M3`），addon 后端探测结果 `backend=Metal`。即：不存在"CEF Metal ↔ Godot Vulkan"跨 API 互操作问题——两端同在 Metal 上，`accelerated_osr_supported=true, reason=Metal backend supports accelerated OSR on macOS` 开箱成立。`--rendering-driver metal` 旗标可加可不加（加了显式、不加也是 Metal）。

## 2. 上游资产与放置契约

### 2.1 资产

- release zip：`godot_cef-v1.15.4.zip`（995.9MB，含全平台）；mac 侧 = `dist/addons/godot_cef/bin/universal-apple-darwin/`（965 条目，~666MB 解压后）。
- framework：`Godot CEF.framework`，`libgdcef.dylib` 为 **universal（x86_64+arm64）**；CEF 本体按架构分 `Chromium Embedded Framework (ARM64|X86_64).framework` 双份，helper 启动时按当前架构选。
- 签名：CI **ad-hoc**（identifier `me.delton.gdcef.*`），`codesign --verify --deep` 通过；curl 下载不落 quarantine 属性（若经浏览器下载需 `xattr -dr com.apple.quarantine`）。
- `.gdextension` 清单关键条目（与仓库 v1.15.4 tag 逐字一致）：`compatibility_minimum = 4.5`、`macos = "bin/universal-apple-darwin/Godot CEF.framework"`——路径相对 addon 根，**framework 必须在 `addons/godot_cef/bin/universal-apple-darwin/` 下**，helper 由 addon 按 framework 内固定结构自寻：`Godot CEF.framework/Helpers/Godot CEF.app/Contents/Frameworks/Godot CEF Helper.app/Contents/MacOS/Godot CEF Helper`。

### 2.2 B2 正式集成步骤清单（vit-daw-frontend 工程）

1. 取上游 v1.15.4 release zip，只抽 mac 侧：
   ```bash
   unzip godot_cef-v1.15.4.zip "dist/addons/godot_cef/godot_cef.gdextension*" \
     "dist/addons/godot_cef/icons/*" "dist/addons/godot_cef/bin/universal-apple-darwin/*" -d /tmp/cef-extract
   rsync -a /tmp/cef-extract/dist/addons/godot_cef/ <前端工程>/addons/godot_cef/
   ```
2. **生成 `.godot/extension_list.cfg`**（关键坑，见 §4.1）：`godot --headless --import --path <前端工程>`（或开一次编辑器）。没有这步，`CefTexture` 类不存在、脚本解析失败。
3. 运行期无需任何旗标：默认渲染即 Metal 加速路径；`enable_accelerated_osr` 保持默认 true。
4. WebUI 面板：`CefTexture` 节点（TextureRect 子类）+ `url = "res://<vite 构建产物>/index.html"`；输入由节点内建路由处理。Vite 产物需保持源文件不被 Godot 导入改写（上游建议 `vite-plugin-godot-keep-import`，按需评估）。
5. 安全基线（上游 `docs/api/security-baseline.md`）：确认工程设置 `godot_cef/security/allow_insecure_content=false`、`ignore_certificate_errors=false`、`disable_web_security=false`、`default_permission_policy=2`；`custom_command_line_switches` 保持空。
6. 验收锚点（grep godot 日志）：
   - `[CefInit] Startup summary: backend=Metal, accelerated_osr_supported=true, ...`
   - `[CefTexture] Creating browser in accelerated rendering mode`
   - 页面侧 `load_finished status=200`；`console_message`/`ipc_message` 回传。
7. 发布/打包：framework 与 helper app 须随包分发且保持目录结构；zip 分发会丢执行位（见 §4.2，addon 自愈，但导出模板流程需复验）。

## 3. 渲染路径与纹理行为

- **加速（默认）**：CEF GPU 进程 Metal 合成 → 与 Godot Metal RenderingDevice 共享纹理（零拷贝路径）。**CPU 回读被引擎拒绝**（RD 纹理未设 `TEXTURE_USAGE_CAN_COPY_FROM_BIT`，`texture.get_image()` 报错返回空）——需要像素级读回（如截图/分析）的场合必须走 viewport 截图或软件模式，这是上游设计特性不是 bug。
- **软件（`enable_accelerated_osr=false`）**：CPU 渲染 → `ImageTexture`，`get_image()` 可直读（实测 1280x800 完整出图）。WebUI 面板内容更新率低的话软件模式也够用；动画重的面板用默认加速。
- 后备语义：加速不可用时 addon 自动回软件并在启动日志给 `reason`；`enable_accelerated_osr=false` 是显式软件。探测按 Godot 实际后端（`RenderBackend::detect()`），与 CLI 旗标一致。
- DevTools：编辑器二进制跑工程时自动开（`ws://127.0.0.1:9229`）；release 模板构建下关闭（上游按 `is_debug_build() || is_editor_hint()` 决定）。

## 4. 已知坑位清单（实测踩到/验证）

1. **gdextension 不自动注册**：把 addon 拷进工程后直接跑游戏不扫描新 `.gdextension`——必须先 `--headless --import`（或编辑器开一次）生成 `.godot/extension_list.cfg`。症状：`Could not find type "CefTexture"` 脚本解析错误；若脚本里自带超时退出逻辑会失效挂窗。
2. **zip 丢失执行位**：helper 二进制解压后 644。addon 初始化时自愈（日志 `[CefInit] Set executable permissions for:` ×5，`--import` 阶段即发生）——无需人工 chmod，但分发链路若绕过 addon 初始化需自查。
3. **ANGLE 重复类警告**：`objc ... Class ANGLESwapCGLLayer is implemented in both Godot ... and Chromium Embedded Framework`——Godot 自带 ANGLE 与 CEF 内 ANGLE 同名， presently 无功能影响（三轮全过），记录为噪声。
4. **user-data 落点**：CEF 用户数据在 `~/Library/Application Support/Godot/app_userdata/<工程名>/cef-data`（helper `--user-data-dir` 实拍）——多工程并行/清理时要意识到这个共享根下的按工程名分目录。
5. **helper 进程面**：每浏览器实例常驻 5 helper（GPU/network/renderer/plugin 壳/alerts 壳）；Godot 退出时全灭（实测无残留）。进程名含空格（`Godot CEF Helper (GPU)` 等），ps/grep 与杀进程脚本要按全名匹配。
6. **H.264/AAC/MP3 缺席**（上游预编译 CEF 的授权限制）：视频仅 VP8/VP9/AV1/Theora，音频仅 Opus/Vorbis/FLAC/WAV，容器 WebM/Ogg/WAV。**Vit-DAW WebUI 无媒体内容 → 无影响**；若未来要在面板播 H.264/MP4 需自编译 CEF（上游 README "Building CEF with Proprietary Codecs"），当前不做。
7. **下载通道**：release zip（~1GB）直连 GitHub CDN 在本机网络仅 ~5KB/s 且反复超时；走本机代理（127.0.0.1:7890，与 git 同通道）后 ~1.3MB/s。B2 取资产沿用代理路径。
8. **加速纹理不可 CPU 回读**（§3）：凡依赖 `texture.get_image()` 的既有代码（波形快照类）不要指向 CefTexture 的加速实例。

## 5. 与 B2 的边界

- 本手册只覆盖"上游 addon 在 mac Godot 4.6 的可用性与放置契约"。**未测**：导出模板打包后的 framework 分发、Vite 产物在 `res://` 的导入交互（上游提示的 keep-import 插件）、多浏览器实例并存、输入法（上游有 IME 文档）、前端工程真实 UI 加载——这些属 B2 正式集成的验收面。
- 探针工程副本：`~/Documents/vit-b2p-probe/artifacts/probe-project-src/`（不含 666MB bin）；复刻只需把 zip 的 mac 侧抽回 `addons/godot_cef/bin/` 再 `--import`。
