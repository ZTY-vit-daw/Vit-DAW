# SHELL-ENTRY-REFRESH-1 预阶段记录（2026-11-18 晚）

- 执行：`lead/entry413`（DeepSeek-V4.1-Flash）；只读两仓、零 git 写、零源码改动、零 GUI/真栈；全部写入 `%TEMP%`。
- 隔离目录：`C:Users	imozAppDataLocalTempshell413-ext-449f0a7d\`（extension/ + godot-cpp/ + logs/）。
- 基线：frontend `bc8daf6`、主仓 `d86f8c55`；Godot 4.6.1.stable.official.14d19694e；scons 4.10.1（VS18 自动探测）。

## 结果

| 项 | 结果 |
|---|---|
| scons 构建（temp 副本，删 .sconsign/旧产物后干净构建） | exit 0，27.5s，4 源文件编译+链接 |
| 产物 | `%TEMP%\shell413-ext-449f0a7d\extension\bin\vit_extension.windows.template_debug.x86_64.dll`，520,192 B |
| SHA256 | `B270362D1E3380D27A709747611FED149A8A0F54B11BA00C1229834510CD2EB7` |
| dumpbin | 导出 `vit_extension_library_init`；依赖 ole32/USER32/GDI32/dcomp/COMCTL32/SHELL32/KERNEL32 |
| 对照旧产品 DLL | 495,616 B，`E23872FAFEC8F5216E20E56860DD958AD9104E9D4F85683BCCC50406602ACEF0`（2026-11-14，无 409 守卫） |
| Godot 4.6.1 headless 加载探针 | exit 0；`EXTENSION_REGISTERED=true / METHODS_OK=true / FEATURE_ENABLED=true`；installed editor=true/debug=true |
| 无 env 对照 | `FEATURE_ENABLED=false` exit 1 → feature 仅由 `GODOT_EDITOR_CUSTOM_FEATURES` 注入，与 godot_manager 注入语义一致 |

## 观察（诚实边界，不阻塞）

- 用 `--headless --import`（编辑器导入模式，非产品 `--path` 项目模式）对**最小探针工程**生成 extension_list 时崩溃（0xC0000005）；无扩展/伪 entry_symbol 不崩，前端仓 2026-11-14 旧 DLL 同样崩溃 → 非 409 守卫回归、非产品路径。根因未查，越出本卡预阶段；最终刷新时改用真实前端工程导入验证（409 曾以完整 staged 工程 import exit 0），若仍崩另开取证卡。
- 预构建 DLL 保留在 temp 供最终刷新复用；最终刷新仍以 410/411 合入后的源码与主管放行为准。
