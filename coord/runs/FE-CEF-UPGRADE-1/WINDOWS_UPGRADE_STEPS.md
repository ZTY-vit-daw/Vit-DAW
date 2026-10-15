# FE-CEF-UPGRADE-1 回归⑦：Windows 侧同版本替换步骤留档（只记录不执行）

- 立档：2026-10-15，Mac 执行侧。PC 线后续自行排期执行；本文件只记录资产与步骤。

## 1. 资产

- 上游：dsh0416/godot-cef release **v1.16.2**（2026-09-26）
- 资产名：`godot_cef-v1.16.2.zip`（全平台单包，1,063,681,432 字节）
- 下载 URL：https://github.com/dsh0416/godot-cef/releases/download/v1.16.2/godot_cef-v1.16.2.zip
- Mac 侧实测 sha256（2026-10-15，接管会话）：`e8f7a24486e77156862baf5a4433e4b9c9da540fa26205be3fd21e1a027b4117`
  （PC 侧下载后核对同值；zip 可枚举 1,919 文件）
- **Mac 侧回归结论（2026-10-15，PC 排期前必读）**：七项回归中 ②⑥④ 过，
  **①焦点面硬门 FAIL（⌘V 粘贴不送达）+ ⑤webui 主链 FAIL（chrome.webview.postMessage
  →ipc_message 不送达）**——PC 侧升级前先看本目录 VALIDATION.md 与
  UPSTREAM_ISSUE_DRAFT.md；两缺陷若与平台无关（建议 PC 先以同型探针验证），
  v1.16.2 在 Windows 侧同样不可放行。

## 2. Windows 侧目标文件域

- `addons/godot_cef/bin/x86_64-pc-windows-msvc/`（或 arm64：`aarch64-pc-windows-msvc/`，按 PC 实机）
- 明细以 `addons/godot_cef/godot_cef.gdextension` 的 `windows.x86_64` 依赖节为准：
  gdcef.dll、gdcef_helper.exe、bootstrap.exe、bootstrapc.exe、locales/、
  chrome_100_percent.pak、chrome_200_percent.pak、chrome_elf.dll、d3dcompiler_47.dll、
  dxcompiler.dll、dxil.dll、icudtl.dat、libcef.dll、libEGL.dll、libGLESv2.dll、
  resources.pak、v8_context_snapshot.bin、vk_swiftshader.dll、
  vk_swiftshader_icd.json、vulkan-1.dll
- `godot_cef.gdextension` 清单文件：**v1.16.2 与 v1.15.4 不逐字节同构**（Mac 侧实测）——
  上游版删 Windows libEGL.dll/libGLESv2.dll 依赖条目（zip 内已无此二文件，自洽）并新增
  Linux 展开.pak 条目；我方 v1.15.4 版多出的 macos 导出映射块与 EGL 条目系 B4A 运输快照
  携带（非上游原装证据）。Mac 侧已整文件换用上游 v1.16.2 版，PC 侧同样直接采用 zip 内
  清单（含 Windows 节差异，与其 bin 内容自洽）。

## 3. 替换步骤（建议）

1. 归档：`addons/godot_cef/bin/` 整目录拷贝到仓外备份，记录文件清单+sha256
   （对齐 Mac 侧 BIN_ARCHIVE_V1154.sha256 模式）。
2. 解压 `godot_cef-v1.16.2.zip`，取 `bin/x86_64-pc-windows-msvc/`（及清单文件如变动）。
3. drop-in 覆盖前端仓 `addons/godot_cef/bin/x86_64-pc-windows-msvc/`。
4. 回归清单按 UPSTREAM.md §3 七项执行，Windows 侧重点：
   - ①焦点面四步序列（autodrive 场景跨平台可用；探针页服务需在 PC 起 vite preview）
   - ②加速 OSR（Windows=D3D12 路径）渲染+resize 不闪黑
   - ⑤webui 主链（Ask Vit 打开→加载→ipc→关闭重开）
   - ⑥helper watchdog（gdcef_helper.exe 随父进程退出）
   - ③IME：Windows 输入法（微软拼音）composition 路径
   - ④cef-data 落点（Windows 下 %LOCALAPPDATA% 语义）

## 4. 回滚

- 任一硬门失败且不可绕：恢复步骤 1 归档的 bin/ 整目录（sha256 核对后回拷），
  与 Mac 侧停止条件同语义。

## 5. 平台注意事项（Mac 侧实测教训，Windows 供对照）

- **来源标记/信任评估**：浏览器下载的 zip 解包件在 macOS 26 上带 `com.apple.provenance`
  xattr，dyld 签名校验（fcntl）会挂起或被系统策略拒载；`xattr -d` 被 SIP 拒绝。Mac 侧
  解法=整树 `cp -R` 重物化（bit-identical，sha256 清单复核不变）+ 用户放行一次安全弹窗。
  Windows 侧等价物=MotW/SmartScreen：PowerShell `Unblock-File` 解包件或先解包后核对，
  预期无同型挂起，但首启 SmartScreen 评估需要留意。
- 探针页服务：PC 侧在 `agent/webui/input_probe` 下 `../node_modules/.bin/vite preview
  --host 127.0.0.1 --port 7879 --strictPort`（Mac 侧 SERVICE.md 模式）。
