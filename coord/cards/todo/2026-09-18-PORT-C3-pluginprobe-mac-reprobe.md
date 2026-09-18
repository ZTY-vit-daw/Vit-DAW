# PORT-C3：pluginprobe 原生观察宿主 mac 化 + 24 主体参数面重探针（指纹对照）

- 优先级 / 预估 / 依赖：P1 / 1-2 天 / 依赖 C2（已验收：24 主体认证晋升、白名单草稿、语义索引就位）
- 模型分级：L3 / GLM-5.3（C++ 平台等价改写 + 跨端指纹语义对照；加密原语替换方案如有多解，关键点找参谋决策模型讨论）
- 目标：`PluginProbe/native-host`（Windows-bound：`src/main.cpp` 的 windows.h/bcrypt 面向）在 mac 可构建可执行；用其对 C2 晋升的 24 个 Waves 主体跑**观察级参数面重探针**，产出每主体 ParameterSurface 指纹，与 PC 侧现存指纹逐主体对照——验证 PORT_AUDIT §1.4 的"参数元数据哈希跨平台稳定"预期；顺带确认 R4（`FingerprintPath` 拒绝 bundle 内 symlink）在 mac 实测的表现
- 文件域：`PluginProbe/`（native-host 平台等价实现）；`agent/internal/pluginprobe/`（如需宿主调用适配，列明最小化）；不改 `VitApp/Source`、不改 `scripts/`、不改 `agent/` 其余包
- 验收标准：① native-host mac 构建产物（CMake/Make exit 0）+ 24 主体参数面探针 exit 0；② 指纹对照工件：每主体 mac 指纹 + PC 侧指纹 + 一致/不一致标注，不一致者附参数面差异摘要；③ R4 symlink 遇遇记录（遇到/未遇到，遇到附原始错误）；④ 工件目录含 run ID、各段日志、指纹 JSON（对照前后两台机各一份）
- 停止条件：某主体探针崩溃或参数面解析失败 → 按类型记录证据转 blocked；指纹大面积不一致（跨平台稳定预期失效，如 >1/3 主体不一致）→ 停止并上交对照证据，由决策侧裁定口径；发现需动 `agent/` 大面接线的适配 → 域外上报流程
- 领取：
- 回执：
- 验收：
