# PORT-A3：mac CMake 试编译整备（内核首次 darwin 编译）

- 优先级 / 预估 / 依赖：P1 / 1-2 天（PORT_AUDIT 层 A，R1 消解点）/ 无——**当前最高优先，内核一切后续卡的前置**
- 模型分级：L3 / GLM-5.3（首次 mac 编译，未知面大；如需构建系统大改方案先上交讨论）
- 目标：VitApp 内核在 mac（clang / Xcode CLT）下 CMake configure + 编译通过：clang 警告/错误清理、libsodium `GLOB_RECURSE`（VitApp/cmake/libsodium_bundle/CMakeLists.txt:9）mac 实证或改显式源列表、VitApp/Tests 目标同步可编译。审计前置已核：路径全走 `juce::File`、无 WASAPI/ASIO 直接引用（音频面由 JUCE 抽象，真机行为归 A5）——预期阻塞集中在构建系统与编译警告
- 文件域：`VitApp/CMakeLists.txt`、`VitApp/cmake/**`、`VitApp/Source/**`、`VitApp/Tests/**`（Source/Tests 仅限编译通过所需最小改动：头包含、显式类型转换、平台 `#if` 守卫；不改运行语义）；不触碰 `agent/` 与 `scripts/`
- 验收标准：① mac 本机 cmake configure + build 核心目标 exit 0（build 目录与运行态写独立临时目录，AGENTS §10）；② libsodium mac 编译方式有实证结论（GLOB 原样可用 / 改显式源列表，附证据）；③ Tests 目标 mac 编译通过（或记录首个不可达点转 blocked）；④ Windows 零回退：改动一律平台守卫包住，Windows 分支逻辑零变化，由决策侧 PC 会话复编译确认；⑤ 构建命令与退出码入回执
- 停止条件：需改运行语义才能编译过 → blocked；发现 mac 无等价物的 Windows-only 依赖（库/API）→ 列清单上交决策侧
- 领取：
- 回执：
- 验收：
